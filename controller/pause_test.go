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

func TestPoolSetupIsOnlyAutomaticallyResumablePause(t *testing.T) {
	cases := []struct {
		body     string
		eligible bool
	}{
		{`{"reason":"No mining pools configured; add a pool before resuming"}`, true},
		{`{"reason":"Mining settings reset; add a pool before mining resumes","kind":"pool-setup"}`, true},
		{`{"reason":"No mining pools configured; add a pool before resuming","kind":"fault"}`, false},
		{`{"reason":"Temperature safety stop"}`, false},
		{`{"reason":"Fan RPM fault"}`, false},
		{`{"reason":"Watchdog fault"}`, false},
		{`{"reason":"Paused for a software update; review settings before resuming"}`, false},
		{`{"reason":"Recovered after missing installation settings; review configuration before resuming"}`, false},
		{`{"reason":"unknown","kind":"pool-setup"}`, false},
		{`broken`, false}, {`{}`, false},
	}
	for _, tc := range cases {
		c := Controller{dir: t.TempDir(), halted: true}
		os.WriteFile(filepath.Join(c.dir, "halted.json"), []byte(tc.body), 0600)
		_, got := c.pauseStatus()
		if got != tc.eligible {
			t.Fatalf("%s: eligible=%t", tc.body, got)
		}
	}
}

func poolResumeFixture(t *testing.T, failure bool, changeStop bool) (*Controller, *int) {
	t.Helper()
	writes := 0
	native := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/mcb/pools":
			jsonReply(w, 200, []map[string]any{{"url": "stratum+tcp://pool.example:3333", "user": "worker"}})
		case "/user/login":
			jsonReply(w, 200, map[string]string{"JWT Token": "native"})
		case "/mcb/setting":
			if r.Method == "PUT" {
				writes++
				if failure {
					w.WriteHeader(503)
					return
				}
			}
			jsonReply(w, 200, map[string]any{"manualPowerplan": "740 MHz 0.430 V"})
		case "/mcb/algosetting":
			jsonReply(w, 200, map[string]any{"algo_select": 1})
		default:
			t.Errorf("unexpected endpoint %s", r.URL.Path)
		}
	}))
	t.Cleanup(native.Close)
	c := &Controller{dir: t.TempDir(), halted: true, approved: defaultTuning, cfg: Config{AdminPassword: "owner"}, backend: native.URL, client: native.Client(), algorithm: 1}
	c.firmware = "2.2.1"
	atomicJSON(filepath.Join(c.dir, "halted.json"), map[string]string{"reason": poolSetupPause})
	if changeStop {
		c.reload = func() error {
			return atomicJSON(filepath.Join(c.dir, "halted.json"), map[string]string{"reason": "New thermal fault", "kind": "fault"})
		}
	}
	return c, &writes
}

func TestPoolSetupResumesOnceAndStartupFailureLatches(t *testing.T) {
	for _, failure := range []bool{false, true} {
		c, writes := poolResumeFixture(t, failure, false)
		c.tryResumePoolSetup()
		first := *writes
		c.tryResumePoolSetup()
		if first == 0 || *writes != first {
			t.Fatal("startup was skipped or retried")
		}
		if failure {
			kind, automatic := c.pauseStatus()
			if !c.halted || kind != "fault" || automatic {
				t.Fatal("failed startup was not latched")
			}
		} else if c.halted {
			t.Fatal("adding a valid pool did not resume")
		}
	}
}

func TestAutoResumeCannotClearChangedStopOrPendingAdjustment(t *testing.T) {
	c, _ := poolResumeFixture(t, false, true)
	c.tryResumePoolSetup()
	b, _ := os.ReadFile(filepath.Join(c.dir, "halted.json"))
	if !c.halted || !strings.Contains(string(b), "New thermal fault") {
		t.Fatal("new fault was lost")
	}
	for _, pending := range []string{"performance", "network", "reset"} {
		c, writes := poolResumeFixture(t, false, false)
		switch pending {
		case "performance":
			c.pending = &Pending{}
		case "network":
			c.networkPending = &NetworkTrial{}
		case "reset":
			atomicJSON(filepath.Join(c.dir, "mining-reset.json"), miningResetJournal{Version: 1})
		}
		c.tryResumePoolSetup()
		if !c.halted || *writes != 0 {
			t.Fatal("pending adjustment was resumed", pending)
		}
	}
}

