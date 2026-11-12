package main

import (
	"os"
	"strings"
	"testing"
)

func TestEmbeddedBrandAndNotices(t *testing.T) {
	for _, name := range []string{"index.html", "app.js", "settings.js", "style.css"} {
		data, err := assets.ReadFile("web/" + name)
		if err != nil {
			t.Fatal(err)
		}
		if len(data) == 0 {
			t.Fatalf("empty dashboard asset %s", name)
		}
	}
	for _, name := range []string{"favicon-32.png", "favicon-48.png", "spectre-logo.png", "licenses.html", "open-source.txt"} {
		if _, err := assets.ReadFile("web/" + name); err != nil {
			t.Fatal(err)
		}
	}
	data, _ := assets.ReadFile("web/open-source.txt")
	license, err := os.ReadFile("../LICENSE")
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{strings.TrimSpace(string(license)), "Permission is hereby granted", "THE SOFTWARE IS PROVIDED"} {
		if !strings.Contains(string(data), required) {
			t.Fatalf("missing notice: %s", required)
		}
	}
}
