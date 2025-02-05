package nack

import (
	"testing"
	"time"

	"github.com/gotolive/sfu/rtc"
	"github.com/pion/rtcp"
	rtp2 "github.com/pion/rtp"
)

func TestNewNackGenerator(t *testing.T) {
	t.Run("should be no nack", func(t *testing.T) {
		callCh := make(chan struct{})
		nack := NewReceiver(func(packet rtcp.Packet) {
			go func() {
				callCh <- struct{}{}
			}()
		})
		defer nack.Close()
		noacks := []struct {
			seq              uint16
			isKeyFrame       bool
			firstNacked      uint16
			numAck           int
			keyFrameRequired bool
			nackListSize     int
		}{
			{2371, false, 0, 0, false, 0},
			{2372, false, 0, 0, false, 0},
			{2373, false, 0, 0, false, 0},
			{2374, false, 0, 0, false, 0},
			{2375, false, 0, 0, false, 0},
			{2376, false, 0, 0, false, 0},
			{2377, false, 0, 0, false, 0},
			{2378, false, 0, 0, false, 0},
			{2379, false, 0, 0, false, 0},
			{2380, false, 0, 0, false, 0},
			{2254, false, 0, 0, false, 0},
			{2250, false, 0, 0, false, 0},
		}
		for _, v := range noacks {
			pkt := buildPacket(v.seq, v.isKeyFrame, false)
			nack.IncomingPacket(0, pkt)
		}
		select {
		case <-time.After(time.Second):
		case <-callCh:
			t.Fatal("should not be called")
		}
	})

	t.Run("nack should be called", func(t *testing.T) {
		callCh := make(chan struct{})
		nack := NewReceiver(func(packet rtcp.Packet) {
			go func() {
				callCh <- struct{}{}
			}()
			batch := packet.(*rtcp.TransportLayerNack)
			if len(batch.Nacks) != 1 || batch.Nacks[0].PacketID != 2382 {
				t.Fatal("call wrong")
			}
		})
		defer nack.Close()
		noacks := []struct {
			seq              uint16
			isKeyFrame       bool
			firstNacked      uint16
			numAck           int
			keyFrameRequired bool
			nackListSize     int
		}{
			{2381, false, 0, 0, false, 0},
			{2383, false, 2382, 1, false, 1},
		}
		for _, v := range noacks {
			pkt := buildPacket(v.seq, v.isKeyFrame, false)
			nack.IncomingPacket(0, pkt)
		}

		select {
		case <-time.After(time.Second):
			t.Fatal("should be called")
		case <-callCh:
		}
	})

	t.Run("no ack max", func(t *testing.T) {
		callCh := make(chan struct{})
		nack := NewReceiver(func(packet rtcp.Packet) {
			go func() {
				callCh <- struct{}{}
			}()
		})
		defer nack.Close()
		noacks := []struct {
			seq              uint16
			isKeyFrame       bool
			firstNacked      uint16
			numAck           int
			keyFrameRequired bool
			nackListSize     int
		}{
			{65534, false, 0, 0, false, 0},
			{65535, false, 0, 0, false, 0},
			{0, false, 0, 0, false, 0},
		}
		for _, v := range noacks {
			pkt := buildPacket(v.seq, v.isKeyFrame, false)
			nack.IncomingPacket(0, pkt)
		}
		select {
		case <-time.After(time.Second):
		case <-callCh:
			t.Fatal("should not be called")
		}
	})

	t.Run("nack should be called", func(t *testing.T) {
		callCh := make(chan struct{})
		nack := NewReceiver(func(packet rtcp.Packet) {
			go func() {
				callCh <- struct{}{}
			}()
			batch := packet.(*rtcp.TransportLayerNack)
			if len(batch.Nacks) != 1 || batch.Nacks[0].PacketID != 0 {
				t.Fatal("call wrong")
			}
		})
		defer nack.Close()
		noacks := []struct {
			seq              uint16
			isKeyFrame       bool
			firstNacked      uint16
			numAck           int
			keyFrameRequired bool
			nackListSize     int
		}{
			{65534, false, 0, 0, false, 0},
			{65535, false, 0, 0, false, 0},
			{1, false, 0, 1, false, 1},
		}
		for _, v := range noacks {
			pkt := buildPacket(v.seq, v.isKeyFrame, false)
			nack.IncomingPacket(0, pkt)
		}
		select {
		case <-time.After(time.Second):
			t.Fatal("should be called")
		case <-callCh:
		}
	})

	t.Run("nack should be called MY", func(t *testing.T) {
		callCh := make(chan struct{})
		nack := NewReceiver(func(packet rtcp.Packet) {
			go func() {
				callCh <- struct{}{}
			}()
		})
		defer nack.Close()
		noacks := []struct {
			seq              uint16
			isKeyFrame       bool
			firstNacked      uint16
			numAck           int
			keyFrameRequired bool
			nackListSize     int
		}{
			{65534, false, 0, 0, false, 0},
			{65535, false, 0, 0, false, 0},
			{1, false, 0, 1, false, 1},
			{11, false, 2, 9, false, 10},
			{12, true, 0, 0, false, 10},
			{13, true, 0, 0, false, 0},
		}
		for _, v := range noacks {
			pkt := buildPacket(v.seq, v.isKeyFrame, false)
			nack.IncomingPacket(0, pkt)
		}
		select {
		case <-time.After(time.Second):
			t.Fatal("should be called")
		case <-callCh:
		}
	})

	t.Run("nack should remove keyframe", func(t *testing.T) {
		callCh := make(chan struct{})
		nack := NewReceiver(func(packet rtcp.Packet) {
			go func() {
				callCh <- struct{}{}
			}()
		})
		defer nack.Close()
		noacks := []struct {
			seq              uint16
			isKeyFrame       bool
			firstNacked      uint16
			numAck           int
			keyFrameRequired bool
			nackListSize     int
		}{
			{1, true, 0, 0, false, 0},
			{500, true, 0, 0, false, 0},
			{3000, true, 0, 1, false, 1},
			{3001, false, 2, 9, false, 10},
			{3001, true, 0, 0, false, 10},
			{13, true, 0, 0, false, 0},
		}
		for _, v := range noacks {
			pkt := buildPacket(v.seq, v.isKeyFrame, false)
			nack.IncomingPacket(0, pkt)
		}
		select {
		case <-time.After(time.Second):
			t.Fatal("should be called")
		case <-callCh:
		}
	})

	t.Run("nack should recover", func(t *testing.T) {
		callCh := make(chan struct{})
		nack := NewReceiver(func(packet rtcp.Packet) {
			go func() {
				callCh <- struct{}{}
			}()
		})
		defer nack.Close()
		noacks := []struct {
			seq              uint16
			isKeyFrame       bool
			firstNacked      uint16
			numAck           int
			keyFrameRequired bool
			nackListSize     int
		}{
			{1, true, 0, 0, false, 0},
			{2, true, 0, 0, false, 0},
			{3, true, 0, 1, false, 1},
			{5, false, 2, 9, false, 10},
			{4, true, 0, 0, false, 10},
			{6, true, 0, 0, false, 0},
		}
		for _, v := range noacks {
			pkt := buildPacket(v.seq, v.isKeyFrame, false)
			nack.IncomingPacket(0, pkt)
		}
		select {
		case <-time.After(time.Second):
			t.Fatal("should be called")
		case <-callCh:
		}
	})

	t.Run("nack should not be called", func(t *testing.T) {
		callCh := make(chan struct{})
		nack := NewReceiver(func(packet rtcp.Packet) {
			go func() {
				callCh <- struct{}{}
			}()
		})
		defer nack.Close()
		noacks := []struct {
			seq              uint16
			isKeyFrame       bool
			rtx              bool
			firstNacked      uint16
			numAck           int
			keyFrameRequired bool
			nackListSize     int
		}{
			{1, true, false, 0, 0, false, 0},
			{2, true, false, 0, 0, false, 0},
			{3, true, false, 0, 1, false, 1},
			{5, false, true, 2, 9, false, 10},
			{4, true, false, 0, 0, false, 10},
			{6, true, false, 0, 0, false, 0},
		}
		for _, v := range noacks {
			pkt := buildPacket(v.seq, v.isKeyFrame, v.rtx)
			nack.IncomingPacket(0, pkt)
		}
		select {
		case <-time.After(time.Second):

		case <-callCh:
			t.Fatal("should not be called")
		}
	})
}

