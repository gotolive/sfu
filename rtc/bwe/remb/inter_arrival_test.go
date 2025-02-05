package remb

import (
	"math"
	"testing"
)

var (
	kTimestampGroupLengthUs         int64 = 5000
	kMinStep                        int64 = 20
	kTriggerNewGroupUs                    = kTimestampGroupLengthUs + kMinStep
	kBurstThresholdMs               int64 = 5
	kAbsSendTimeFraction                  = 18
	kAbsSendTimeInterArrivalUpshift       = 8
	// kInterArrivalShift              = kAbsSendTimeFraction + kAbsSendTimeInterArrivalUpshift
	kAstToMs                       = 1000.0 / float64(1<<26)
	kRtpTimestampToMs              = 1.0 / 90
	kStartAbsSendTimeWrapUs  int64 = 63999995
	kStartRtpTimestampWrapUs int64 = 47721858827
	interArrivalRtp                = newInterArrival(uint32(kTimestampGroupLengthUs/1000), 1.0, true)
	interArrivalAst                = newInterArrival(uint32(kTimestampGroupLengthUs/1000), 1.0, true)
	intArrival                     = newInterArrival(uint32(kTimestampGroupLengthUs/1000), 1.0, true)
)

func internalExpectFalse(t *testing.T, intArrival *interArrival, timestamp uint32, arrivalTimeMs int64, packetSize int) {
	// dummyTimestamp := 101
	// dummyArrivalTimeMs := 303
	// dummyPacketSize := 909
	delta := intArrival.computeDeltas(timestamp, arrivalTimeMs, arrivalTimeMs, packetSize)
	if delta != nil {
		t.Fatal("groupDelta should be nil")
	}
}

func internalExpectTrue(t *testing.T,
	intArrival *interArrival,
	timestampUs uint32,
	arrivalTimeMs int64,
	packetSize int,
	expectDeltaUs uint32,
	expectArrivalTimeDelta int64,
	expectPacketSizeDelta int,
	timestampNear uint32,
) {
	delta := intArrival.computeDeltas(timestampUs, arrivalTimeMs, arrivalTimeMs, packetSize)
	if delta == nil {
		t.Fatal("groupDelta should not be nil")
	}
	if expectArrivalTimeDelta != delta.tDelta {
		t.Fatal("expected arrival time groupDelta wrong, expected:", expectArrivalTimeDelta, " got:", delta.tDelta)
	}
	if expectPacketSizeDelta != delta.sizeDelta {
		t.Fatal("expected size groupDelta wrong, expected:", expectPacketSizeDelta, " got:", delta.sizeDelta)
	} // if expectDeltaUs != groupDelta.tsDelta {
	//	t.Fatal("expected groupDelta wrong, expected:", expectDeltaUs, " got:", groupDelta.tsDelta)
	//}
	if math.Abs(float64(expectDeltaUs)-float64(delta.tsDelta)) > float64(timestampNear) {
		t.Fatal("expected timestampNear wrong, expected:", timestampNear, " got:", math.Abs(float64(expectDeltaUs)-float64(delta.tsDelta)))
	}
}

func ExpectFalse(t *testing.T, timestamp, arrivalMs int64, packetSize int) {
	internalExpectFalse(t, interArrivalRtp, makeRtpTimestamp(timestamp), arrivalMs, packetSize)
	internalExpectFalse(t, interArrivalAst, makeAbsSendTime(timestamp), arrivalMs, packetSize)
}

func ExpectTrue(t *testing.T,
	timestampUs int64,
	arrivalTimeMs int64,
	packetSize int,
	expectDeltaUs int64,
	expectArrivalTimeDelta int64,
	expectPacketSizeDelta int,
	timestampNear uint32,
) {
	internalExpectTrue(t, interArrivalRtp, makeRtpTimestamp(timestampUs), arrivalTimeMs, packetSize, makeRtpTimestamp(expectDeltaUs), expectArrivalTimeDelta, expectPacketSizeDelta, timestampNear)
	internalExpectTrue(t, interArrivalAst, makeAbsSendTime(timestampUs), arrivalTimeMs, packetSize, makeAbsSendTime(expectDeltaUs), expectArrivalTimeDelta, expectPacketSizeDelta, timestampNear<<8)
}

