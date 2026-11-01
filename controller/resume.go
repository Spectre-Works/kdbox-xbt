package main

import "encoding/json"

// Some native firmware does not restart a stopped engine after applying tuning alone.
func (c *Controller) reapplyAlgorithm() error {
	b, err := c.native("GET", "/mcb/algosetting", nil, false)
	if err != nil {
		return err
	}
	_, err = c.native("PUT", "/mcb/algosetting", json.RawMessage(b), true)
	return err
}
