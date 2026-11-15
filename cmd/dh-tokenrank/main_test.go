package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"toktrail-collector/internal/local/exportcommit"
	"toktrail-collector/internal/local/knowncache"
	"toktrail-collector/internal/local/outbox"
	"toktrail-collector/internal/source/adapter"
	"toktrail-collector/internal/source/pipeline"
)

type transport func(*http.Request) (*http.Response, error)

func (f transport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func fake(t *testing.T, body string, status int) *int {
	count := 0
	t.Helper()
	old := http.DefaultTransport
	http.DefaultTransport = transport(func(r *http.Request) (*http.Response, error) {
		count++
		if r.URL.Host != "tokenrank.demonh.site" || r.URL.Scheme != "https" {
			t.Fatal("destination escaped DH pin")
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Fatal("missing key")
		}
		return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})
	t.Cleanup(func() { http.DefaultTransport = old })
	return &count
}
func TestPinnedDestinationAndACK(t *testing.T) {
	fake(t, `{"added":1,"duplicates":0}`, 200)
	if _, e := call("POST", "/api/import", "test-key", []byte(`{}`)); e != nil {
		t.Fatal(e)
	}
}
func TestInvalidACKRetainsQueue(t *testing.T) {
	fake(t, `{"added":0,"duplicates":0}`, 200)
	dir := filepath.Join(t.TempDir(), "dh-queue")
	os.MkdirAll(dir, 0700)
	file := filepath.Join(dir, "pending.json")
	if e := os.WriteFile(file, []byte(`{"batchId":"test","records":[{"recordKey":"a"}]}`), 0600); e != nil {
		t.Fatal(e)
	}
	registerPending(t, dir, file)
	if e := uploadQueue(dir, "test-key"); e == nil {
		t.Fatal("mismatched acknowledgement accepted")
	}
	if _, e := os.Stat(file); e != nil {
		t.Fatal("pending batch lost")
	}
}
func TestSuccessfulQueueACKRetainsImmutableArchive(t *testing.T) {
	calls := fake(t, `{"added":0,"duplicates":1}`, 200)
	dir := filepath.Join(t.TempDir(), "dh-queue")
	os.MkdirAll(dir, 0700)
	file := filepath.Join(dir, "pending.json")
	os.WriteFile(file, []byte(`{"batchId":"test","records":[{"recordKey":"a"}]}`), 0600)
	registerPending(t, dir, file)
	if e := uploadQueue(dir, "test-key"); e != nil {
		t.Fatal(e)
	}
	if _, e := os.Stat(file); e != nil {
		t.Fatal("acknowledged immutable archive was deleted")
	}
	if e := uploadQueue(dir, "test-key"); e != nil {
		t.Fatal(e)
	}
	if *calls != 1 {
		t.Fatal("synced archive uploaded again")
	}
	if _, e := knowncache.LoadBaseline(context.Background(), filepath.Dir(dir), "codex-jsonl", "Codex"); e != nil {
		t.Fatal(e)
	}
}
func TestUnauthorizedRetainsQueue(t *testing.T) {
	fake(t, `{"error":"unauthorized"}`, 401)
	dir := filepath.Join(t.TempDir(), "dh-queue")
	os.MkdirAll(dir, 0700)
	file := filepath.Join(dir, "pending.json")
	os.WriteFile(file, []byte(`{"records":[]}`), 0600)
	registerPending(t, dir, file)
	if e := uploadQueue(dir, "test-key"); e == nil {
		t.Fatal("unauthorized accepted")
	}
	if _, e := os.Stat(file); e != nil {
		t.Fatal("queue lost")
	}
}
func TestNoGrantsDoesNotConfigure(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DH_TOKEN", strings.Repeat("a", 64))
	if e := run([]string{"setup", "--state-dir", dir}); e == nil {
		t.Fatal("setup without grants accepted")
	}
	if _, e := os.Stat(filepath.Join(dir, "dh-config.json")); !os.IsNotExist(e) {
		t.Fatal("unexpected config")
	}
}

func TestDiscoverAllRegisteredLocalTools(t *testing.T) {
	home := t.TempDir()
	for _, p := range []string{".codex/sessions", ".claude/projects", ".kimi/sessions", ".local/share/opencode"} {
		if e := os.MkdirAll(filepath.Join(home, filepath.FromSlash(p)), 0700); e != nil {
			t.Fatal(e)
		}
	}
	gs := discover(home)
	found := map[string]bool{}
	for _, g := range gs {
		found[g.Source] = true
	}
	for _, s := range []string{"codex-jsonl", "claude-code-jsonl", "kimi-cli-jsonl", "opencode-sqlite"} {
		if !found[s] {
			t.Fatalf("missed local tool %s", s)
		}
	}
	if found["cursor-dashboard-api"] {
		t.Fatal("network account source granted without login")
	}
}

func registerPending(t *testing.T, queue, file string) {
	t.Helper()
	raw, e := os.ReadFile(file)
	if e != nil {
		t.Fatal(e)
	}
	var b adapter.Batch
	if e = json.Unmarshal(raw, &b); e != nil {
		t.Fatal(e)
	}
	if b.BatchID == "" {
		b.BatchID = "test"
		raw, _ = json.Marshal(b)
		os.WriteFile(file, raw, 0600)
	}
	if _, e = outbox.New(filepath.Dir(queue)).AddPending(outbox.AddBatchOptions{BatchID: b.BatchID, File: file, Source: "codex-jsonl", RecordCount: len(b.Records)}); e != nil {
		t.Fatal(e)
	}
}
func TestRepeatedExportAfterUploadKeepsHistoryAndOnlyExportsNewUsage(t *testing.T) {
	for _, source := range []string{adapter.CodexJSONLSource, adapter.DeepSeekHarnessSource} {
		t.Run(source, func(t *testing.T) {
			calls := fake(t, `{"added":1,"duplicates":0}`, 200)
			state := t.TempDir()
			queue := filepath.Join(state, "dh-queue")
			os.MkdirAll(queue, 0700)
			row := adapter.Record{"recordKey": source + ":first", "kind": "delta", "occurredAt": "2026-11-15T01:00:00Z", "tool": "test", "model": "test-model", "adapterVersion": "1", "sourceType": source, "input": "100", "output": "10", "cacheRead": "0", "cacheWrite": "0"}
			prepare := func(id string, rows []adapter.Record) pipeline.PreparedExport {
				p, e := pipeline.PrepareFromLoaded(adapter.Options{Source: source, Input: state, StateDir: state, Output: filepath.Join(queue, id+".json"), BatchID: id}, adapter.LoadResult{Records: rows})
				if e != nil {
					t.Fatal(e)
				}
				return p
			}
			commit := func(p pipeline.PreparedExport) {
				if _, e := exportcommit.CommitExport(exportcommit.ExportOptions{StateDir: state, Prepared: p, Source: source, Input: state}); e != nil {
					t.Fatal(e)
				}
			}
			first := prepare("first", []adapter.Record{row})
			commit(first)
			if e := uploadQueue(queue, "test-key"); e != nil {
				t.Fatal(e)
			}
			second := prepare("second", []adapter.Record{row})
			if second.Records != 0 {
				t.Fatal("known history exported again")
			}
			next := adapter.Record{}
			for k, v := range row {
				next[k] = v
			}
			next["recordKey"] = source + ":new"
			next["occurredAt"] = "2026-11-15T01:01:00Z"
			third := prepare("third", []adapter.Record{row, next})
			if third.Records != 1 {
				t.Fatal("new usage not isolated")
			}
			commit(third)
			if e := uploadQueue(queue, "test-key"); e != nil {
				t.Fatal(e)
			}
			if *calls != 2 {
				t.Fatal("historical batch was reuploaded")
			}
		})
	}
}
