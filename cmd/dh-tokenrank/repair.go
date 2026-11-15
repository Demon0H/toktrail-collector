package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"time"

	"toktrail-collector/internal/local/outbox"
	"toktrail-collector/internal/source/adapter"
	"toktrail-collector/internal/source/pipeline"
)

// repairHistory is an explicit, verified migration for releases that deleted
// acknowledged archives. It never resets cursors or uploads historical records.
// New snapshots are identified as recovery snapshots; originals are not forged.
func repairHistory(state string, cfg config) error {
	store := outbox.New(state)
	ledger, err := store.Load()
	if err != nil {
		return err
	}
	missing := map[string]bool{}
	for _, entry := range ledger.Batches {
		if _, e := os.Stat(entry.File); errors.Is(e, os.ErrNotExist) {
			if entry.Status != outbox.StatusSynced {
				return fmt.Errorf("batch %s was not acknowledged; cannot reconstruct it from server history", entry.BatchID)
			}
			missing[entry.Source] = true
		} else if e != nil {
			return e
		}
	}
	if len(missing) == 0 {
		fmt.Println("No missing history archives; no state changes.")
		return nil
	}
	history, err := downloadHistory(cfg.Token)
	if err != nil {
		return err
	}
	replacements := map[string]outbox.BatchEntry{}
	payloads := map[string][]byte{}
	for source := range missing {
		expected := 0
		input := ""
		for _, entry := range ledger.Batches {
			if entry.Source == source {
				if entry.Status != outbox.StatusSynced {
					return errors.New("repair requires all batches of the affected source to be acknowledged")
				}
				expected += entry.RecordCount
				input = entry.Input
			}
		}
		var fresh []adapter.Record
		if source == adapter.CodexJSONLSource || source == adapter.DeepSeekHarnessSource {
			seen := map[string]bool{}
			for _, g := range cfg.Grants {
				if g.Source != source || seen[g.Path] {
					continue
				}
				seen[g.Path] = true
				loaded, e := pipeline.LoadSource(adapter.Options{Source: source, Input: g.Path})
				if e != nil {
					return fmt.Errorf("repair %s local verification: %w", source, e)
				}
				fresh = append(fresh, loaded.Records...)
			}
		} else if source == adapter.ClaudeCodeJSONLSource {
			// Claude's interval baseline lives in message-progress.json, untouched by
			// this migration. Historical usage is restored as an explicit server projection.
			for _, r := range history {
				if adapter.StringField(r, "tool") == "Claude Code" {
					copy := adapter.Record{}
					for k, v := range r {
						copy[k] = v
					}
					copy["kind"] = "delta"
					copy["adapterVersion"] = "dh-server-recovery-v1"
					copy["sourceType"] = source
					fresh = append(fresh, copy)
				}
			}
		} else {
			return fmt.Errorf("automatic recovery is not supported for %s; restore its original archive backup", source)
		}
		verified, e := verifiedRecords(fresh, history, expected)
		if e != nil {
			return fmt.Errorf("repair %s: %w", source, e)
		}
		id := fmt.Sprintf("dh-recovery-%s-%d", source, time.Now().UnixNano())
		file := filepath.Join(state, "dh-archive", id+".json")
		payload, e := json.MarshalIndent(adapter.Batch{BatchID: id, Records: verified}, "", "  ")
		if e != nil {
			return e
		}
		replacements[source] = outbox.BatchEntry{BatchID: id, File: file, Source: source, Input: input, RecordCount: len(verified), Status: outbox.StatusSynced, CreatedAt: time.Now().UTC(), SyncedAt: time.Now().UTC(), ReceiptStatus: "verified-server-recovery", AcceptedRecords: len(verified)}
		payloads[source] = payload
	}
	// Every source must verify before any active ledger is changed. Preserve the
	// exact previous ledger and immutable replacement snapshots before atomic Save.
	previous, err := os.ReadFile(store.Path())
	if err != nil {
		return err
	}
	backup := filepath.Join(state, fmt.Sprintf("outbox.before-repair-%d.json", time.Now().UnixNano()))
	if err = os.WriteFile(backup, previous, 0600); err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Join(state, "dh-archive"), 0700); err != nil {
		return err
	}
	for source, entry := range replacements {
		if err = os.WriteFile(entry.File, payloads[source], 0600); err != nil {
			return err
		}
	}
	repaired := outbox.Outbox{}
	for _, entry := range ledger.Batches {
		if !missing[entry.Source] {
			repaired.Batches = append(repaired.Batches, entry)
		}
	}
	for _, entry := range replacements {
		repaired.Batches = append(repaired.Batches, entry)
	}
	if err = store.Save(repaired); err != nil {
		return err
	}
	for source, entry := range replacements {
		fmt.Printf("Recovered %s: %d server-confirmed records, no uploads.\n", source, entry.RecordCount)
	}
	fmt.Println("Historical ledger backup:", backup)
	return nil
}

func downloadHistory(token string) (map[string]adapter.Record, error) {
	records := map[string]adapter.Record{}
	after := ""
	for {
		raw, e := call(http.MethodGet, "/api/history?after="+url.QueryEscape(after), token, nil)
		if e != nil {
			return nil, e
		}
		var page struct {
			Records []adapter.Record `json:"records"`
			Next    string           `json:"next"`
		}
		if e = json.Unmarshal(raw, &page); e != nil {
			return nil, e
		}
		for _, r := range page.Records {
			key := adapter.StringField(r, "recordKey")
			if key == "" {
				return nil, errors.New("server history record is missing its identity")
			}
			records[key] = r
		}
		if page.Next == "" {
			return records, nil
		}
		if page.Next <= after {
			return nil, errors.New("server history pagination did not advance")
		}
		after = page.Next
	}
}

func verifiedRecords(fresh []adapter.Record, history map[string]adapter.Record, expected int) ([]adapter.Record, error) {
	found := map[string]adapter.Record{}
	for _, r := range fresh {
		key := adapter.StringField(r, "recordKey")
		remote, ok := history[key]
		if !ok {
			continue
		}
		if !sameWireUsage(r, remote) {
			return nil, fmt.Errorf("record %s differs from acknowledged server usage; refusing to adopt changed history", key)
		}
		if old, ok := found[key]; ok {
			a, _ := json.Marshal(old)
			b, _ := json.Marshal(r)
			if !bytes.Equal(a, b) {
				return nil, errors.New("conflicting local records during recovery")
			}
		}
		found[key] = r
	}
	if len(found) != expected {
		return nil, fmt.Errorf("verified %d records, ledger requires exactly %d; restore archive backup instead", len(found), expected)
	}
	keys := make([]string, 0, len(found))
	for k := range found {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]adapter.Record, 0, len(keys))
	for _, k := range keys {
		out = append(out, found[k])
	}
	return out, nil
}
func sameWireUsage(a, b adapter.Record) bool {
	at, e := time.Parse(time.RFC3339Nano, adapter.StringField(a, "occurredAt"))
	bt, e2 := time.Parse(time.RFC3339Nano, adapter.StringField(b, "occurredAt"))
	if e != nil || e2 != nil || !at.Equal(bt) {
		return false
	}
	for _, k := range []string{"tool", "input", "output", "cacheRead", "cacheWrite"} {
		av, _ := json.Marshal(a[k])
		bv, _ := json.Marshal(b[k])
		if !bytes.Equal(av, bv) {
			return false
		}
	}
	am, bm := adapter.StringField(a, "model"), adapter.StringField(b, "model")
	if am == "" {
		am = "unknown"
	}
	if bm == "" {
		bm = "unknown"
	}
	return am == bm
}
