package main

import "testing"

func TestSoftwareIdentityLabelsCleanDirtyAndUnversionedBuilds(t *testing.T) {
	originalVersion, originalCommit, originalDirty := buildVersion, buildCommit, buildDirty
	defer func() { buildVersion, buildCommit, buildDirty = originalVersion, originalCommit, originalDirty }()
	buildVersion = "1.0"
	for _, tc := range []struct {
		commit, dirty, short string
		changed              bool
	}{
		{"0123456789abcdef0123456789abcdef01234567", "false", "0123456", false},
		{"0123456789abcdef0123456789abcdef01234567", "true", "0123456", true},
		{"unknown", "true", "", true},
	} {
		buildCommit, buildDirty = tc.commit, tc.dirty
		info := softwareInfo()
		if info.Version != "1.0" || info.Commit != tc.commit || info.ShortCommit != tc.short || info.Dirty != tc.changed {
			t.Fatal(info)
		}
	}
}
