package main

import (
	"strings"
	"testing"
)

func TestEmbeddedBrandAndNotices(t *testing.T) {
	for _, name := range []string{"index.html", "app.js", "settings.js", "style.css"} {
		data, err := assets.ReadFile("web/" + name)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(strings.ToLower(string(data)), "paperclip") {
			t.Fatalf("upstream promotional branding remains in %s", name)
		}
	}
	for _, name := range []string{"favicon.svg", "licenses.html", "open-source.txt"} {
		if _, err := assets.ReadFile("web/" + name); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := assets.ReadFile("web/paperclip.svg"); err == nil {
		t.Fatal("retired upstream artwork is still embedded")
	}
	data, _ := assets.ReadFile("web/open-source.txt")
	for _, required := range []string{"Copyright (c) 2026 Paperclip Flasher contributors", "Permission is hereby granted", "THE SOFTWARE IS PROVIDED"} {
		if !strings.Contains(string(data), required) {
			t.Fatalf("missing notice: %s", required)
		}
	}
}
