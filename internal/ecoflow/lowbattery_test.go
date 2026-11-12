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

func TestLowBatteryTrackerUsesBatteryPower(t *testing.T) {
	device := config.Device{LowBatteryPercent: 20, LowRuntimeSeconds: 300, LowRuntimeMaxPercent: 50, LowRuntimeGraceSeconds: 60}
	cases := []struct {
		name      string
		telemetry Telemetry
		wantLow   bool
	}{
		{"pack charging without AC", Telemetry{BatteryPower: intPtr(400), InputPresent: boolPtr(false)}, false},
		{"pack charging without AC state", Telemetry{BatteryPower: intPtr(400)}, false},
		{"pack resting without AC", Telemetry{BatteryPower: intPtr(0), InputPresent: boolPtr(false)}, false},
		{"pack discharging with AC", Telemetry{BatteryPower: intPtr(-100), InputPresent: boolPtr(true)}, true},
		{"solar charging from sums", Telemetry{InputPower: intPtr(500), OutputPower: intPtr(100), InputPresent: boolPtr(false)}, false},
		{"balanced DC sums", Telemetry{InputPower: intPtr(100), OutputPower: intPtr(100), InputPresent: boolPtr(false)}, false},
		{"solar below load", Telemetry{InputPower: intPtr(100), OutputPower: intPtr(500), InputPresent: boolPtr(false)}, true},
		{"battery output without input", Telemetry{InputPower: intPtr(0), OutputPower: intPtr(4), InputPresent: boolPtr(false)}, true},
		{"AC fallback without power readings", Telemetry{InputPresent: boolPtr(true)}, false},
		{"battery fallback without power readings", Telemetry{InputPresent: boolPtr(false)}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			telemetry := tc.telemetry
			telemetry.Charge = intPtr(40)
			telemetry.RuntimeSeconds = intPtr(120)
			var tracker lowBatteryTracker
			now := time.Unix(1_700_000_000, 0)
			if tracker.Low(device, telemetry, now) {
				t.Fatal("LB before grace elapsed")
			}
			if low := tracker.Low(device, telemetry, now.Add(time.Minute)); low != tc.wantLow {
				t.Errorf("LB after grace = %v, want %v", low, tc.wantLow)
			}
		})
	}
}

func TestLowBatteryTrackerRestartsGraceAfterCharging(t *testing.T) {
	device := config.Device{LowBatteryPercent: 20, LowRuntimeSeconds: 300, LowRuntimeMaxPercent: 50, LowRuntimeGraceSeconds: 60}
	for _, chargingCharge := range []int{40, 20} {
		var tracker lowBatteryTracker
		telemetry := Telemetry{Charge: intPtr(40), RuntimeSeconds: intPtr(120), BatteryPower: intPtr(-100), InputPresent: boolPtr(false)}
		now := time.Unix(1_700_000_000, 0)
		if tracker.Low(device, telemetry, now) {
			t.Fatal("LB before grace elapsed")
		}
		telemetry.BatteryPower = intPtr(400)
		telemetry.Charge = intPtr(chargingCharge)
		if low := tracker.Low(device, telemetry, now.Add(30*time.Second)); low != (chargingCharge <= device.LowBatteryPercent) {
			t.Errorf("charging at %d%%: LB=%v, want only the immediate charge threshold to apply", chargingCharge, low)
		}
		telemetry.BatteryPower = intPtr(-100)
		telemetry.Charge = intPtr(40)
		for _, seconds := range []int{70, 129} {
			if tracker.Low(device, telemetry, now.Add(time.Duration(seconds)*time.Second)) {
				t.Errorf("charging at %d%% did not reset grace: LB at second %d", chargingCharge, seconds)
			}
		}
		if !tracker.Low(device, telemetry, now.Add(130*time.Second)) {
			t.Error("no LB after a full grace period of discharging again")
		}
	}
}
