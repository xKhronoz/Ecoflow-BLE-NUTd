package ecoflow

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func TestEncFrameRejectsInvalidLengths(t *testing.T) {
	for _, length := range []uint16{0, 1, 10001, 65535} {
		bad := []byte{0x5a, 0x5a, 0, 1, 0, 0, 0, 0}
		binary.LittleEndian.PutUint16(bad[4:6], length)
		good := (&SimplePacketAssembler{}).Encode([]byte("reply"))
		data := append(bad, good...)
		if got, ok := (&SimplePacketAssembler{}).Parse(data); !ok || !bytes.Equal(got, []byte("reply")) {
			t.Fatalf("handshake length %d: payload = %x, ok = %v", length, got, ok)
		}
		got, err := (&EncPacketAssembler{}).Reassemble(data)
		if err != nil || len(got) != 1 || !bytes.Equal(got[0], []byte("reply")) {
			t.Fatalf("telemetry length %d: payloads = %x, error = %v", length, got, err)
		}
	}
}

func TestEncFramesSurviveEverySplit(t *testing.T) {
	// A prefix can be split across notifications, and 0x5a5a can also occur
	// inside a valid payload. Neither case should discard a partial frame.
	payload := []byte{1, 0, 0, 0x5a, 0x5a, 6, 7, 8, 9}
	wire := (&SimplePacketAssembler{}).Encode(payload)
	for split := 1; split < len(wire); split++ {
		simple := &SimplePacketAssembler{}
		if _, ok := simple.Parse(wire[:split]); ok {
			t.Fatalf("split %d: parsed incomplete handshake", split)
		}
		if got, ok := simple.Parse(wire[split:]); !ok || !bytes.Equal(got, payload) {
			t.Fatalf("split %d: handshake = %x, ok = %v", split, got, ok)
		}
		enc := &EncPacketAssembler{}
		if got, err := enc.Reassemble(wire[:split]); err != nil || len(got) != 0 {
			t.Fatalf("split %d: incomplete telemetry = %x, %v", split, got, err)
		}
		if got, err := enc.Reassemble(wire[split:]); err != nil || len(got) != 1 || !bytes.Equal(got[0], payload) {
			t.Fatalf("split %d: telemetry = %x, %v", split, got, err)
		}
	}
}

func TestSentinelFramesWithoutTrailingCRC(t *testing.T) {
	for _, version := range []byte{0x12, 0x13} {
		packet := Packet{Src: 2, CmdSet: 0xfe, CmdID: 0x15, Version: version & 0x0f, Payload: []byte{1, 2, 0xbb, 0xbb}}
		// Incoming sentinel packets include the sentinel in their payload length,
		// without the CRC that MarshalBinary adds to outgoing requests.
		raw := packet.MarshalBinary()
		raw[1] = version
		raw[4] = crc8CCITT(raw[:4])
		raw = raw[:len(raw)-2]
		passthrough := &PassthroughAssembler{}
		got, err := passthrough.Reassemble(raw)
		if err != nil || len(got) != 1 || !bytes.Equal(got[0], raw) {
			t.Fatalf("version %x: passthrough = %x, %v", version, got, err)
		}
		encryption := Type1Encryption{SessionKey: bytes.Repeat([]byte{1}, 16), IV: bytes.Repeat([]byte{2}, 16)}
		body, err := encryption.Encrypt(raw[5:])
		if err != nil {
			t.Fatal(err)
		}
		got, err = (&RawHeaderAssembler{encryption: encryption}).Reassemble(append(append([]byte(nil), raw[:5]...), body...))
		if err != nil || len(got) != 1 || !bytes.Equal(got[0], raw) {
			t.Fatalf("version %x: type1 = %x, %v", version, got, err)
		}
		decoded, err := ParsePacket(got[0], false)
		if err != nil || !bytes.Equal(decoded.Payload, []byte{1, 2}) {
			t.Fatalf("version %x: decoded = %#v, %v", version, decoded, err)
		}
	}
}

func TestSimpleAssemblerPreservesMultipleReplies(t *testing.T) {
	simple := &SimplePacketAssembler{}
	wire := append(simple.Encode([]byte{1, 2, 3}), simple.Encode([]byte{2, 4, 5})...)
	if got, ok := simple.Parse(wire); !ok || !bytes.Equal(got, []byte{1, 2, 3}) {
		t.Fatalf("first reply = %x, ok = %v", got, ok)
	}
	if got, ok := simple.Parse(nil); !ok || !bytes.Equal(got, []byte{2, 4, 5}) {
		t.Fatalf("buffered reply = %x, ok = %v", got, ok)
	}
}

func FuzzEncFrameFragmentation(f *testing.F) {
	f.Add([]byte{1, 2, 3}, uint16(1))
	f.Add([]byte{0x5a, 0x5a, 0, 0, 0x5a}, uint16(9))
	f.Add([]byte{}, uint16(0))
	f.Fuzz(func(t *testing.T, payload []byte, position uint16) {
		if len(payload) > 9998 {
			return // Maximum supported outer-frame payload, excluding CRC16.
		}
		wire := (&SimplePacketAssembler{}).Encode(payload)
		split := 1 + int(position)%(len(wire)-1)
		simple := &SimplePacketAssembler{}
		if _, ok := simple.Parse(wire[:split]); ok {
			t.Fatal("parsed incomplete handshake frame")
		}
		if got, ok := simple.Parse(wire[split:]); !ok || !bytes.Equal(got, payload) {
			t.Fatalf("handshake did not survive fragmentation at %d", split)
		}
		enc := &EncPacketAssembler{}
		if got, err := enc.Reassemble(wire[:split]); err != nil || len(got) != 0 {
			t.Fatalf("incomplete outer frame: %x, %v", got, err)
		}
		if got, err := enc.Reassemble(wire[split:]); err != nil || len(got) != 1 || !bytes.Equal(got[0], payload) {
			t.Fatalf("outer frame did not survive fragmentation at %d: %v", split, err)
		}
	})
}
