package peer

import (
	"encoding/binary"
	"errors"
	"time"

	"github.com/gotolive/sfu/rtc"
	"github.com/gotolive/sfu/rtc/logger"
	"github.com/pion/rtp"
)

var ErrInvalidRtx = errors.New("invalid rtx")

type rtpPacket struct {
	packet             rtp.Packet
	ms                 int64
	rtx                bool
	headerExtensionIds rtc.HeaderExtensionIDs
	payloadDescriptor  rtc.PayloadDescriptor
}

func (p *rtpPacket) Parse(d []byte) error {
	p.ms = time.Now().UnixMilli()
	return p.packet.Unmarshal(d)
}

func (p *rtpPacket) HeaderExtensions() []rtp.Extension {
	return p.packet.Extensions
}

func (p *rtpPacket) ReceiveMS() int64 {
	return p.ms
}

func (p *rtpPacket) SetRTX(b bool) {
	p.rtx = true
}

func (p *rtpPacket) SetHeaderExtensionIDs(h rtc.HeaderExtensionIDs) {
	p.headerExtensionIds = h
}

func (p *rtpPacket) SetMidExtensionID(mid rtc.HeaderExtensionID) {
	p.headerExtensionIds[rtc.HeaderExtensionMid] = rtc.HeaderExtension{
		URI: rtc.HeaderExtensionMid,
		ID:  mid,
	}
}

func (p *rtpPacket) SSRC() uint32 {
	return p.packet.SSRC
}

func (p *rtpPacket) Mid(mid rtc.HeaderExtensionID) string {
	return string(p.packet.GetExtension(uint8(mid)))
}

func (p *rtpPacket) Rid(rid rtc.HeaderExtensionID) string {
	return string(p.packet.GetExtension(uint8(rid)))
}

// Rrid is for rid in rtx packet
func (p *rtpPacket) RRid(rrid rtc.HeaderExtensionID) string {
	return string(p.packet.GetExtension(uint8(rrid)))
}

func (p *rtpPacket) PayloadType() rtc.PayloadType {
	return rtc.PayloadType(p.packet.PayloadType)
}

func (p *rtpPacket) SetPayloadType(payloadType rtc.PayloadType) {
	p.packet.PayloadType = uint8(payloadType)
}

func (p *rtpPacket) SetSsrc(i uint32) {
	p.packet.SSRC = i
}

func (p *rtpPacket) IsKeyFrame() bool {
	if p.payloadDescriptor == nil {
		return false
	}
	return p.payloadDescriptor.IsKeyFrame()
}

func (p *rtpPacket) ReadTransportWideCc01(tcc uint8) (uint16, bool) {
	res := p.packet.GetExtension(tcc)
	if len(res) != 2 {
		return 0, false
	}
	return binary.BigEndian.Uint16(res), true
}

func (p *rtpPacket) ReadAbsSendTime(abs uint8) (uint32, bool) {
	res := p.packet.GetExtension(abs)
	if len(res) == 0 {
		return 0, false
	}
	// 3 bytes
	for len(res) < 4 {
		res = append([]byte{0}, res...)
	}
	return binary.BigEndian.Uint32(res), true
}

func (p *rtpPacket) SequenceNumber() uint16 {
	return p.packet.SequenceNumber
}

func (p *rtpPacket) SetSequenceNumber(seq uint16) {
	p.packet.SequenceNumber = seq
}

const (
	uint64ByteCount = 8
)

func (p *rtpPacket) UpdateAbsSendTime(id rtc.HeaderExtensionID, nowMs time.Time) {
	abs := rtp.NewAbsSendTimeExtension(nowMs)
	payload, err := abs.Marshal()
	if err != nil {
		logger.Error("something wrong:", err)
		// should not happened
		return
	}
	err = p.packet.SetExtension(uint8(id), payload)
	if err != nil {
		logger.Error("something wrong:", err)
	}
}

func (p *rtpPacket) UpdateTransportWideCc01(i int) bool {
	payload := make([]byte, uint64ByteCount)
	binary.BigEndian.PutUint64(payload, uint64(i))
	err := p.packet.SetExtension(uint8(p.headerExtensionIds.TransportWideCC()), payload)
	if err != nil {
		logger.Error("something wrong:", err)
		return false
	}
	return true
}

func (p *rtpPacket) Size() int {
	return p.packet.MarshalSize()
}

func (p *rtpPacket) HasMarker() bool {
	return p.packet.Marker
}

func (p *rtpPacket) Resolution() (int, int, bool) {
	return 960, 540, true
	// if p.payloadDescriptor == nil {
	//	return 0, 0, false
	// }
	// return p.payloadDescriptor.Resolution()
}

func (p *rtpPacket) ProfileLevelID() (int, bool) {
	return 0x42001f, true
	// if p.payloadDescriptor == nil {
	//	return 0, false
	// }
	// return p.payloadDescriptor.ProfileLevelID()
}

func (p *rtpPacket) Timestamp() uint32 {
	return p.packet.Timestamp
}

func (p *rtpPacket) Payload() []byte {
	return p.packet.Payload
}

func (p *rtpPacket) PayloadLength() int {
	return len(p.packet.Payload)
}

func (p *rtpPacket) Packet() *rtp.Packet {
	return &p.packet
}

func (p *rtpPacket) SetPayloadDescriptor(pd rtc.PayloadDescriptor) {
	p.payloadDescriptor = pd
}

func (p *rtpPacket) Marshal() ([]byte, error) {
	return p.packet.Marshal()
}

func (p *rtpPacket) MarshalTo(dst []byte) (int, error) {
	return p.packet.MarshalTo(dst)
}

func (p *rtpPacket) UpdateMid(mid string) {
	err := p.packet.SetExtension(uint8(p.headerExtensionIds.Mid()), []byte(mid))
	if err != nil {
		logger.Error("something wrong:", err)
	}
}

func (p *rtpPacket) RtxDecode(payloadType rtc.PayloadType, ssrc uint32) error {
	if len(p.packet.Payload) < 2 {
		return ErrInvalidRtx
	}
	p.packet.Header.PayloadType = uint8(payloadType)
	p.packet.Header.SequenceNumber = binary.BigEndian.Uint16(p.packet.Payload)
	p.packet.Header.SSRC = ssrc
	p.packet.Payload = p.packet.Payload[2:]
	p.rtx = true
	return nil
}

func (p *rtpPacket) TemporalLayer() int {
	if p.payloadDescriptor == nil {
		return 0
	}
	return 0
	// return p.payloadDescriptor.TemporalLayer()
}

func (p *rtpPacket) SetTimestamp(timestamp uint32) {
	p.packet.Timestamp = timestamp
}

func (p *rtpPacket) UpdateHeader(extensions []rtp.Extension) {
	p.packet.Header.Extensions = extensions
}

func (p *rtpPacket) IsRTX() bool {
	return p.rtx
}
