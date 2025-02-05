package peer

import (
	"encoding/binary"

	"github.com/gotolive/sfu/rtc"
	"github.com/gotolive/sfu/rtc/nack"
	"github.com/pion/rtcp"
)

type SenderStreamListener interface {
	OnRTPStreamRetransmitRTPPacket(packet rtc.Packet)
}
type SenderStream interface {
	Stream
	ReceiveNack(report *rtcp.TransportLayerNack)
	GetRtcpSenderReport(ms int64) rtcp.Packet
	GetRtcpSdesChunk() rtcp.Packet
	ReceiveRtcpReceiverReport(report rtcp.ReceptionReport)
	ReceivePacket(packet rtc.Packet) error
}

func NewSenderStream(lis SenderStreamListener, mediaType string, stream StreamOption, codec Codec) SenderStream {
	s := &senderStream{
		listener:       lis,
		internalStream: newStream(mediaType, stream, codec),
	}
	if stream.RTX != 0 && codec.RTX != 0 {
		s.internalStream.SetRtx(codec.RTX, stream.RTX)
	}

	s.nackSender = nack.NewSender(nack.NewBuffer(100), s.onRTP)
	return s
}

type senderStream struct {
	internalStream
	listener   SenderStreamListener
	nackSender nack.Sender
	rtxSeq     uint16
}

func (r *senderStream) ReceivePacket(packet rtc.Packet) error {
	r.stats.outcomingRTP(packet)
	if err := r.receivePacket(packet); err != nil {
		return err
	}
	if r.useNack {
		r.nackSender.ReceivePacket(packet)
	}
	return nil
}

func (r *senderStream) ReceiveNack(report *rtcp.TransportLayerNack) {
	if !r.useNack {
		return
	}
	r.nackSender.OnNack(report)
}

// GetRtcpSenderReport returns an RTCP sender report packet for the senderStream.
// It returns nil if no packets have been sent.
// The packet contains the SSRC, NTP time, RTP time, packet count, octet count, and profile extensions.
// The NTP time is calculated from the given milliseconds using the timeToNtp function.
// The RTP time is calculated from the maximum packet timestamp and the difference in milliseconds.
// The difference in milliseconds is calculated by subtracting the maximum packet milliseconds from the given milliseconds,
// and then multiplying it by the clock rate divided by 1000.
// The packet count and octet count are obtained from the transmission counter.
// The reports field is set to nil.
// The function returns a pointer to the packet.
func (r *senderStream) GetRtcpSenderReport(ms int64) rtcp.Packet {
	if r.stats.PacketsSent() == 0 {
		return nil
	}
	diffMs := ms - r.maxPacketMs
	diffTimestamp := diffMs * int64(r.GetClockRate()) / 1000
	packet := rtcp.SenderReport{
		SSRC:              r.SSRC(),
		NTPTime:           timeToNtp(ms),
		RTPTime:           r.maxPacketRTPTimestamp + uint32(diffTimestamp),
		PacketCount:       uint32(r.stats.PacketsSent()),
		OctetCount:        uint32(r.stats.BytesSent()),
		Reports:           nil,
		ProfileExtensions: nil,
	}
	return &packet
}

func (r *senderStream) GetRtcpSdesChunk() rtcp.Packet {
	packet := rtcp.SourceDescription{}
	packet.Chunks = append(packet.Chunks, rtcp.SourceDescriptionChunk{
		Source: 0,
		Items: []rtcp.SourceDescriptionItem{
			{
				Type: rtcp.SDESCNAME,
				Text: r.cname,
			},
		},
	})
	return &packet
}

func (r *senderStream) ReceiveRtcpReceiverReport(report rtcp.ReceptionReport) {
	r.fractionLost = report.FractionLost
}

func (r *senderStream) onRTP(packet rtc.Packet) {
	if r.rtxStream != nil {
		r.rtxSeq++
		packet.SetPayloadType(r.rtxPayloadType)
		packet.SetSsrc(r.rtxSSRC)
		seqByte := make([]byte, 2)
		packet.SetRTX(true)
		binary.BigEndian.PutUint16(seqByte, packet.SequenceNumber())
		packet.Packet().Payload = append(seqByte, packet.Packet().Payload...)
		packet.SetSequenceNumber(r.rtxSeq)
	}
	r.listener.OnRTPStreamRetransmitRTPPacket(packet)
}
