package main

import (
	"encoding/json"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"toktrail-collector/internal/scheduler"
)

func printDiagnosis(state string, task scheduler.TaskConfig, compact bool) error {
	result, err := diagnosis(state, task)
	if err != nil {
		return err
	}
	encoder := json.NewEncoder(os.Stdout)
	if !compact {
		encoder.SetIndent("", "  ")
	}
	return encoder.Encode(result)
}
func diagnosis(state string, task scheduler.TaskConfig) (map[string]any, error) {
	status, err := statusResult(state, task)
	if err != nil {
		return nil, err
	}
	result := map[string]any{"version": version, "platform": runtime.GOOS + "/" + runtime.GOARCH, "server": serverURL, "status": status, "configAvailable": false, "stateDirectoryExists": false, "stateWritable": nil, "sources": []map[string]any{}, "lastError": nil}
	if info, e := os.Stat(state); e == nil && info.IsDir() {
		result["stateDirectoryExists"] = true
		if e = checkStateWritable(state); e != nil {
			result["stateWritable"] = false
			result["stateWriteError"] = e.Error()
		} else {
			result["stateWritable"] = true
		}
	}
	var cfg config
	if raw, e := os.ReadFile(filepath.Join(state, "dh-config.json")); e == nil {
		if e = json.Unmarshal(raw, &cfg); e != nil {
			result["configError"] = "invalid local configuration"
		} else {
			result["configAvailable"] = true
		}
	}
	grants := cfg.Grants
	if cfg.All {
		grants = mergeGrants(cfg.Explicit, discover(task.HomeDir))
	}
	sources := []map[string]any{}
	for _, g := range grants {
		count := 0
		var scanErr error
		scanErr = filepath.WalkDir(g.Path, func(_ string, entry fs.DirEntry, e error) error {
			if e != nil {
				return e
			}
			if !entry.IsDir() {
				count++
			}
			return nil
		})
		row := map[string]any{"source": g.Source, "path": g.Path, "fileCount": count}
		if scanErr != nil {
			row["error"] = redact(scanErr.Error(), []string{cfg.Token})
		}
		sources = append(sources, row)
	}
	result["sources"] = sources
	if file, e := os.Open(filepath.Join(state, "logs", "periodic.err.log")); e == nil {
		defer file.Close()
		if info, e := file.Stat(); e == nil && info.Size() > 65536 {
			_, _ = file.Seek(-65536, io.SeekEnd)
		}
		raw, _ := io.ReadAll(io.LimitReader(file, 65536))
		lines := strings.Split(string(raw), "\n")
		for i := len(lines) - 1; i >= 0; i-- {
			if strings.HasPrefix(lines[i], "DH燃源榜:") {
				result["lastError"] = redact(strings.TrimSpace(lines[i]), []string{cfg.Token, os.Getenv("DH_TOKEN"), os.Getenv("DH_TICKET")})
				break
			}
		}
	}
	return result, nil
}
