package peer

import (
	"math"
	"time"

	"github.com/gotolive/sfu/rtc"
	"github.com/gotolive/sfu/rtc/codec"
	"github.com/gotolive/sfu/rtc/logger"
	"github.com/gotolive/sfu/rtc/nack"
	"github.com/pion/rtcp"
)

const (
	inactivityCheckInterval        = 1500
	inactivityCheckIntervalWithDtx = 5000
)

type ReceiverStreamListener interface {
	OnRTPStreamNeedWorstRemoteFractionLost(ssrc uint32) uint8
	sendRtcp(packet rtcp.Packet)
}

type ReceiverStream interface {
	Stream
	RequestKeyFrame()
	ReceivePacket(packet rtc.Packet) error

	GetSenderReportNtpMs() uint64
	GetSenderReportTS() int64

	ReceiveRtcpSenderReport(report *rtcp.SenderReport)
	GetRtcpReceiverReport() *rtcp.ReceptionReport
	GetRtxReceiverReport() *rtcp.ReceptionReport

	UpdateSSRC(uint32)
	UpdateRtxSSRC(uint32)
	SetRtx(payloadType rtc.PayloadType, ssrc uint32)
	Close()
}

func NewReceiverStream(lis ReceiverStreamListener, mediaType string, stream StreamOption, codec Codec) ReceiverStream {
	r := &receiverStream{
		internalStream:           newStream(mediaType, stream, codec),
		listener:                 lis,
		mediaTransmissionCounter: newStreamStats(stream.SSRC),
	}
	if r.useNack {
		r.nackReceiver = nack.NewReceiver(r.onNackRTCP)
	}
	if r.useDtx {
		r.inactiveInterval = inactivityCheckIntervalWithDtx * time.Millisecond
	} else {
		r.inactiveInterval = inactivityCheckInterval * time.Millisecond
	}
	r.inactivityCheckPeriodicTicker = time.NewTicker(r.inactiveInterval)

	return r
}

type receiverStream struct {
	internalStream
	listener ReceiverStreamListener

	pliCount     int
	firCount     int
	firSeqNumber int

	nackCount       int
	nackPacketCount int

	mediaTransmissionCounter *StreamStats

	lastSenderReportTS    int64  // last we received sender report
	lastSenderReportNtpMs uint64 // NTP Time
	lastSrReceived        int64  // Local Timestamp

	nackReceiver nack.Receiver

	inactive                      bool
	inactiveInterval              time.Duration
	inactivityCheckPeriodicTicker *time.Ticker

	packetsLost          int64
	expectedPrior        int64
	receivedPrior        int64
	reportPacketLost     int64
	jitter               int32
	lastReceiveTimeMs    int64
	lastReceiveTimestamp uint32 // RTP Timestamp

	videoFrameCount int
}

func (r *receiverStream) Close() {
	if r.nackReceiver != nil {
		r.nackReceiver.Close()
	}
}

// GetSenderReportTS return the last time we received sender report, will be used for cal simulcast layer
func (r *receiverStream) GetSenderReportTS() int64 {
	return r.lastSenderReportTS
}

// GetSenderReportNtpMs return the last time we received sender report, will be used for cal simulcast layer
func (r *receiverStream) GetSenderReportNtpMs() uint64 {
	return r.lastSenderReportNtpMs
}

// GetRtcpReceiverReport return the rr report for feedback.
func (r *receiverStream) GetRtcpReceiverReport() *rtcp.ReceptionReport {
	report := &rtcp.ReceptionReport{
		SSRC:               r.SSRC(),
		FractionLost:       0,
		TotalLost:          0,
		LastSequenceNumber: 0,
		Jitter:             0,
		LastSenderReport:   0,
		Delay:              0,
	}
	var worstRemoteFractionLost uint8
	if r.useInBandFec {
		worstRemoteFractionLost = r.listener.OnRTPStreamNeedWorstRemoteFractionLost(r.ssrc)
	}
	prevPacketLost := r.packetsLost
	expected := r.getExpectedPackets()
	if expected > r.mediaTransmissionCounter.PacketsReceived() {
		r.packetsLost = expected - r.mediaTransmissionCounter.PacketsReceived()
	} else {
		r.packetsLost = 0
	}
	expectedInterval := expected - r.expectedPrior
	r.expectedPrior = expected
	receivedInterval := r.mediaTransmissionCounter.PacketsReceived() - r.receivedPrior
	r.receivedPrior = r.mediaTransmissionCounter.PacketsReceived()
	lostInterval := expectedInterval - receivedInterval
	if expectedInterval == 0 || lostInterval == 0 {
		r.fractionLost = 0
	} else {
		if lostInterval<<8/expectedInterval > 255 {
			r.fractionLost = math.MaxUint8
		} else {
			r.fractionLost = uint8((lostInterval << 8) / expectedInterval)
		}
	}
	if worstRemoteFractionLost <= r.fractionLost {
		r.reportPacketLost += r.packetsLost - prevPacketLost
		report.TotalLost = uint32(r.reportPacketLost)
		report.FractionLost = r.fractionLost
	} else {
		newLostInterval := (int64(worstRemoteFractionLost) * expectedInterval) >> 8
		r.reportPacketLost += newLostInterval
		report.TotalLost = uint32(r.reportPacketLost)
		report.FractionLost = worstRemoteFractionLost
	}
	report.LastSequenceNumber = uint32(r.maxSeq) + r.cycles
	report.Jitter = uint32(r.jitter)
	if r.lastSrReceived != 0 {
		delayMs := time.Now().UnixMilli() - r.lastSrReceived
		dlsr := delayMs * (65535 / 1000)
		report.Delay = uint32(dlsr)
		report.LastSenderReport = (uint32(r.lastSenderReportNtpMs>>32) << 16) + (uint32(r.lastSenderReportNtpMs) >> 16)
	}
	return report
}

