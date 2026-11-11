package ecoflow

import (
	"testing"

	"github.com/xkhronoz/ecoflow-ble-nutd/internal/config"
	"github.com/xkhronoz/ecoflow-ble-nutd/internal/ecoflow/pb/mr521pb"
	"github.com/xkhronoz/ecoflow-ble-nutd/internal/ecoflow/pb/pd335pb"
	"github.com/xkhronoz/ecoflow-ble-nutd/internal/ecoflow/pb/pr705pb"
	"google.golang.org/protobuf/proto"
)

func TestExtendedVarsFromDelta3Display(t *testing.T) {
	msg := &pd335pb.DisplayPropertyUpload{
		CmsBattSoc:               proto.Float32(87.4),
		CmsDsgRemTime:            proto.Uint32(5400),
		PowInSumW:                proto.Float32(210),
		PowOutSumW:               proto.Float32(126.6),
		PlugInInfoAcChargerFlag:  proto.Bool(true),
		PowGetBms:                proto.Float32(80.2),
		PlugInInfoAcInFeq:        proto.Uint32(50),
		PlugInInfoAcOutDsgPowMax: proto.Uint32(2000),
		PowGetAcIn:               proto.Float32(210),
		PowGetAcOut:              proto.Float32(-100.4),
		FlowInfoAcOut:            proto.Uint32(2),
		PowGetTypec1:             proto.Float32(-26.2),
		FlowInfoTypec1:           proto.Uint32(2),
		FlowInfoQcusb1:           proto.Uint32(0),
		BmsMaxCellTemp:           proto.Int32(31),
		BmsMaxMosTemp:            proto.Int32(38),
		BmsDesignCap:             proto.Uint32(40000),
		CmsMinDsgSoc:             proto.Uint32(20),
	}
	telemetry := telemetryFromProto(msg)
	if telemetry.BatteryPower == nil || *telemetry.BatteryPower != 80 {
		t.Fatalf("battery power = %v, want 80", telemetry.BatteryPower)
	}
	want := map[string]string{
		"input.frequency":          "50",
		"input.realpower":          "210",
		"ups.realpower":            "127",
		"ups.realpower.nominal":    "2000",
		"battery.temperature":      "31",
		"battery.capacity.nominal": "40.0",
		"battery.charge.low":       "20",
		"outlet.1.desc":            "AC 1",
		"outlet.1.realpower":       "100",
		"outlet.1.status":          "on",
		"outlet.3.desc":            "USB-C 1",
		"outlet.3.realpower":       "26",
		"outlet.3.status":          "on",
		"outlet.6.desc":            "USB-A 1",
		"outlet.6.status":          "off",
	}
	for name, value := range want {
		if got := telemetry.Vars[name]; got != value {
			t.Errorf("%s = %q, want %q", name, got, value)
		}
	}
	for _, absent := range []string{"input.voltage", "outlet.2.desc", "outlet.6.realpower", "outlet.8.desc", "outlet.9.desc"} {
		if got, ok := telemetry.Vars[absent]; ok {
			t.Errorf("%s = %q, want absent", absent, got)
		}
	}

	vars := liveVars(config.Device{Name: "ups", LowBatteryPercent: 20, LowRuntimeSeconds: 300}, DeviceDescriptor{}, telemetry, false)
	if vars["ups.status"] != "OL CHRG" {
		t.Errorf("ups.status = %q, want %q", vars["ups.status"], "OL CHRG")
	}
	if vars["battery.charger.status"] != "charging" {
		t.Errorf("battery.charger.status = %q", vars["battery.charger.status"])
	}
	if vars["ups.realpower.nominal"] != "2000" || vars["battery.charge"] != "87" || vars["output.power"] != "127" {
		t.Errorf("core/extended mix wrong: nominal=%q charge=%q output.power=%q", vars["ups.realpower.nominal"], vars["battery.charge"], vars["output.power"])
	}
	if vars["ups.load"] != "6" || vars["ups.temperature"] != "38" {
		t.Errorf("derived: load=%q temperature=%q, want 6 and 38", vars["ups.load"], vars["ups.temperature"])
	}
}

