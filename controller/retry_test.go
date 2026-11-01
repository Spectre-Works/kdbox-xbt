package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestReadRetryIsBoundedAndDoesNotReplayWritesOrLogin(t *testing.T) {
	for _, test := range []struct {
		method, path string
		status, want int
	}{
		{"GET", "/mcb/status", 503, 2},
		{"GET", "/dbg/icinfo", 401, 1},
		{"PUT", "/mcb/setting", 503, 1},
		{"GET", "/user/login?username=admin", 503, 1},
	} {
		calls := 0
		s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls++; w.WriteHeader(test.status) }))
		c := Controller{backend: s.URL, client: s.Client()}
		_, err := c.native(test.method, test.path, nil, false)
		s.Close()
		if err == nil || calls != test.want {
			t.Fatalf("%s %s: calls=%d err=%v", test.method, test.path, calls, err)
		}
	}
}

func TestReadRecoversAfterTransientFailure(t *testing.T) {
	calls := 0
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			w.WriteHeader(503)
			return
		}
		w.Write([]byte(`{"ready":true}`))
	}))
	defer s.Close()
	c := Controller{backend: s.URL, client: s.Client()}
	b, err := c.native("GET", "/mcb/status", nil, false)
	if err != nil || string(b) != `{"ready":true}` || calls != 2 {
		t.Fatal(string(b), err, calls)
	}
}