func WrapTestHelper(t *testing.T, wrapStartUs int64, timestampNear uint32, unorderlyWithinGroup bool) {
	// G1
	var arrivalTime int64 = 17
	ExpectFalse(t, 0, arrivalTime, 1)

	// G2
	arrivalTime += kBurstThresholdMs + 1
	ExpectFalse(t, wrapStartUs/4, arrivalTime, 1)
	// G3
	arrivalTime += kBurstThresholdMs + 1
	ExpectTrue(t, wrapStartUs/2, arrivalTime, 1, wrapStartUs/4, 6,
		0, // groupDelta G2-G1
		0)

	// G4
	arrivalTime += kBurstThresholdMs + 1
	var g4ArrivalTime int64 = arrivalTime
	ExpectTrue(t, wrapStartUs/2+wrapStartUs/4, arrivalTime, 1,
		wrapStartUs/4, 6, 0, // groupDelta G3-G2
		timestampNear)

	// G5
	arrivalTime += kBurstThresholdMs + 1
	ExpectTrue(t, wrapStartUs, arrivalTime, 2, wrapStartUs/4, 6,
		0, // groupDelta G4-G3
		timestampNear)
	for i := 0; i < 10; i++ {
		// Slowly step across the wrap point.
		arrivalTime += kBurstThresholdMs + 1
		if unorderlyWithinGroup {
			// These packets arrive with timestamps in decreasing order but are
			// nevertheless accumulated to group because their timestamps are higher
			// than the initial timestamp of the group.
			ExpectFalse(t, wrapStartUs+kMinStep*(9-int64(i)), arrivalTime, 1)
		} else {
			ExpectFalse(t, wrapStartUs+kMinStep*int64(i), arrivalTime, 1)
		}
	}
	// wrapStartUs/2+wrapStartUs/4
	// wrapStartUs+kMinStep*int64(i)

	// last timestamp should be wrapStartUs+kMinStep*9
	// new timestamp should be wrapstartUs+kTriggerNewGroupUs
	// groupDelta should be kTriggerNewGroupUs-kMinStep*9
	// Real 5020-180 = 4840
	// real last 15
	// new last 450
	// real groupDelta = 450-15 =
	// 15
	var g5ArrivalTime int64 = arrivalTime

	// This packet is out of order and should be dropped.
	arrivalTime += kBurstThresholdMs + 1
	ExpectFalse(t, wrapStartUs-100, arrivalTime, 100)

	// G6 //
	arrivalTime += kBurstThresholdMs + 1
	var g6ArrivalTime int64 = arrivalTime

	ExpectTrue(t, wrapStartUs+kTriggerNewGroupUs, arrivalTime, 10,
		wrapStartUs/4+9*kMinStep,
		g5ArrivalTime-g4ArrivalTime,
		(2+10)-1, // groupDelta G5-G4
		timestampNear)

	// This packet is out of order and should be dropped.
	arrivalTime += kBurstThresholdMs + 1
	ExpectFalse(t, wrapStartUs+kTimestampGroupLengthUs, arrivalTime, 100)

	// G7
	arrivalTime += kBurstThresholdMs + 1
	ExpectTrue(t, wrapStartUs+2*kTriggerNewGroupUs, arrivalTime, 100,
		// groupDelta G6-G5
		kTriggerNewGroupUs-9*kMinStep,
		g6ArrivalTime-g5ArrivalTime, 10-(2+10),
		timestampNear)
}

func setup() {
	intArrival = newInterArrival(uint32(kTimestampGroupLengthUs/1000), 1.0, true)
	interArrivalRtp = newInterArrival(makeRtpTimestamp(kTimestampGroupLengthUs), kRtpTimestampToMs, true)
	interArrivalAst = newInterArrival(makeAbsSendTime(kTimestampGroupLengthUs), kAstToMs, true)
}

func makeRtpTimestamp(us int64) uint32 {
	// 4840
	return uint32(uint64(us*90+500) / 1000)
}

func makeAbsSendTime(us int64) uint32 {
	absSendTime := uint32((uint64(us)<<18+500000)/1000000) & 0x00FFFFFF
	return absSendTime << 8
}

