package ecoflow

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
)

const (
	packetPrefix    = 0xAA
	encPacketPrefix = 0x5A5A
)

type Packet struct {
	Src       byte
	Dst       byte
	CmdSet    byte
	CmdID     byte
	Payload   []byte
	DSrc      byte
	DDst      byte
	Version   byte
	Seq       [4]byte
	ProductID byte
}

func (p Packet) MarshalBinary() []byte {
	payload := p.Payload
	if payload == nil {
		payload = []byte{}
	}
	buf := bytes.NewBuffer(make([]byte, 0, 5+15+len(payload)))
	buf.WriteByte(packetPrefix)
	buf.WriteByte(p.Version)
	_ = binary.Write(buf, binary.LittleEndian, uint16(len(payload)))
	buf.WriteByte(crc8CCITT(buf.Bytes()))
	product := p.ProductID
	if product == 0 {
		product = 0x0D
	}
	buf.WriteByte(product)
	buf.Write(p.Seq[:])
	buf.Write([]byte{0x00, 0x00})
	buf.WriteByte(p.Src)
	buf.WriteByte(p.Dst)
	if p.Version >= 0x03 {
		dsrc := p.DSrc
		ddst := p.DDst
		if dsrc == 0 {
			dsrc = 0x01
		}
		if ddst == 0 {
			ddst = 0x01
		}
		buf.WriteByte(dsrc)
		buf.WriteByte(ddst)
	}
	buf.WriteByte(p.CmdSet)
	buf.WriteByte(p.CmdID)
	buf.Write(payload)
	crc := crc16ARC(buf.Bytes())
	_ = binary.Write(buf, binary.LittleEndian, crc)
	return buf.Bytes()
}

func ParsePacket(data []byte, xorPayload bool) (Packet, error) {
	if len(data) < 5 || data[0] != packetPrefix {
		return Packet{}, fmt.Errorf("invalid packet prefix")
	}
	version := data[1]
	if version == 0x04 {
		return Packet{}, fmt.Errorf("v4 packets are not supported")
	}
	payloadLen := int(binary.LittleEndian.Uint16(data[2:4]))
	if crc8CCITT(data[:4]) != data[4] {
		return Packet{}, fmt.Errorf("invalid packet header crc8")
	}
	packetVersion := version & 0x0F
	sentinelFormat := version&0x10 != 0
	payloadStart := 16
	if packetVersion >= 0x03 {
		payloadStart = 18
	}
	totalLen := payloadStart + payloadLen + 2
	if sentinelFormat {
		totalLen = payloadStart + payloadLen
	}
	if len(data) < totalLen {
		return Packet{}, fmt.Errorf("incomplete packet")
	}
	if !sentinelFormat {
		want := binary.LittleEndian.Uint16(data[totalLen-2 : totalLen])
		if crc16ARC(data[:totalLen-2]) != want {
			return Packet{}, fmt.Errorf("invalid packet crc16")
		}
	}
	var pkt Packet
	pkt.Version = version
	pkt.ProductID = data[5]
	copy(pkt.Seq[:], data[6:10])
	pkt.Src = data[12]
	pkt.Dst = data[13]
	if packetVersion >= 0x03 {
		pkt.DSrc = data[14]
		pkt.DDst = data[15]
		pkt.CmdSet = data[16]
		pkt.CmdID = data[17]
	} else {
		pkt.CmdSet = data[14]
		pkt.CmdID = data[15]
	}
	if payloadLen > 0 {
		payload := append([]byte(nil), data[payloadStart:payloadStart+payloadLen]...)
		if xorPayload && pkt.Seq[0] != 0 {
			for i := range payload {
				payload[i] ^= pkt.Seq[0]
			}
		}
		if sentinelFormat && len(payload) >= 2 && bytes.Equal(payload[len(payload)-2:], []byte{0xBB, 0xBB}) {
			payload = payload[:len(payload)-2]
		}
		pkt.Payload = payload
	}
	return pkt, nil
}

type EncPacket struct {
	FrameType   byte
	PayloadType byte
	Payload     []byte
}

func (e EncPacket) MarshalBinary() []byte {
	payload := e.Payload
	buf := bytes.NewBuffer(make([]byte, 0, 8+len(payload)))
	_ = binary.Write(buf, binary.BigEndian, uint16(encPacketPrefix))
	buf.WriteByte(e.FrameType << 4)
	buf.WriteByte(0x01)
	_ = binary.Write(buf, binary.LittleEndian, uint16(len(payload)+2))
	buf.Write(payload)
	crc := crc16ARC(buf.Bytes())
	_ = binary.Write(buf, binary.LittleEndian, crc)
	return buf.Bytes()
}

var errIncompleteFrame = errors.New("incomplete frame")