type pkt struct {
	seq uint16
	key bool
	rtx bool
}

func (p *pkt) RRid(rrid rtc.HeaderExtensionID) string {
	panic("implement me")
}

func (p *pkt) Parse(d []byte) error {
	panic("implement me")
}

func (p *pkt) UpdateAbsSendTime(id rtc.HeaderExtensionID, nowMs time.Time) {
	panic("implement me")
}

func (p *pkt) UpdateHeader(extensions []rtp2.Extension) {
	panic("implement me")
}

func (p *pkt) HeaderExtensions() []rtp2.Extension {
	panic("implement me")
}

func (p *pkt) ReceiveMS() int64 {

	panic("implement me")
}

func (p *pkt) SetRTX(b bool) {

	panic("implement me")
}

func (p *pkt) SetHeaderExtensionIDs(h rtc.HeaderExtensionIDs) {

	panic("implement me")
}

func (p *pkt) SetMidExtensionID(mid rtc.HeaderExtensionID) {

	panic("implement me")
}

func (p *pkt) SSRC() uint32 {

	panic("implement me")
}

func (p *pkt) Mid(mid rtc.HeaderExtensionID) string {

	panic("implement me")
}

func (p *pkt) Rid(rid rtc.HeaderExtensionID) string {

	panic("implement me")
}

