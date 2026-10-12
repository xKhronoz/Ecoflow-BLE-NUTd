package ecoflow

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"encoding/binary"
	"fmt"
)

type EncryptionStrategy interface {
	Encrypt([]byte) ([]byte, error)
	Decrypt([]byte) ([]byte, error)
}

type Type7Encryption struct {
	SessionKey []byte
	IV         []byte
}

func (e Type7Encryption) Encrypt(plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(e.SessionKey)
	if err != nil {
		return nil, err
	}
	plaintext, err = pkcs7Pad(plaintext, aes.BlockSize)
	if err != nil {
		return nil, err
	}
	out := make([]byte, len(plaintext))
	cipher.NewCBCEncrypter(block, e.IV).CryptBlocks(out, plaintext)
	return out, nil
}

func (e Type7Encryption) Decrypt(ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(e.SessionKey)
	if err != nil {
		return nil, err
	}
	aligned := len(ciphertext) - (len(ciphertext) % aes.BlockSize)
	if aligned == 0 {
		return append([]byte(nil), ciphertext...), nil
	}
	out := make([]byte, aligned)
	cipher.NewCBCDecrypter(block, e.IV).CryptBlocks(out, ciphertext[:aligned])
	if unpadded, err := pkcs7Unpad(out, aes.BlockSize); err == nil {
		return unpadded, nil
	}
	return out, nil
}

type Type1Encryption struct {
	SessionKey []byte
	IV         []byte
}

func (e Type1Encryption) Encrypt(plaintext []byte) ([]byte, error) {
	if len(plaintext) > maxPacketSize {
		return nil, fmt.Errorf("plaintext too large: %d bytes, maximum %d", len(plaintext), maxPacketSize)
	}
	block, err := aes.NewCipher(e.SessionKey)
	if err != nil {
		return nil, err
	}
	paddedLen := (len(plaintext) + aes.BlockSize - 1) / aes.BlockSize * aes.BlockSize
	padded := make([]byte, paddedLen)
	copy(padded, plaintext)
	out := make([]byte, paddedLen)
	cipher.NewCBCEncrypter(block, e.IV).CryptBlocks(out, padded)
	return out, nil
}

func (e Type1Encryption) Decrypt(ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(e.SessionKey)
	if err != nil {
		return nil, err
	}
	if len(ciphertext)%aes.BlockSize != 0 {
		return nil, fmt.Errorf("ciphertext is not block aligned")
	}
	out := make([]byte, len(ciphertext))
	cipher.NewCBCDecrypter(block, e.IV).CryptBlocks(out, ciphertext)
	return out, nil
}

type FrameAssembler interface {
	WriteWithResponse() bool
	Encode(Packet) ([]byte, error)
	Reassemble([]byte) ([][]byte, error)
}

type EncPacketAssembler struct {
	buffer     []byte
	encryption EncryptionStrategy
}

func (a *EncPacketAssembler) WriteWithResponse() bool { return true }

func (a *EncPacketAssembler) Encode(pkt Packet) ([]byte, error) {
	payload, err := pkt.MarshalBinary()
	if err != nil {
		return nil, err
	}
	if a.encryption != nil {
		payload, err = a.encryption.Encrypt(payload)
		if err != nil {
			return nil, err
		}
	}
	return EncPacket{FrameType: 0x01, PayloadType: 0x00, Payload: payload}.MarshalBinary()
}

func (a *EncPacketAssembler) Reassemble(data []byte) ([][]byte, error) {
	if len(a.buffer) != 0 {
		data = append(a.buffer, data...)
		a.buffer = nil
	}
	var payloads [][]byte
	for len(data) > 0 {
		start := bytes.Index(data, []byte{0x5A, 0x5A})
		if start < 0 {
			// Keep a possible first prefix byte for the next notification.
			if data[len(data)-1] == 0x5A {
				a.buffer = []byte{0x5A}
			}
			break
		}
		data = data[start:]
		if len(data) < 8 {
			a.buffer = append([]byte(nil), data...)
			break
		}
		payloadLen := int(binary.LittleEndian.Uint16(data[4:6]))
		if payloadLen < 2 || payloadLen > 10000 {
			data = data[2:]
			continue
		}
		frameLen := 6 + payloadLen
		if len(data) < frameLen {
			// A prefix inside ciphertext is not evidence of a new frame.
			a.buffer = append([]byte(nil), data...)
			break
		}
		payload := data[6 : frameLen-2]
		want := binary.LittleEndian.Uint16(data[frameLen-2 : frameLen])
		if crc16ARC(data[:frameLen-2]) != want {
			data = data[2:]
			continue
		}
		data = data[frameLen:]
		if a.encryption != nil {
			var err error
			payload, err = a.encryption.Decrypt(payload)
			if err != nil {
				return nil, err
			}
		}
		payloads = append(payloads, payload)
	}
	return payloads, nil
}

type PassthroughAssembler struct {
	buffer []byte
}

func (a *PassthroughAssembler) WriteWithResponse() bool { return false }

func (a *PassthroughAssembler) Encode(pkt Packet) ([]byte, error) {
	return pkt.MarshalBinary()
}

func (a *PassthroughAssembler) Reassemble(data []byte) ([][]byte, error) {
	if len(a.buffer) != 0 {
		data = append(a.buffer, data...)
		a.buffer = nil
	}
	var payloads [][]byte
	for len(data) > 0 {
		start := bytes.IndexByte(data, packetPrefix)
		if start < 0 {
			break
		}
		data = data[start:]
		if len(data) < 5 {
			a.buffer = append([]byte(nil), data...)
			break
		}
		if crc8CCITT(data[:4]) != data[4] {
			data = data[1:]
			continue
		}
		payloadLen := int(binary.LittleEndian.Uint16(data[2:4]))
		version := data[1]
		frameLen := 5 + packetBodyLen(version, payloadLen)
		if len(data) < frameLen {
			a.buffer = append([]byte(nil), data...)
			break
		}
		payloads = append(payloads, append([]byte(nil), data[:frameLen]...))
		data = data[frameLen:]
	}
	return payloads, nil
}

