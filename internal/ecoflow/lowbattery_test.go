package ecoflow

import (
	"testing"
	"time"

	"github.com/xkhronoz/ecoflow-ble-nutd/internal/config"
)

func TestLowBatteryTracker(t *testing.T) {
	device := config.Device{LowBatteryPercent: 20, LowRuntimeSeconds: 300, LowRuntimeMaxPercent: 50, LowRuntimeGraceSeconds: 60}
	at := func(s int) time.Time { return time.Unix(1_700_000_000+int64(s), 0) }
	snap := func(charge, runtime int, input bool) Telemetry {
		return Telemetry{Charge: intPtr(charge), RuntimeSeconds: intPtr(runtime), InputPresent: boolPtr(input)}
	}

	var l lowBatteryTracker
	// Charging at 93 % with "2 min left": the stale estimate, not low battery.
	if l.Low(device, snap(93, 120, true), at(0)) {
		t.Fatal("charging with a short runtime estimate counted as LB")
	}
	// The first seconds on battery still carry it: still no.
	if l.Low(device, snap(93, 120, false), at(1)) {
		t.Fatal("a short runtime estimate at 93 % counted as LB")
	}
	// Discharging at 40 % with 4 min left: low, but only after the grace.
	for s := 10; s < 69; s += 5 {
		if l.Low(device, snap(40, 240, false), at(s)) {
			t.Fatalf("LB at %d s into a low runtime, before the %d s grace", s-10, device.LowRuntimeGraceSeconds)
		}
	}
	if !l.Low(device, snap(40, 240, false), at(70)) {
		t.Fatal("no LB after the grace")
	}
	// A better estimate ends it and restarts the count.
	if l.Low(device, snap(40, 900, false), at(75)) {
		t.Fatal("LB kept after the runtime recovered")
	}
	if l.Low(device, snap(40, 240, false), at(80)) {
		t.Fatal("LB right after the runtime dropped again: the count did not restart")
	}
	// The charge threshold is immediate, whatever the runtime or the input.
	if !l.Low(device, snap(20, 36000, true), at(81)) {
		t.Fatal("no LB at the charge threshold")
	}
}
