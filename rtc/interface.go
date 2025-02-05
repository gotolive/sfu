package rtc

import (
	"time"

	"github.com/pion/rtcp"
	"github.com/pion/rtp"
)

type SendRTCP func(rtcp.Packet)

// Packet as a interface could make other tests easier, but not final decision.
type Packet interface {
	SetHeaderExtensionIDs(h HeaderExtensionIDs)
	SSRC() uint32
	Mid(mid HeaderExtensionID) string
	Rid(rid HeaderExtensionID) string
	RRid(rrid HeaderExtensionID) string
	PayloadType() PayloadType
	SetPayloadType(payloadType PayloadType)
	SetSsrc(ssrc uint32)
	IsKeyFrame() bool
	ReadTransportWideCc01(tcc uint8) (uint16, bool)
	ReadAbsSendTime(abs uint8) (uint32, bool)
	SequenceNumber() uint16
	SetSequenceNumber(seq uint16)
	UpdateAbsSendTime(id HeaderExtensionID, nowMs time.Time)
	UpdateTransportWideCc01(i int) bool
	Size() int
	HasMarker() bool
	Resolution() (int, int, bool)
	ProfileLevelID() (int, bool)
	Timestamp() uint32
	// ReceiveMS is when we receive the packet
	ReceiveMS() int64
	Payload() []byte
	PayloadLength() int
	Packet() *rtp.Packet
	SetPayloadDescriptor(pd PayloadDescriptor)
	Marshal() ([]byte, error)
	RtxDecode(payloadType PayloadType, ssrc uint32) error
	TemporalLayer() int
	SetTimestamp(timestamp uint32)
	UpdateHeader(extensions []rtp.Extension)
	IsRTX() bool
	SetRTX(b bool)
	HeaderExtensions() []rtp.Extension
	Parse(d []byte) error
}

type PayloadDescriptor interface {
	IsKeyFrame() bool
}
