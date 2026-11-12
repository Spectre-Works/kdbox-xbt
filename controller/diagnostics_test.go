package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStopRecordSurvivesRestart(t *testing.T) {
	p := filepath.Join(t.TempDir(), "halted.json")
	if _, err := saveStop(p, "Telemetry lost", map[string]any{"peak_temp": 72}); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(p)
	reason, err := saveStop(p, "Generic startup message", nil)
	after, _ := os.ReadFile(p)
	if err != nil || reason != "Telemetry lost" || string(before) != string(after) {
		t.Fatal(reason, err)
	}
	var record map[string]any
	json.Unmarshal(after, &record)
	if record["at"] == "" || record["device"].(map[string]any)["peak_temp"] != float64(72) {
		t.Fatal(record)
	}
}

func TestSavedMiningLogReadsCompleteRecentLines(t *testing.T) {
	path := filepath.Join(t.TempDir(), "minerlog")
	last := "temperature avg 47.0 max 52.0\n"
	if err := os.WriteFile(path, []byte(strings.Repeat("older entry\n", diagnosticTailSize/10)+last), 0600); err != nil {
		t.Fatal(err)
	}
	report, err := readLogTail(path)
	if err != nil || !report.Truncated || len(report.Body) > diagnosticTailSize ||
		!strings.HasPrefix(report.Body, "older entry\n") || !strings.HasSuffix(report.Body, last) {
		t.Fatalf("log tail invalid: bytes=%d truncated=%t err=%v", len(report.Body), report.Truncated, err)
	}
}

func TestLegacyAndMalformedStopRecordsArePreserved(t *testing.T) {
	for _, data := range []string{`{"reason":"Fan RPM below 3000"}`, `broken`} {
		p := filepath.Join(t.TempDir(), "halted.json")
		os.WriteFile(p, []byte(data), 0600)
		reason, err := saveStop(p, "Startup", nil)
		after, _ := os.ReadFile(p)
		if string(after) != data {
			t.Fatal("original evidence overwritten")
		}
		if data == "broken" && err == nil {
			t.Fatal("malformed record accepted")
		}
		if data != "broken" && (err != nil || reason != "Fan RPM below 3000") {
			t.Fatal(reason, err)
		}
	}
}
