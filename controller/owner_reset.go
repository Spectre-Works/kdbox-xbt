package main

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const factoryWebPassword = "123456789"
const factorySSHPassword = "!@#$%123"

func currentBootID() string {
	b, _ := os.ReadFile("/proc/sys/kernel/random/boot_id")
	return strings.TrimSpace(string(b))
}

func finishOwnerResetBoot(dir, bootID string) error {
	j, err := readResetJournal(dir)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return nil
	} // Normal reset inspection handles malformed journals.
	if !j.OwnerReset || (j.Phase != "reboot-requested" && j.Phase != "reboot") || j.BootID == "" || bootID == "" || j.BootID == bootID {
		return nil
	}
	b, err := os.ReadFile(filepath.Join(dir, "config.json"))
	var cfg Config
	if err != nil || json.Unmarshal(b, &cfg) != nil || cfg.AdminPassword != factoryWebPassword || cfg.FallbackPassword != "" {
		return errors.New("factory reset credentials need inspection")
	}
	return os.Remove(filepath.Join(dir, "mining-reset.json"))
}

func resetRootPassword() error {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "/usr/bin/passwd", "root")
	cmd.Stdin = strings.NewReader(factorySSHPassword + "\n" + factorySSHPassword + "\n")
	// Do not return password-tool output or owner credential material.
	if err := cmd.Run(); err != nil {
		return errors.New("could not restore the factory SSH password")
	}
	return nil
}

// Suspend vendor writers before replacing owner state. Restart them afterward
// so cached credentials/settings cannot overwrite the reset files. Their init
// respawn entries re-read the defaults; mining remains held independently.
func holdOwnerServices() (func(), error) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil, err
	}
	var pids []int
	for _, entry := range entries {
		pid, e := strconv.Atoi(entry.Name())
		if e != nil {
			continue
		}
		target, e := os.Readlink("/proc/" + entry.Name() + "/exe")
		if e != nil {
			continue
		}
		if target == "/usr/bin/minerd" || target == "/usr/bin/wifi" {
			pids = append(pids, pid)
		}
	}
	var held []int
	for _, pid := range pids {
		target, _ := os.Readlink(fmt.Sprintf("/proc/%d/exe", pid))
		if target != "/usr/bin/minerd" && target != "/usr/bin/wifi" {
			continue
		}
		if e := syscall.Kill(pid, syscall.SIGSTOP); e != nil && e != syscall.ESRCH {
			for _, old := range held {
				_ = syscall.Kill(old, syscall.SIGCONT)
			}
			return nil, e
		}
		held = append(held, pid)
	}
	return func() {
		for _, pid := range pids {
			target, _ := os.Readlink(fmt.Sprintf("/proc/%d/exe", pid))
			if target == "/usr/bin/minerd" || target == "/usr/bin/wifi" {
				_ = syscall.Kill(pid, syscall.SIGKILL)
			}
		}
	}, nil
}

