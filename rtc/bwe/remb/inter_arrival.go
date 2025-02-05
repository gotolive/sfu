package remb

import (
	"math"
)

const (
	arrivalTimeOffsetThresholdMs = 3000
	reorderedResetThreshold      = 3
	burstDeltaThresholdMs        = 5
	maxBurstDurationMs           = 100
)

type groupDelta struct {
	tsDelta   uint32 // send delta
	tDelta    int64  // arrival delta
	sizeDelta int    // size delta
}
type timeGroup struct {
	timestamp        uint32
	firstTimestamp   uint32
	firstArrivalMs   int64
	completeTimeMs   int64
	lastSystemTimeMs int64
	size             int
}

func (g *timeGroup) firstPacket() bool {
	return g.completeTimeMs == -1
}

func newTimeGroup() *timeGroup {
	return &timeGroup{
		completeTimeMs: -1,
		firstArrivalMs: -1,
	}
}

func newInterArrival(timestampGroupLengthTicks uint32, timestampToMsCoeff float64, burstGrouping bool) *interArrival {
	return &interArrival{
		timestampGroupLengthTicks: timestampGroupLengthTicks,
		currentTimeGroup:          newTimeGroup(),
		prevTimeGroup:             newTimeGroup(),
		timestampToMsCoeff:        timestampToMsCoeff,
		burstGrouping:             burstGrouping,
	}
}

type interArrival struct {
	currentTimeGroup              *timeGroup
	prevTimeGroup                 *timeGroup
	numConsecutiveReorderedPacket int
	burstGrouping                 bool
	timestampToMsCoeff            float64
	timestampGroupLengthTicks     uint32
}

func (i *interArrival) computeDeltas(timestamp uint32, arrivalTimeMs, nowMs int64, payloadSize int) *groupDelta {
	var delta *groupDelta
	if i.currentTimeGroup.firstPacket() {
		i.currentTimeGroup.timestamp = timestamp
		i.currentTimeGroup.firstTimestamp = timestamp
		i.currentTimeGroup.firstArrivalMs = arrivalTimeMs
	} else if !i.packetInOrder(timestamp) {
		return nil
	} else if i.newTimestampGroup(arrivalTimeMs, timestamp) {
		if i.prevTimeGroup.completeTimeMs >= 0 {
			tsDelta := i.currentTimeGroup.timestamp - i.prevTimeGroup.timestamp
			tDelta := i.currentTimeGroup.completeTimeMs - i.prevTimeGroup.completeTimeMs
			systemTimeDeltaMs := i.currentTimeGroup.lastSystemTimeMs - i.prevTimeGroup.lastSystemTimeMs
			if tDelta-systemTimeDeltaMs >= arrivalTimeOffsetThresholdMs {
				i.reset()
				return nil
			}
			if tDelta < 0 {
				i.numConsecutiveReorderedPacket++
				if i.numConsecutiveReorderedPacket >= reorderedResetThreshold {
					i.reset()
				}
				return nil
			}
			i.numConsecutiveReorderedPacket = 0
			delta = &groupDelta{
				tsDelta:   tsDelta,
				tDelta:    tDelta,
				sizeDelta: i.currentTimeGroup.size - i.prevTimeGroup.size,
			}
		}
		i.prevTimeGroup = i.currentTimeGroup
		i.currentTimeGroup = newTimeGroup()
		i.currentTimeGroup.firstTimestamp = timestamp
		i.currentTimeGroup.timestamp = timestamp
		i.currentTimeGroup.firstArrivalMs = arrivalTimeMs
		i.numConsecutiveReorderedPacket = 0
	} else {
		i.currentTimeGroup.timestamp = i.latestTimestamp(i.currentTimeGroup.timestamp, timestamp)
	}
	i.currentTimeGroup.size += payloadSize
	i.currentTimeGroup.completeTimeMs = arrivalTimeMs
	i.currentTimeGroup.lastSystemTimeMs = nowMs

	return delta
}

func (i *interArrival) packetInOrder(timestamp uint32) bool {
	if i.currentTimeGroup.firstPacket() {
		return true
	}
	timestampDiff := timestamp - i.currentTimeGroup.firstTimestamp
	return timestampDiff < 0x80000000
}

func (i *interArrival) newTimestampGroup(arriveTimeMs int64, timestamp uint32) bool {
	if i.currentTimeGroup.firstPacket() {
		return false
	} else if i.belongsToBurst(arriveTimeMs, timestamp) {
		return false
	}
	timestampDiff := timestamp - i.currentTimeGroup.firstTimestamp
	return timestampDiff > i.timestampGroupLengthTicks
}

func (i *interArrival) reset() {
	i.numConsecutiveReorderedPacket = 0
	i.currentTimeGroup = newTimeGroup()
	i.prevTimeGroup = newTimeGroup()
}

func (i *interArrival) latestTimestamp(timestamp uint32, timestamp2 uint32) uint32 {
	breakPoint := uint32((math.MaxUint32 >> 1) + 1)
	if timestamp-timestamp2 == breakPoint {
		if timestamp > timestamp2 {
			return timestamp
		}
		return timestamp2
	}

	if timestamp == timestamp2 {
		return timestamp
	}
	if timestamp-timestamp2 < breakPoint {
		return timestamp
	}

	return timestamp2
}

func (i *interArrival) belongsToBurst(arrivalTimeMs int64, timestamp uint32) bool {
	if !i.burstGrouping || i.currentTimeGroup.completeTimeMs < 0 {
		return false
	}

	arrivalTimeDeltaMs := arrivalTimeMs - i.currentTimeGroup.completeTimeMs
	timestampDiff := timestamp - i.currentTimeGroup.firstTimestamp
	tsDeltaMs := int64(i.timestampToMsCoeff*float64(timestampDiff) + 0.5)
	if tsDeltaMs == 0 {
		return true
	}

	propagationDeltaMs := arrivalTimeDeltaMs - tsDeltaMs
	if propagationDeltaMs < 0 && arrivalTimeDeltaMs <= burstDeltaThresholdMs && arrivalTimeMs-i.currentTimeGroup.firstArrivalMs < maxBurstDurationMs {
		return true
	}
	return false
}
