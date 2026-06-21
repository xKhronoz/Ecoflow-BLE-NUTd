package ecoflow

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/xkhronoz/ecoflow-ble-nutd/internal/ecoflow/pb/pr705pb"
	"google.golang.org/protobuf/proto"
)

func TestParseScanRecordBytes(t *testing.T) {
	t.Parallel()

	data := makeManufacturerBytes("R631123456789012", 7)
	record, err := parseScanRecordBytes(data)
	if err != nil {
		t.Fatalf("parseScanRecordBytes: %v", err)
	}
	if record.SerialNumber != "R631123456789012" {
		t.Fatalf("serial = %q", record.SerialNumber)
	}
	if record.EncryptType != 7 {
		t.Fatalf("encrypt_type = %d", record.EncryptType)
	}
}

func TestFrameAssemblers(t *testing.T) {
	t.Parallel()

	packet := Packet{
		Src:     0x21,
		Dst:     0x35,
		CmdSet:  0x35,
		CmdID:   0x89,
		Payload: []byte("hello"),
		DSrc:    0x01,
		DDst:    0x01,
		Version: 0x13,
	}
	type testCase struct {
		name      string
		assembler FrameAssembler
	}
	cases := []testCase{
		{name: "type0", assembler: &PassthroughAssembler{}},
		{name: "type1", assembler: &RawHeaderAssembler{encryption: Type1Encryption{SessionKey: bytes.Repeat([]byte{0x11}, 16), IV: bytes.Repeat([]byte{0x22}, 16)}}},
		{name: "type7", assembler: &EncPacketAssembler{encryption: Type7Encryption{SessionKey: bytes.Repeat([]byte{0x33}, 16), IV: bytes.Repeat([]byte{0x44}, 16)}}},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			wire, err := tc.assembler.Encode(packet)
			if err != nil {
				t.Fatalf("Encode: %v", err)
			}
			chunks := [][]byte{wire[:len(wire)/2], wire[len(wire)/2:]}
			var payloads [][]byte
			for _, chunk := range chunks {
				got, err := tc.assembler.Reassemble(chunk)
				if err != nil {
					t.Fatalf("Reassemble: %v", err)
				}
				payloads = append(payloads, got...)
			}
			if len(payloads) != 1 {
				t.Fatalf("payloads = %d", len(payloads))
			}
			if !bytes.Equal(payloads[0], packet.MarshalBinary()) {
				t.Fatalf("decoded packet mismatch")
			}
		})
	}
}

func TestV2HandlerDecodesDelta2Telemetry(t *testing.T) {
	t.Parallel()

	handler := newV2Handler(ProfileDelta2)

	pdPayload := bytes.Buffer{}
	_ = binary.Write(&pdPayload, binary.LittleEndian, v2PDPrefix{
		SOC:         78,
		WattsInSum:  120,
		WattsOutSum: 85,
		RemainTime:  3600,
	})
	telemetry, _, handled, err := handler.HandlePacket(Packet{Src: 0x02, CmdSet: 0x20, CmdID: 0x02, Payload: pdPayload.Bytes()})
	if err != nil {
		t.Fatalf("HandlePacket: %v", err)
	}
	if !handled || telemetry == nil {
		t.Fatalf("packet not handled")
	}
	if *telemetry.Charge != 78 || *telemetry.InputPower != 120 || *telemetry.OutputPower != 85 || *telemetry.RuntimeSeconds != 3600 {
		t.Fatalf("telemetry = %#v", *telemetry)
	}
}

func TestV3HandlerDecodesRiver3Telemetry(t *testing.T) {
	t.Parallel()

	handler := newV3Handler(ProfileRiver3)
	charge := float32(64)
	input := float32(143)
	output := float32(87)
	inputPresent := true
	runtime := uint32(5400)
	msg := &pr705pb.DisplayPropertyUpload{
		CmsBattSoc:              &charge,
		PowInSumW:               &input,
		PowOutSumW:              &output,
		PlugInInfoAcChargerFlag: &inputPresent,
		CmsDsgRemTime:           &runtime,
	}
	payload, err := proto.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal proto: %v", err)
	}
	telemetry, _, handled, err := handler.HandlePacket(Packet{Src: 0x02, CmdSet: 0xFE, CmdID: 0x15, Payload: payload})
	if err != nil {
		t.Fatalf("HandlePacket: %v", err)
	}
	if !handled || telemetry == nil {
		t.Fatalf("packet not handled")
	}
	if *telemetry.Charge != 64 || *telemetry.InputPower != 143 || *telemetry.OutputPower != 87 || *telemetry.RuntimeSeconds != 5400 || !*telemetry.InputPresent {
		t.Fatalf("telemetry = %#v", *telemetry)
	}
}

func makeManufacturerBytes(serial string, encryptType int) []byte {
	data := make([]byte, 23)
	data[0] = 0x02
	copy(data[1:17], []byte(serial))
	data[22] = byte((encryptType << 3) | 0x01)
	return data
}