func TestExtendedVarsMergeKeepsEarlierKeys(t *testing.T) {
	first := telemetryFromProto(&pd335pb.DisplayPropertyUpload{
		PlugInInfoAcInFeq: proto.Uint32(50),
		PowGetTypec1:      proto.Float32(-10),
	})
	second := telemetryFromProto(&pd335pb.DisplayPropertyUpload{
		PowGetTypec1: proto.Float32(-12),
	})
	merged := first.Merge(second)
	if merged.Vars["input.frequency"] != "50" {
		t.Errorf("input.frequency lost on merge: %q", merged.Vars["input.frequency"])
	}
	if merged.Vars["outlet.3.realpower"] != "12" {
		t.Errorf("outlet.3.realpower = %q, want 12", merged.Vars["outlet.3.realpower"])
	}
	if first.Vars["outlet.3.realpower"] != "10" {
		t.Errorf("merge mutated the receiver: %q", first.Vars["outlet.3.realpower"])
	}
	if telemetry := telemetryFromProto(&pd335pb.DisplayPropertyUpload{}); !telemetry.Empty() {
		t.Errorf("an empty message should decode to empty telemetry: %+v", telemetry)
	}
}

func TestExtendedVarsFromDelta3Runtime(t *testing.T) {
	telemetry := telemetryFromProto(&pd335pb.RuntimePropertyUpload{
		PlugInInfoAcInVol:   proto.Float32(231.4),
		PlugInInfoAcInAmp:   proto.Float32(0.93),
		PlugInInfoAcOutVol:  proto.Float32(229.9),
		PlugInInfoAcOutAmp:  proto.Float32(0.55),
		PlugInInfoAcOutFreq: proto.Uint32(50),
		BmsBattVol:          proto.Float32(52.17),
		BmsBattAmp:          proto.Float32(-1.5),
		TempPcsAc:           proto.Float32(41.3),
		BmsFullCap:          proto.Uint32(39500),
		PlugInInfo_12VVol:   proto.Float32(13.1),
		PlugInInfo_12VAmp:   proto.Float32(0.2),
	})
	want := map[string]string{
		"input.voltage":    "231.4",
		"input.current":    "0.93",
		"output.voltage":   "229.9",
		"output.current":   "0.55",
		"output.frequency": "50",
		"battery.voltage":  "52.17",
		"battery.current":  "-1.50",
		"battery.capacity": "39.5",
		"outlet.8.voltage": "13.1",
		"outlet.8.current": "0.20",
	}
	for name, value := range want {
		if got := telemetry.Vars[name]; got != value {
			t.Errorf("%s = %q, want %q", name, got, value)
		}
	}
	if telemetry.Charge != nil || telemetry.InputPower != nil {
		t.Errorf("runtime message should not invent core readings: %+v", telemetry)
	}
	if telemetry.InverterTemperature == nil || *telemetry.InverterTemperature != 41 {
		t.Errorf("inverter temperature = %v, want 41", telemetry.InverterTemperature)
	}
}

// The load is a ratio of two readings that may arrive in different
// messages: it follows whichever came last, after the merge.
func TestLoadFollowsPartialUpdates(t *testing.T) {
	device := config.Device{Name: "ups"}
	merged := telemetryFromProto(&pd335pb.DisplayPropertyUpload{
		PowOutSumW:               proto.Float32(600),
		PlugInInfoAcOutDsgPowMax: proto.Uint32(2400),
	})
	if load := liveVars(device, DeviceDescriptor{}, merged, false)["ups.load"]; load != "25" {
		t.Fatalf("load = %q, want 25", load)
	}
	// Output alone: the rating is remembered.
	merged = merged.Merge(telemetryFromProto(&pd335pb.DisplayPropertyUpload{PowOutSumW: proto.Float32(1200)}))
	if load := liveVars(device, DeviceDescriptor{}, merged, false)["ups.load"]; load != "50" {
		t.Errorf("load after an output-only update = %q, want 50", load)
	}
	// Rating alone: the output is remembered.
	merged = merged.Merge(telemetryFromProto(&pd335pb.DisplayPropertyUpload{PlugInInfoAcOutDsgPowMax: proto.Uint32(3000)}))
	if load := liveVars(device, DeviceDescriptor{}, merged, false)["ups.load"]; load != "40" {
		t.Errorf("load after a rating-only update = %q, want 40", load)
	}
	// No rating from the device at all: the config's, still after the merge.
	merged = telemetryFromProto(&pd335pb.DisplayPropertyUpload{PlugInInfoAcInFeq: proto.Uint32(50)})
	merged = merged.Merge(telemetryFromProto(&pd335pb.DisplayPropertyUpload{PowOutSumW: proto.Float32(900)}))
	vars := liveVars(config.Device{Name: "ups", RatedPowerWatts: 3000}, DeviceDescriptor{}, merged, false)
	if vars["ups.load"] != "30" || vars["ups.realpower.nominal"] != "3000" {
		t.Errorf("config rating: load=%q nominal=%q, want 30 and 3000", vars["ups.load"], vars["ups.realpower.nominal"])
	}
}

