package main

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func ownerFixture(t *testing.T) (string, string, []byte) {
	t.Helper()
	root := t.TempDir()
	dir := ownerPath(root, "/usr/config/kdbox-xbt")
	files := map[string][]byte{
		"/root/settings/login.json":      []byte(`{"username":"admin","password":"123456789","version":"v1.0"}`),
		"/root/settings/cpbsetting.json": []byte(`{"name":"vendor","tempcontrol":true}`),
		"/root/settings/qomo.json":       []byte(`{"version":"v1.0"}`),
		"/root/wifi/hostapd.conf":        []byte("interface=wlan0\nssid=factory\n"),
		"/root/wifi/dnsmasq.conf":        []byte("interface=wlan0\n"),
	}
	for path, b := range files {
		if err := ownerWrite(ownerPath(root, path), b, 0600); err != nil {
			t.Fatal(err)
		}
	}
	for _, path := range []string{"/usr/config/minerd/settings/login.json", "/usr/config/minerd/settings/interfaces", "/usr/config/wifi/wpa_supplicant.conf", "/usr/config/bfgminer/bfgminer.json", "/usr/config/miner_history/history", "/usr/config/private-extra/secret", "/root/.ssh/authorized_keys", "/root/.ssh/id_rsa", "/root/.bash_history", "/backup/bfgminer/trash/minerlog", "/tmp/kdbox-xbt-history.json", "/etc/network/interfaces", "/etc/hostname"} {
		ownerWrite(ownerPath(root, path), []byte("old-owner-secret"), 0600)
	}
	for _, name := range []string{"controller", "launcher.sh", "supervisor.sh", "boot.sh"} {
		ownerWrite(filepath.Join(dir, name), []byte("application"), 0700)
	}
	for _, name := range []string{"config.json", "approved.json", "algorithm.json", "clock-limit-request.json", "old-private-backup"} {
		ownerWrite(filepath.Join(dir, name), []byte("old-owner-secret"), 0600)
	}
	fault := []byte(`{"reason":"Original thermal stop","kind":"fault"}`)
	ownerWrite(filepath.Join(dir, "halted.json"), fault, 0600)
	atomicJSON(filepath.Join(dir, "mining-reset.json"), miningResetJournal{Version: 2, OwnerReset: true, Phase: "owner", BootID: "before"})
	return root, dir, fault
}

