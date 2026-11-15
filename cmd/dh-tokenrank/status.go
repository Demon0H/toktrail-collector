package main

import (
	"encoding/json"
	"os"
	"toktrail-collector/internal/local/outbox"
	"toktrail-collector/internal/scheduler"
)

func printStatus(state string, task scheduler.TaskConfig) error {
	ledger, err := outbox.New(state).Load()
	if err != nil {
		return err
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
	registered, artifacts, taskErr := scheduler.IsInstalled(task)
	result := map[string]any{"stateDir": state, "pendingBatches": pending, "syncedBatches": synced, "missingArchives": missing, "scheduled": registered, "schedulerArtifacts": artifacts}
	if task.Platform == "windows" && registered {
		info, e := scheduler.WindowsTaskStatus(task)
		if e != nil {
			return e
		}
		result["background"] = info.Background
		result["collector"] = info.Collector
		result["launcher"] = info.Launcher
	}
	if taskErr != nil {
		result["schedulerError"] = taskErr.Error()
	}
	return json.NewEncoder(os.Stdout).Encode(result)
}
