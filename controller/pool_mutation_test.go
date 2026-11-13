package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRemoveLastPoolAndMalformedSavedPool(t *testing.T) {
	for _, address := range []string{"stratum+tcp://down.example:3333", "invalid old address"} {
		t.Run(address, func(t *testing.T) {
			original := map[string]any{"url": address, "user": "worker", "pass": "saved", "dragid": "existing"}
			pools := []map[string]any{original}
			writes := 0
			native := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch {
				case r.Method == "GET" && r.URL.Path == "/user/login":
					jsonReply(w, 200, map[string]string{"JWT Token": "native"})
				case r.Method == "GET" && r.URL.Path == "/mcb/pools":
					jsonReply(w, 200, pools)
				case r.Method == "PUT" && r.URL.Path == "/mcb/delpool":
					var sent map[string]any
					if err := json.NewDecoder(r.Body).Decode(&sent); err != nil || sent["url"] != address || sent["pass"] != "saved" || sent["dragid"] != "existing" {
						t.Error("deletion did not use the existing saved row")
					}
					writes++
					pools = []map[string]any{}
					jsonReply(w, 200, map[string]bool{"ok": true})
				default:
					t.Errorf("unexpected stock operation: %s %s", r.Method, r.URL.Path)
				}
			}))
			defer native.Close()
			c := &Controller{dir: t.TempDir(), backend: native.URL, client: native.Client(), sessions: map[string]Session{"valid": {time.Now().Add(time.Minute)}}}
			body, _ := json.Marshal(map[string]string{"url": address, "user": "worker", "pass": "untrusted request value"})
			r := httptest.NewRequest("POST", "http://miner/api/pool-delete", strings.NewReader(string(body)))
			r.Header.Set("Content-Type", "application/json")
			r.AddCookie(&http.Cookie{Name: "kdbox-xbt", Value: "valid"})
			w := httptest.NewRecorder()
			c.handler(w, r)
			if w.Code != 200 || writes != 1 || len(pools) != 0 {
				t.Fatalf("last pool was not removed: %d %s, writes=%d", w.Code, w.Body.String(), writes)
			}
			stop, err := os.ReadFile(filepath.Join(c.dir, "halted.json"))
			if err != nil || !c.halted || !strings.Contains(string(stop), poolSetupPause) {
				t.Fatal("removing the final pool did not safely pause mining", err)
			}
		})
	}
}

func TestResumeRequiresPoolWithoutClearingExistingStop(t *testing.T) {
	native := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" || r.URL.Path != "/mcb/pools" {
			t.Errorf("unexpected mutation: %s %s", r.Method, r.URL.Path)
		}
		jsonReply(w, 200, []map[string]any{})
	}))
	defer native.Close()
	c := &Controller{dir: t.TempDir(), halted: true, message: "Original thermal stop", backend: native.URL, client: native.Client(), sessions: map[string]Session{"valid": {time.Now().Add(time.Minute)}}}
	path := filepath.Join(c.dir, "halted.json")
	original := []byte(`{"reason":"Original thermal stop"}`)
	os.WriteFile(path, original, 0600)
	r := httptest.NewRequest("POST", "http://miner/api/resume", strings.NewReader(`{}`))
	r.Header.Set("Content-Type", "application/json")
	r.AddCookie(&http.Cookie{Name: "kdbox-xbt", Value: "valid"})
	w := httptest.NewRecorder()
	c.handler(w, r)
	current, err := os.ReadFile(path)
	if w.Code != 400 || !c.halted || err != nil || string(current) != string(original) {
		t.Fatalf("no-pool resume changed the retained stop: %d %s %v", w.Code, w.Body.String(), err)
	}
}

func TestPoolRemovalRejectsStaleRowsAndUnconfirmedRemoval(t *testing.T) {
	for _, stale := range []bool{true, false} {
		writes := 0
		native := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/user/login":
				jsonReply(w, 200, map[string]string{"JWT Token": "native"})
			case "/mcb/pools":
				if r.Method == "PUT" {
					writes++
					jsonReply(w, 200, map[string]bool{"ok": true})
				} else {
					jsonReply(w, 200, []map[string]any{{"url": "stratum+tcp://current.example:3333", "user": "worker"}})
				}
			case "/mcb/delpool":
				writes++
				jsonReply(w, 200, map[string]bool{"ok": true})
			default:
				t.Error(r.URL.Path)
			}
		}))
		c := &Controller{dir: t.TempDir(), halted: true, backend: native.URL, client: native.Client()}
		address := "stratum+tcp://current.example:3333"
		if stale {
			address = "stratum+tcp://old.example:3333"
		}
		body, _ := json.Marshal(map[string]string{"url": address, "user": "worker"})
		r := httptest.NewRequest("POST", "/api/pool-delete", strings.NewReader(string(body)))
		w := httptest.NewRecorder()
		c.settings(w, r)
		if w.Code != 400 || (stale && writes != 0) || (!stale && writes != 1) {
			t.Fatalf("stale=%t: %d %s, writes=%d", stale, w.Code, w.Body.String(), writes)
		}
		native.Close()
	}
}

func TestPoolListAcceptsEmptyArrayButRejectsNull(t *testing.T) {
	for _, body := range []string{"[]", "null"} {
		writes := 0
		pools := []map[string]any{{"url": "first", "user": "owner"}, {"url": "second", "user": "owner"}}
		native := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch {
			case r.URL.Path == "/user/login":
				jsonReply(w, 200, map[string]string{"JWT Token": "native"})
			case r.Method == "PUT" && r.URL.Path == "/mcb/delpool":
				var sent map[string]any
				if err := json.NewDecoder(r.Body).Decode(&sent); err != nil || len(pools) == 0 || sent["url"] != pools[0]["url"] {
					t.Error("wrong pool deletion")
				}
				writes++
				pools = pools[1:]
				jsonReply(w, 200, map[string]bool{"ok": true})
			case r.Method == "GET" && r.URL.Path == "/mcb/pools":
				jsonReply(w, 200, pools)
			default:
				t.Errorf("unexpected stock operation: %s %s", r.Method, r.URL.Path)
			}
		}))
		c := &Controller{dir: t.TempDir(), halted: true, backend: native.URL, client: native.Client()}
		w := httptest.NewRecorder()
		c.settings(w, httptest.NewRequest("POST", "/api/pools", strings.NewReader(body)))
		if (body == "[]" && (w.Code != 200 || writes != 2 || len(pools) != 0)) || (body == "null" && (w.Code != 400 || writes != 0)) {
			t.Fatalf("body=%s: %d %s, writes=%d", body, w.Code, w.Body.String(), writes)
		}
		native.Close()
	}
}
