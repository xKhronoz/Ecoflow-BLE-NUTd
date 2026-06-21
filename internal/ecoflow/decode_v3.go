package ecoflow

import (
	"fmt"

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

func (h *v3Handler) handleRiver3(packet Packet) (*Telemetry, []Packet, bool, error) {
	if packet.Src != 0x02 || packet.CmdSet != 0xFE || packet.CmdID != 0x15 {
		return nil, nil, false, nil
	}
	msg := &pr705pb.DisplayPropertyUpload{}
	if err := proto.Unmarshal(packet.Payload, msg); err != nil {
		return nil, nil, true, fmt.Errorf("decode river3 packet: %w", err)
	}
	telemetry := telemetryFromProto(msg)
	return &telemetry, nil, true, nil
}

func (h *v3Handler) handleDelta3(packet Packet) (*Telemetry, []Packet, bool, error) {
	if packet.Src != 0x02 || packet.CmdSet != 0xFE || packet.CmdID != 0x15 {
		return nil, nil, false, nil
	}
	msg := &pd335pb.DisplayPropertyUpload{}
	if err := proto.Unmarshal(packet.Payload, msg); err != nil {
		return nil, nil, true, fmt.Errorf("decode delta3 packet: %w", err)
	}
	telemetry := telemetryFromProto(msg)
	return &telemetry, nil, true, nil
}

func (h *v3Handler) handleDeltaPro3(packet Packet) (*Telemetry, []Packet, bool, error) {
	if packet.Src != 0x02 || packet.CmdSet != 0xFE || packet.CmdID != 0x15 {
		return nil, nil, false, nil
	}
	msg := &mr521pb.DisplayPropertyUpload{}
	if err := proto.Unmarshal(packet.Payload, msg); err != nil {
		return nil, nil, true, fmt.Errorf("decode delta pro 3 packet: %w", err)
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
