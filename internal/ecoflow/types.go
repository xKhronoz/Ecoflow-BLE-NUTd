package ecoflow

import (
	"fmt"
	"math"
	"strings"
	"time"
)

type ManufacturerData struct {
	CompanyID uint16
	Data      []byte
}

type Discovery struct {
	MAC              string
	LocalName        string
	ManufacturerData []ManufacturerData
}

type ScanRecord struct {
	ProtoVersion    byte
	SerialNumber    string
	Status          byte
	ProductType     byte
	CapabilityFlags byte
	Encrypt         bool
	SupportVerified bool
	Verified        bool
	EncryptType     int
	Support5G       bool
	ActiveFlag      bool
}

type Telemetry struct {
	Charge         *int
	RuntimeSeconds *int
	InputPower     *int
	OutputPower    *int
	InputPresent   *bool
}

func (t Telemetry) Clone() Telemetry {
	return Telemetry{
		Charge:         cloneInt(t.Charge),
		RuntimeSeconds: cloneInt(t.RuntimeSeconds),
		InputPower:     cloneInt(t.InputPower),
		OutputPower:    cloneInt(t.OutputPower),
		InputPresent:   cloneBool(t.InputPresent),
	}
}

func (t Telemetry) Merge(other Telemetry) Telemetry {
	out := t.Clone()
	if other.Charge != nil {
		out.Charge = cloneInt(other.Charge)
	}
	if other.RuntimeSeconds != nil {
		out.RuntimeSeconds = cloneInt(other.RuntimeSeconds)
	}
	if other.InputPower != nil {
		out.InputPower = cloneInt(other.InputPower)
	}
	if other.OutputPower != nil {
		out.OutputPower = cloneInt(other.OutputPower)
	}
	if other.InputPresent != nil {
		out.InputPresent = cloneBool(other.InputPresent)
	}
	return out
}

func (t Telemetry) Empty() bool {
	return t.Charge == nil && t.RuntimeSeconds == nil && t.InputPower == nil && t.OutputPower == nil && t.InputPresent == nil
}

type DeviceDescriptor struct {
	Serial         string
	Model          string
	PacketVersion  byte
	EncryptType    int
	AuthHeaderDst  byte
	SupportsWake   bool
	SupportsTime   bool
	XORPayload     bool
	Profile        ProfileKind
	Discovery      Discovery
	ScanRecord     ScanRecord
	ProfileName    string
	UnsupportedMsg string
}

type ProfileKind string

const (
	ProfileDelta2        ProfileKind = "delta2"
	ProfileRiver2        ProfileKind = "river2"
	ProfileRiver3        ProfileKind = "river3"
	ProfileDelta3        ProfileKind = "delta3"
	ProfileDeltaPro3     ProfileKind = "delta-pro-3"
	ProfileDeltaProUltra ProfileKind = "delta-pro-ultra"
)

type PacketHandler interface {
	HandlePacket(Packet) (*Telemetry, []Packet, bool, error)
}

type profileSpec struct {
	Kind          ProfileKind
	Name          string
	Prefixes      []string
	PacketVersion byte
	XORPayload    bool
	SupportsTime  bool
	NewHandler    func() PacketHandler
}

