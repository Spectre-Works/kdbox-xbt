package main

import (
	_ "embed"
	"encoding/json"
)

//go:embed compatibility.json
var compatibilityJSON []byte

func supportedDevice(id map[string]any) bool {
	var profile struct {
		Model     string   `json:"model"`
		Board     string   `json:"board"`
		Firmwares []string `json:"firmwares"`
	}
	if json.Unmarshal(compatibilityJSON, &profile) != nil || id["model"] != profile.Model || id["mcbversion"] != profile.Board {
		return false
	}
	for _, version := range profile.Firmwares {
		if id["firmware"] == version {
			return true
		}
	}
	return false
}

// Firmware-specific limits can only tighten the normal thermal shutdown.
func thermalLimits(version string) (float64, float64) {
	var profile struct {
		Settings map[string]struct {
			Average float64 `json:"average_limit"`
			Peak    float64 `json:"peak_limit"`
		} `json:"firmware_settings"`
	}
	average, peak := 84.0, 90.0
	if json.Unmarshal(compatibilityJSON, &profile) == nil {
		if limits, ok := profile.Settings[version]; ok {
			if limits.Average > 0 && limits.Average < average {
				average = limits.Average
			}
			if limits.Peak > 0 && limits.Peak < peak {
				peak = limits.Peak
			}
		}
	}
	return average, peak
}
