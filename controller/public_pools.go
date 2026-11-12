package main

import (
	"encoding/json"
	"net/url"
)

// Allowlist display fields; never publish passwords or URL credentials.
func publicPools(body []byte) ([]map[string]any, error) {
	var raw []map[string]any
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, err
	}
	result := make([]map[string]any, 0, len(raw))
	for _, p := range raw {
		row := map[string]any{}
		for _, key := range []string{"user", "pool-priority", "mining", "connected", "telemetry_available", "accepted"} {
			row[key] = p[key]
		}
		if address, ok := p["url"].(string); ok {
			if u, err := url.Parse(address); err == nil {
				u.User = nil
				u.RawQuery = ""
				u.Fragment = ""
				row["url"] = u.String()
			}
		}
		result = append(result, row)
	}
	return result, nil
}