type RawHeaderAssembler struct {
	buffer     []byte
	encryption EncryptionStrategy
}

func (a *RawHeaderAssembler) WriteWithResponse() bool { return false }

func (a *RawHeaderAssembler) Encode(pkt Packet) ([]byte, error) {
	raw, err := pkt.MarshalBinary()
	if err != nil {
		return nil, err
	}
	encrypted, err := a.encryption.Encrypt(raw[5:])
	if err != nil {
		return nil, err
	}
	return append(append([]byte(nil), raw[:5]...), encrypted...), nil
}

func (a *RawHeaderAssembler) Reassemble(data []byte) ([][]byte, error) {
	if len(a.buffer) != 0 {
		data = append(a.buffer, data...)
		a.buffer = nil
	}
	var payloads [][]byte
	for len(data) > 0 {
		start := bytes.IndexByte(data, packetPrefix)
		if start < 0 {
			break
		}
		data = data[start:]
		if len(data) < 5 {
			a.buffer = append([]byte(nil), data...)
			break
		}
		if crc8CCITT(data[:4]) != data[4] {
			data = data[1:]
			continue
		}
		payloadLen := int(binary.LittleEndian.Uint16(data[2:4]))
		version := data[1]
		innerLen := packetBodyLen(version, payloadLen)
		encryptedLen := ((innerLen + aes.BlockSize - 1) / aes.BlockSize) * aes.BlockSize
		frameLen := 5 + encryptedLen
		if len(data) < frameLen {
			a.buffer = append([]byte(nil), data...)
			break
		}
		body, err := a.encryption.Decrypt(data[5:frameLen])
		if err != nil {
			return nil, err
		}
		if len(body) < innerLen {
			return nil, fmt.Errorf("decrypted packet body too short: got %d bytes, need %d", len(body), innerLen)
		}
		payloads = append(payloads, append(append([]byte(nil), data[:5]...), body[:innerLen]...))
		data = data[frameLen:]
	}
	return payloads, nil
}

func packetBodyLen(version byte, payloadLen int) int {
	if version == 0x04 {
		return 5 + payloadLen
	}
	overhead := 13
	if version&0x0F >= 0x03 {
		overhead = 15
	}
	// Sentinel-format replies include their terminator in payloadLen and
	// have no trailing CRC16. Compare the low nibble for the header layout.
	if version&0x10 != 0 {
		overhead -= 2
	}
	return overhead + payloadLen
}

type SimplePacketAssembler struct {
	buffer []byte
}

func (a *SimplePacketAssembler) Encode(payload []byte) ([]byte, error) {
	return EncPacket{FrameType: 0x00, PayloadType: 0x00, Payload: payload}.MarshalBinary()
}

func (a *SimplePacketAssembler) Parse(data []byte) ([]byte, bool) {
	if len(a.buffer) != 0 {
		data = append(a.buffer, data...)
		a.buffer = nil
	}
	for len(data) > 0 {
		start := bytes.Index(data, []byte{0x5A, 0x5A})
		if start < 0 {
			if data[len(data)-1] == 0x5A {
				a.buffer = []byte{0x5A}
			}
			return nil, false
		}
		data = data[start:]
		if len(data) < 8 {
			a.buffer = append([]byte(nil), data...)
			return nil, false
		}
		payloadLen := int(binary.LittleEndian.Uint16(data[4:6]))
		if payloadLen < 2 || payloadLen > 10000 {
			data = data[2:]
			continue
		}
		frameLen := 6 + payloadLen
		if len(data) < frameLen {
			a.buffer = append([]byte(nil), data...)
			return nil, false
		}
		payload := data[6 : frameLen-2]
		want := binary.LittleEndian.Uint16(data[frameLen-2 : frameLen])
		if crc16ARC(data[:frameLen-2]) != want {
			data = data[2:]
			continue
		}
		a.buffer = append([]byte(nil), data[frameLen:]...)
		return append([]byte(nil), payload...), true
	}
	return nil, false
}

func pkcs7Pad(src []byte, blockSize int) ([]byte, error) {
	if blockSize < 1 || blockSize > 255 {
		return nil, fmt.Errorf("invalid pkcs7 block size %d", blockSize)
	}
	// Only EcoFlow packets and handshake messages are encrypted. Bound the
	// input before adding padding, including on 32-bit ARM hosts.
	if len(src) > maxPacketSize {
		return nil, fmt.Errorf("plaintext too large: %d bytes, maximum %d", len(src), maxPacketSize)
	}
	pad := blockSize - (len(src) % blockSize)
	out := make([]byte, len(src)+pad)
	copy(out, src)
	for i := len(src); i < len(out); i++ {
		out[i] = byte(pad)
	}
	return out, nil
}

func pkcs7Unpad(src []byte, blockSize int) ([]byte, error) {
	if len(src) == 0 || len(src)%blockSize != 0 {
		return nil, fmt.Errorf("invalid pkcs7 data length")
	}
	pad := int(src[len(src)-1])
	if pad == 0 || pad > blockSize || pad > len(src) {
		return nil, fmt.Errorf("invalid pkcs7 padding")
	}
	for _, b := range src[len(src)-pad:] {
		if int(b) != pad {
			return nil, fmt.Errorf("invalid pkcs7 padding")
		}
	}
	return src[:len(src)-pad], nil
}
