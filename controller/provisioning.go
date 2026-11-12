package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// Wait for the installer to record the dashboard login and tuning settings.
func provisioningReady(dir string) error {
	b, err := os.ReadFile(filepath.Join(dir, "config.json"))
	var cfg Config
	if err != nil || json.Unmarshal(b, &cfg) != nil || cfg.AdminPassword == "" {
		return errors.New("owner credential provisioning required")
	}
	if err := cfg.validatePerformance(); err != nil {
		return err
	}
	b, err = os.ReadFile(filepath.Join(dir, "approved.json"))
	var approved Tuning
	if err != nil || json.Unmarshal(b, &approved) != nil {
		return errors.New("confirmed tuning provisioning required")
	}
	if approved.Millivolts == 0 {
		approved.Millivolts = 430
	}
	return approved.Validate(approved)
}

func preflightDevice(root string) error {
	id := map[string]any{}
	for key, name := range map[string]string{"model": "minername", "mcbversion": "mcbversion", "firmware": "version"} {
		data, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			return errors.New("cannot verify local firmware before mining startup")
		}
		id[key] = strings.TrimSpace(string(data))
	}
	if !supportedDevice(id) {
		return errors.New("unsupported local firmware; mining startup held")
	}
	return nil
}
