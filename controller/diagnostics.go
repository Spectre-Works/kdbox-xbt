package main

import (
	"encoding/json"
	"fmt"
	"os"
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
	err := atomicJSON(path, map[string]any{"reason": reason, "at": time.Now().UTC().Format(time.RFC3339), "device": device})
	return reason, err
}
