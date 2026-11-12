package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Native API setters require xminerd; restore disk parameters before it starts.
func diskRecover(dir, settingsDir, minerDir string) error {
	b, e := os.ReadFile(filepath.Join(dir, "pending.json"))
	if os.IsNotExist(e) {
		return nil
	}
	if e != nil {
		return e
	}
	var p Pending
	if e = json.Unmarshal(b, &p); e != nil {
		return e
	}
	if p.Previous.Millivolts == 0 {
		p.Previous.Millivolts = 430
	}
	if e = p.Previous.Validate(p.Previous); e != nil {
		return e
	}
	var cfg Config
	if data, err := os.ReadFile(filepath.Join(dir, "config.json")); err == nil {
		if err := json.Unmarshal(data, &cfg); err != nil {
			return err
		}
		if err := cfg.validatePerformance(); err != nil {
			return err
		}
	}
	p.Previous = initialPerformancePoint(cfg, p.Previous)
	for _, name := range []string{"cpbsetting.json", ".cpbsetting.json"} {
		path := filepath.Join(settingsDir, name)
		b, e = os.ReadFile(path)
		if e != nil {
			return e
		}
		var v map[string]any
		if e = json.Unmarshal(b, &v); e != nil {
			return e
		}
		v["manualPowerplan"] = p.Previous.Powerplan()
		v["powerplans"] = []any{map[string]any{"level": 0, "info": p.Previous.Powerplan()}}
		v["select"] = 0
		v["manual"] = false
		v["tempcontrol"] = true
		if e = atomicJSON(path, v); e != nil {
			return e
		}
	}
	for _, name := range []string{"bfgminer.json", ".bfgminer.json"} {
		path := filepath.Join(minerDir, name)
		b, e = os.ReadFile(path)
		if os.IsNotExist(e) && strings.HasPrefix(name, ".") {
			continue
		}
		if e != nil {
			return e
		}
		var v map[string]any
		if e = json.Unmarshal(b, &v); e != nil {
			return e
		}
		settings, ok := v["set"].([]any)
		if !ok {
			return fmt.Errorf("missing native hardware settings")
		}
		found := false
		for i, setting := range settings {
			if text, ok := setting.(string); ok && strings.HasPrefix(text, "intchains_qomo:vfff=") {
				settings[i] = fmt.Sprintf("intchains_qomo:vfff=%.6f:%d:100:100", float64(p.Previous.Millivolts)/1000, p.Previous.Clock)
				found = true
			}
		}
		if !found {
			return fmt.Errorf("missing native voltage/clock entry")
		}
		v["set"] = settings
		if e = atomicJSON(path, v); e != nil {
			return e
		}
	}
	return atomicJSON(filepath.Join(dir, "approved.json"), p.Previous)
}