var supportedProfiles = []profileSpec{
	{
		Kind:          ProfileDelta2,
		Name:          "EcoFlow DELTA 2 family",
		Prefixes:      []string{"R331", "R335", "R351", "R354", "P341"},
		PacketVersion: 0x02,
		XORPayload:    true,
		SupportsTime:  false,
		NewHandler:    func() PacketHandler { return newV2Handler(ProfileDelta2) },
	},
	{
		Kind:          ProfileRiver2,
		Name:          "EcoFlow RIVER 2 family",
		Prefixes:      []string{"R601", "R603", "R611", "R613", "R621", "R623"},
		PacketVersion: 0x02,
		XORPayload:    false,
		SupportsTime:  false,
		NewHandler:    func() PacketHandler { return newV2Handler(ProfileRiver2) },
	},
	{
		Kind:          ProfileRiver3,
		Name:          "EcoFlow RIVER 3 family",
		Prefixes:      []string{"R631", "R634", "R635", "R651", "R653", "R654", "R655"},
		PacketVersion: 0x13,
		XORPayload:    true,
		SupportsTime:  true,
		NewHandler:    func() PacketHandler { return newV3Handler(ProfileRiver3) },
	},
	{
		Kind:          ProfileDelta3,
		Name:          "EcoFlow DELTA 3 family",
		Prefixes:      []string{"P231", "D361", "P351", "D3N1", "D3M1", "D3E1", "D3U1", "D3UP", "PR11", "PR12", "PR21"},
		PacketVersion: 0x13,
		XORPayload:    true,
		SupportsTime:  true,
		NewHandler:    func() PacketHandler { return newV3Handler(ProfileDelta3) },
	},
	{
		Kind:          ProfileDeltaPro3,
		Name:          "EcoFlow DELTA Pro 3 family",
		Prefixes:      []string{"MR51", "MR53", "MR54"},
		PacketVersion: 0x13,
		XORPayload:    true,
		SupportsTime:  true,
		NewHandler:    func() PacketHandler { return newV3Handler(ProfileDeltaPro3) },
	},
	{
		Kind:          ProfileDeltaProUltra,
		Name:          "EcoFlow DELTA Pro Ultra",
		Prefixes:      []string{"Y711"},
		PacketVersion: 0x13,
		XORPayload:    true,
		SupportsTime:  true,
		NewHandler:    func() PacketHandler { return newV3Handler(ProfileDeltaProUltra) },
	},
}

func lookupProfile(serial string) (profileSpec, error) {
	serial = strings.ToUpper(serial)
	for _, spec := range supportedProfiles {
		for _, prefix := range spec.Prefixes {
			if strings.HasPrefix(serial, prefix) {
				return spec, nil
			}
		}
	}
	return profileSpec{}, fmt.Errorf("unsupported EcoFlow power-station serial prefix %q", serialPrefix(serial))
}

func serialPrefix(serial string) string {
	switch {
	case len(serial) >= 4:
		return serial[:4]
	case len(serial) >= 2:
		return serial[:2]
	default:
		return serial
	}
}

func resolveDescriptor(discovery Discovery) (DeviceDescriptor, error) {
	record, err := ParseScanRecord(discovery.ManufacturerData)
	if err != nil {
		return DeviceDescriptor{}, err
	}
	spec, err := lookupProfile(record.SerialNumber)
	if err != nil {
		return DeviceDescriptor{}, err
	}
	return DeviceDescriptor{
		Serial:        record.SerialNumber,
		Model:         spec.Name,
		PacketVersion: spec.PacketVersion,
		EncryptType:   record.EncryptType,
		AuthHeaderDst: 0x35,
		SupportsWake:  true,
		SupportsTime:  spec.SupportsTime,
		XORPayload:    spec.XORPayload,
		Profile:       spec.Kind,
		Discovery:     discovery,
		ScanRecord:    record,
		ProfileName:   spec.Name,
	}, nil
}

func newHandler(kind ProfileKind) PacketHandler {
	for _, spec := range supportedProfiles {
		if spec.Kind == kind {
			return spec.NewHandler()
		}
	}
	return nil
}

func intPtr(v int) *int { return &v }

func boolPtr(v bool) *bool { return &v }

func cloneInt(v *int) *int {
	if v == nil {
		return nil
	}
	x := *v
	return &x
}

func cloneBool(v *bool) *bool {
	if v == nil {
		return nil
	}
	x := *v
	return &x
}

func roundFloat32(v float32) int {
	return int(math.Round(float64(v)))
}

func clampNonNegative(v int) int {
	if v < 0 {
		return 0
	}
	return v
}

func staleAfter(lastFresh time.Time, timeout time.Duration, now time.Time) bool {
	if lastFresh.IsZero() {
		return true
	}
	return now.Sub(lastFresh) >= timeout
}
