package main

import (
	"encoding/json"
	"os"
	"path/filepath"
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
