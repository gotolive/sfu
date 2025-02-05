package peer

import (
	"reflect"
	"testing"
	"time"

	"github.com/gotolive/sfu/rtc"
	rtp2 "github.com/pion/rtp"
)

func TestStream(t *testing.T) {
	tests := []testHelper{
		{
			name:        "Test Init Seq",
			description: "",
			method: func(t *testing.T) {
				s := newStream(rtc.MediaTypeVideo, StreamOption{}, Codec{})
				s.receivePacket(&pkt{seq: 1000})
				assert(t, s.getExpectedPackets(), int64(1))
			},
		},
		{
			name:        "Test Real Seq",
			description: "",
			method: func(t *testing.T) {
				seq := []*pkt{
					{seq: 1000},
					{seq: 1001},
					{seq: 1002},
					{seq: 1003},
					{seq: 1004},
					{seq: 1005},
					{seq: 1006},
					{seq: 1007},
					{seq: 1008},
					{seq: 1009},
				}
				s := newStream(rtc.MediaTypeVideo, StreamOption{}, Codec{})
				for _, p := range seq {
					s.receivePacket(p)
				}
				assert(t, s.getExpectedPackets(), int64(10))
			},
		},

		{
			name:        "Test misorder seq",
			description: "",
			method: func(t *testing.T) {
				seq := []*pkt{
					{seq: 1000},
					{seq: 1001},
					{seq: 1004},
					{seq: 1003},
					{seq: 1002},
					{seq: 1005},
					{seq: 1006},
					{seq: 1009},
					{seq: 1007},
					{seq: 1008},
				}
				s := newStream(rtc.MediaTypeVideo, StreamOption{}, Codec{})
				for _, p := range seq {
					s.receivePacket(p)
				}
				assert(t, s.getExpectedPackets(), int64(10))
			},
		},

		{
			name:        "Test lost seq",
			description: "",
			method: func(t *testing.T) {
				seq := []*pkt{
					{seq: 1000},
					{seq: 1009},
				}
				s := newStream(rtc.MediaTypeVideo, StreamOption{}, Codec{})
				for _, p := range seq {
					s.receivePacket(p)
				}
				assert(t, s.getExpectedPackets(), int64(10))
			},
		},

		{
			name:        "Test wrap seq",
			description: "",
			method: func(t *testing.T) {
				seq := []*pkt{
					{seq: 65534},
					{seq: 65535},
					{seq: 0},
					{seq: 1},
					{seq: 2},
					{seq: 3},
					{seq: 4},
					{seq: 5},
					{seq: 6},
					{seq: 7},
				}
				s := newStream(rtc.MediaTypeVideo, StreamOption{}, Codec{})
				for _, p := range seq {
					s.receivePacket(p)
				}
				assert(t, s.getExpectedPackets(), int64(10))
			},
		},
		{
			name:        "Test drop seq",
			description: "",
			method: func(t *testing.T) {
				seq := []*pkt{
					{seq: 3000, timestamp: 1, receiveMs: 1},
					{seq: 3001, timestamp: 2, receiveMs: 2},
					{seq: 7001, timestamp: 5, receiveMs: 5},
					{seq: 7002, timestamp: 6, receiveMs: 6},
				}
				s := newStream(rtc.MediaTypeVideo, StreamOption{}, Codec{})
				for _, p := range seq {
					s.receivePacket(p)
				}
				assert(t, s.getExpectedPackets(), int64(2))
				assert(t, s.packetDiscarded, 2)
				assert(t, s.maxPacketRTPTimestamp, uint32(2))
				assert(t, s.maxPacketMs, int64(2))
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.method(t)
		})
	}
}

func assert(t *testing.T, actual, expected any) {
	if !reflect.DeepEqual(actual, expected) {
		t.Logf("%v expected: %v, but got: %v", t.Name(), expected, actual)
		t.FailNow()
	}
}

type testHelper struct {
	name        string
	description string
	method      func(t *testing.T)
}

type pkt struct {
	seq         uint16
	key         bool
	rtx         bool
	timestamp   uint32
	receiveMs   int64
	payloadType rtc.PayloadType
	marker      bool
	size        int
	ssrc        uint32
}

