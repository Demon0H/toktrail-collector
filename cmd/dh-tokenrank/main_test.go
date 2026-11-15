package main

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type transport func(*http.Request) (*http.Response, error)

func (f transport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func fake(t *testing.T, body string, status int) {
	t.Helper()
	old := http.DefaultTransport
	http.DefaultTransport = transport(func(r *http.Request) (*http.Response, error) {
		if r.URL.Host != "tokenrank.demonh.site" || r.URL.Scheme != "https" {
			t.Fatal("destination escaped DH pin")
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Fatal("missing key")
		}
		return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})
	t.Cleanup(func() { http.DefaultTransport = old })
}
func TestPinnedDestinationAndACK(t *testing.T) {
	fake(t, `{"added":1,"duplicates":0}`, 200)
	if _, e := call("POST", "/api/import", "test-key", []byte(`{}`)); e != nil {
		t.Fatal(e)
	}
}
func TestInvalidACKRetainsQueue(t *testing.T) {
	fake(t, `{"added":0,"duplicates":0}`, 200)
	dir := t.TempDir()
	file := filepath.Join(dir, "pending.json")
	if e := os.WriteFile(file, []byte(`{"batchId":"test","records":[{"recordKey":"a"}]}`), 0600); e != nil {
		t.Fatal(e)
	}
	if e := uploadQueue(dir, "test-key"); e == nil {
		t.Fatal("mismatched acknowledgement accepted")
	}
	if _, e := os.Stat(file); e != nil {
		t.Fatal("pending batch lost")
	}
}
func TestSuccessfulQueueACKRemovesFile(t *testing.T) {
	fake(t, `{"added":0,"duplicates":1}`, 200)
	dir := filepath.Join(t.TempDir(), "dh-queue")
	os.MkdirAll(dir, 0700)
	file := filepath.Join(dir, "pending.json")
	os.WriteFile(file, []byte(`{"batchId":"test","records":[{"recordKey":"a"}]}`), 0600)
	if e := uploadQueue(dir, "test-key"); e != nil {
		t.Fatal(e)
	}
	if _, e := os.Stat(file); !os.IsNotExist(e) {
		t.Fatal("acknowledged queue file retained")
	}
}
func TestUnauthorizedRetainsQueue(t *testing.T) {
	fake(t, `{"error":"unauthorized"}`, 401)
	dir := t.TempDir()
	file := filepath.Join(dir, "pending.json")
	os.WriteFile(file, []byte(`{"records":[]}`), 0600)
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