func TestOwnerResetErasesPersonalizationAndKeepsApplicationAndFault(t *testing.T) {
	root, dir, fault := ownerFixture(t)
	passwordCalls, holds, restarts := 0, 0, 0
	hold := func() (func(), error) { holds++; return func() { restarts++ }, nil }
	setPassword := func() error {
		passwordCalls++
		return ownerWrite(ownerPath(root, "/etc/shadow"), []byte("factory-root-credential"), 0600)
	}
	if err := resetOwnerFiles(root, dir, "old-owner-secret", setPassword, hold); err != nil {
		t.Fatal(err)
	}
	if holds != 1 || restarts != 1 || passwordCalls != 1 {
		t.Fatal("services/password were not reset exactly once")
	}
	b, _ := os.ReadFile(filepath.Join(dir, "halted.json"))
	if string(b) != string(fault) {
		t.Fatal("genuine fault was altered")
	}
	for _, name := range []string{"controller", "launcher.sh", "supervisor.sh", "boot.sh"} {
		b, _ := os.ReadFile(filepath.Join(dir, name))
		if string(b) != "application" {
			t.Fatal("mod application was erased")
		}
	}
	var cfg Config
	b, _ = os.ReadFile(filepath.Join(dir, "config.json"))
	json.Unmarshal(b, &cfg)
	if cfg.AdminPassword != factoryWebPassword || cfg.FallbackPassword != "" || cfg.AutomaticTuning || cfg.AutomaticLimits != nil || cfg.coolingTarget() != 75 {
		t.Fatal("personalized mod settings retained")
	}
	for _, path := range []string{"/root/.ssh", "/usr/config/private-extra", "/usr/config/minerd/settings/interfaces", "/backup/bfgminer/trash/minerlog"} {
		if _, err := os.Stat(ownerPath(root, path)); !os.IsNotExist(err) {
			t.Fatal("personal data retained", path)
		}
	}
	b, _ = os.ReadFile(ownerPath(root, "/etc/network/interfaces"))
	if !strings.Contains(string(b), "iface eth0 inet dhcp") {
		t.Fatal("static networking retained")
	}
	b, _ = os.ReadFile(ownerPath(root, "/usr/config/wifi/wpa_supplicant.conf"))
	if strings.Contains(string(b), "network=") {
		t.Fatal("saved Wi-Fi retained")
	}
	var wifi map[string]any
	b, _ = os.ReadFile(ownerPath(root, "/usr/config/minerd/settings/wifi.json"))
	json.Unmarshal(b, &wifi)
	if wifi["enable"] != false {
		t.Fatal("Wi-Fi not disconnected")
	}
	b, _ = os.ReadFile(ownerPath(root, "/usr/config/minerd/settings/key.pem"))
	block, _ := pem.Decode(b)
	key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil || key.N.BitLen() != 2048 {
		t.Fatal("fresh native signing key missing")
	}
	b, _ = os.ReadFile(ownerPath(root, "/usr/config/minerd/settings/pubkey.pem"))
	block, _ = pem.Decode(b)
	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil || pub.(*rsa.PublicKey).N.Cmp(key.N) != 0 {
		t.Fatal("native session keys do not match")
	}
	// The stock daemon restores its signing key from these templates at startup.
	// Simulate that copy and prove old-session signing authority cannot return.
	for _, name := range []string{"key.pem", "pubkey.pem"} {
		template, err := os.ReadFile(ownerPath(root, "/root/settings/"+name))
		if err != nil {
			t.Fatal(err)
		}
		runtime, _ := os.ReadFile(ownerPath(root, "/usr/config/minerd/settings/"+name))
		if string(template) != string(runtime) {
			t.Fatal("stock startup would restore a different signing key")
		}
	}
	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.Mode().IsRegular() {
			b, _ := os.ReadFile(path)
			if strings.Contains(string(b), "old-owner-secret") {
				t.Error("old owner secret remains", path)
			}
		}
		return nil
	})
}

func TestOwnerResetRejectsUntrustedTemplatesBeforeAnyDestructiveAction(t *testing.T) {
	root, dir, _ := ownerFixture(t)
	ownerWrite(ownerPath(root, "/root/settings/login.json"), []byte(`{"username":"admin","password":"unexpected"}`), 0600)
	if err := resetOwnerFiles(root, dir, "owner", func() error { t.Fatal("password changed"); return nil }, func() (func(), error) { t.Fatal("services held"); return nil, nil }); err == nil {
		t.Fatal("unverified template accepted")
	}
	b, _ := os.ReadFile(ownerPath(root, "/usr/config/wifi/wpa_supplicant.conf"))
	if string(b) != "old-owner-secret" {
		t.Fatal("rejected reset erased state")
	}
}

func TestOwnerResetFailureRetainsCredentialFallbackAndStopsBeforeCompletion(t *testing.T) {
	root, dir, _ := ownerFixture(t)
	restarted := false
	err := resetOwnerFiles(root, dir, "old-owner-secret", func() error { return errors.New("password reset failed") }, func() (func(), error) { return func() { restarted = true }, nil })
	if err == nil || !restarted {
		t.Fatal("partial failure was hidden or left writers suspended")
	}
	var cfg Config
	b, _ := os.ReadFile(filepath.Join(dir, "config.json"))
	json.Unmarshal(b, &cfg)
	if cfg.AdminPassword != factoryWebPassword || cfg.FallbackPassword != "old-owner-secret" {
		t.Fatal("partial credential recovery cannot authenticate")
	}
	if _, err := os.Stat(filepath.Join(dir, "mining-reset.json")); err != nil {
		t.Fatal("partial reset lost journal")
	}
}

