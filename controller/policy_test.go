package main

import (
	"encoding/json"
	"testing"
)

func TestPreviousFanSettingsDoNotControlCooling(t *testing.T) {
	var saved Tuning
	if err := json.Unmarshal([]byte(`{"millivolts":430,"clock":740,"fan":80,"mode":"curve"}`), &saved); err != nil {
		t.Fatal(err)
	}
	if saved.Validate(saved) != nil || saved.Powerplan() != "740 MHz 0.430 V 100 RPM 100 RPM" {
		t.Fatal("existing tuning record did not preserve clock and voltage with safe startup cooling", saved)
	}
}

func TestBounds(t *testing.T) {
	base := Tuning{430, 740}
	for _, v := range []Tuning{{430, 800}, {430, 850}, {430, 741}} {
		if v.Validate(base) == nil {
			t.Fatalf("accepted unsafe adjustment %#v", v)
		}
	}
	if (Tuning{430, 725}).Validate(base) != nil {
		t.Fatal("valid adjustment rejected")
	}
}
func TestRateReset(t *testing.T) {
	if _, ok := errorRate(20, 2000, 2, 200); ok {
		t.Fatal("reset misreported")
	}
	r, ok := errorRate(20, 2000, 22, 4000)
	if !ok || r != 0.1 {
		t.Fatal(r, ok)
	}
}

func TestVoltageBounds(t *testing.T) {
	base := Tuning{430, 740}
	for _, mv := range []int{0, 395, 400, 420, 426, 435, 450} {
		v := base
		v.Millivolts = mv
		if v.Validate(base) == nil {
			t.Fatalf("unsafe voltage step accepted: %d", mv)
		}
	}
	v := base
	v.Millivolts = 425
	if e := v.Validate(base); e != nil {
		t.Fatal(e)
	}
	if v.Powerplan() != "740 MHz 0.425 V 100 RPM 100 RPM" {
		t.Fatal(v.Powerplan())
	}
}
