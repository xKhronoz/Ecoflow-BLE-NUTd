package ecoflow

import (
	"encoding/binary"
	"time"

	"github.com/xkhronoz/ecoflow-ble-nutd/internal/ecoflow/pb/utcsyncpb"
	"google.golang.org/protobuf/proto"
)

type timeResponder struct {
	lastSent time.Time
}

func (t *timeResponder) Packets(now time.Time) ([]Packet, bool, error) {
	if !t.lastSent.IsZero() && now.Sub(t.lastSent) < 30*time.Second {
		return nil, false, nil
	}
	t.lastSent = now
	utcTime := uint32(now.Unix())
	msg := &utcsyncpb.SysUTCSync{SysUtcTime: &utcTime}
	payload, err := proto.Marshal(msg)
	if err != nil {
		return nil, false, err
	}
	rtcPayload := make([]byte, 6)
	binary.LittleEndian.PutUint32(rtcPayload[:4], utcTime)
	tzMajor, tzMinor := timezoneOffset(now)
	rtcPayload[4] = byte(int8(tzMajor))
	rtcPayload[5] = byte(int8(tzMinor))
	return []Packet{
		{Src: 0x21, Dst: 0x0B, CmdSet: 0x01, CmdID: 0x55, Payload: payload, DSrc: 0x01, DDst: 0x01, Version: 0x13},
		{Src: 0x21, Dst: 0x35, CmdSet: 0x01, CmdID: 0x52, Payload: rtcPayload, DSrc: 0x01, DDst: 0x01, Version: 0x03},
		{Src: 0x21, Dst: 0x35, CmdSet: 0x01, CmdID: 0x53, Payload: rtcPayload, DSrc: 0x01, DDst: 0x01, Version: 0x03},
	}, true, nil
}

func timezoneOffset(now time.Time) (int, int) {
	_, secondsEast := now.Zone()
	offset := float64(secondsEast) / 3600
	major := int(offset)
	minor := int((offset - float64(major)) * 100)
	return major, minor
}

func isTimeRequest(packet Packet) bool {
	return packet.Src == 0x35 && packet.CmdSet == 0x01 && packet.CmdID == 0x52 && len(packet.Payload) == 0
}
