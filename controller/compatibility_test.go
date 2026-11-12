package main

import (
	"encoding/json"
	"testing"
)

func TestFirmwareCompatibility(t *testing.T) {
	var profile struct {
		Firmwares []string `json:"firmwares"`
	}
	if err := json.Unmarshal(compatibilityJSON, &profile); err != nil || len(profile.Firmwares) == 0 {
		t.Fatal("missing firmware allowlist")
	}
	for _, version := range profile.Firmwares {
		if !supportedDevice(map[string]any{"model": "Goldshell-KDBox", "mcbversion": "MCB_V5_4", "firmware": version}) {
			t.Fatal("listed version rejected", version)
		}
	}
	for _, id := range []map[string]any{
		{"model": "Goldshell-KDBox", "mcbversion": "MCB_V5_4", "firmware": "99.0.0"},
		{"model": "Goldshell-KDBox-Pro", "mcbversion": "MCB_V5_4", "firmware": profile.Firmwares[0]},
		{"model": "Goldshell-KDBox", "mcbversion": "MCB_V6", "firmware": profile.Firmwares[0]},
		{},
	} {
		if supportedDevice(id) {
			t.Fatal("unsupported identity accepted", id)
		}
	}
}

func TestFirmwareThermalLimits(t *testing.T) {
	for _, version := range []string{"", "2.2.1", "99.0.0"} {
		average, peak := thermalLimits(version)
		if average != 84 || peak != 90 {
			t.Fatalf("unexpected limits for %s: %v/%v", version, average, peak)
		}
	}
	average, peak := thermalLimits("2.0.6")
	if average != 80 || peak != 85 {
		t.Fatalf("2.0.6 must retain stricter limits: %v/%v", average, peak)
	}
}
