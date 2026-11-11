package ecoflow

import (
	"math"
	"strconv"

	"google.golang.org/protobuf/reflect/protoreflect"
)

// outletPorts gives every port a fixed outlet number, so a reading keeps its
// meaning across models (a port a model lacks simply never appears) and
// across partial property updates. power is the per-port watts field, which
// the devices report negative for output; flow is the port's flow bitmask.
// listOnly marks a port whose only evidence is its slot in a per-group watts
// list: its flow field is shared with another port, so without the slot the
// port does not exist on this model.
var outletPorts = []struct {
	index    int
	desc     string
	power    string
	flow     string
	listOnly bool
}{
	// The two AC groups' watts are slots 0 and 2 of pow_get_ac_out_list
	// (checked against the app on a DELTA 3 Max Plus: 155 W / 11 W); slot 0
	// doubles as pow_get_ac_out, the fallback for group 1.
	{1, "AC 1", "pow_get_ac_out", "flow_info_ac_out", false},
	{2, "AC 2", "", "flow_info_ac2_out", false},
	{3, "USB-C 1", "pow_get_typec1", "flow_info_typec1", false},
	{4, "USB-C 2", "pow_get_typec2", "flow_info_typec2", false},
	{5, "USB-C 3", "pow_get_typec3", "flow_info_typec3", false},
	{6, "USB-A 1", "pow_get_qcusb1", "flow_info_qcusb1", false},
	{7, "USB-A 2", "pow_get_qcusb2", "flow_info_qcusb2", false},
	// The two 12 V outputs' watts are the slots of pow_get_12v_list (car
	// socket, then the Anderson port); pow_get_12v is the fallback for the
	// first. Both share the one 12 V switch, so the Anderson port is only
	// there when the list has its slot: a RIVER 3 has the switch and one
	// socket.
	{8, "12 V car", "pow_get_12v", "flow_info_12v", false},
	{9, "12 V Anderson", "", "flow_info_12v", true},
}

// extendedVars maps whatever electrical detail a property message carries
// onto standard NUT variable names: the AC side, the battery, and one
// outlet per port. Fields are looked up by name, so every profile yields
// what its message has and nothing is made up for a missing field. Values
// come back formatted, ready to serve; a later partial update merges over
// an earlier one key by key (Telemetry.Merge). Anything derived from two
// readings (ups.load, ups.temperature) is not made here but after the
// merge, in liveVars, so the two need not arrive in the same message.
func extendedVars(msg protoreflect.ProtoMessage) map[string]string {
	vars := map[string]string{}
	// number stores the first present field under name with precision
	// decimals; whole numbers (frequency, degrees, percent) use 0.
	number := func(name string, precision int, fields ...string) {
		if v, ok := protoNumberField(msg, fields...); ok {
			vars[name] = strconv.FormatFloat(v, 'f', precision, 64)
		}
	}

	// The AC side: the runtime message carries the volts and amps, the
	// display message the input frequency and the rated output.
	number("input.voltage", 1, "plug_in_info_ac_in_vol")
	number("input.frequency", 0, "plug_in_info_ac_in_feq")
	number("input.current", 2, "plug_in_info_ac_in_amp")
	number("input.realpower", 0, "pow_get_ac_in")
	number("output.voltage", 1, "plug_in_info_ac_out_vol")
	number("output.frequency", 0, "plug_in_info_ac_out_freq", "ac_out_freq")
	number("output.current", 2, "plug_in_info_ac_out_amp")
	number("ups.realpower", 0, "pow_out_sum_w", "watts_out_sum")
	number("ups.realpower.nominal", 0, "plug_in_info_ac_out_dsg_pow_max")

	// The battery: the pack's own BMS first, the pack manager as fallback.
	number("battery.voltage", 2, "bms_batt_vol", "cms_batt_vol")
	number("battery.current", 2, "bms_batt_amp", "cms_batt_amp")
	number("battery.temperature", 0, "bms_max_cell_temp")
	capacity := func(name string, fields ...string) {
		if v, ok := protoNumberField(msg, fields...); ok {
			vars[name] = strconv.FormatFloat(v/1000, 'f', 1, 64) // mAh -> Ah
		}
	}
	capacity("battery.capacity.nominal", "bms_design_cap")
	capacity("battery.capacity", "bms_full_cap")
	number("battery.charge.low", 0, "cms_min_dsg_soc")
	// The 12 V socket's own rail (runtime message).
	number("outlet.8.voltage", 1, "plug_in_info_12v_vol")
	number("outlet.8.current", 2, "plug_in_info_12v_amp")

	acGroups, _ := protoNumberListField(msg, "pow_get_ac_out_list", "pow_get_ac_out_item")
	dc12Groups, _ := protoNumberListField(msg, "pow_get_12v_list", "pow_get_12v_item")
	// outlet number -> (list, slot) for the ports whose watts come from a list.
	listSlot := map[int]struct {
		list []float64
		slot int
	}{
		1: {acGroups, 0}, 2: {acGroups, 2},
		8: {dc12Groups, 0}, 9: {dc12Groups, 1},
	}
	for _, port := range outletPorts {
		prefix := "outlet." + strconv.Itoa(port.index) + "."
		present := false
		ls, fromList := listSlot[port.index]
		if port.listOnly && !(fromList && len(ls.list) > ls.slot) {
			continue
		}
		if fromList && len(ls.list) > ls.slot {
			vars[prefix+"realpower"] = strconv.Itoa(clampNonNegative(int(math.Round(-ls.list[ls.slot]))))
			present = true
		} else if port.power != "" {
			if v, ok := protoNumberField(msg, port.power); ok {
				vars[prefix+"realpower"] = strconv.Itoa(clampNonNegative(int(math.Round(-v))))
				present = true
			}
		}
		if v, ok := protoNumberField(msg, port.flow); ok {
			vars[prefix+"status"] = flowStatus(uint32(v))
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
