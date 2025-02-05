package remb

import "math"

const (
	LimitNumPackets = 20
)

type LinkCapacityTracker struct{}

func (l *LinkCapacityTracker) onRateUpdate(rate *uint64, target uint64, ms int64) {
	panic("implements me")
}

type SendSideBandwidthEstimation struct {
	receiverLimit                      uint64
	currentTarget                      uint64
	minBitrateConfigured               uint64
	linkCapacity                       *LinkCapacityTracker
	acknowledgedRate                   *uint64
	delayBasedLimit                    uint64
	receiverLimitCapsOnly              bool
	maxBitrateConfigured               uint64
	lastLostFeedback                   int64
	firstLostFeedback                  int64
	expectedPacketsSinceLastLostUpdate int64
	lostPacketsSinceLastLossUpdate     int64
	hasDecreasedSinceLastFractionLoss  bool
	lostFractionLoss                   int64
	lastLostPacketReport               int64
}

func NewSendSideBandwidthEstimation() *SendSideBandwidthEstimation {
	return &SendSideBandwidthEstimation{}
}

func (e *SendSideBandwidthEstimation) UpdatePacketsLost(packetsLost, numberOfPackets, nowMs int64) {
	e.lastLostFeedback = nowMs
	if e.firstLostFeedback == -1 {
		e.firstLostFeedback = nowMs
	}
	// nack logic to
	if numberOfPackets > 0 {
		expected := e.expectedPacketsSinceLastLostUpdate + numberOfPackets
		if expected < LimitNumPackets {
			e.expectedPacketsSinceLastLostUpdate = expected
			e.lostPacketsSinceLastLossUpdate += packetsLost
			return
		}
		e.hasDecreasedSinceLastFractionLoss = false
		lostQ8 := (e.lostPacketsSinceLastLossUpdate + packetsLost) << 8
		e.lostFractionLoss = 255
		if lostQ8/expected < e.lostFractionLoss {
			e.lostFractionLoss = lostQ8 / expected
		}

		e.lostPacketsSinceLastLossUpdate = 0
		e.expectedPacketsSinceLastLostUpdate = 0
		e.lastLostPacketReport = nowMs
		e.updateEstimate(nowMs)
	}
	// this seems update the stats graph only. so drop it.
	// e.updateUmaStatsPacketsLost(nowMs, packetsLost)
}

func (e *SendSideBandwidthEstimation) UpdateReceiverEstimate(nowMs int64, bitrate uint64) {
	e.receiverLimit = bitrate
	if bitrate == 0 {
		e.receiverLimit = math.MaxUint64
	}
	e.applyTargetLimits(nowMs)
}

func (e *SendSideBandwidthEstimation) applyTargetLimits(nowMs int64) {
	e.updateTargetBitrate(e.currentTarget, nowMs)
}

func (e *SendSideBandwidthEstimation) updateTargetBitrate(target uint64, nowMs int64) {
	newBitrate := target
	if newBitrate > e.getUpperLimit() {
		newBitrate = e.getUpperLimit()
	}
	if newBitrate < e.minBitrateConfigured {
		newBitrate = e.minBitrateConfigured
	}
	e.currentTarget = newBitrate
	e.linkCapacity.onRateUpdate(e.acknowledgedRate, e.currentTarget, nowMs)
}

func (e *SendSideBandwidthEstimation) getUpperLimit() uint64 {
	upperLimit := e.delayBasedLimit
	if !e.receiverLimitCapsOnly {
		if upperLimit > e.receiverLimit {
			upperLimit = e.receiverLimit
		}
	}
	if upperLimit > e.maxBitrateConfigured {
		upperLimit = e.maxBitrateConfigured
	}
	return upperLimit
}

func (e *SendSideBandwidthEstimation) updateEstimate(nowMs int64) {
	// if e.rttBackoff.CorrectedRtt(nowMs) > e.rttBackoff.rttLimit {
	panic("implements me")
	//}
}
