// DH TokenRank uses the upstream privacy-filtering adapters with an isolated
// local state directory and the DH participation-key API.
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"toktrail-collector/internal/local/exportcommit"
	"toktrail-collector/internal/local/outbox"
	"toktrail-collector/internal/local/runlock"
	"toktrail-collector/internal/scheduler"
	"toktrail-collector/internal/source/adapter"
	"toktrail-collector/internal/source/pipeline"
	"toktrail-collector/internal/source/registry"
)

var version = "dev"

const serverURL = "https://tokenrank.demonh.site"

type grant struct {
	Source string `json:"source"`
	Path   string `json:"path"`
}
type config struct {
	Token    string  `json:"token"`
	Grants   []grant `json:"grants"`
	All      bool    `json:"all"`
	Explicit []grant `json:"explicit,omitempty"`
}
type grantsFlag []string

func (v *grantsFlag) String() string     { return strings.Join(*v, ",") }
func (v *grantsFlag) Set(s string) error { *v = append(*v, s); return nil }
func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "DH燃源榜:", err)
		os.Exit(1)
	}
}
func run(args []string) error {
	if len(args) == 0 || args[0] == "help" {
		fmt.Println("DH燃源榜", version, "\nsetup --ticket ONE_TIME_CODE --all --install (or set DH_TOKEN and use --grant)\ndiscover | status | repair | sync | periodic | uninstall | version\nAll commands accept --state-dir. Destination: "+serverURL)
		return nil
	}
	if args[0] == "version" {
		fmt.Println("DH燃源榜", version)
		return nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	if args[0] == "discover" {
		return json.NewEncoder(os.Stdout).Encode(discover(home))
	}
	fs := flag.NewFlagSet(args[0], flag.ContinueOnError)
	state := fs.String("state-dir", filepath.Join(home, ".dh-tokenrank"), "isolated DH state directory")
	install := fs.Bool("install", false, "install a per-user 30-minute task after setup")
	all := fs.Bool("all", false, "authorize all supported local tools in the current user home, including tools added later")
	reconnect := fs.Bool("reconnect", false, "rebind an existing state to the same account using a fresh ticket")
	ticket := fs.String("ticket", "", "single-use website enrollment code")
	var gs grantsFlag
	fs.Var(&gs, "grant", "explicit read authorization, source=absolute-path; repeatable")
	if err = fs.Parse(args[1:]); err != nil {
		return err
	}
	if *reconnect && *ticket == "" {
		return errors.New("--reconnect requires a fresh one-time --ticket")
	}
	if fs.NArg() != 0 {
		return errors.New("unexpected arguments")
	}
	*state, err = filepath.Abs(*state)
	if err != nil {
		return err
	}
	binary, err := os.Executable()
	if err != nil {
		return err
	}
	task := scheduler.TaskConfig{Platform: runtime.GOOS, StateDir: *state, Binary: binary, HomeDir: home, Interval: 30 * time.Minute}
	if args[0] == "status" {
		return printStatus(*state, task)
	}
	if args[0] == "uninstall" {
		_, err = scheduler.Uninstall(scheduler.UninstallOptions{Config: task})
		return err
	}
	if args[0] != "setup" && args[0] != "sync" && args[0] != "periodic" && args[0] != "repair" {
		return errors.New("unknown command; use help")
	}
	if err = os.MkdirAll(*state, 0700); err != nil {
		return err
	}
	lock, err := runlock.Acquire(*state)
	if err != nil {
		return err
	}
	defer lock.Close()
	cfgPath := filepath.Join(*state, "dh-config.json")
	if args[0] == "setup" {
		token := strings.TrimSpace(os.Getenv("DH_TOKEN"))
		if token == "" && *ticket == "" {
			if previous, e := os.ReadFile(cfgPath); e == nil {
				var saved config
				if json.Unmarshal(previous, &saved) == nil {
					token = saved.Token
				}
			}
		}
		if *ticket != "" {
			if _, e := os.Stat(cfgPath); e == nil && !*reconnect {
				return errors.New("already configured; use setup --all --install to reuse it, or --reconnect with a fresh ticket to rebind the same device")
			}
			enrollment := map[string]string{"ticket": *ticket, "platform": runtime.GOOS + "/" + runtime.GOARCH}
			if *reconnect {
				previous, e := os.ReadFile(cfgPath)
				if e != nil {
					return errors.New("reconnect requires an existing device configuration")
				}
				var old config
				if json.Unmarshal(previous, &old) != nil {
					return errors.New("existing device configuration is invalid")
				}
				enrollment["previousToken"] = old.Token
			}
			payload, _ := json.Marshal(enrollment)
			response, e := call(http.MethodPost, "/api/enroll", "", payload)
			if e != nil {
				return e
			}
			var receipt struct {
				Token string `json:"token"`
			}
			if json.Unmarshal(response, &receipt) != nil {
				return errors.New("invalid enrollment response")
			}
			token = receipt.Token
		}
		if len(token) != 64 {
			return errors.New("set DH_TOKEN to your participation key from the DH website")
		}
		if len(gs) == 0 && !*all {
			return errors.New("at least one explicit --grant is required; no paths are read by default")
		}
		if previous, e := os.ReadFile(cfgPath); e == nil {
			var old config
			if json.Unmarshal(previous, &old) != nil {
				return errors.New("existing config is invalid; preserve state and investigate")
			}
			if old.Token != token && !*reconnect {
				return errors.New("participation key differs from existing identity; use a separate --state-dir")
			}
		}
		cfg := config{Token: token, All: *all}
		for _, v := range gs {
			s, p, ok := strings.Cut(v, "=")
			if !ok || !registry.Production().IsKnownSource(s) || !filepath.IsAbs(p) {
				return errors.New("grant must be a supported source=absolute-path")
			}
			if _, err = os.Stat(p); err != nil {
				return errors.New("granted path does not exist")
			}
			cfg.Grants = append(cfg.Grants, grant{s, p})
		}
		if *all {
			cfg.Explicit = append([]grant(nil), cfg.Grants...)
			cfg.Grants = mergeGrants(cfg.Grants, discover(home))
		}
		if _, err = call(http.MethodGet, "/api/me", token, nil); err != nil {
			return err
		}
		data, _ := json.MarshalIndent(cfg, "", "  ")
		if err = os.WriteFile(cfgPath, data, 0600); err != nil {
			return err
		}
		if err = os.Chmod(cfgPath, 0600); err != nil {
			return err
		}
		if *install {
			if _, err = scheduler.Install(scheduler.InstallOptions{Config: task}); err != nil {
				return err
			}
		}
		fmt.Println("Configured DH-only destination; only explicit grants will be read. Run sync to upload. Public visibility is controlled separately on the website.")
		return nil
	}
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		return errors.New("run setup first")
	}
	var cfg config
	if err = json.Unmarshal(data, &cfg); err != nil {
		return errors.New("invalid local configuration")
	}
	if cfg.All {
		cfg.Grants = mergeGrants(cfg.Explicit, discover(home))
	}
	if len(cfg.Grants) == 0 {
		return errors.New("no grants; nothing read or uploaded")
	}
	if _, err = call(http.MethodGet, "/api/me", cfg.Token, nil); err != nil {
		return err
	}
	if args[0] == "repair" {
		return repairHistory(*state, cfg)
	}
	if _, err = exportcommit.Recover(*state); err != nil {
		return errors.New("local export recovery failed; data retained")
	}
	queue := filepath.Join(*state, "dh-queue")
	if err = os.MkdirAll(queue, 0700); err != nil {
		return err
	}
	// Upload previously committed batches first. A lost ACK is safe: the server
	// deduplicates recordKey per participant. Never discard an unacknowledged file.
	if err = uploadQueue(queue, cfg.Token); err != nil {
		return err
	}
	var collectionErrors []error
	var failedSources []string
	for _, g := range cfg.Grants {
		id := fmt.Sprintf("dh-%d", time.Now().UnixNano())
		out := filepath.Join(queue, id+".json")
		prepared, e := pipeline.PrepareExport(adapter.Options{Source: g.Source, Input: g.Path, Output: out, BatchID: id, StateDir: *state})
		if e != nil {
			failedSources = append(failedSources, g.Source)
			collectionErrors = append(collectionErrors, fmt.Errorf("%s collection failed: %w", g.Source, e))
			continue
		}
		chunks, e := pipeline.SplitPreparedExport(prepared)
		if e != nil {
			return e
		}
		for _, chunk := range chunks {
			if chunk.Records == 0 {
				continue
			}
			if _, e = exportcommit.CommitExport(exportcommit.ExportOptions{StateDir: *state, Prepared: chunk, Source: g.Source, Input: g.Path}); e != nil {
				return errors.New("local export failed; queued data retained")
			}
		}
		if err = uploadQueue(queue, cfg.Token); err != nil {
			return err
		}
	}
	if len(collectionErrors) > 0 {
		report, _ := json.Marshal(map[string]any{"ok": false, "failedSources": failedSources})
		_, _ = call(http.MethodPost, "/api/device-status", cfg.Token, report)
		return errors.Join(collectionErrors...)
	}
	report, _ := json.Marshal(map[string]any{"ok": true})
	if _, err = call(http.MethodPost, "/api/device-status", cfg.Token, report); err != nil {
		return err
	}
	fmt.Println("DH sync complete. Destination:", serverURL)
	return nil
}
func uploadQueue(queue, token string) error {
	store := outbox.New(filepath.Dir(queue))
	ledger, err := store.Load()
	if err != nil {
		return err
	}
	for _, entry := range ledger.Batches {
		if entry.Status != outbox.StatusPending {
			continue
		}
		if filepath.Clean(filepath.Dir(entry.File)) != filepath.Clean(queue) {
			return errors.New("pending batch is outside DH queue; refusing to upload")
		}
		b, e := os.ReadFile(entry.File)
		if e != nil {
			return fmt.Errorf("read pending batch %s: %w", entry.BatchID, e)
		}
		var batch adapter.Batch
		if json.Unmarshal(b, &batch) != nil || batch.BatchID != entry.BatchID || len(batch.Records) != entry.RecordCount {
			return errors.New("pending batch identity/count mismatch")
		}
		receipt, e := call(http.MethodPost, "/api/import", token, b)
		if e != nil {
			return e
		}
		var ack struct {
			Added       int `json:"added"`
			Duplicates  int `json:"duplicates"`
			Quarantined int `json:"quarantined"`
		}
		if json.Unmarshal(receipt, &ack) != nil || ack.Added < 0 || ack.Duplicates < 0 || ack.Added+ack.Duplicates != len(batch.Records) || ack.Quarantined < 0 || ack.Quarantined > ack.Added {
			return errors.New("DH acknowledgement count mismatch; pending batch retained")
		}
		if e = store.MarkSynced(outbox.Receipt{BatchID: batch.BatchID, Status: "accepted", AcceptedRecords: len(batch.Records), ReceivedAt: time.Now()}); e != nil {
			return e
		}
		if ack.Quarantined > 0 {
			fmt.Fprintf(os.Stderr, "Batch %s: %d anomalous records are held by the server and excluded from ranking.\n", batch.BatchID, ack.Quarantined)
		}
		// Immutable archives remain at the ledger path. Baseline validation must be
		// able to re-read them; status=synced prevents future uploads.
	}
	return nil
}

