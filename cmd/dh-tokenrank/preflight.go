package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"toktrail-collector/internal/scheduler"
	"toktrail-collector/internal/source/registry"
)

func prepareSetup(home, state string, task scheduler.TaskConfig, all, install bool, gs grantsFlag) (config, error) {
	cfg := config{All: all}
	if len(gs) == 0 && !all {
		return cfg, errors.New("at least one explicit --grant or --all is required; no paths are read by default")
	}
	for _, value := range gs {
		source, path, ok := strings.Cut(value, "=")
		if !ok || !registry.Production().IsKnownSource(source) || !filepath.IsAbs(path) {
			return cfg, errors.New("grant must be a supported source=absolute-path")
		}
		if _, err := os.Stat(path); err != nil {
			return cfg, fmt.Errorf("granted path is unavailable: %w", err)
		}
		cfg.Grants = append(cfg.Grants, grant{source, path})
	}
	if all {
		cfg.Explicit = append([]grant(nil), cfg.Grants...)
		cfg.Grants = mergeGrants(cfg.Grants, discover(home))
	}
	if err := checkStateWritable(state); err != nil {
		return cfg, fmt.Errorf("setup local preflight failed; setup code has not been redeemed: %w", err)
	}
	if install {
		if task.Platform == "windows" {
			for _, file := range []string{task.Binary, filepath.Join(filepath.Dir(task.Binary), "toktrail.exe"), scheduler.WindowsBackgroundPath(task.Binary)} {
				info, err := os.Stat(file)
				if err != nil || !info.Mode().IsRegular() || info.Size() == 0 {
					return cfg, fmt.Errorf("setup local preflight failed; incomplete Windows release: %s; setup code has not been redeemed", filepath.Base(file))
				}
			}
		}
		if err := scheduler.ValidateInstall(task); err != nil {
			return cfg, fmt.Errorf("setup local preflight failed; setup code has not been redeemed: %w", err)
		}
	}
	return cfg, nil
}
func checkStateWritable(state string) error {
	file, err := os.CreateTemp(state, ".dh-write-check-")
	if err != nil {
		return err
	}
	name := file.Name()
	if err = file.Close(); err != nil {
		_ = os.Remove(name)
		return err
	}
	return os.Remove(name)
}
