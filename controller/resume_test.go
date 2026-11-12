package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
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
		c := Controller{backend: s.URL, client: s.Client(), token: "owner", algorithm: 1}
		err := c.reapplyAlgorithm()
		s.Close()
		if puts != 1 || (status == 200 && (gets != 2 || err != nil)) || (status != 200 && (gets != 1 || err == nil)) {
			t.Fatal(gets, puts, err)
		}
	}
}

func TestSafetyStopQueuesAlgorithmWithoutRestartingEngine(t *testing.T) {
	c := Controller{dir: t.TempDir(), halted: true, algorithm: 0}
	if err := c.chooseAlgorithm(1); err != nil {
		t.Fatal(err)
	}
	if !c.halted || !c.algorithmQueued || c.algorithm != 1 {
		t.Fatal("selection should be saved for resume")
	}
	if saved, err := os.ReadFile(c.algorithmPath()); err != nil || string(saved) != "1" {
		t.Fatalf("saved algorithm: %q, %v", saved, err)
	}
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" || r.URL.Path != "/mcb/algosetting" {
			t.Errorf("unexpected native request during safety stop: %s %s", r.Method, r.URL.Path)
		}
		w.Write([]byte(`{"algo_select":0}`))
	}))
	defer s.Close()
	restarted := Controller{dir: c.dir, halted: true, backend: s.URL, client: s.Client()}
	if err := restarted.restoreAlgorithm(); err != nil || restarted.algorithm != 1 || !restarted.algorithmQueued {
		t.Fatalf("algorithm selection did not survive restart: %v", err)
	}
}

func TestAlgorithmApplicationRejectsUnchangedFirmwareSetting(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/user/login" {
			w.Write([]byte(`{"JWT Token":"owner"}`))
			return
		}
		w.Write([]byte(`{"algo_select":0}`))
	}))
	defer s.Close()
	c := Controller{dir: t.TempDir(), backend: s.URL, client: s.Client()}
	if err := c.chooseAlgorithm(1); err == nil || c.algorithm != 0 {
		t.Fatalf("the native engine did not confirm the new selection: %v", err)
	}
	if _, err := os.Stat(c.algorithmPath()); !os.IsNotExist(err) {
		t.Fatalf("unconfirmed selection was persisted: %v", err)
	}
}

func TestMissingAlgorithmDefaultsToXBTAndSavedKDAIsRetained(t *testing.T) {
	for _, saved := range []string{"", "0", "1"} {
		t.Run("saved_"+saved, func(t *testing.T) {
			actual, puts := 0, 0
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == "PUT" {
					var menu struct {
						Algorithm int `json:"algo_select"`
					}
					if err := json.NewDecoder(r.Body).Decode(&menu); err != nil {
						t.Error(err)
					}
					actual = menu.Algorithm
					puts++
				}
				fmt.Fprintf(w, `{"algo_select":%d}`, actual)
			}))
			defer s.Close()
			c := Controller{dir: t.TempDir(), backend: s.URL, client: s.Client()}
			if saved != "" {
				if err := os.WriteFile(c.algorithmPath(), []byte(saved), 0600); err != nil {
					t.Fatal(err)
				}
			}
			want := 1
			if saved == "0" {
				want = 0
			}
			if err := c.restoreAlgorithm(); err != nil {
				t.Fatal(err)
			}
			if actual != want || c.algorithm != want || puts != 1 {
				t.Fatalf("actual=%d, saved=%d, writes=%d", actual, c.algorithm, puts)
			}
			data, err := os.ReadFile(c.algorithmPath())
			if err != nil || string(data) != fmt.Sprint(want) {
				t.Fatal(string(data), err)
			}
		})
	}
}

func TestDefaultXBTDoesNotResumeSafetyStoppedMiner(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Error("default selection must not write stock settings during a safety stop")
		}
		w.Write([]byte(`{"algo_select":0}`))
	}))
	defer s.Close()
	c := Controller{dir: t.TempDir(), backend: s.URL, client: s.Client(), halted: true}
	if err := c.restoreAlgorithm(); err != nil {
		t.Fatal(err)
	}
	if c.algorithm != 1 || !c.algorithmQueued || !c.halted {
		t.Fatal("XBT must be queued while the safety stop remains")
	}
}