func TestMiningResetPreservesFaultAndCredentialsAndRecoversJournal(t *testing.T) {
	pools := []map[string]any{{"url": "stratum+tcp://old.example:3333", "user": "owner"}}
	failDelete := true
	deletes := 0
	native := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/mcb/pools":
			jsonReply(w, 200, pools)
		case "/user/login":
			jsonReply(w, 200, map[string]string{"JWT Token": "native"})
		case "/mcb/delpool":
			deletes++
			if failDelete {
				w.WriteHeader(503)
				return
			}
			pools = []map[string]any{}
			jsonReply(w, 200, map[string]bool{"ok": true})
		default:
			t.Errorf("unexpected reset operation %s", r.URL.Path)
		}
	}))
	defer native.Close()
	cfg := Config{AdminPassword: "unchanged", ChipTarget: 80, AutomaticTuning: true}
	c := &Controller{dir: t.TempDir(), halted: true, cfg: cfg, approved: defaultTuning, backend: native.URL, client: native.Client()}
	original := []byte(`{"reason":"Original thermal stop","kind":"fault"}`)
	os.WriteFile(filepath.Join(c.dir, "halted.json"), original, 0600)
	if err := c.resetMiningSettings(); err != nil {
		t.Fatal(err)
	}
	c.advanceMiningReset()
	c.advanceMiningReset()
	if deletes != 1 {
		t.Fatal("ambiguous deletion was retried")
	}
	if pending, failed := c.miningResetState(); !pending || !failed {
		t.Fatal("failure not journaled")
	}
	failDelete = false
	if err := c.resetMiningSettings(); err != nil {
		t.Fatal(err)
	}
	// New instance represents a restart after the confirmed retry.
	next := &Controller{dir: c.dir, halted: true, cfg: cfg, backend: native.URL, client: native.Client()}
	next.advanceMiningReset()
	next.advanceMiningReset()
	if pending, _ := next.miningResetState(); pending {
		t.Fatal("reset did not finish")
	}
	b, _ := os.ReadFile(filepath.Join(c.dir, "halted.json"))
	if string(b) != string(original) {
		t.Fatal("original fault changed")
	}
	data, _ := os.ReadFile(filepath.Join(c.dir, "config.json"))
	var got Config
	json.Unmarshal(data, &got)
	if got.AdminPassword != "unchanged" || got.AutomaticTuning || got.coolingTarget() != 75 || next.approved != defaultTuning || next.algorithm != 1 || len(pools) != 0 {
		t.Fatal("reset defaults or preserved account are wrong")
	}
	if err := next.resumeMiningFromPause([]byte("wrong")); err == nil {
		t.Fatal("changed stop should reject resume")
	}
}

func TestResetRequiresAuthenticationAndExactConfirmation(t *testing.T) {
	for _, tc := range []struct {
		auth bool
		body string
	}{{false, `{"confirm":"FACTORY RESET"}`}, {true, `{"confirm":"RESET"}`}} {
		c := &Controller{dir: t.TempDir(), sessions: map[string]Session{"valid": {Expires: time.Now().Add(time.Minute)}}}
		r := httptest.NewRequest("POST", "http://miner/api/factory-reset", strings.NewReader(tc.body))
		r.Header.Set("Content-Type", "application/json")
		if tc.auth {
			r.AddCookie(&http.Cookie{Name: "kdbox-xbt", Value: "valid"})
		}
		w := httptest.NewRecorder()
		c.handler(w, r)
		if w.Code == 200 {
			t.Fatal("unconfirmed reset accepted")
		}
		if _, err := os.Stat(filepath.Join(c.dir, "mining-reset.json")); !os.IsNotExist(err) {
			t.Fatal("unconfirmed reset wrote a journal")
		}
	}
}

func TestResetJournalWriteFailureDoesNotChangeMiningState(t *testing.T) {
	c := &Controller{dir: filepath.Join(t.TempDir(), "missing-directory")}
	if err := c.resetMiningSettings(); err == nil || c.halted {
		t.Fatal("failed write-ahead journal changed mining state")
	}
}

func TestInvalidPoolsAndUnverifiedFirmwareCannotAutoResume(t *testing.T) {
	for _, tc := range []struct {
		pool     string
		firmware string
	}{{`[]`, "2.2.1"}, {`[{"url":"broken","user":"worker"}]`, "2.2.1"}, {`[{"url":"stratum+tcp://pool.example:3333","user":"worker"}]`, "2.2.4"}} {
		native := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != "GET" || r.URL.Path != "/mcb/pools" {
				t.Error("invalid setup triggered a mutation")
			}
			w.Write([]byte(tc.pool))
		}))
		c := &Controller{dir: t.TempDir(), halted: true, firmware: tc.firmware, algorithm: 1, approved: defaultTuning, backend: native.URL, client: native.Client()}
		atomicJSON(filepath.Join(c.dir, "halted.json"), map[string]string{"reason": poolSetupPause})
		c.tryResumePoolSetup()
		if !c.halted {
			t.Fatal("invalid setup resumed")
		}
		native.Close()
	}
}