// The inverter's temperature comes in the rarer runtime message; the
// display message that follows every second must not replace it with the
// MOSFET fallback.
func TestUPSTemperaturePrefersInverterAfterMerge(t *testing.T) {
	device := config.Device{Name: "ups"}
	merged := telemetryFromProto(&pd335pb.DisplayPropertyUpload{BmsMaxMosTemp: proto.Int32(38)})
	if got := liveVars(device, DeviceDescriptor{}, merged, false)["ups.temperature"]; got != "38" {
		t.Fatalf("ups.temperature from the display message = %q, want 38", got)
	}
	merged = merged.Merge(telemetryFromProto(&pd335pb.RuntimePropertyUpload{TempPcsAc: proto.Float32(41.3)}))
	if got := liveVars(device, DeviceDescriptor{}, merged, false)["ups.temperature"]; got != "41" {
		t.Fatalf("ups.temperature after the runtime message = %q, want 41", got)
	}
	merged = merged.Merge(telemetryFromProto(&pd335pb.DisplayPropertyUpload{BmsMaxMosTemp: proto.Int32(39)}))
	if got := liveVars(device, DeviceDescriptor{}, merged, false)["ups.temperature"]; got != "41" {
		t.Errorf("ups.temperature after the next display message = %q, want 41 kept", got)
	}
	if merged.MOSFETTemperature == nil || *merged.MOSFETTemperature != 39 {
		t.Errorf("the MOSFET reading should still be tracked: %v", merged.MOSFETTemperature)
	}
}

func TestRatedPowerFromConfigWhenDeviceReportsNone(t *testing.T) {
	telemetry := telemetryFromProto(&pd335pb.DisplayPropertyUpload{
		CmsBattSoc: proto.Float32(50),
		PowOutSumW: proto.Float32(600),
	})
	vars := liveVars(config.Device{Name: "ups", RatedPowerWatts: 3000}, DeviceDescriptor{}, telemetry, false)
	if vars["ups.realpower.nominal"] != "3000" || vars["ups.load"] != "20" {
		t.Errorf("nominal=%q load=%q", vars["ups.realpower.nominal"], vars["ups.load"])
	}
	// A rating the device does report wins over the config.
	telemetry = telemetryFromProto(&pd335pb.DisplayPropertyUpload{
		PowOutSumW:               proto.Float32(600),
		PlugInInfoAcOutDsgPowMax: proto.Uint32(2400),
	})
	vars = liveVars(config.Device{Name: "ups", RatedPowerWatts: 3000}, DeviceDescriptor{}, telemetry, false)
	if vars["ups.realpower.nominal"] != "2400" || vars["ups.load"] != "25" {
		t.Errorf("device rating should win: nominal=%q load=%q", vars["ups.realpower.nominal"], vars["ups.load"])
	}
}

func TestAcGroupsFromList(t *testing.T) {
	telemetry := telemetryFromProto(&pd335pb.DisplayPropertyUpload{
		PowGetAcOut:     proto.Float32(-150),
		PowGetAcOutList: &pd335pb.PowGetAcOutList{PowGetAcOutItem: []float32{-100.4, 0, -49.6, 0, 0}},
		FlowInfoAcOut:   proto.Uint32(2),
		FlowInfoAc2Out:  proto.Uint32(0),
	})
	want := map[string]string{
		"outlet.1.realpower": "100", "outlet.1.status": "on", "outlet.1.desc": "AC 1",
		"outlet.2.realpower": "50", "outlet.2.status": "off", "outlet.2.desc": "AC 2",
	}
	for name, value := range want {
		if got := telemetry.Vars[name]; got != value {
			t.Errorf("%s = %q, want %q", name, got, value)
		}
	}
	// Without the list, group 1 falls back to the total and group 2 has no watts.
	telemetry = telemetryFromProto(&pd335pb.DisplayPropertyUpload{PowGetAcOut: proto.Float32(-150)})
	if telemetry.Vars["outlet.1.realpower"] != "150" {
		t.Errorf("outlet.1.realpower = %q, want 150", telemetry.Vars["outlet.1.realpower"])
	}
	if _, ok := telemetry.Vars["outlet.2.realpower"]; ok {
		t.Error("outlet.2.realpower should be absent without the list")
	}
}

