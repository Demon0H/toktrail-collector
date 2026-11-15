package main

import (
	"testing"
	"toktrail-collector/internal/source/adapter"
)

func TestRecoveryRequiresServerConfirmationWithoutAdoptingChangedUsage(t *testing.T) {
	row := adapter.Record{"recordKey": "codex-jsonl:one", "occurredAt": "2026-11-15T01:00:00Z", "tool": "Codex", "model": "test-model", "input": "9007199254740993", "output": "1", "cacheRead": nil, "cacheWrite": "0"}
	history := map[string]adapter.Record{"codex-jsonl:one": row}
	verified, e := verifiedRecords([]adapter.Record{row}, history, 1)
	if e != nil || len(verified) != 1 {
		t.Fatal("matching historical usage was not recovered", e)
	}
	if _, e = verifiedRecords([]adapter.Record{row}, history, 2); e == nil {
		t.Fatal("incomplete historical proof accepted")
	}
	changed := adapter.Record{}
	for k, v := range row {
		changed[k] = v
	}
	changed["input"] = "9007199254740994"
	if _, e = verifiedRecords([]adapter.Record{changed}, history, 1); e == nil {
		t.Fatal("changed usage accepted")
	}
	changed["input"] = row["input"]
	changed["cacheRead"] = "0"
	if _, e = verifiedRecords([]adapter.Record{changed}, history, 1); e == nil {
		t.Fatal("unknown cache count coerced to zero")
	}
}
