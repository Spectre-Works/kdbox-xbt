package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestResumeReappliesExistingAlgorithmExactlyOnce(t *testing.T) {
	const menu = `{"algo_select":1,"algos":[{"id":1,"name":"blake2b(SC)"}]}`
	for _, status := range []int{200, 503} {
		gets, puts := 0, 0
		s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/mcb/algosetting" {
				t.Error(r.URL.Path)
			}
			if r.Method == "GET" {
				gets++
				w.Write([]byte(menu))
				return
			}
			puts++
			b, _ := io.ReadAll(r.Body)
			if string(b) != menu || r.Header.Get("Authorization") != "Bearer owner" {
				t.Error("configuration or authentication changed")
			}
			w.WriteHeader(status)
		}))
		c := Controller{backend: s.URL, client: s.Client(), token: "owner"}
		err := c.reapplyAlgorithm()
		s.Close()
		if gets != 1 || puts != 1 || (err != nil) != (status != 200) {
			t.Fatal(gets, puts, err)
		}
	}
}