func TestTwelveVoltOutletsFromList(t *testing.T) {
	telemetry := telemetryFromProto(&pd335pb.DisplayPropertyUpload{
		PowGet_12V:     proto.Float32(-30),
		PowGet_12VList: &pd335pb.PowGet12VList{PowGet_12VItem: []float32{-20, -10}},
		FlowInfo_12V:   proto.Uint32(2),
	})
	want := map[string]string{
		"outlet.8.desc": "12 V car", "outlet.8.realpower": "20", "outlet.8.status": "on",
		"outlet.9.desc": "12 V Anderson", "outlet.9.realpower": "10", "outlet.9.status": "on",
	}
	for name, value := range want {
		if got := telemetry.Vars[name]; got != value {
			t.Errorf("%s = %q, want %q", name, got, value)
		}
	}
	// One socket behind the switch: no list, or a list with one slot. The
	// Anderson port must not appear from the shared flow flag alone.
	for _, msg := range []*pd335pb.DisplayPropertyUpload{
		{PowGet_12V: proto.Float32(-30), FlowInfo_12V: proto.Uint32(2)},
		{PowGet_12V: proto.Float32(-30), FlowInfo_12V: proto.Uint32(2), PowGet_12VList: &pd335pb.PowGet12VList{PowGet_12VItem: []float32{-30}}},
	} {
		telemetry := telemetryFromProto(msg)
		if telemetry.Vars["outlet.8.desc"] != "12 V car" || telemetry.Vars["outlet.8.realpower"] != "30" || telemetry.Vars["outlet.8.status"] != "on" {
			t.Errorf("outlet 8 wrong without a second slot: %v", telemetry.Vars)
		}
		for _, name := range []string{"outlet.9.desc", "outlet.9.realpower", "outlet.9.status"} {
			if got, ok := telemetry.Vars[name]; ok {
				t.Errorf("%s = %q, want absent without a second 12 V slot", name, got)
			}
		}
	}
}

// A RIVER 3: one AC group, two USB-C, two USB-A, one 12 V socket behind the
// switch, no per-group lists. Nothing beyond that may appear.
func TestExtendedVarsFromRiver3Display(t *testing.T) {
	telemetry := telemetryFromProto(&pr705pb.DisplayPropertyUpload{
		CmsBattSoc:               proto.Float32(64),
		PowInSumW:                proto.Float32(0),
		PowOutSumW:               proto.Float32(85.5),
		PlugInInfoAcChargerFlag:  proto.Bool(false),
		PowGetBms:                proto.Float32(-90),
		PlugInInfoAcOutDsgPowMax: proto.Uint32(300),
		PowGetAcOut:              proto.Float32(-60.2),
		FlowInfoAcOut:            proto.Uint32(2),
		PowGetTypec1:             proto.Float32(-18),
		FlowInfoTypec1:           proto.Uint32(2),
		FlowInfoTypec2:           proto.Uint32(0),
		PowGetQcusb1:             proto.Float32(-7.3),
		FlowInfoQcusb1:           proto.Uint32(2),
		PowGet_12V:               proto.Float32(0),
		FlowInfo_12V:             proto.Uint32(0),
		BmsMaxCellTemp:           proto.Int32(27),
		BmsMaxMosTemp:            proto.Int32(33),
		BmsDesignCap:             proto.Uint32(20000),
		CmsMinDsgSoc:             proto.Uint32(0),
	})
	want := map[string]string{
		"ups.realpower":            "86",
		"ups.realpower.nominal":    "300",
		"battery.temperature":      "27",
		"battery.capacity.nominal": "20.0",
		"battery.charge.low":       "0",
		"outlet.1.desc":            "AC 1",
		"outlet.1.realpower":       "60",
		"outlet.1.status":          "on",
		"outlet.3.desc":            "USB-C 1",
		"outlet.3.realpower":       "18",
		"outlet.4.desc":            "USB-C 2",
		"outlet.4.status":          "off",
		"outlet.6.desc":            "USB-A 1",
		"outlet.6.realpower":       "7",
		"outlet.8.desc":            "12 V car",
		"outlet.8.realpower":       "0",
		"outlet.8.status":          "off",
	}
	for name, value := range want {
		if got := telemetry.Vars[name]; got != value {
			t.Errorf("%s = %q, want %q", name, got, value)
		}
	}
	for _, absent := range []string{"outlet.2.desc", "outlet.5.desc", "outlet.9.desc", "outlet.9.status", "input.voltage"} {
		if got, ok := telemetry.Vars[absent]; ok {
			t.Errorf("%s = %q, want absent on a RIVER 3", absent, got)
		}
	}
	vars := liveVars(config.Device{Name: "ups"}, DeviceDescriptor{}, telemetry, false)
	if vars["ups.status"] != "OB DISCHRG" || vars["ups.load"] != "29" || vars["ups.temperature"] != "33" {
		t.Errorf("status=%q load=%q temperature=%q, want OB DISCHRG, 29, 33", vars["ups.status"], vars["ups.load"], vars["ups.temperature"])
	}
}