func ownerPath(root, path string) string { return filepath.Join(root, strings.TrimPrefix(path, "/")) }
func ownerRead(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > 64<<10 {
		return nil, errors.New("unexpected factory template")
	}
	return io.ReadAll(io.LimitReader(f, 64<<10))
}
func ownerWrite(path string, data []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	temp := path + ".factory-new"
	f, err := os.OpenFile(temp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	if _, err = f.Write(data); err == nil {
		err = f.Chmod(mode)
	}
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(temp, path)
}

// The only caller supplies constant device paths. Tests supply an isolated root;
// this function never follows paths or erase lists received from an HTTP client.
func resetOwnerFiles(root, dir, oldPassword string, setPassword func() error, hold func() (func(), error)) error {
	settings := ownerPath(root, "/root/settings")
	entries, err := os.ReadDir(settings)
	if err != nil {
		return err
	}
	defaults := map[string][]byte{}
	for _, entry := range entries {
		if !entry.Type().IsRegular() {
			return errors.New("unexpected factory template type")
		}
		b, e := ownerRead(filepath.Join(settings, entry.Name()))
		if e != nil {
			return e
		}
		if len(b) > 64<<10 {
			return errors.New("factory template exceeded limit")
		}
		defaults[entry.Name()] = b
	}
	var login map[string]any
	if json.Unmarshal(defaults["login.json"], &login) != nil || login["username"] != "admin" || login["password"] != factoryWebPassword {
		return errors.New("factory dashboard credential template is not the pinned default")
	}
	var board map[string]any
	if json.Unmarshal(defaults["cpbsetting.json"], &board) != nil {
		return errors.New("factory board template is missing")
	}
	board["name"] = ""
	board["manualPowerplan"] = defaultTuning.Powerplan()
	board["powerplans"] = []any{map[string]any{"level": 0, "info": defaultTuning.Powerplan()}}
	board["select"] = 0
	board["manual"] = false
	board["tempcontrol"] = true
	board["ledcontrol"] = false
	defaults["cpbsetting.json"], _ = json.Marshal(board)
	defaults["wifi.json"] = []byte(`{"enable":false,"mode":"sta","version":"v1.0"}`)
	defaults["cmos.json"] = []byte(`{"enable":false,"userkey":null,"version":"v1.0"}`)
	defaults["algosetting.json"] = []byte(`{"algo_select":1,"algos":[{"id":0,"name":"blake2s(KDA)"},{"id":1,"name":"blake2b(SC)"}]}`)
	defaults["pools.json"] = []byte("[]\n")
	// Rotate native RS256 keys as well as invalidating dashboard sessions.
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return err
	}
	defaults["key.pem"] = pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	pub, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		return err
	}
	defaults["pubkey.pem"] = pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pub})
	wifi := map[string][]byte{}
	for _, name := range []string{"hostapd.conf", "dnsmasq.conf"} {
		b, e := ownerRead(ownerPath(root, "/root/wifi/"+name))
		if e != nil {
			return e
		}
		if len(b) > 64<<10 {
			return errors.New("factory Wi-Fi template exceeded limit")
		}
		wifi[name] = b
	}
	wifi["wpa_supplicant.conf"] = []byte("ctrl_interface=/var/run/wpa_supplicant\nupdate_config=1\n")
	restart, err := hold()
	if err != nil {
		return err
	}
	defer restart()
	// A partial reset can authenticate against either the old or default login
	// after a restart. The fallback is removed before completion is checkpointed.
	cfg := Config{AdminPassword: factoryWebPassword, ChipTarget: chipTemperatureTarget}
	if oldPassword != factoryWebPassword {
		cfg.FallbackPassword = oldPassword
	}
	if err = atomicJSON(filepath.Join(dir, "config.json"), cfg); err != nil {
		return err
	}
	// Stock minerd reinstalls key.pem from /root/settings when it starts.
	// Rotate the source templates too, or a reboot resurrects old RS256 sessions.
	for _, name := range []string{"key.pem", "pubkey.pem"} {
		if err = ownerWrite(filepath.Join(settings, name), defaults[name], 0600); err != nil {
			return err
		}
	}
	usr := ownerPath(root, "/usr/config")
	children, err := os.ReadDir(usr)
	if err != nil {
		return err
	}
	for _, entry := range children {
		if filepath.Join(usr, entry.Name()) != filepath.Clean(dir) {
			if err = os.RemoveAll(filepath.Join(usr, entry.Name())); err != nil {
				return err
			}
		}
	}
	for name, data := range defaults {
		if err = ownerWrite(filepath.Join(usr, "minerd/settings", name), data, 0600); err != nil {
			return err
		}
		if strings.HasSuffix(name, ".json") {
			if err = ownerWrite(filepath.Join(usr, "minerd/settings", "."+name), data, 0600); err != nil {
				return err
			}
		}
	}
	for name, data := range wifi {
		if err = ownerWrite(filepath.Join(usr, "wifi", name), data, 0600); err != nil {
			return err
		}
	}
	for _, name := range []string{"bfgminer", "miner_history"} {
		if err = os.MkdirAll(filepath.Join(usr, name), 0700); err != nil {
			return err
		}
	}
	interfaces := []byte("auto lo\niface lo inet loopback\n\nauto eth0\niface eth0 inet dhcp\n")
	if err = ownerWrite(ownerPath(root, "/etc/network/interfaces"), interfaces, 0644); err != nil {
		return err
	}
	if err = ownerWrite(ownerPath(root, "/etc/hostname"), []byte("goldshell-kdbox\n"), 0644); err != nil {
		return err
	}
	if err = ownerWrite(ownerPath(root, "/etc/hosts"), []byte("127.0.0.1 localhost\n::1 localhost ip6-localhost ip6-loopback\n"), 0644); err != nil {
		return err
	}
	resolver := ownerPath(root, "/etc/resolv.conf")
	if info, e := os.Lstat(resolver); e == nil && info.Mode()&os.ModeSymlink != 0 {
		link, e := os.Readlink(resolver)
		if e != nil {
			return e
		}
		if link != "../tmp/resolv.conf" && link != "/tmp/resolv.conf" {
			return errors.New("unexpected resolver link; reset needs inspection")
		}
		resolver = ownerPath(root, "/tmp/resolv.conf")
	}
	if err = ownerWrite(resolver, nil, 0644); err != nil {
		return err
	}
	for _, path := range []string{"/root/.ssh", "/root/.ash_history", "/root/.bash_history", "/backup/minerd/trash", "/backup/bfgminer/trash", "/tmp/kdbox-xbt-history.json", "/tmp/kdbox-xbt-controller.log", "/tmp/kdbox-xbt-preflight.log", "/tmp/syslog", "/var/log/messages", "/var/lib/dhcp"} {
		if err = os.RemoveAll(ownerPath(root, path)); err != nil {
			return err
		}
	}
	keep := map[string]bool{"controller": true, "boot.sh": true, "launcher.sh": true, "supervisor.sh": true, "halted.json": true, "mining-reset.json": true, "config.json": true, "approved.json": true, "algorithm.json": true}
	runtime, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, entry := range runtime {
		if !keep[entry.Name()] {
			if err = os.RemoveAll(filepath.Join(dir, entry.Name())); err != nil {
				return err
			}
		}
	}
	if err = setPassword(); err != nil {
		return err
	}
	cfg.FallbackPassword = ""
	if err = atomicJSON(filepath.Join(dir, "config.json"), cfg); err != nil {
		return err
	}
	if err = atomicJSON(filepath.Join(dir, "approved.json"), defaultTuning); err != nil {
		return err
	}
	return atomicJSON(filepath.Join(dir, "algorithm.json"), defaultAlgorithm)
}

func performOwnerReset(c *Controller) error {
	if err := preflightDevice("/root"); err != nil {
		return err
	}
	version, err := os.ReadFile("/root/version")
	if err != nil || strings.TrimSpace(string(version)) != "2.2.1" {
		return errors.New("factory reset requires stock 2.2.1")
	}
	var profile struct {
		Base struct {
			SHA string `json:"minerd_sha256"`
		} `json:"install_base"`
	}
	if json.Unmarshal(compatibilityJSON, &profile) != nil || profile.Base.SHA == "" {
		return errors.New("missing pinned factory reset profile")
	}
	f, err := os.Open("/usr/bin/minerd")
	if err != nil {
		return err
	}
	hash := sha256.New()
	_, err = io.Copy(hash, f)
	f.Close()
	if err != nil || fmt.Sprintf("%x", hash.Sum(nil)) != profile.Base.SHA {
		return errors.New("native firmware changed; factory reset needs inspection")
	}
	old := c.cfg.AdminPassword
	if old == factoryWebPassword && c.cfg.FallbackPassword != "" {
		old = c.cfg.FallbackPassword
	}
	return resetOwnerFiles("/", c.dir, old, resetRootPassword, holdOwnerServices)
}