func (p *pkt) RRid(rrid rtc.HeaderExtensionID) string {
	//TODO implement me
	panic("implement me")
}

func (p *pkt) Parse(d []byte) error {
	// TODO implement me
	panic("implement me")
}

func (p *pkt) UpdateAbsSendTime(id rtc.HeaderExtensionID, nowMs time.Time) {
	// TODO implement me
	panic("implement me")
}

func (p *pkt) UpdateHeader(extensions []rtp2.Extension) {
	// TODO implement me
	panic("implement me")
}

func (p *pkt) HeaderExtensions() []rtp2.Extension {
	// TODO implement me
	panic("implement me")
}

func (p *pkt) ReceiveMS() int64 {
	return p.receiveMs
}

func (p *pkt) SetRTX(b bool) {
	p.rtx = b
}

func (p *pkt) SetHeaderExtensionIDs(h rtc.HeaderExtensionIDs) {
	// TODO implement me
	panic("implement me")
}

func (p *pkt) SetMidExtensionID(mid rtc.HeaderExtensionID) {
	// TODO implement me
	panic("implement me")
}

func (p *pkt) SSRC() uint32 {
	return p.ssrc
}

func (p *pkt) Mid(mid rtc.HeaderExtensionID) string {
	// TODO implement me
	panic("implement me")
}

func (p *pkt) Rid(rid rtc.HeaderExtensionID) string {
	// TODO implement me
	panic("implement me")
}

func (p *pkt) PayloadType() rtc.PayloadType {
	return p.payloadType
}

func (p *pkt) SetPayloadType(payloadType rtc.PayloadType) {
}

func (p *pkt) SetSsrc(i uint32) {
}

func (p *pkt) IsKeyFrame() bool {
	return p.key
}

func (p *pkt) ReadTransportWideCc01(tcc uint8) (uint16, bool) {
	// TODO implement me
	panic("implement me")
}

func (p *pkt) ReadAbsSendTime(abs uint8) (uint32, bool) {
	// TODO implement me
	panic("implement me")
}

func (p *pkt) SequenceNumber() uint16 {
	return p.seq
}

func (p *pkt) SetSequenceNumber(seq uint16) {
}

func (p *pkt) UpdateTransportWideCc01(i int) bool {
	// TODO implement me
	panic("implement me")
}

func (p *pkt) Size() int {
	return p.size
}

func (p *pkt) HasMarker() bool {
	return p.marker
}

func (p *pkt) Resolution() (int, int, bool) {
	// TODO implement me
	panic("implement me")
}

func (p *pkt) ProfileLevelID() (int, bool) {
	// TODO implement me
	panic("implement me")
}

func (p *pkt) Timestamp() uint32 {
	return p.timestamp
}

func (p *pkt) Payload() []byte {
	// TODO implement me
	panic("implement me")
}

func (p *pkt) PayloadLength() int {
	// TODO implement me
	panic("implement me")
}

func (p *pkt) Packet() *rtp2.Packet {
	return &rtp2.Packet{}
}

func (p *pkt) SetPayloadDescriptor(pd rtc.PayloadDescriptor) {
	// TODO implement me
	panic("implement me")
}

func (p *pkt) Marshal() ([]byte, error) {
	// TODO implement me
	panic("implement me")
}

func (p *pkt) UpdateMid(mid string) {
	// TODO implement me
	panic("implement me")
}

func (p *pkt) RtxDecode(payloadType rtc.PayloadType, ssrc uint32) error {
	return nil
}

func (p *pkt) TemporalLayer() int {
	// TODO implement me
	panic("implement me")
}

func (p *pkt) SetTimestamp(timestamp uint32) {
	// TODO implement me
	panic("implement me")
}

func (p *pkt) RestorePayload() {
	// TODO implement me
	panic("implement me")
}

func (p *pkt) IsRTX() bool {
	return p.rtx
}

func buildPacket(seq uint16, key bool, rtx bool) rtc.Packet {
	return &pkt{
		seq: seq,
		key: key,
		rtx: rtx,
	}
}
