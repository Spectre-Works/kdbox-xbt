package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Keep the first fault intact across controller restarts.
func saveStop(path, reason string, device map[string]any) (string, error) {
	if b, err := os.ReadFile(path); err == nil {
		var saved struct {
			Reason string `json:"reason"`
		}
		if json.Unmarshal(b, &saved) != nil || saved.Reason == "" {
			return reason, fmt.Errorf("existing stop record is unreadable; preserved for diagnosis")
		}
		return saved.Reason, nil
	} else if !os.IsNotExist(err) {
		return reason, err
	}
	err := atomicJSON(path, map[string]any{"reason": reason, "kind": pauseKind(reason), "at": time.Now().UTC().Format(time.RFC3339), "device": device})
	return reason, err
}

type diagnosticReport struct {
	Body      string `json:"body"`
	Note      string `json:"note,omitempty"`
	Truncated bool   `json:"truncated,omitempty"`
}

const (
	diagnosticTailSize = 64 << 10
	diagnosticReadSize = 1 << 20
	stockMiningLog     = "/backup/bfgminer/trash/minerlog"
)

func (c *Controller) diagnostics(ctx context.Context, w http.ResponseWriter, kind string, unlocked, halted bool, password string) {
	if kind == "safety-stop" {
		if !halted {
			jsonReply(w, http.StatusOK, diagnosticReport{Note: "No active mining pause or safety stop."})
			return
		}
		report, err := readStopReport(filepath.Join(c.dir, "halted.json"))
		if err != nil {
			jsonReply(w, http.StatusInternalServerError, map[string]string{"error": "The saved stop record could not be read. See the stop details above."})
			return
		}
		jsonReply(w, http.StatusOK, report)
		return
	}
	if !unlocked {
		jsonReply(w, http.StatusUnauthorized, map[string]string{"error": "Unlock with your miner password first"})
		return
	}
	switch kind {
	case "icinfo", "kmsg", "syslog", "psinfo", "meminfo", "vsinfo", "minerinfo", "minersyslog", "monitorlog", "minerhistory":
	default:
		jsonReply(w, http.StatusBadRequest, map[string]string{"error": "Unknown diagnostic report."})
		return
	}
	if halted && kind == "icinfo" {
		jsonReply(w, http.StatusServiceUnavailable, map[string]string{"error": "Live chip readings require a running mining engine. Select Safety stop or Mining log for recorded readings."})
		return
	}
	if !c.diagnosticMu.TryLock() {
		w.Header().Set("Retry-After", "2")
		jsonReply(w, http.StatusServiceUnavailable, map[string]string{"error": "Another diagnostic report is loading. Please retry shortly."})
		return
	}
	defer c.diagnosticMu.Unlock()
	// Large mining logs are read from their tail, never copied through stock JSON.
	if kind == "minersyslog" {
		if saved, err := readLogTail(stockMiningLog); err == nil && strings.TrimSpace(saved.Body) != "" {
			jsonReply(w, http.StatusOK, saved)
			return
		}
	}
	report, err := c.stockDiagnostic(ctx, kind, password)
	if kind == "minersyslog" && (err != nil || strings.TrimSpace(report.Body) == "") {
		if saved, readErr := readLogTail(stockMiningLog); readErr == nil && strings.TrimSpace(saved.Body) != "" {
			report, err = saved, nil
		}
	}
	if err != nil {
		jsonReply(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	jsonReply(w, http.StatusOK, report)
}

// Expose only the stop reason and selected telemetry from the private record.
func readStopReport(path string) (diagnosticReport, error) {
	f, err := os.Open(path)
	if err != nil {
		return diagnosticReport{}, err
	}
	defer f.Close()
	var saved struct {
		Reason string `json:"reason"`
		At     string `json:"at"`
		Device struct {
			Temperature string  `json:"temp"`
			Peak        float64 `json:"peak_temp"`
			FanSpeed    string  `json:"fanspeed"`
		} `json:"device"`
	}
	if err := json.NewDecoder(io.LimitReader(f, diagnosticTailSize)).Decode(&saved); err != nil {
		return diagnosticReport{}, err
	}
	if saved.Reason == "" {
		return diagnosticReport{}, errors.New("stop record has no reason")
	}
	var report strings.Builder
	if saved.At != "" {
		fmt.Fprintln(&report, "Recorded at", saved.At)
	}
	fmt.Fprintln(&report, saved.Reason)
	if saved.Device.Temperature != "" {
		fmt.Fprintln(&report, "Chip average / hashboard", saved.Device.Temperature)
	}
	if saved.Device.Peak > 0 {
		fmt.Fprintf(&report, "Hottest chip %.1f °C\n", saved.Device.Peak)
	}
	if saved.Device.FanSpeed != "" {
		fmt.Fprintln(&report, "Fans", saved.Device.FanSpeed)
	}
	return diagnosticReport{Body: report.String(), Note: "Showing the saved pause or safety-stop record."}, nil
}

func (c *Controller) stockDiagnostic(ctx context.Context, kind, password string) (diagnosticReport, error) {
	token, err := c.loginContext(ctx, password)
	if err != nil {
		return diagnosticReport{}, errors.New("Could not authenticate to the stock diagnostics service.")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.backend+"/dbg/"+kind, nil)
	if err != nil {
		return diagnosticReport{}, errors.New("Could not request the stock report.")
	}
	req.Header.Set("Authorization", "Bearer "+token)
	res, err := c.client.Do(req)
	if err != nil {
		return diagnosticReport{}, errors.New("The stock diagnostics service did not respond.")
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return diagnosticReport{}, fmt.Errorf("The stock diagnostics service returned HTTP %d for this report.", res.StatusCode)
	}
	var report diagnosticReport
	if err = json.NewDecoder(io.LimitReader(res.Body, diagnosticReadSize)).Decode(&report); err != nil {
		return diagnosticReport{}, errors.New("The stock report was unreadable or exceeded the 1 MiB read limit. Use original maintenance for larger reports.")
	}
	if kind == "icinfo" || kind == "minerhistory" {
		if len(report.Body) > diagnosticTailSize {
			return diagnosticReport{}, errors.New("The stock report exceeded the display limit.")
		}
		return report, nil
	}
	return diagnosticTail(report.Body), nil
}

func readLogTail(path string) (diagnosticReport, error) {
	f, err := os.Open(path)
	if err != nil {
		return diagnosticReport{}, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return diagnosticReport{}, err
	}
	if !info.Mode().IsRegular() {
		return diagnosticReport{}, errors.New("mining log is not a regular file")
	}
	offset := max(0, info.Size()-diagnosticTailSize-1)
	data, err := io.ReadAll(io.NewSectionReader(f, offset, diagnosticTailSize+1))
	if err != nil {
		return diagnosticReport{}, err
	}
	report := diagnosticTail(string(data))
	report.Note = "Showing the saved stock mining log."
	return report, nil
}

func diagnosticTail(body string) diagnosticReport {
	if len(body) <= diagnosticTailSize {
		return diagnosticReport{Body: body}
	}
	tail := body[len(body)-diagnosticTailSize:]
	if i := strings.IndexByte(tail, '\n'); i >= 0 && i+1 < len(tail) {
		tail = tail[i+1:]
	}
	return diagnosticReport{Body: strings.Clone(tail), Truncated: true}
}
