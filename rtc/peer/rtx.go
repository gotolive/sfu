package peer

import (
	"time"

	"github.com/gotolive/sfu/rtc"
	"github.com/gotolive/sfu/rtc/codec"
	"github.com/pion/rtcp"
)

type RtxStreamParams struct {
	EncodingIdx int
	Ssrc        uint32
	PayloadType rtc.PayloadType
	Codec       *codec.Codec
	ClockRate   int
	Rrid        string
	Cname       string
}

// RtxStream only used for cal rr report, huge waste.
type RtxStream struct {
	params             RtxStreamParams
	started            bool
	maxSeq             uint16
	maxPacketTS        uint32
	maxPacketMs        int64
	packetCount        int // the real rtpPacket count we received
	baseSeq            uint16
	badSeq             uint32
	cycles             uint32
	packetDiscarded    int
	packetLost         int
	expectedPrior      int
	receiverPrior      int
	fractionLost       int
	reportedPacketLost int
	lastSrReceived     int
	lastSrTimestamp    uint32
}

func NewRtxStream(params RtxStreamParams) *RtxStream {
	return &RtxStream{params: params}
}

func (r *RtxStream) ReceivePacket(packet rtc.Packet) error {
	if !r.started {
		r.InitSeq(packet.SequenceNumber())
		r.started = true
		r.maxSeq = packet.SequenceNumber()
		r.maxPacketTS = packet.Timestamp()
		r.maxPacketMs = packet.ReceiveMS()
	}
	if !r.UpdateSeq(packet) {
		return ErrInvalidPacket
	}
	if packet.Timestamp() > r.maxPacketTS {
		r.maxPacketTS = packet.Timestamp()
		r.maxPacketMs = packet.ReceiveMS()
	}
	r.packetCount++

	return nil
}

func (r *RtxStream) InitSeq(seq uint16) {
	r.baseSeq = seq
	r.maxSeq = seq
	r.badSeq = RTPSeqMod + 1
}

func (r *RtxStream) UpdateSeq(packet rtc.Packet) bool {
	udelta := packet.SequenceNumber() - r.maxSeq
	if udelta < MaxDropout {
		if packet.SequenceNumber() < r.maxSeq {
			r.cycles += RTPSeqMod
		}
		r.maxSeq = packet.SequenceNumber()
	} else if udelta <= uint16(RTPSeqMod-uint32(MaxMisorder)) {
		if uint32(packet.SequenceNumber()) == r.badSeq {
			r.InitSeq(packet.SequenceNumber())
			r.maxPacketTS = packet.Timestamp()
			r.maxPacketMs = time.Now().UnixMilli()
		} else {
			r.badSeq = uint32(packet.SequenceNumber()+1) & (RTPSeqMod - 1)
			r.packetDiscarded++
			return false
		}
	}
	return true
}

func (r *RtxStream) GetRtcpReceiverReport() *rtcp.ReceptionReport {
	report := &rtcp.ReceptionReport{
		SSRC: r.params.Ssrc,
	}
	prevPacketLost := r.packetLost
	expected := r.GetExpectedPackets()
	if expected > r.packetCount {
		// Total rtpPacket Lost
		r.packetLost = expected - r.packetCount
	} else {
		r.packetLost = 0
	}
	// the gap between last time
	expectedInterval := expected - r.expectedPrior
	r.expectedPrior = expected
	// the received between last time
	receivedInterval := r.packetCount - r.receiverPrior
	r.receiverPrior = r.packetCount
	lostInterval := expectedInterval - receivedInterval
	if expectedInterval == 0 || lostInterval == 0 {
		r.fractionLost = 0
	} else {
		r.fractionLost = (lostInterval << 8) / expectedInterval
	}
	r.reportedPacketLost += r.packetLost - prevPacketLost
	report.TotalLost = uint32(r.reportedPacketLost)
	report.FractionLost = uint8(r.fractionLost)
	report.LastSequenceNumber = uint32(r.maxSeq) + r.cycles
	report.Jitter = 0
	if r.lastSrReceived != 0 {
		delayMs := time.Now().UnixMilli() - int64(r.lastSrReceived)
		dlsr := (delayMs / 1000) << 16
		dlsr |= (delayMs % 1000) * 65536 / 1000
		report.Delay = uint32(dlsr)
		report.LastSenderReport = r.lastSrTimestamp
	}

	return report
}

func (r *RtxStream) GetExpectedPackets() int {
	return int(r.cycles + uint32(r.maxSeq-r.baseSeq) + 1)
}
