package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPreflightRequiresOwnerProvisioning(t *testing.T) {
	dir := t.TempDir()
	if provisioningReady(dir) == nil {
		t.Fatal("unprovisioned image must not start mining")
	}
	if err := atomicJSON(filepath.Join(dir, "config.json"), Config{AdminPassword: "123456789"}); err != nil {
		t.Fatal(err)
	}
	if provisioningReady(dir) == nil {
		t.Fatal("credentials alone must not start mining")
	}
	if err := atomicJSON(filepath.Join(dir, "approved.json"), Tuning{430, 740}); err != nil {
		t.Fatal(err)
	}
	if err := provisioningReady(dir); err != nil {
		t.Fatal(err)
	}
}

func TestCandidateFirmwareCannotPassMiningPreflight(t *testing.T) {
	root := t.TempDir()
	for name, value := range map[string]string{"version": "2.2.4", "mcbversion": "MCB_V5_4", "minername": "Goldshell-KDBox"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(value), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if preflightDevice(root) == nil {
		t.Fatal("candidate base must not enable mining")
	}
	if err := os.WriteFile(filepath.Join(root, "version"), []byte("2.1.3"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := preflightDevice(root); err != nil {
		t.Fatal(err)
	}
}