func TestInterArrival(t *testing.T) {
	tests := []struct {
		name       string
		method     func(t *testing.T)
		timestamp  int64
		arrivalMs  int64
		packetSize int
	}{
		{
			name: "firstPacket",
			method: func(t *testing.T) {
				ExpectFalse(t, 0, 17, 1)
			},
		},
		{
			name: "FirstGroup",
			method: func(t *testing.T) {
				var arrivalTime int64 = 17
				g1ArrivalTime := arrivalTime
				ExpectFalse(t, 0, arrivalTime, 1)
				arrivalTime += kBurstThresholdMs + 1
				g2ArrivalTime := arrivalTime
				ExpectFalse(t, kTriggerNewGroupUs, arrivalTime, 2)
				arrivalTime += kBurstThresholdMs + 1
				ExpectTrue(t, 2*kTriggerNewGroupUs, arrivalTime, 1, kTriggerNewGroupUs, g2ArrivalTime-g1ArrivalTime, 1, 0)
			},
		},
		{
			name: "SecondGroup",
			method: func(t *testing.T) {
				var arrivalTime int64 = 17
				var g1ArrivalTime int64 = arrivalTime
				ExpectFalse(t, 0, arrivalTime, 1)

				arrivalTime += kBurstThresholdMs + 1
				var g2ArrivalTime int64 = arrivalTime
				ExpectFalse(t, kTriggerNewGroupUs, arrivalTime, 2)

				arrivalTime += kBurstThresholdMs + 1
				var g3ArrivalTime int64 = arrivalTime
				ExpectTrue(t, 2*kTriggerNewGroupUs, arrivalTime, 1,
					kTriggerNewGroupUs, g2ArrivalTime-g1ArrivalTime, 1, 0)

				arrivalTime += kBurstThresholdMs + 1
				ExpectTrue(t, 3*kTriggerNewGroupUs, arrivalTime, 2,
					kTriggerNewGroupUs, g3ArrivalTime-g2ArrivalTime, -1, 0)
			},
		},
		{
			name: "AccumulatedGroup",
			method: func(t *testing.T) {
				var arrivalTime int64 = 17
				var g1ArrivalTime int64 = arrivalTime
				ExpectFalse(t, 0, arrivalTime, 1)

				arrivalTime += kBurstThresholdMs + 1
				ExpectFalse(t, kTriggerNewGroupUs, 28, 2)
				var timestamp int64 = kTriggerNewGroupUs
				for i := 0; i < 10; i++ {
					// A bunch of packets arriving within the same group.
					arrivalTime += kBurstThresholdMs + 1
					timestamp += kMinStep
					ExpectFalse(t, timestamp, arrivalTime, 1)
				}
				var g2ArrivalTime int64 = arrivalTime
				var g2Timestamp int64 = timestamp

				arrivalTime = 500
				ExpectTrue(t, 2*kTriggerNewGroupUs, arrivalTime, 100, g2Timestamp,
					g2ArrivalTime-g1ArrivalTime,
					(2+10)-1, // groupDelta G2-G1
					0)
			},
		},
		{
			name: "OutOfOrderPacket",
			method: func(t *testing.T) {
				var arrivalTime int64 = 17
				var timestamp int64 = 0
				ExpectFalse(t, timestamp, arrivalTime, 1)
				var g1Timestamp int64 = timestamp
				var g1ArrivalTime int64 = arrivalTime

				arrivalTime += 11
				timestamp += kTriggerNewGroupUs
				ExpectFalse(t, timestamp, 28, 2)
				for i := 0; i < 10; i++ {
					arrivalTime += kBurstThresholdMs + 1
					timestamp += kMinStep
					ExpectFalse(t, timestamp, arrivalTime, 1)
				}
				var g2Timestamp int64 = timestamp
				var g2ArrivalTime int64 = arrivalTime

				// This packet is out of order and should be dropped.
				arrivalTime = 281
				ExpectFalse(t, g1Timestamp, arrivalTime, 100)

				// G3
				arrivalTime = 500
				timestamp = 2 * kTriggerNewGroupUs
				ExpectTrue(t, timestamp, arrivalTime, 100,
					// groupDelta G2-G1
					g2Timestamp-g1Timestamp, g2ArrivalTime-g1ArrivalTime,
					(2+10)-1, 0)
			},
		},
		{
			name: "TwoBursts",
			method: func(t *testing.T) {
				// G1
				var g1ArrivalTime int64 = 17
				ExpectFalse(t, 0, g1ArrivalTime, 1)

				// G2
				var timestamp int64 = kTriggerNewGroupUs
				var arrivalTime int64 = 100 // Simulate no packets arriving for 100 ms.
				for i := 0; i < 10; i++ {
					// A bunch of packets arriving in one burst (within 5 ms apart).
					timestamp += 30000
					arrivalTime += kBurstThresholdMs
					ExpectFalse(t, timestamp, arrivalTime, 1)

				}
				var g2ArrivalTime int64 = arrivalTime
				var g2Timestamp int64 = timestamp

				// G3
				timestamp += 30000
				arrivalTime += kBurstThresholdMs + 1
				ExpectTrue(t, timestamp, arrivalTime, 100, g2Timestamp,
					g2ArrivalTime-g1ArrivalTime,
					10-1, // groupDelta G2-G1
					0)
			},
		},
		{
			name: "NoBursts",
			method: func(t *testing.T) {
				// G1
				ExpectFalse(t, 0, 17, 1)

				// G2
				var timestamp int64 = kTriggerNewGroupUs
				var arrivalTime int64 = 28
				ExpectFalse(t, timestamp, arrivalTime, 2)

				// G3
				ExpectTrue(t, kTriggerNewGroupUs+30000, arrivalTime+kBurstThresholdMs+1,
					100, timestamp-0, arrivalTime-17,
					2-1, // groupDelta G2-G1
					0)
			},
		},
		{
			name: "RtpTimestampWrap",
			method: func(t *testing.T) {
				WrapTestHelper(t, kStartRtpTimestampWrapUs, 1, false)
			},
		},
		{
			name: "AbsSendTimeWrap",
			method: func(t *testing.T) {
				WrapTestHelper(t, kStartAbsSendTimeWrapUs, 1, false)
			},
		},
		{
			name: "RtpTimestampWrapOutOfOrderWithinGroup",
			method: func(t *testing.T) {
				WrapTestHelper(t, kStartRtpTimestampWrapUs, 1, true)
			},
		},
		{
			name: "AbsSendTimeWrap",
			method: func(t *testing.T) {
				WrapTestHelper(t, kStartAbsSendTimeWrapUs, 1, true)
			},
		},
		{
			name: "PositiveArrivalTimeJump",
			method: func(t *testing.T) {
				kPacketSize := 1000
				var sendTimeMs uint32 = 10000
				var arrivalTimeMs int64 = 20000
				var systemTimeMs int64 = 30000

				if intArrival.computeDeltas(
					sendTimeMs, arrivalTimeMs, systemTimeMs, kPacketSize) != nil {
					t.Fatal("groupDelta should be nil")
				}

				kTimeDeltaMs := 30
				sendTimeMs += uint32(kTimeDeltaMs)
				arrivalTimeMs += int64(kTimeDeltaMs)
				systemTimeMs += int64(kTimeDeltaMs)
				if intArrival.computeDeltas(
					sendTimeMs, arrivalTimeMs, systemTimeMs, kPacketSize) != nil {
					t.Fatal("groupDelta should be nil")
				}

				sendTimeMs += uint32(kTimeDeltaMs)
				arrivalTimeMs += int64(kTimeDeltaMs) + arrivalTimeOffsetThresholdMs
				systemTimeMs += int64(kTimeDeltaMs)
				if delta := intArrival.computeDeltas(
					sendTimeMs, arrivalTimeMs, systemTimeMs, kPacketSize); delta == nil {
					t.Fatal("groupDelta should be nil")
				} else {
					if int(delta.tsDelta) != kTimeDeltaMs {
						t.Fatal("tsdelta fail")
					}
					if int(delta.tDelta) != kTimeDeltaMs {
						t.Fatal("tdelta fail")
					}
					if delta.sizeDelta != 0 {
						t.Fatal("size groupDelta fail")
					}

				}

				sendTimeMs += uint32(kTimeDeltaMs)
				arrivalTimeMs += int64(kTimeDeltaMs)
				systemTimeMs += int64(kTimeDeltaMs)
				// The previous arrival time jump should now be detected and cause a reset.
				if intArrival.computeDeltas(
					sendTimeMs, arrivalTimeMs, systemTimeMs, kPacketSize) != nil {
					t.Fatal("groupDelta should be nil")
				}

				// The two next packets will not give a valid groupDelta since we're in the initial
				// state.
				for i := 0; i < 2; i++ {
					sendTimeMs += uint32(kTimeDeltaMs)
					arrivalTimeMs += int64(kTimeDeltaMs)
					systemTimeMs += int64(kTimeDeltaMs)
					if intArrival.computeDeltas(
						sendTimeMs, arrivalTimeMs, systemTimeMs, kPacketSize) != nil {
						t.Fatal("groupDelta should be nil")
					}
				}

				sendTimeMs += uint32(kTimeDeltaMs)
				arrivalTimeMs += int64(kTimeDeltaMs)
				systemTimeMs += int64(kTimeDeltaMs)
				if delta := intArrival.computeDeltas(
					sendTimeMs, arrivalTimeMs, systemTimeMs, kPacketSize); delta == nil {
					t.Fatal("groupDelta should be nil")
				} else {
					if int(delta.tsDelta) != kTimeDeltaMs {
						t.Fatal("tsdelta fail")
					}
					if int(delta.tDelta) != kTimeDeltaMs {
						t.Fatal("tdelta fail")
					}
					if delta.sizeDelta != 0 {
						t.Fatal("size groupDelta fail")
					}

				}
			},
		},
		{
			name: "NegativeArrivalTimeJump",
			method: func(t *testing.T) {
				kPacketSize := 1000
				var sendTimeMs uint32 = 10000
				var arrivalTimeMs int64 = 20000
				var systemTimeMs int64 = 30000

				if intArrival.computeDeltas(
					sendTimeMs, arrivalTimeMs, systemTimeMs, kPacketSize) != nil {
					t.Fatal("groupDelta should be nil")
				}

				kTimeDeltaMs := 30
				sendTimeMs += uint32(kTimeDeltaMs)
				arrivalTimeMs += int64(kTimeDeltaMs)
				systemTimeMs += int64(kTimeDeltaMs)
				if intArrival.computeDeltas(
					sendTimeMs, arrivalTimeMs, systemTimeMs, kPacketSize) != nil {
					t.Fatal("groupDelta should be nil")
				}

				sendTimeMs += uint32(kTimeDeltaMs)
				arrivalTimeMs += int64(kTimeDeltaMs)
				systemTimeMs += int64(kTimeDeltaMs)
				if delta := intArrival.computeDeltas(
					sendTimeMs, arrivalTimeMs, systemTimeMs, kPacketSize); delta == nil {
					t.Fatal("groupDelta should be nil")
				} else {
					if int(delta.tsDelta) != kTimeDeltaMs {
						t.Fatal("tsdelta fail")
					}
					if int(delta.tDelta) != kTimeDeltaMs {
						t.Fatal("tdelta fail")
					}
					if delta.sizeDelta != 0 {
						t.Fatal("size groupDelta fail")
					}

				}

				arrivalTimeMs -= 1000
				// The two next packets will not give a valid groupDelta since we're in the initial
				// state.
				for i := 0; i < reorderedResetThreshold+3; i++ {
					sendTimeMs += uint32(kTimeDeltaMs)
					arrivalTimeMs += int64(kTimeDeltaMs)
					systemTimeMs += int64(kTimeDeltaMs)
					if intArrival.computeDeltas(
						sendTimeMs, arrivalTimeMs, systemTimeMs, kPacketSize) != nil {
						t.Log(i)
						t.Fatal("groupDelta should be nil")
					}
				}

				sendTimeMs += uint32(kTimeDeltaMs)
				arrivalTimeMs += int64(kTimeDeltaMs)
				systemTimeMs += int64(kTimeDeltaMs)
				if delta := intArrival.computeDeltas(
					sendTimeMs, arrivalTimeMs, systemTimeMs, kPacketSize); delta == nil {
					t.Fatal("groupDelta should be nil")
				} else {
					if int(delta.tsDelta) != kTimeDeltaMs {
						t.Fatal("tsdelta fail")
					}
					if int(delta.tDelta) != kTimeDeltaMs {
						t.Fatal("tdelta fail")
					}
					if delta.sizeDelta != 0 {
						t.Fatal("size groupDelta fail")
					}

				}
			},
		},
	}
	for _, test := range tests {
		setup()
		t.Run(test.name, func(t *testing.T) {
			test.method(t)
		})
	}
}
