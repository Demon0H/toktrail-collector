package main

import (
	"encoding/json"
	"os"
	"strings"
	"toktrail-collector/internal/local/outbox"
	"toktrail-collector/internal/scheduler"
)

func printStatus(state string, task scheduler.TaskConfig) error {
	result, err := statusResult(state, task)
	if err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(result)
}
func statusResult(state string, task scheduler.TaskConfig) (map[string]any, error) {
	ledger, err := outbox.New(state).Load()
	if err != nil {
		return nil, err
	}
	pending, synced, missing := 0, 0, 0
	for _, entry := range ledger.Batches {
		if entry.Status == outbox.StatusPending {
			pending++
		}
		if entry.Status == outbox.StatusSynced {
			synced++
		}
		if _, e := os.Stat(entry.File); e != nil {
			missing++
		}
	}
	artifacts := []string{}
	result := map[string]any{"stateDir": state, "pendingBatches": pending, "syncedBatches": synced, "missingArchives": missing, "scheduled": false, "schedulerArtifacts": artifacts, "schedulerState": "not-registered"}
	if task.Platform == "windows" {
		launcherPath := scheduler.WindowsBackgroundPath(task.Binary)
		file, fileErr := os.Stat(launcherPath)
		result["launcherPath"] = launcherPath
		result["launcherPresent"] = fileErr == nil && file.Mode().IsRegular()
		result["background"] = false
		// One authoritative registration query avoids contradictory results if a
		// task is removed between two independent scheduler queries.
		info, e := scheduler.WindowsTaskStatus(task)
		if e != nil {
			result["schedulerError"] = e.Error()
			result["schedulerState"] = "unknown"
			return result, nil
		}
		result["scheduled"] = info.Registered
		result["background"] = info.Registered && info.Background
		if info.Registered {
			identity, e := scheduler.TaskIdentity(state)
			if e != nil {
				return nil, e
			}
			result["schedulerArtifacts"] = []string{"schtasks:\\TokTrailCollector-" + strings.TrimPrefix(identity, "toktrail-collector-")}
			result["schedulerState"] = "registered"
			result["collector"] = info.Collector
			result["launcher"] = info.Launcher
		}
	} else {
		registered, found, e := scheduler.IsInstalled(task)
		result["scheduled"] = registered
		if found != nil {
			result["schedulerArtifacts"] = found
		}
		if e != nil {
			result["schedulerError"] = e.Error()
			result["schedulerState"] = "unknown"
		} else if registered {
			result["schedulerState"] = "registered"
		}
	}
	return result, nil
}
