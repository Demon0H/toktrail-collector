package main

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"toktrail-collector/internal/scheduler"
)

func TestCommandHelpDoesNotExecuteAndUnknownArgumentsFail(t *testing.T) {
	for command := range commandUsage {
		if err := run([]string{command, "--help"}); err != nil {
			t.Fatalf("%s help: %v", command, err)
		}
	}
	for _, args := range [][]string{{"nosuchcmd"}, {"privacy", "--nonsense"}, {"version", "extra"}, {"discover", "--nonsense"}, {"status", "--install"}, {"sync", "--all"}} {
		if err := run(args); err == nil {
			t.Fatalf("invalid command/flags accepted: %q", args)
		}
	}
}
func TestTicketEnvironmentAndFileWithoutCommandLineSecret(t *testing.T) {
	t.Setenv("DH_TICKET", "synthetic-code")
	_, opt, _, err := parseCLI([]string{"setup", "--all"})
	if err != nil || opt.Ticket != "synthetic-code" {
		t.Fatal("DH_TICKET not read", err)
	}
	t.Setenv("DH_TICKET", "")
	path := filepath.Join(t.TempDir(), "ticket.txt")
	os.WriteFile(path, []byte("\ufeffsynthetic-file-code\n"), 0600)
	_, opt, _, err = parseCLI([]string{"setup", "--all", "--ticket-file", path})
	if err != nil || opt.Ticket != "synthetic-file-code" {
		t.Fatal("ticket file not read", err)
	}
	t.Setenv("DH_TICKET", "another-code")
	if _, _, _, err = parseCLI([]string{"setup", "--all", "--ticket-file", path}); err == nil {
		t.Fatal("ambiguous ticket sources accepted")
	}
}
func TestLocalPreflightFailsBeforeRedeemingTicket(t *testing.T) {
	t.Setenv("DH_TICKET", "synthetic-code")
	calls := 0
	old := http.DefaultTransport
	http.DefaultTransport = transport(func(*http.Request) (*http.Response, error) {
		calls++
		t.Fatal("one-time code reached network before local prerequisites passed")
		return nil, nil
	})
	t.Cleanup(func() { http.DefaultTransport = old })
	state := t.TempDir()
	err := run([]string{"setup", "--state-dir", state, "--grant", "codex-jsonl=" + filepath.Join(state, "missing-input")})
	if err == nil {
		t.Fatal("nonexistent grant accepted")
	}
	if calls != 0 {
		t.Fatal("ticket consumed on preflight failure")
	}
	if _, err = os.Stat(filepath.Join(state, "dh-config.json")); !os.IsNotExist(err) {
		t.Fatal("failed preflight wrote configuration")
	}
}
func TestMissingWindowsLauncherRejectedInLocalPreflight(t *testing.T) {
	dir := t.TempDir()
	binary := filepath.Join(dir, "dh-tokenrank.exe")
	os.WriteFile(binary, []byte("synthetic"), 0600)
	os.WriteFile(filepath.Join(dir, "toktrail.exe"), []byte("synthetic"), 0600)
	state := filepath.Join(dir, "state")
	os.MkdirAll(state, 0700)
	task := scheduler.TaskConfig{Platform: "windows", StateDir: state, Binary: binary, HomeDir: dir, Interval: 30 * time.Minute}
	_, err := prepareSetup(dir, state, task, true, true, nil)
	if err == nil || !strings.Contains(err.Error(), "toktrail-background.exe") || !strings.Contains(err.Error(), "has not been redeemed") {
		t.Fatalf("missing local dependency did not fail preflight: %v", err)
	}
}
func TestDiagnosisDoesNotExposeStoredCredential(t *testing.T) {
	state := t.TempDir()
	secret := strings.Repeat("private-key-", 8)
	raw, _ := json.Marshal(config{Token: secret})
	os.WriteFile(filepath.Join(state, "dh-config.json"), raw, 0600)
	os.MkdirAll(filepath.Join(state, "logs"), 0700)
	os.WriteFile(filepath.Join(state, "logs", "periodic.err.log"), []byte("DH燃元榜: reflected "+secret+"\n"), 0600)
	result, err := diagnosis(state, scheduler.TaskConfig{Platform: "linux", StateDir: state, Binary: filepath.Join(state, "dh-tokenrank"), HomeDir: state, Interval: 30 * time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(result)
	if strings.Contains(string(encoded), secret) {
		t.Fatal("diagnosis exposed credential")
	}
	if result["stateWritable"] != true || result["configAvailable"] != true {
		t.Fatalf("diagnosis omitted readiness: %#v", result)
	}
}

func TestEnrollmentCredentialSurvivesFollowupNetworkFailure(t *testing.T) {
	t.Setenv("DH_TICKET", "synthetic-code")
	t.Setenv("DH_TOKEN", "")
	secret := strings.Repeat("a", 64)
	state := t.TempDir()
	old := http.DefaultTransport
	http.DefaultTransport = transport(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path == "/api/enroll" {
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"token":"` + secret + `"}`)), Header: make(http.Header)}, nil
		}
		return nil, errors.New("temporary network failure")
	})
	t.Cleanup(func() { http.DefaultTransport = old })
	if err := run([]string{"setup", "--all", "--state-dir", state}); err == nil {
		t.Fatal("network failure not reported")
	}
	raw, err := os.ReadFile(filepath.Join(state, "dh-config.json"))
	if err != nil {
		t.Fatal("redeemed credential lost", err)
	}
	var cfg config
	if json.Unmarshal(raw, &cfg) != nil || cfg.Token != secret {
		t.Fatal("credential not safely persisted")
	}
}