func call(method, path, token string, b []byte) ([]byte, error) {
	u, err := url.Parse(serverURL)
	if err != nil || u.Scheme != "https" {
		return nil, errors.New("invalid pinned DH destination")
	}
	req, err := http.NewRequest(method, serverURL+path, bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 45 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	res, err := client.Do(req)
	if err != nil {
		return nil, errors.New("DH server unreachable or TLS verification failed; pending batches retained")
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 1024*1024))
	if err != nil {
		return nil, errors.New("failed to read DH acknowledgement")
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		var problem struct {
			Error string `json:"error"`
		}
		message := "request rejected"
		if json.Unmarshal(body, &problem) == nil && problem.Error != "" {
			message = problem.Error
			if len(message) > 400 {
				message = message[:400]
			}
			if token != "" {
				message = strings.ReplaceAll(message, token, "[redacted]")
			}
		}
		return nil, fmt.Errorf("DH server returned HTTP %d: %s; pending batch retained", res.StatusCode, message)
	}
	var ack map[string]json.RawMessage
	if json.Unmarshal(body, &ack) != nil {
		return nil, errors.New("invalid DH JSON acknowledgement")
	}
	if path == "/api/import" {
		if ack["added"] == nil || ack["duplicates"] == nil {
			return nil, errors.New("missing DH import acknowledgement")
		}
	}
	return body, nil
}

// discover only stats registered locations; it does not parse conversation files.
func discover(home string) []grant {
	out := []grant{}
	for _, spec := range registry.SupportedTools() {
		if spec.IsAccountNetwork() {
			continue
		}
		for _, p := range spec.DefaultInputCandidates(home) {
			if _, err := os.Stat(p); err == nil {
				out = append(out, grant{spec.Source, p})
			}
		}
	}
	return mergeGrants(nil, out)
}
func mergeGrants(a, b []grant) []grant {
	seen := map[string]bool{}
	out := []grant{}
	for _, list := range [][]grant{a, b} {
		for _, g := range list {
			key := g.Source + "=" + filepath.Clean(g.Path)
			if !seen[key] {
				seen[key] = true
				out = append(out, g)
			}
		}
	}
	return out
}
