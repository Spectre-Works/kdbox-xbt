package main

import (
	"encoding/json"
	"errors"
	"io"
	"net"
	"os/exec"
	"strconv"
	"time"
)

// Wait for the signaled engines to exit (zombies cannot hash). Do not signal
// newly spawned PIDs: the stock supervisor may already be calibrating a restart.
func restartMiningEngine() error {
	return exec.Command("/bin/sh", "-c", `
pids=$(pidof intminer kdaminer)
for p in $pids; do
    state=$(awk '/^State:/ {print $2}' /proc/$p/status 2>/dev/null)
    case "$state" in ''|Z) continue;; esac
    case "$(readlink /proc/$p/exe)" in /usr/bin/intminer|/usr/bin/kdaminer) kill -TERM "$p";; *) exit 1;; esac
done
for tick in 1 2 3; do
    alive=''
    for p in $pids; do
        state=$(awk '/^State:/ {print $2}' /proc/$p/status 2>/dev/null)
        case "$state" in ''|Z) ;; *) alive="$alive $p";; esac
    done
    test -n "$alive" || exit 0
    sleep 1
done
for p in $alive; do
    case "$(readlink /proc/$p/exe)" in /usr/bin/intminer|/usr/bin/kdaminer) kill -KILL "$p";; *) exit 1;; esac
done
`).Run()
}

// This read-only local RPC provides the running clock, rather than saved settings.
func minerRPC(command string) (map[string]json.RawMessage, error) {
	return minerCommand(command, "")
}

func minerCommand(command, parameter string) (map[string]json.RawMessage, error) {
	c, e := net.DialTimeout("tcp", "127.0.0.1:4028", 2*time.Second)
	if e != nil {
		return nil, e
	}
	defer c.Close()
	_ = c.SetDeadline(time.Now().Add(2 * time.Second))
	request := map[string]string{"command": command}
	if parameter != "" {
		request["parameter"] = parameter
	}
	data, e := json.Marshal(request)
	if e != nil {
		return nil, e
	}
	if _, e = c.Write(data); e != nil {
		return nil, e
	}
	var v map[string]json.RawMessage
	// Stock engines can delimit a reply with NUL and leave the socket open.
	if e = json.NewDecoder(io.LimitReader(c, 128*1024)).Decode(&v); e != nil {
		return nil, e
	}
	return v, nil
}

// This clock-only setter is advertised by the pinned stock driver and requires
// privileged loopback access. Verify both setpoints before accepting its reply.
func setMinerClock(clock int) error {
	if (Tuning{430, clock}).automaticBounds() != nil {
		return errors.New("clock is outside the characterization bounds")
	}
	v, err := minerCommand("pgaset", "0,clock,"+strconv.Itoa(clock))
	if err != nil {
		return err
	}
	var status []struct {
		Status string `json:"STATUS"`
	}
	if json.Unmarshal(v["STATUS"], &status) != nil || len(status) != 1 || (status[0].Status != "S" && status[0].Status != "I") {
		return errors.New("stock driver rejected the clock adjustment")
	}
	live, err := liveMiner()
	if err != nil {
		return err
	}
	if number(live["clock"]) != float64(clock) || int(number(live["voltage"])*1000+0.5) != 430 {
		return errors.New("clock or fixed voltage readback did not match")
	}
	return nil
}

func liveMiner() (map[string]any, error) {
	v, e := minerRPC("devs")
	if e != nil {
		return nil, e
	}
	for _, key := range []string{"DEVS", "PGA"} {
		var a []map[string]any
		if json.Unmarshal(v[key], &a) == nil && len(a) == 1 {
			return a[0], nil
		}
	}
	return nil, errors.New("missing live miner telemetry")
}