func TestOwnerResetJournalRequiresExplicitConfirmationAndRealReboot(t *testing.T) {
	for _, fault := range []bool{false, true} {
		c := &Controller{dir: t.TempDir(), halted: true, firmware: "2.2.1", cfg: Config{AdminPassword: "owner"}, sessions: map[string]Session{"old": {Expires: time.Now().Add(time.Hour)}}}
		reason := recoveryPause
		if fault {
			reason = "Original hardware fault"
		}
		original := []byte(`{"reason":"` + reason + `"}`)
		ownerWrite(filepath.Join(c.dir, "halted.json"), original, 0600)
		operations, reboots := 0, 0
		c.ownerReset = func() error {
			operations++
			return atomicJSON(filepath.Join(c.dir, "config.json"), Config{AdminPassword: factoryWebPassword})
		}
		c.ownerReboot = func() error { reboots++; return nil }
		if err := c.resetOwnerSettings(); err != nil {
			t.Fatal(err)
		}
		c.advanceMiningReset()
		if operations != 1 || len(c.sessions) != 0 || !c.halted {
			t.Fatal("owner state or sessions not reset")
		}
		r, _, _ := c.readPause()
		if fault && r.Reason != reason {
			t.Fatal("fault cleared")
		}
		if !fault && r.Reason != ownerResetPause {
			t.Fatal("benign personalization pause retained")
		}
		c.advanceMiningReset()
		c.advanceMiningReset()
		if reboots != 1 {
			t.Fatal("ambiguous reboot was repeated")
		}
		j, _ := readResetJournal(c.dir)
		if err := finishOwnerResetBoot(c.dir, j.BootID); err != nil {
			t.Fatal(err)
		}
		if pending, _ := c.miningResetState(); !pending {
			t.Fatal("completion claimed without reboot")
		}
		if err := finishOwnerResetBoot(c.dir, "new-boot"); err != nil {
			t.Fatal(err)
		}
		if pending, _ := c.miningResetState(); pending {
			t.Fatal("completed reset remains pending")
		}
	}
}

func TestOwnerResetNeverReplaysAmbiguousWipe(t *testing.T) {
	c := &Controller{dir: t.TempDir(), halted: true, firmware: "2.2.1"}
	atomicJSON(filepath.Join(c.dir, "halted.json"), map[string]string{"reason": recoveryPause})
	calls := 0
	c.ownerReset = func() error { calls++; return errors.New("interrupted") }
	c.ownerReboot = func() error { return nil }
	if err := c.resetOwnerSettings(); err != nil {
		t.Fatal(err)
	}
	c.advanceMiningReset()
	c.advanceMiningReset()
	if calls != 1 {
		t.Fatal("failed wipe repeated")
	}
	j, _ := readResetJournal(c.dir)
	if !j.OwnerReset || !j.Failed || j.Phase != "owner" {
		t.Fatal("factory reset failure lost its mode")
	}
	// A legacy mining-only journal must never silently upgrade into a personal-data wipe.
	atomicJSON(filepath.Join(c.dir, "mining-reset.json"), miningResetJournal{Version: 1})
	j, _ = readResetJournal(c.dir)
	if j.OwnerReset {
		t.Fatal("legacy reset upgraded into destructive reset")
	}
}

func TestFactoryResetHTTPIsConfirmedAndAsynchronous(t *testing.T) {
	c := &Controller{dir: t.TempDir(), halted: true, firmware: "2.2.1", sessions: map[string]Session{"owner": {Expires: time.Now().Add(time.Hour)}}}
	atomicJSON(filepath.Join(c.dir, "halted.json"), map[string]string{"reason": "Original fault"})
	calls := 0
	c.ownerReset = func() error { calls++; return nil }
	c.ownerReboot = func() error { return nil }
	r := httptest.NewRequest("POST", "http://miner/api/factory-reset", strings.NewReader(`{"confirm":"FACTORY RESET"}`))
	r.Header.Set("Content-Type", "application/json")
	r.AddCookie(&http.Cookie{Name: "kdbox-xbt", Value: "owner"})
	w := httptest.NewRecorder()
	c.handler(w, r)
	if w.Code != 200 || calls != 0 {
		t.Fatal("reset was rejected or wiped inside acknowledgement request", w.Code, w.Body.String())
	}
	j, err := readResetJournal(c.dir)
	if err != nil || !j.OwnerReset || j.Version != 2 {
		t.Fatal("owner intent was not recorded")
	}
}
