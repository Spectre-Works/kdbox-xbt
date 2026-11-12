package main

const productName = "KD BOX XBT Mod"

// Set by tools/build.py. The revision belongs to the compiled controller;
// an installer cannot accidentally label an older binary with its own HEAD.
var buildVersion = "development"
var buildCommit = "unknown"
var buildDirty = "true"

type softwareVersion struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Commit      string `json:"commit"`
	ShortCommit string `json:"short_commit"`
	Dirty       bool   `json:"dirty"`
}

func softwareInfo() softwareVersion {
	short := ""
	valid := len(buildCommit) == 40 || len(buildCommit) == 64
	for _, c := range buildCommit {
		if !(c >= '0' && c <= '9') && !(c >= 'a' && c <= 'f') {
			valid = false
			break
		}
	}
	if valid {
		short = buildCommit[:7]
	}
	return softwareVersion{productName, buildVersion, buildCommit, short, buildDirty != "false"}
}