func (p *pkt) PayloadType() rtc.PayloadType {

	panic("implement me")
}

func (p *pkt) SetPayloadType(payloadType rtc.PayloadType) {

	panic("implement me")
}

func (p *pkt) SetSsrc(i uint32) {

	panic("implement me")
}

func (p *pkt) IsKeyFrame() bool {
	return p.key
}

func (p *pkt) ReadTransportWideCc01(tcc uint8) (uint16, bool) {

	panic("implement me")
}

func (p *pkt) ReadAbsSendTime(abs uint8) (uint32, bool) {

	panic("implement me")
}

func (p *pkt) SequenceNumber() uint16 {
	return p.seq
}

func (p *pkt) SetSequenceNumber(seq uint16) {

	panic("implement me")
}

func (p *pkt) UpdateTransportWideCc01(i int) bool {

	panic("implement me")
}

func (p *pkt) Size() int {

	panic("implement me")
}

func (p *pkt) HasMarker() bool {

	panic("implement me")
}

func (p *pkt) Resolution() (int, int, bool) {

	panic("implement me")
}

func (p *pkt) ProfileLevelID() (int, bool) {

	panic("implement me")
}

func (p *pkt) Timestamp() uint32 {

	panic("implement me")
}

func (p *pkt) Payload() []byte {

	panic("implement me")
}

func (p *pkt) PayloadLength() int {

	panic("implement me")
}

func (p *pkt) Packet() *rtp2.Packet {

	panic("implement me")
}

func (p *pkt) SetPayloadDescriptor(pd rtc.PayloadDescriptor) {

	panic("implement me")
}

func (p *pkt) Marshal() ([]byte, error) {

	panic("implement me")
}

func (p *pkt) UpdateMid(mid string) {

	panic("implement me")
}

func (p *pkt) RtxDecode(payloadType rtc.PayloadType, ssrc uint32) error {

	panic("implement me")
}

func (p *pkt) TemporalLayer() int {

	panic("implement me")
}

func (p *pkt) SetTimestamp(timestamp uint32) {

	panic("implement me")
}

func (p *pkt) RestorePayload() {

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
