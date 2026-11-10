package ecoflow

import (
	"strconv"

	"google.golang.org/protobuf/reflect/protoreflect"
)

// outletPorts gives every port a fixed outlet number, so a reading keeps its
// meaning across models (a port a model lacks simply never appears) and
// across partial property updates. power is the per-port watts field, which
// the devices report negative for output; flow is the port's flow bitmask.
var outletPorts = []struct {
	index int
	desc  string
	power string
	flow  string
}{
	{1, "AC", "pow_get_ac_out", "flow_info_ac_out"},
	{2, "AC 2", "", "flow_info_ac2_out"},
	{3, "USB-C 1", "pow_get_typec1", "flow_info_typec1"},
	{4, "USB-C 2", "pow_get_typec2", "flow_info_typec2"},
	{5, "USB-C 3", "pow_get_typec3", "flow_info_typec3"},
	{6, "USB-A 1", "pow_get_qcusb1", "flow_info_qcusb1"},
	{7, "USB-A 2", "pow_get_qcusb2", "flow_info_qcusb2"},
	{8, "12 V", "pow_get_12v", "flow_info_12v"},
}

// extendedVars maps whatever electrical detail a property message carries
// onto standard NUT variable names: the AC side, the battery, temperatures,
// and one outlet per port. Fields are looked up by name, so every profile
// yields what its message has and nothing is made up for a missing field.
// Values come back formatted, ready to serve; a later partial update merges
// over an earlier one key by key (Telemetry.Merge).
func extendedVars(msg protoreflect.ProtoMessage) map[string]string {
	vars := map[string]string{}
	float := func(name string, precision int, fields ...string) {
		if v, ok := protoFloat32Field(msg, fields...); ok {
			vars[name] = formatFloat(v, precision)
		}
	}
	uint := func(name string, fields ...string) {
		if v, ok := protoUint32Field(msg, fields...); ok {
			vars[name] = strconv.FormatUint(uint64(v), 10)
		}
	}
	integer := func(name string, fields ...string) {
		if v, ok := protoInt32Field(msg, fields...); ok {
			vars[name] = strconv.Itoa(int(v))
		}
	}

	// The AC side. RuntimePropertyUpload carries the volts and amps, the
	// display message the frequency and the rated output.
	float("input.voltage", 1, "plug_in_info_ac_in_vol")
	uint("input.frequency", "plug_in_info_ac_in_feq")
	float("input.current", 2, "plug_in_info_ac_in_amp")
	float("input.realpower", 0, "pow_get_ac_in")
	float("output.voltage", 1, "plug_in_info_ac_out_vol")
	uint("output.frequency", "plug_in_info_ac_out_freq", "ac_out_freq")
	float("output.current", 2, "plug_in_info_ac_out_amp")
	float("ups.realpower", 0, "pow_out_sum_w", "watts_out_sum")
	uint("ups.realpower.nominal", "plug_in_info_ac_out_dsg_pow_max")
	if out, ok := protoFloat32Field(msg, "pow_out_sum_w", "watts_out_sum"); ok {
		if nominal, ok := protoUint32Field(msg, "plug_in_info_ac_out_dsg_pow_max"); ok && nominal > 0 {
			vars["ups.load"] = strconv.Itoa(clampNonNegative(roundFloat32(out * 100 / float32(nominal))))
		}
	}

	// The battery: the pack's own BMS first, the pack manager as fallback.
	float("battery.voltage", 2, "bms_batt_vol", "cms_batt_vol")
	float("battery.current", 2, "bms_batt_amp", "cms_batt_amp")
	integer("battery.temperature", "bms_max_cell_temp")
	integer("ups.temperature", "bms_max_mos_temp")
	if v, ok := protoUint32Field(msg, "bms_design_cap"); ok {
		vars["battery.capacity"] = formatFloat(float32(v)/1000, 1) // mAh -> Ah
	}
	uint("battery.charge.low", "cms_min_dsg_soc")

	for _, port := range outletPorts {
		prefix := "outlet." + strconv.Itoa(port.index) + "."
		present := false
		if port.power != "" {
			if v, ok := protoFloat32Field(msg, port.power); ok {
				vars[prefix+"realpower"] = strconv.Itoa(clampNonNegative(roundFloat32(-v)))
				present = true
			}
		}
		if v, ok := protoUint32Field(msg, port.flow); ok {
			vars[prefix+"status"] = flowStatus(v)
			present = true
		}
		if present {
			vars[prefix+"desc"] = port.desc
		}
	}
	return vars
}

// flowStatus reads a port's flow bitmask the way the app does: the low two
// bits are 0 for off and 2 for on, anything else is unknown and counts as on.
func flowStatus(flow uint32) string {
	if flow&0b11 >= 0b10 {
		return "on"
	}
	return "off"
}

func formatFloat(v float32, precision int) string {
	return strconv.FormatFloat(float64(v), 'f', precision, 32)
}
