package ecoflow

import (
	"testing"

	"github.com/xkhronoz/ecoflow-ble-nutd/internal/config"
	"github.com/xkhronoz/ecoflow-ble-nutd/internal/ecoflow/pb/pd335pb"
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
		"ups.load":                 "6",
		"battery.temperature":      "31",
		"ups.temperature":          "38",
		"battery.capacity.nominal": "40.0",
		"battery.charge.low":       "20",
		"outlet.1.desc":            "AC",
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

	vars := liveVars(config.Device{Name: "ups", LowBatteryPercent: 20, LowRuntimeSeconds: 300}, DeviceDescriptor{}, telemetry)
	if vars["ups.status"] != "OL CHRG" {
		t.Errorf("ups.status = %q, want %q", vars["ups.status"], "OL CHRG")
	}
	if vars["battery.charger.status"] != "charging" {
		t.Errorf("battery.charger.status = %q", vars["battery.charger.status"])
	}
	if vars["ups.realpower.nominal"] != "2000" || vars["battery.charge"] != "87" || vars["output.power"] != "127" {
		t.Errorf("core/extended mix wrong: nominal=%q charge=%q output.power=%q", vars["ups.realpower.nominal"], vars["battery.charge"], vars["output.power"])
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
		"ups.temperature":  "41",
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
}

func TestRatedPowerFromConfigWhenDeviceReportsNone(t *testing.T) {
	telemetry := telemetryFromProto(&pd335pb.DisplayPropertyUpload{
		CmsBattSoc: proto.Float32(50),
		PowOutSumW: proto.Float32(600),
	})
	vars := liveVars(config.Device{Name: "ups", RatedPowerWatts: 3000}, DeviceDescriptor{}, telemetry)
	if vars["ups.realpower.nominal"] != "3000" || vars["ups.load"] != "20" {
		t.Errorf("nominal=%q load=%q", vars["ups.realpower.nominal"], vars["ups.load"])
	}
	// A rating the device does report wins over the config.
	telemetry = telemetryFromProto(&pd335pb.DisplayPropertyUpload{
		PowOutSumW:               proto.Float32(600),
		PlugInInfoAcOutDsgPowMax: proto.Uint32(2400),
	})
	vars = liveVars(config.Device{Name: "ups", RatedPowerWatts: 3000}, DeviceDescriptor{}, telemetry)
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
}
