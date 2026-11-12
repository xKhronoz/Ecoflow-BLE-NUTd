package ecoflow

import (
	"fmt"
	"math"

	"github.com/xkhronoz/ecoflow-ble-nutd/internal/ecoflow/pb/mr521pb"
	"github.com/xkhronoz/ecoflow-ble-nutd/internal/ecoflow/pb/pd335pb"
	"github.com/xkhronoz/ecoflow-ble-nutd/internal/ecoflow/pb/pr705pb"
	"github.com/xkhronoz/ecoflow-ble-nutd/internal/ecoflow/pb/yj751pb"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type v3Handler struct {
	kind ProfileKind
}

func newV3Handler(kind ProfileKind) PacketHandler {
	return &v3Handler{kind: kind}
}

func (h *v3Handler) HandlePacket(packet Packet) (*Telemetry, []Packet, bool, error) {
	switch h.kind {
	case ProfileRiver3:
		return h.handleRiver3(packet)
	case ProfileDelta3:
		return h.handleDelta3(packet)
	case ProfileDeltaPro3:
		return h.handleDeltaPro3(packet)
	case ProfileDeltaProUltra:
		return h.handleDeltaProUltra(packet)
	default:
		return nil, nil, false, nil
	}
}

// The property profiles (River 3, Delta 3, Delta Pro 3) have two messages
// on cmd set 0xFE: DisplayPropertyUpload (0x15), what the app's main screen
// shows, pushed unprompted; and RuntimePropertyUpload (0x16), the
// electrical detail (AC volts and amps, inverter temperatures, cell
// voltages). Both decode the same way. What makes a device push the runtime
// message over BLE is still unknown: a ConfigWrite with the
// cfg_runtime_property_*_upload_period fields is neither acknowledged nor
// acted on by a DELTA 3 Max Plus, so nothing is requested yet.
const (
	cmdIDDisplayProperty = 0x15
	cmdIDRuntimeProperty = 0x16
)

func (h *v3Handler) handleRiver3(packet Packet) (*Telemetry, []Packet, bool, error) {
	return handleProperty(packet, "river3", &pr705pb.DisplayPropertyUpload{}, &pr705pb.RuntimePropertyUpload{})
}

func (h *v3Handler) handleDelta3(packet Packet) (*Telemetry, []Packet, bool, error) {
	return handleProperty(packet, "delta3", &pd335pb.DisplayPropertyUpload{}, &pd335pb.RuntimePropertyUpload{})
}

func (h *v3Handler) handleDeltaPro3(packet Packet) (*Telemetry, []Packet, bool, error) {
	return handleProperty(packet, "delta pro 3", &mr521pb.DisplayPropertyUpload{}, &mr521pb.RuntimePropertyUpload{})
}

func handleProperty(packet Packet, profile string, display, runtime proto.Message) (*Telemetry, []Packet, bool, error) {
	if packet.Src != 0x02 || packet.CmdSet != 0xFE {
		return nil, nil, false, nil
	}
	var msg proto.Message
	switch packet.CmdID {
	case cmdIDDisplayProperty:
		msg = display
	case cmdIDRuntimeProperty:
		msg = runtime
	default:
		return nil, nil, false, nil
	}
	if err := proto.Unmarshal(packet.Payload, msg); err != nil {
		return nil, nil, true, fmt.Errorf("decode %s packet 0x%02x: %w", profile, packet.CmdID, err)
	}
	telemetry := telemetryFromProto(msg)
	return &telemetry, nil, true, nil
}

func (h *v3Handler) handleDeltaProUltra(packet Packet) (*Telemetry, []Packet, bool, error) {
	switch {
	case packet.Src == 0x02 && packet.CmdSet == 0x02 && packet.CmdID == 0x01:
		msg := &yj751pb.AppShowHeartbeatReport{}
		if err := proto.Unmarshal(packet.Payload, msg); err != nil {
			return nil, nil, true, fmt.Errorf("decode dpu heartbeat: %w", err)
		}
		telemetry := dpuHeartbeatTelemetry(msg)
		outbound, err := marshalDPUHeartbeatRequest()
		if err != nil {
			return nil, nil, true, err
		}
		return &telemetry, []Packet{outbound}, true, nil
	case packet.Src == 0x02 && packet.CmdSet == 0xFE && packet.CmdID == 0x15:
		msg := &yj751pb.DisplayPropertyUpload{}
		if err := proto.Unmarshal(packet.Payload, msg); err != nil {
			return nil, nil, true, fmt.Errorf("decode dpu display packet: %w", err)
		}
		telemetry := telemetryFromProto(msg)
		return &telemetry, nil, true, nil
	default:
		return nil, nil, false, nil
	}
}

func telemetryFromProto(msg protoreflect.ProtoMessage) Telemetry {
	var t Telemetry
	if value, ok := protoFloat32Field(msg, "cms_batt_soc", "bms_batt_soc"); ok {
		t.Charge = intPtr(roundFloat32(value))
	} else if value, ok := protoUint32Field(msg, "soc"); ok {
		t.Charge = intPtr(int(value))
	}
	if value, ok := protoUint32Field(msg, "cms_dsg_rem_time", "generator_remain_time", "remain_time"); ok {
		t.RuntimeSeconds = intPtr(int(value))
	}
	if value, ok := protoFloat32Field(msg, "pow_in_sum_w", "watts_in_sum"); ok {
		t.InputPower = intPtr(roundFloat32(value))
	}
	if value, ok := protoFloat32Field(msg, "pow_out_sum_w", "watts_out_sum"); ok {
		t.OutputPower = intPtr(roundFloat32(value))
	}
	if value, ok := protoBoolField(msg, "plug_in_info_ac_charger_flag"); ok {
		t.InputPresent = boolPtr(value)
	} else if t.InputPower != nil {
		t.InputPresent = boolPtr(*t.InputPower > 0)
	}
	if value, ok := protoFloat32Field(msg, "pow_get_bms"); ok {
		t.BatteryPower = intPtr(roundFloat32(value))
	}
	if value, ok := protoNumberField(msg, "temp_pcs_ac"); ok {
		t.InverterTemperature = intPtr(int(math.Round(value)))
	}
	if value, ok := protoNumberField(msg, "bms_max_mos_temp"); ok {
		t.MOSFETTemperature = intPtr(int(math.Round(value)))
	}
	t.Vars = extendedVars(msg)
	return t
}

func dpuHeartbeatTelemetry(msg *yj751pb.AppShowHeartbeatReport) Telemetry {
	t := Telemetry{}
	if value, ok := protoUint32Field(msg, "soc"); ok {
		t.Charge = intPtr(int(value))
	}
	if value, ok := protoUint32Field(msg, "remain_time"); ok {
		t.RuntimeSeconds = intPtr(int(value))
	}
	if value, ok := protoFloat32Field(msg, "watts_in_sum"); ok {
		t.InputPower = intPtr(roundFloat32(value))
	}
	if value, ok := protoFloat32Field(msg, "watts_out_sum"); ok {
		t.OutputPower = intPtr(roundFloat32(value))
	}
	if t.InputPower != nil {
		t.InputPresent = boolPtr(*t.InputPower > 0)
	}
	return t
}

func marshalDPUHeartbeatRequest() (Packet, error) {
	msg := &yj751pb.SystemParamGet{GetParamType: 8}
	payload, err := proto.Marshal(msg)
	if err != nil {
		return Packet{}, err
	}
	return Packet{
		Src:     0x21,
		Dst:     0x02,
		CmdSet:  0x02,
		CmdID:   0x67,
		Payload: payload,
		DSrc:    0x01,
		DDst:    0x01,
		Version: 0x13,
	}, nil
}
