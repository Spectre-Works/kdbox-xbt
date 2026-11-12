package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDiskRecoveryBeforeNativeStartup(t *testing.T) {
	dir := t.TempDir()
	settings := filepath.Join(dir, "settings")
	miner := filepath.Join(dir, "miner")
	os.Mkdir(settings, 0700)
	os.Mkdir(miner, 0700)
	p := Pending{Tuning{430, 740}, Tuning{425, 725}}
	atomicJSON(filepath.Join(dir, "pending.json"), p)
	for _, name := range []string{"cpbsetting.json", ".cpbsetting.json"} {
		atomicJSON(filepath.Join(settings, name), map[string]any{"manualPowerplan": "725 MHz 0.425 V 90 RPM 90 RPM"})
	}
	for _, name := range []string{"bfgminer.json", ".bfgminer.json"} {
		atomicJSON(filepath.Join(miner, name), map[string]any{"set": []string{"intchains_qomo:algo=blake2b(SC)", "intchains_qomo:vfff=0.425000:725:90:90"}})
	}
	if e := diskRecover(dir, settings, miner); e != nil {
		t.Fatal(e)
	}
	for _, name := range []string{"bfgminer.json", ".bfgminer.json"} {
		b, _ := os.ReadFile(filepath.Join(miner, name))
		if !strings.Contains(string(b), "0.430000:740:100:100") || !strings.Contains(string(b), "blake2b(SC)") {
			t.Fatal(string(b))
		}
	}
	var approved Tuning
	b, _ := os.ReadFile(filepath.Join(dir, "approved.json"))
	json.Unmarshal(b, &approved)
	if approved != p.Previous {
		t.Fatal(approved)
	}
	if _, e := os.Stat(filepath.Join(dir, "pending.json")); e != nil {
		t.Fatal("API recovery record cleared too early")
	}
}
