package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"
	"toktrail-collector/internal/scheduler"
)

func TestWindowsPackagedScheduler(t *testing.T) {
	if runtime.GOOS != "windows" || os.Getenv("DH_WINDOWS_RELEASE_E2E") != "1" {
		t.Skip("opt-in native Windows packaging gate")
	}
	dir := t.TempDir()
	binary := filepath.Join(dir, "dh-tokenrank.exe")
	for _, build := range []struct {
		out, pkg string
		flags    []string
	}{{binary, ".", nil}, {filepath.Join(dir, scheduler.WindowsBackgroundBinary), "../toktrail-background", []string{"-ldflags=-H=windowsgui"}}} {
		args := append([]string{"build", "-mod=vendor", "-o", build.out}, build.flags...)
		args = append(args, build.pkg)
		if out, e := exec.Command("go", args...).CombinedOutput(); e != nil {
			t.Fatalf("build packaged binary: %v %s", e, out)
		}
	}
	state := filepath.Join(dir, "state")
	os.MkdirAll(state, 0700)
	home, e := os.UserHomeDir()
	if e != nil {
		t.Fatal(e)
	}
	cfg := scheduler.TaskConfig{Platform: "windows", StateDir: state, Binary: binary, HomeDir: home, Interval: 30 * time.Minute}
	if _, e = scheduler.Install(scheduler.InstallOptions{Config: cfg}); e != nil {
		t.Fatal(e)
	}
	defer func() {
		if _, e := scheduler.Uninstall(scheduler.UninstallOptions{Config: cfg}); e != nil {
			t.Error(e)
		}
	}()
	info, e := scheduler.WindowsTaskStatus(cfg)
	if e != nil {
		t.Fatal(e)
	}
	if !info.Registered || !info.Background || info.Collector != binary || info.Launcher != scheduler.WindowsBackgroundPath(binary) {
		t.Fatalf("wrong packaged task action: %+v", info)
	}
}