// A DELTA Pro 3: the same port set as the RIVER 3 at a different scale, and
// the runtime message's readings on top when it comes.
func TestExtendedVarsFromDeltaPro3(t *testing.T) {
	merged := telemetryFromProto(&mr521pb.DisplayPropertyUpload{
		CmsBattSoc:               proto.Float32(91.6),
		CmsDsgRemTime:            proto.Uint32(36000),
		PowInSumW:                proto.Float32(1500),
		PowOutSumW:               proto.Float32(410),
		PlugInInfoAcChargerFlag:  proto.Bool(true),
		PowGetBms:                proto.Float32(1050),
		PlugInInfoAcInFeq:        proto.Uint32(60),
		PlugInInfoAcOutDsgPowMax: proto.Uint32(4000),
		PowGetAcIn:               proto.Float32(1500),
		PowGetAcOut:              proto.Float32(-380),
		FlowInfoAcOut:            proto.Uint32(2),
		PowGetTypec2:             proto.Float32(-30),
		FlowInfoTypec2:           proto.Uint32(2),
		PowGet_12V:               proto.Float32(-9.6),
		FlowInfo_12V:             proto.Uint32(2),
		BmsMaxCellTemp:           proto.Int32(29),
		BmsMaxMosTemp:            proto.Int32(35),
		BmsDesignCap:             proto.Uint32(80000),
	})
	want := map[string]string{
		"input.frequency":          "60",
		"input.realpower":          "1500",
		"ups.realpower":            "410",
		"ups.realpower.nominal":    "4000",
		"battery.capacity.nominal": "80.0",
		"outlet.1.realpower":       "380",
		"outlet.4.desc":            "USB-C 2",
		"outlet.4.realpower":       "30",
		"outlet.8.desc":            "12 V car",
		"outlet.8.realpower":       "10",
	}
	for name, value := range want {
		if got := merged.Vars[name]; got != value {
			t.Errorf("%s = %q, want %q", name, got, value)
		}
	}
	if _, ok := merged.Vars["outlet.9.desc"]; ok {
		t.Error("outlet 9 should be absent on a DELTA Pro 3")
	}
	device := config.Device{Name: "ups"}
	vars := liveVars(device, DeviceDescriptor{}, merged, false)
	if vars["ups.status"] != "OL CHRG" || vars["ups.load"] != "10" || vars["ups.temperature"] != "35" {
		t.Errorf("status=%q load=%q temperature=%q, want OL CHRG, 10, 35", vars["ups.status"], vars["ups.load"], vars["ups.temperature"])
	}
	merged = merged.Merge(telemetryFromProto(&mr521pb.RuntimePropertyUpload{
		PlugInInfoAcInVol:  proto.Float32(120.4),
		PlugInInfoAcOutVol: proto.Float32(119.8),
		BmsBattVol:         proto.Float32(51.2),
		TempPcsAc:          proto.Float32(44),
		BmsFullCap:         proto.Uint32(79000),
	}))
	vars = liveVars(device, DeviceDescriptor{}, merged, false)
	if vars["input.voltage"] != "120.4" || vars["output.voltage"] != "119.8" || vars["battery.voltage"] != "51.20" || vars["battery.capacity"] != "79.0" {
		t.Errorf("runtime readings: %v", vars)
	}
	if vars["ups.temperature"] != "44" || vars["ups.load"] != "10" {
		t.Errorf("after the runtime message: temperature=%q load=%q, want 44 and 10", vars["ups.temperature"], vars["ups.load"])
	}
}
