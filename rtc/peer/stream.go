package peer

import (
	"time"

	"github.com/gotolive/sfu/rtc"
	"github.com/gotolive/sfu/rtc/bwe/remb"
)

var _ Stream = new(internalStream)

const (
	ReceiveRTPPacketMedia          = "media"
	ReceiveRTPPacketDiscarded      = "discarded"
	ReceiveRTPPacketRetransmission = "retransmission"
)

const (
	RTPSeqMod   = uint32(1) << 16
	MaxDropout  = uint16(3000)
	MaxMisorder = uint16(1500)
)

// Stream is a internal wrap for parameter.StreamOption
type Stream interface {
	// SSRC return the ssrc of the stream. It could be zero if no ssrc set yet.
	SSRC() uint32
	// RtxSSRC return the rtx ssrc of the stream. It could be zero if no rtx ssrc set yet.
	RtxSSRC() uint32
	// PayloadType return the payload type of the stream.
	PayloadType() rtc.PayloadType
	// RtxPayloadType return the rtx payload type of the stream.
	RtxPayloadType() rtc.PayloadType
	// RID return the rid of the stream.
	RID() string
	// GetActiveMs return how long since last rtp rtpPacket.
	GetActiveMs() int64
	// GetClockRate return the clock rate of the stream Ccodec.
	GetClockRate() int

	Cname() string
	// GetMaxPacketTS return the max rtpPacket rtp timestamp of the stream.
	GetMaxPacketTS() uint32
	// FractionLost return the fractionLost.
	FractionLost() uint8

	Stats() *StreamStats
}

const (
	ntpEpoch = 2208988800
)

func timeToNtp(unixMilli int64) uint64 {
	// Convert Unix time to NTP format
	// Unix time starts from 1970, NTP time starts from 1900
	seconds := uint64(unixMilli/1000 + ntpEpoch)
	// 1<<32 = 1s,  1<<32*(ms/1000) = fractional = ms*1<<32/1000
	fractional := uint64((unixMilli % 1000) * (1 << 32) / 1000)
	// Combine the integer and fractional parts to form the NTP timestamp
	return seconds<<32 | fractional
}

func newStream(mediaType string, s StreamOption, codec Codec) internalStream {
	stream := internalStream{
		mediaType:      mediaType,
		ssrc:           s.SSRC,
		rtxSSRC:        s.RTX,
		rid:            s.RID,
		cname:          s.Cname,
		codec:          codec,
		payloadType:    codec.PayloadType,
		rtxPayloadType: codec.RTX,
		stats: &StreamStats{
			sendBps:    remb.NewRateStatistics(1000, 8000),
			receiveBps: remb.NewRateStatistics(1000, 8000),
		},
	}
	if v, ok := codec.Parameters["useinbandfec"]; ok && v == "1" {
		stream.useInBandFec = true
	}
	if v, ok := codec.Parameters["usedtx"]; ok && v == "1" {
		if s.Dtx {
			stream.useDtx = true
		}
	}

	for _, fb := range codec.FeedbackParams {
		if fb.Type == "nack" && len(fb.Parameter) == 0 {
			stream.useNack = true
		}
		if fb.Type == "nack" && fb.Parameter == "pli" {
			stream.usePli = true
		}
		if fb.Type == "ccm" && fb.Parameter == "fir" {
			stream.useFir = true
		}
	}
	return stream
}

type internalStream struct {
	mediaType      string
	ssrc           uint32
	rtxSSRC        uint32
	rid            string
	cname          string
	codec          Codec
	payloadType    rtc.PayloadType
	rtxPayloadType rtc.PayloadType
	rtxStream      *RtxStream

	stats *StreamStats

	usePli       bool
	useFir       bool
	useInBandFec bool
	useNack      bool
	useDtx       bool

	baseSeq uint16
	maxSeq  uint16
	cycles  uint32

	started               bool
	maxPacketRTPTimestamp uint32 // Max RTP Timestamp
	maxPacketMs           int64  // Max Received MS
	packetDiscarded       int
	fractionLost          uint8
	badSeq                uint16
}

func (s *internalStream) Stats() *StreamStats {
	return s.stats
}

func (s *internalStream) Cname() string {
	return s.cname
}

func (s *internalStream) GetMaxPacketTS() uint32 {
	return s.maxPacketRTPTimestamp
}

func (s *internalStream) FractionLost() uint8 {
	return s.fractionLost
}

func (s *internalStream) GetClockRate() int {
	return s.codec.ClockRate
}

func (s *internalStream) GetActiveMs() int64 {
	return time.Now().UnixMilli() - s.maxPacketMs
}

func (s *internalStream) SetRtx(payloadType rtc.PayloadType, ssrc uint32) {
	s.rtxSSRC = ssrc
	s.rtxPayloadType = payloadType
	params := RtxStreamParams{
		Ssrc:        ssrc,
		PayloadType: payloadType,
		ClockRate:   s.codec.ClockRate,
		Rrid:        s.rid,
		Cname:       s.cname,
	}
	s.rtxStream = NewRtxStream(params)
}

func (s *internalStream) SSRC() uint32 {
	return s.ssrc
}

func (s *internalStream) RtxSSRC() uint32 {
	return s.rtxSSRC
}

func (s *internalStream) PayloadType() rtc.PayloadType {
	return s.payloadType
}

func (s *internalStream) RtxPayloadType() rtc.PayloadType {
	return s.rtxPayloadType
}

func (s *internalStream) UpdateSSRC(ssrc uint32) {
	s.ssrc = ssrc
}

func (s *internalStream) UpdateRtxSSRC(ssrc uint32) {
	s.rtxSSRC = ssrc
}

func (s *internalStream) RID() string {
	return s.rid
}

func (s *internalStream) receivePacket(packet rtc.Packet) error {
	if !s.started {
		seq := packet.SequenceNumber()
		s.initSeq(seq)
		s.started = true
		s.maxPacketRTPTimestamp = packet.Timestamp()
		s.maxPacketMs = packet.ReceiveMS()
	}
	if err := s.updateSeq(packet); err != nil {
		return err
	}
	if packet.Timestamp() > s.maxPacketRTPTimestamp {
		s.maxPacketRTPTimestamp = packet.Timestamp()
		s.maxPacketMs = packet.ReceiveMS()
	}
	return nil
}

func (s *internalStream) initSeq(seq uint16) {
	s.baseSeq = seq
	s.maxSeq = seq
}

func (s *internalStream) updateSeq(packet rtc.Packet) error {
	seq := packet.SequenceNumber()
	if udelta := seq - s.maxSeq; udelta < MaxDropout {
		if seq < s.maxSeq {
			s.cycles += RTPSeqMod
		}
		s.maxSeq = seq
	} else if udelta < uint16(RTPSeqMod-uint32(MaxMisorder)) {
		s.packetDiscarded++
		s.badSeq = seq + 1
		return ErrBadSeq
	}
	return nil
}

func (s *internalStream) getExpectedPackets() int64 {
	return int64(s.cycles + uint32(s.maxSeq) - uint32(s.baseSeq) + 1)
}
