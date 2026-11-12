package main

import "testing"

func TestOrigins(t *testing.T) {
	for _, v := range []struct {
		o  string
		ok bool
	}{{"http://192.168.1.2", true}, {"http://192.168.1.2:8088", true}, {"http://evil.example", false}, {"http://192.168.1.2:9000", false}, {"http://192.168.1.2/path", false}, {"http://user@192.168.1.2", false}} {
		if allowedOrigin(v.o, "192.168.1.2:8088") != v.ok {
			t.Fatal(v)
		}
	}
}
func TestNetworkValidation(t *testing.T) {
	if (NetworkSettings{DHCP: true, Auto: true}).validate() != nil {
		t.Fatal("DHCP rejected")
	}
	for _, v := range []NetworkSettings{{DHCP: false, Auto: true}, {DHCP: true, Auto: false}, {DHCP: false, Auto: true, IP: map[string]string{"IPv4addr": "192.168.1.4", "Subnetmask": "255.0.255.0", "Router": "192.168.1.1"}}} {
		if v.validate() == nil {
			t.Fatal("invalid configuration accepted")
		}
	}
}

func TestPoolValidation(t *testing.T) {
	for _, address := range []string{"stratum+tcp://pool.example:3333", "stratum+ssl://pool.example:443"} {
		if normalized, err := normalizePool(address, "address.worker"); err != nil || normalized != address {
			t.Fatal(address)
		}
	}
	for _, address := range []string{"http://pool.example:3333", "stratum+tcp://pool.example", "stratum+tcp://pool.example:99999", "stratum+tcp://user@pool.example:3333"} {
		if _, err := normalizePool(address, "address.worker"); err == nil {
			t.Fatal(address)
		}
	}
	if _, err := normalizePool("stratum+tcp://pool.example:3333", " "); err == nil {
		t.Fatal("empty worker accepted")
	}
	for _, address := range []string{"pool.example:3333", "tcp://pool.example:3333", " stratum+tcp://pool.example:3333 "} {
		if normalized, err := normalizePool(address, "address.worker"); err != nil || normalized != "stratum+tcp://pool.example:3333" {
			t.Fatalf("normalize %q: %q, %v", address, normalized, err)
		}
	}
	if normalized, err := normalizePool("[2001:db8::1]:3333", "address.worker"); err != nil || normalized != "stratum+tcp://[2001:db8::1]:3333" {
		t.Fatalf("IPv6 normalization: %q, %v", normalized, err)
	}
}

func TestPoolTelemetryMatchesEquivalentAddresses(t *testing.T) {
	pools := []map[string]any{
		{"url": "stratum+tcp://POOL.example:3333", "user": "primary"},
		{"url": "stratum+tcp://pool.example:3333", "user": "backup"},
	}
	rows := []map[string]any{
		{"URL": "tcp://pool.example:3333", "User": "primary", "Stratum Active": true, "Status": "Alive", "Accepted": 4},
	}
	addPoolTelemetry(pools, rows)
	if pools[0]["telemetry_available"] != true || pools[0]["mining"] != true || pools[0]["accepted"] != 4 {
		t.Fatal("primary pool telemetry did not match")
	}
	if pools[1]["telemetry_available"] != false {
		t.Fatal("telemetry must not be reported for an unmatched worker")
	}
}
