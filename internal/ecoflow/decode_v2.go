package ecoflow

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

type v2Handler struct {
	kind ProfileKind

	pd  *v2PDPrefix
	ems *v2EMSPrefix
	bms *v2BMSPrefix
}

type v2PDPrefix struct {
	Model            uint8
	ErrCode          [4]byte
	SysVer           [4]byte
	WifiVer          [4]byte
	WifiAutoRecovery uint8
	SOC              uint8
	WattsOutSum      uint16
	WattsInSum       uint16
	RemainTime       int32
}

type v2EMSPrefix struct {
	ChgState      uint8
	ChgCmd        uint8
	DsgCmd        uint8
	ChgVol        uint32
	ChgAmp        uint32
	FanLevel      uint8
	MaxChargeSOC  uint8
	BMSModel      uint8
	LCDShowSOC    uint8
	OpenUPSFlag   uint8
	BMSWarning    uint8
	ChgRemainTime uint32
	DsgRemainTime uint32
	EmsNormalFlag uint8
	F32LCDShowSOC float32
}

type v2BMSPrefix struct {
	Num          uint8
	Type         uint8
	CellID       uint8
	ErrCode      uint32
	SysVer       uint32
	SOC          uint8
	Vol          uint32
	Amp          uint32
	Temp         uint8
	OpenBMSIdx   uint8
	DesignCap    uint32
	RemainCap    uint32
	FullCap      uint32
	Cycles       uint32
	SOH          uint8
	MaxCellVol   uint16
	MinCellVol   uint16
	MaxCellTemp  uint8
	MinCellTemp  uint8
	MaxMOSTemp   uint8
	MinMOSTemp   uint8
	BMSFault     uint8
	BQSysStatReg uint8
	TagChgAmp    uint32
	F32ShowSOC   float32
	InputWatts   uint32
	OutputWatts  uint32
	RemainTime   uint32
}

func newV2Handler(kind ProfileKind) PacketHandler {
	return &v2Handler{kind: kind}
}

func (h *v2Handler) HandlePacket(packet Packet) (*Telemetry, []Packet, bool, error) {
	switch {
	case packet.Src == 0x02 && packet.CmdSet == 0x20 && packet.CmdID == 0x02:
		var pd v2PDPrefix
		if err := readBinary(packet.Payload, &pd); err != nil {
			return nil, nil, true, fmt.Errorf("decode v2 pd packet: %w", err)
		}
		h.pd = &pd
	case packet.Src == 0x03 && packet.CmdSet == 0x20 && packet.CmdID == 0x02:
		var ems v2EMSPrefix
		if err := readBinary(packet.Payload, &ems); err != nil {
			return nil, nil, true, fmt.Errorf("decode v2 ems packet: %w", err)
		}
		h.ems = &ems
	case packet.Src == 0x03 && packet.CmdSet == 0x20 && packet.CmdID == 0x32:
		var bms v2BMSPrefix
		if err := readBinary(packet.Payload, &bms); err != nil {
			return nil, nil, true, fmt.Errorf("decode v2 bms packet: %w", err)
		}
		h.bms = &bms
	default:
		return nil, nil, false, nil
	}
	telemetry := h.snapshot()
	return &telemetry, nil, true, nil
}

func (h *v2Handler) snapshot() Telemetry {
	var t Telemetry
	if h.bms != nil {
		charge := roundFloat32(h.bms.F32ShowSOC)
		t.Charge = intPtr(charge)
		if h.bms.RemainTime > 0 {
			t.RuntimeSeconds = intPtr(clampNonNegative(int(h.bms.RemainTime)))
		}
		if h.bms.InputWatts > 0 {
			t.InputPower = intPtr(int(h.bms.InputWatts))
		}
		if h.bms.OutputWatts > 0 {
			t.OutputPower = intPtr(int(h.bms.OutputWatts))
		}
	}
	if h.ems != nil {
		if h.ems.F32LCDShowSOC > 0 {
			t.Charge = intPtr(roundFloat32(h.ems.F32LCDShowSOC))
		} else if h.ems.LCDShowSOC > 0 {
			t.Charge = intPtr(int(h.ems.LCDShowSOC))
		}
		if h.ems.DsgRemainTime > 0 {
			t.RuntimeSeconds = intPtr(int(h.ems.DsgRemainTime))
		}
	}
	if h.pd != nil {
		if t.Charge == nil {
			t.Charge = intPtr(int(h.pd.SOC))
		}
		if t.RuntimeSeconds == nil && h.pd.RemainTime > 0 {
			t.RuntimeSeconds = intPtr(clampNonNegative(int(h.pd.RemainTime)))
		}
		if h.pd.WattsInSum > 0 {
			t.InputPower = intPtr(int(h.pd.WattsInSum))
		}
		if h.pd.WattsOutSum >= 0 {
			t.OutputPower = intPtr(int(h.pd.WattsOutSum))
		}
	}
	if t.InputPresent == nil {
		switch {
		case t.InputPower != nil:
			t.InputPresent = boolPtr(*t.InputPower > 0)
		case h.ems != nil:
			t.InputPresent = boolPtr(h.ems.OpenUPSFlag != 0)
		}
	}
	return t
}

func readBinary(data []byte, into any) error {
	return binary.Read(bytes.NewReader(data), binary.LittleEndian, into)
}
