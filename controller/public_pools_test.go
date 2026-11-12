package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestLockedPoolVisibility(t *testing.T) {
	native := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" || r.URL.Path != "/mcb/pools" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		w.Write([]byte(`[{"url":"stratum+tcp://account:secret@pool.example:3333?token=secret","user":"worker","pass":"secret","pool-priority":0}]`))
	}))
	defer native.Close()
	c := &Controller{backend: native.URL, client: native.Client(), sessions: map[string]Session{"valid": {time.Now().Add(time.Minute)}}}
	for _, cookie := range []string{"", "expired", "valid"} {
		r := httptest.NewRequest("GET", "http://miner/api/pool-status", nil)
		if cookie != "" {
			r.AddCookie(&http.Cookie{Name: "kdbox-xbt", Value: cookie})
		}
		w := httptest.NewRecorder()
		c.handler(w, r)
		body := w.Body.String()
		if w.Code != 200 || strings.Contains(body, "secret") || strings.Contains(body, "pass") || !strings.Contains(body, "worker") {
			t.Fatalf("unsafe or missing read: %d %s", w.Code, body)
		}
		if strings.Contains(body, `"editable":true`) != (cookie == "valid") {
			t.Fatal(body)
		}
	}
	for _, path := range []string{"pools", "pool-add", "pool-delete"} {
		r := httptest.NewRequest("POST", "http://miner/api/"+path, strings.NewReader(`{}`))
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		c.handler(w, r)
		if w.Code != 401 {
			t.Fatalf("locked write %s: %d", path, w.Code)
		}
	}
}
