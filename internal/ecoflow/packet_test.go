package ecoflow

import (
	"bytes"
	"crypto/aes"
	"encoding/binary"
	"testing"
)

// Encoding errors in fixed-size fixtures are test failures. Boundary tests
// below inspect errors directly instead of using this helper.
func mustEncode(data []byte, err error) []byte {
	if err != nil {
		panic(err)
	}
	return data
}

func TestPacketPayloadLengthBoundaries(t *testing.T) {
	for _, version := range []byte{2, 3} {
		for _, size := range []int{0, 1, 65535} {
			payload := bytes.Repeat([]byte{0x42}, size)
			wire, err := (Packet{Version: version, Payload: payload}).MarshalBinary()
			if err != nil {
				t.Fatalf("version %d, payload %d: %v", version, size, err)
			}
			if got := int(binary.LittleEndian.Uint16(wire[2:4])); got != size {
				t.Fatalf("payload length wrapped: got %d, want %d", got, size)
			}
			decoded, err := ParsePacket(wire, false)
			if err != nil || !bytes.Equal(decoded.Payload, payload) {
				t.Fatalf("version %d, payload %d: round trip failed: %v", version, size, err)
			}
		}
	}
	if wire, err := (Packet{Payload: make([]byte, 65536)}).MarshalBinary(); err == nil || wire != nil {
		t.Fatal("packet with an unrepresentable payload length was encoded")
	}
}

func TestEncPacketPayloadLengthBoundaries(t *testing.T) {
	for _, size := range []int{0, 1, 65533} {
		payload := bytes.Repeat([]byte{0x42}, size)
		wire, err := (EncPacket{Payload: payload}).MarshalBinary()
		if err != nil {
			t.Fatalf("payload %d: %v", size, err)
		}
		if got := int(binary.LittleEndian.Uint16(wire[4:6])); got != size+2 || len(wire) != 8+size {
			t.Fatalf("payload %d: invalid outer length %d or frame size %d", size, got, len(wire))
		}
		if !bytes.Equal(wire[6:len(wire)-2], payload) || binary.LittleEndian.Uint16(wire[len(wire)-2:]) != crc16ARC(wire[:len(wire)-2]) {
			t.Fatalf("payload %d: payload or CRC changed", size)
		}
	}
	if wire, err := (EncPacket{Payload: make([]byte, 65534)}).MarshalBinary(); err == nil || wire != nil {
		t.Fatal("outer frame with an unrepresentable CRC-inclusive length was encoded")
	}
	if wire, err := (&SimplePacketAssembler{}).Encode(make([]byte, 65534)); err == nil || wire != nil {
		t.Fatal("handshake encoder did not propagate the outer-frame error")
	}
}

func TestPKCS7PaddingBoundaries(t *testing.T) {
	for _, size := range []int{0, 15, 16, 65555} {
		plaintext := bytes.Repeat([]byte{0x42}, size)
		padded, err := pkcs7Pad(plaintext, aes.BlockSize)
		if err != nil || len(padded)%aes.BlockSize != 0 || len(padded) <= size {
			t.Fatalf("plaintext %d: invalid padded length %d: %v", size, len(padded), err)
		}
		unpadded, err := pkcs7Unpad(padded, aes.BlockSize)
		if err != nil || !bytes.Equal(unpadded, plaintext) {
			t.Fatalf("plaintext %d: padding round trip failed: %v", size, err)
		}
	}
	if padded, err := pkcs7Pad(make([]byte, 65556), aes.BlockSize); err == nil || padded != nil {
		t.Fatal("oversized plaintext was padded")
	}
	for _, blockSize := range []int{-1, 0, 256} {
		if padded, err := pkcs7Pad(nil, blockSize); err == nil || padded != nil {
			t.Fatalf("invalid PKCS7 block size %d was accepted", blockSize)
		}
	}
}

func TestEncryptionRejectsOversizedPlaintext(t *testing.T) {
	key := bytes.Repeat([]byte{1}, aes.BlockSize)
	iv := bytes.Repeat([]byte{2}, aes.BlockSize)
	for _, encryption := range []EncryptionStrategy{
		Type1Encryption{SessionKey: key, IV: iv},
		Type7Encryption{SessionKey: key, IV: iv},
	} {
		if ciphertext, err := encryption.Encrypt(make([]byte, 65556)); err == nil || ciphertext != nil {
			t.Fatalf("%T accepted oversized plaintext", encryption)
		}
	}
}

func TestWritePacketRejectsOversizedPayloadBeforeBLEWrite(t *testing.T) {
	key := bytes.Repeat([]byte{1}, aes.BlockSize)
	iv := bytes.Repeat([]byte{2}, aes.BlockSize)
	for _, assembler := range []FrameAssembler{
		&PassthroughAssembler{},
		&RawHeaderAssembler{encryption: Type1Encryption{SessionKey: key, IV: iv}},
		&EncPacketAssembler{},
		&EncPacketAssembler{encryption: Type7Encryption{SessionKey: key, IV: iv}},
	} {
		conn := &fakeConn{}
		if err := (&Provider{}).writePacket(conn, assembler, Packet{Payload: make([]byte, 65536)}); err == nil || len(conn.writes) != 0 {
			t.Fatalf("%T sent a packet with a wrapped payload length", assembler)
		}
	}
	// An inner packet may fit its own uint16 length but exceed the outer
	// frame's length once its header and encryption padding are included.
	for _, encryption := range []EncryptionStrategy{nil, Type7Encryption{SessionKey: key, IV: iv}} {
		conn := &fakeConn{}
		assembler := &EncPacketAssembler{encryption: encryption}
		if err := (&Provider{}).writePacket(conn, assembler, Packet{Payload: make([]byte, 65535)}); err == nil || len(conn.writes) != 0 {
			t.Fatalf("outer frame error was not propagated for %T", encryption)
		}
	}
}