// GetRtxReceiverReport return the rr report for rtx feedback
func (r *receiverStream) GetRtxReceiverReport() *rtcp.ReceptionReport {
	if r.rtxStream != nil {
		return r.rtxStream.GetRtcpReceiverReport()
	}
	return nil
}

// ReceiveRtcpSenderReport handle new sr
func (r *receiverStream) ReceiveRtcpSenderReport(report *rtcp.SenderReport) {
	r.lastSrReceived = time.Now().UnixMilli()
	r.lastSenderReportNtpMs = report.NTPTime
	r.lastSenderReportTS = int64(report.RTPTime)
}

func (r *receiverStream) ReceivePacket(packet rtc.Packet) error {
	if packet.IsRTX() {
		return r.receiveRtxPacket(packet)
	}
	return r.receivePacket(packet)
}

func (r *receiverStream) RequestKeyFrame() {
	if r.internalStream.usePli {
		packet := rtcp.PictureLossIndication{
			SenderSSRC: uint32(0),
			MediaSSRC:  r.SSRC(),
		}
		r.pliCount++
		r.listener.sendRtcp(&packet)
	} else if r.internalStream.useFir {
		packet := rtcp.FullIntraRequest{
			SenderSSRC: uint32(0),
			MediaSSRC:  r.SSRC(),
		}
		r.firSeqNumber++
		packet.FIR = append(packet.FIR, rtcp.FIREntry{
			SSRC:           r.SSRC(),
			SequenceNumber: uint8(r.firSeqNumber),
		})
		r.firCount++
		r.listener.sendRtcp(&packet)
	}
}

func (r *receiverStream) onNackRTCP(packet rtcp.Packet) {
	switch pkt := packet.(type) {
	case *rtcp.TransportLayerNack:
		r.nackCount++
		for _, v := range pkt.Nacks {
			r.nackPacketCount += len(v.PacketList())
		}
	case *rtcp.PictureLossIndication, *rtcp.FullIntraRequest:
		r.RequestKeyFrame()
	}
	r.listener.sendRtcp(packet)
}

func (r *receiverStream) receivePacket(packet rtc.Packet) error {
	if err := r.internalStream.receivePacket(packet); err != nil {
		logger.Error("receive rtpPacket error:", err)
		return err
	}
	if packet.PayloadType() == r.PayloadType() {
		codec.ProcessRTPPacket(packet, r.codec.EncoderName)
	}
	if r.mediaType == rtc.MediaTypeVideo {
		if packet.HasMarker() {
			r.videoFrameCount++
		}
	}
	if r.useNack {
		r.nackReceiver.IncomingPacket(0, packet)
	}
	r.calculateJitter(packet.Timestamp())

	r.stats.incomingRTP(packet)
	r.mediaTransmissionCounter.incomingRTP(packet)
	if r.inactive {
		r.inactive = false
	}
	r.inactivityCheckPeriodicTicker.Reset(r.inactiveInterval)
	return nil
}

func (r *receiverStream) receiveRtxPacket(packet rtc.Packet) error {
	if !r.useNack {
		return ErrNoNack
	}

	if packet.SSRC() != r.RtxSSRC() {
		return ErrInvalidPacket
	}
	if packet.PayloadType() != r.rtxPayloadType {
		return ErrInvalidRtxPacket
	}
	if r.rtxStream != nil {
		if err := r.rtxStream.ReceivePacket(packet); err != nil {
			return err
		}
	}

	if err := packet.RtxDecode(r.payloadType, r.ssrc); err != nil {
		return err
	}
	if err := r.updateSeq(packet); err != nil {
		return err
	}

	if packet.PayloadType() == r.PayloadType() {
		codec.ProcessRTPPacket(packet, r.codec.EncoderName)
	}

	if r.useNack {
		r.nackReceiver.IncomingPacket(0, packet)
	}

	if r.inactive {
		r.inactive = false
	}
	if r.inactivityCheckPeriodicTicker != nil {
		r.inactivityCheckPeriodicTicker.Reset(r.inactiveInterval)
	}

	return nil
}

// RTC Timestamp
func (r *receiverStream) calculateJitter(timestamp uint32) {
	// calculate jitter need clock rate
	if r.codec.ClockRate == 0 {
		return
	}

	currentTime := time.Now().UnixMilli()

	if r.lastReceiveTimeMs == 0 {
		r.lastReceiveTimestamp = timestamp
		r.lastReceiveTimeMs = currentTime
		return
	}

	diffMs := currentTime - r.lastReceiveTimeMs
	// Turn it to rtp time diff
	receiveDiffRTP := uint32(diffMs) * uint32(r.codec.ClockRate) / 1000

	senderDiffRTP := int32(timestamp) - int32(r.lastReceiveTimestamp)
	// make sure it could be neg
	diffSamples := int32(receiveDiffRTP) - senderDiffRTP

	if diffSamples < 0 {
		diffSamples = -diffSamples
	}

	if diffSamples < 450000 {
		// /16
		r.jitter += (diffSamples - r.jitter) / 16
	}
	r.lastReceiveTimeMs = currentTime
	r.lastReceiveTimestamp = timestamp
}
