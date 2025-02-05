package remb

import (
	"math"
	"math/rand"
	"testing"
)

type OveruseDetectorTestHelper struct {
	ia               *interArrival
	overuseEstimator *OveruseEstimator
	detector         *OveruseDetector
	nowMs            int64
	receiveTimeMs    int64
	rtpTimestamp     uint32
	state            uint64
}

func (h *OveruseDetectorTestHelper) UpdateDetector(rtpTimestamp uint32, nowMs int64, packetSize int) {
	if p := h.ia.computeDeltas(rtpTimestamp, nowMs, nowMs, packetSize); p != nil {
		delta := p.tsDelta / 90.0
		h.overuseEstimator.Update(p.tDelta, delta, p.sizeDelta, h.detector.State(), nowMs)
		h.detector.Detect(h.overuseEstimator.offset, delta, h.overuseEstimator.numOfDeltas, nowMs)
	}
}

func (h *OveruseDetectorTestHelper) Run100000Samples(packetPerFrame, packetSize, frameDurationMs, sigmaMs int) int {
	uniqueOveruse := 0
	lastOveruse := -1
	for i := 0; i < 100000; i++ {
		for j := 0; j < packetPerFrame; j++ {
			h.UpdateDetector(h.rtpTimestamp, h.receiveTimeMs, packetSize)
		}
		h.rtpTimestamp += uint32(frameDurationMs) * 90
		h.nowMs += int64(frameDurationMs)
		c := h.nowMs + int64(h.Gaussian(0, sigmaMs)+0.5)
		if h.receiveTimeMs < c {
			h.receiveTimeMs = c
		}
		if h.detector.State() == Overusing {
			if lastOveruse+1 != i {
				uniqueOveruse += 1
			}
			lastOveruse = i
		}
	}
	return uniqueOveruse
}

func (h *OveruseDetectorTestHelper) RunUntilOveruse(packetPerFrame, packetSize, frameDurationMs, sigmaMs, driftPerFrameMs int) int {
	for i := 0; i < 1000; i++ {
		for j := 0; j < packetPerFrame; j++ {
			h.UpdateDetector(h.rtpTimestamp, h.receiveTimeMs, packetSize)
		}
		h.rtpTimestamp += uint32(frameDurationMs) * 90
		h.nowMs += int64(frameDurationMs) + int64(driftPerFrameMs)
		c := h.nowMs + int64(h.Gaussian(0, sigmaMs)+0.5)
		if h.receiveTimeMs < c {
			h.receiveTimeMs = c
		}
		if h.detector.State() == Overusing {
			return i + 1
		}
	}
	return -1
}

func (h *OveruseDetectorTestHelper) NextOutput() uint64 {
	h.state ^= h.state >> 12
	h.state ^= h.state << 25
	h.state ^= h.state >> 27
	return h.state * 2685821657736338717
}

func (h *OveruseDetectorTestHelper) Gaussian(mean, standardDeviation int) float64 {
	kPi := 3.14159265358979323846
	u1 := float64(h.NextOutput()) / 0xFFFFFFFFFFFFFFFF
	u2 := float64(h.NextOutput()) / 0xFFFFFFFFFFFFFFFF

	return float64(mean) + float64(standardDeviation)*math.Sqrt(-2*math.Log(u1))*math.Cos(2*kPi*u2)
}

func TestOveruseDetector(t *testing.T) {
	tests := []struct {
		name   string
		method func(*testing.T, *OveruseDetectorTestHelper)
	}{
		{
			name: "GaussianRandom",
			method: func(t *testing.T, h *OveruseDetectorTestHelper) {
				buckets := make([]int, 100)
				for i := 0; i < 100000; i++ {
					index := int(h.Gaussian(49, 10))
					if index >= 0 && index < 100 {
						buckets[index] += 1
					}
				}
				for n := 0; n < 100; n++ {
					t.Logf("bucket n:%d, %d\n", n, buckets[n])
				}
			},
		},
		{
			name: "SimpleNonOveruse30fps",
			method: func(t *testing.T, h *OveruseDetectorTestHelper) {
				packetSize := 1200
				frameDurationMs := 33
				irtpTimestamp := uint32(10 * 90)
				for i := 0; i < 1000; i++ {
					h.UpdateDetector(irtpTimestamp, h.nowMs, packetSize)
					h.nowMs += int64(frameDurationMs)
					irtpTimestamp += uint32(frameDurationMs) * 90
					if h.detector.State() != Normal {
						t.Fatal("should be normal")
					}
				}
			},
		},
		{
			name: "SimpleNonOveruseWithReceiveVariance",
			method: func(t *testing.T, h *OveruseDetectorTestHelper) {
				packetSize := 1200
				frameDurationMs := 10
				irtpTimestamp := uint32(10 * 90)
				for i := 0; i < 1000; i++ {
					h.UpdateDetector(irtpTimestamp, h.nowMs, packetSize)
					if i%2 == 0 {
						h.nowMs += int64(frameDurationMs) - 5
					} else {
						h.nowMs += int64(frameDurationMs) + 5
					}
					irtpTimestamp += uint32(frameDurationMs) * 90
					if h.detector.State() != Normal {
						t.Fatal("should be normal")
					}
				}
			},
		},
		{
			name: "SimpleNonOveruseWithRtpTimestampVariance",
			method: func(t *testing.T, h *OveruseDetectorTestHelper) {
				packetSize := 1200
				frameDurationMs := 10
				irtpTimestamp := uint32(10 * 90)
				for i := 0; i < 1000; i++ {
					h.UpdateDetector(irtpTimestamp, h.nowMs, packetSize)
					h.nowMs += int64(frameDurationMs)
					if i%2 == 0 {
						irtpTimestamp += uint32(frameDurationMs-5) * 90
					} else {
						irtpTimestamp += uint32(frameDurationMs+5) * 90
					}
					if h.detector.State() != Normal {
						t.Fatal("should be normal")
					}
				}
			},
		},
		{
			name: "SimpleOveruse2000Kbit30fps",
			method: func(t *testing.T, h *OveruseDetectorTestHelper) {
				packetSize := 1200
				packetPerFrame := 6
				frameDurationMs := 33
				driftPerFrameMs := 1
				sigmaMs := 0
				uniqueOveruse := h.Run100000Samples(packetPerFrame, packetSize, frameDurationMs, sigmaMs)
				if uniqueOveruse != 0 {
					t.Fatal("should be equal")
				}
				frameUntilOveruse := h.RunUntilOveruse(packetPerFrame, packetSize, frameDurationMs, sigmaMs, driftPerFrameMs)
				if frameUntilOveruse != 7 {
					t.Fatal("should be equal", frameUntilOveruse)
				}
			},
		},
		{
			name: "SimpleOveruse100kbit10fps",
			method: func(t *testing.T, h *OveruseDetectorTestHelper) {
				packetSize := 1200
				packetPerFrame := 1
				frameDurationMs := 100
				driftPerFrameMs := 1
				sigmaMs := 0
				uniqueOveruse := h.Run100000Samples(packetPerFrame, packetSize, frameDurationMs, sigmaMs)
				if uniqueOveruse != 0 {
					t.Fatal("should be equal")
				}
				frameUntilOveruse := h.RunUntilOveruse(packetPerFrame, packetSize, frameDurationMs, sigmaMs, driftPerFrameMs)
				if frameUntilOveruse != 7 {
					t.Fatal("should be equal", frameUntilOveruse)
				}
			},
		},

		{
			name: "OveruseWithLowVariance2000Kbit30fps",
			method: func(t *testing.T, h *OveruseDetectorTestHelper) {
				frameDurationMs := 33
				driftPerFrameMs := 1
				irtpTimestamp := uint32(frameDurationMs * 90)
				packetSize := 1200
				offset := 0

				// Run 1000 samples to reach steady state.
				for i := 0; i < 1000; i++ {
					h.UpdateDetector(irtpTimestamp, h.nowMs, packetSize)
					h.UpdateDetector(irtpTimestamp, h.nowMs, packetSize)
					h.UpdateDetector(irtpTimestamp, h.nowMs, packetSize)
					h.UpdateDetector(irtpTimestamp, h.nowMs, packetSize)
					h.UpdateDetector(irtpTimestamp, h.nowMs, packetSize)
					h.UpdateDetector(irtpTimestamp, h.nowMs, packetSize)
					irtpTimestamp += uint32(frameDurationMs * 90)
					if i%2 == 0 {
						offset = rand.Intn(1)
						h.nowMs += int64(frameDurationMs - offset)
					} else {
						h.nowMs += int64(frameDurationMs + offset)
					}
					if h.detector.State() != Normal {
						t.Fatal("should be normal")
					}
				}
				for j := 0; j < 3; j++ {
					h.UpdateDetector(irtpTimestamp, h.nowMs, packetSize)
					h.UpdateDetector(irtpTimestamp, h.nowMs, packetSize)
					h.UpdateDetector(irtpTimestamp, h.nowMs, packetSize)
					h.UpdateDetector(irtpTimestamp, h.nowMs, packetSize)
					h.UpdateDetector(irtpTimestamp, h.nowMs, packetSize)
					h.UpdateDetector(irtpTimestamp, h.nowMs, packetSize)
					h.nowMs += int64(frameDurationMs + driftPerFrameMs*6)
					irtpTimestamp += uint32(frameDurationMs * 90)
					if h.detector.State() != Normal {
						t.Fatal("should be normal")
					}
				}
				h.UpdateDetector(irtpTimestamp, h.nowMs, packetSize)
				if h.detector.State() != Overusing {
					t.Fatal("should be overusing")
				}
			},
		},
		{
			name: "LowGaussianVariance30Kbit3fps",
			method: func(t *testing.T, h *OveruseDetectorTestHelper) {
				packetSize := 1200
				packetPerFrame := 1
				frameDurationMs := 333
				driftPerFrameMs := 1
				sigmaMs := 3
				uniqueOveruse := h.Run100000Samples(packetPerFrame, packetSize, frameDurationMs, sigmaMs)
				if uniqueOveruse != 0 {
					t.Fatal("should be equal")
				}
				frameUntilOveruse := h.RunUntilOveruse(packetPerFrame, packetSize, frameDurationMs, sigmaMs, driftPerFrameMs)
				if frameUntilOveruse != 20 {
					t.Fatal("should be equal", frameUntilOveruse)
				}
			},
		},
		{
			name: "LowGaussianVarianceFastDrift30Kbit3fps",
			method: func(t *testing.T, h *OveruseDetectorTestHelper) {
				packetSize := 1200
				packetPerFrame := 1
				frameDurationMs := 333
				driftPerFrameMs := 100
				sigmaMs := 3
				uniqueOveruse := h.Run100000Samples(packetPerFrame, packetSize, frameDurationMs, sigmaMs)
				if uniqueOveruse != 0 {
					t.Fatal("should be equal")
				}
				frameUntilOveruse := h.RunUntilOveruse(packetPerFrame, packetSize, frameDurationMs, sigmaMs, driftPerFrameMs)
				if frameUntilOveruse != 4 {
					t.Fatal("should be equal", frameUntilOveruse, 4)
				}
			},
		},
		{
			name: "HighGaussianVariance30Kbit3fps",
			method: func(t *testing.T, h *OveruseDetectorTestHelper) {
				packetSize := 1200
				packetPerFrame := 1
				frameDurationMs := 333
				driftPerFrameMs := 1
				sigmaMs := 10
				uniqueOveruse := h.Run100000Samples(packetPerFrame, packetSize, frameDurationMs, sigmaMs)
				if uniqueOveruse != 0 {
					t.Fatal("should be equal")
				}
				frameUntilOveruse := h.RunUntilOveruse(packetPerFrame, packetSize, frameDurationMs, sigmaMs, driftPerFrameMs)
				if frameUntilOveruse != 44 {
					t.Fatal("should be equal", frameUntilOveruse)
				}
			},
		},
		{
			name: "HighGaussianVarianceFastDrift30Kbit3fps",
			method: func(t *testing.T, h *OveruseDetectorTestHelper) {
				packetSize := 1200
				packetPerFrame := 1
				frameDurationMs := 333
				driftPerFrameMs := 100
				sigmaMs := 10
				uniqueOveruse := h.Run100000Samples(packetPerFrame, packetSize, frameDurationMs, sigmaMs)
				if uniqueOveruse != 0 {
					t.Fatal("should be equal")
				}
				frameUntilOveruse := h.RunUntilOveruse(packetPerFrame, packetSize, frameDurationMs, sigmaMs, driftPerFrameMs)
				if frameUntilOveruse != 4 {
					t.Fatal("should be equal", frameUntilOveruse)
				}
			},
		},
		{
			name: "LowGaussianVariance100Kbit5fps",
			method: func(t *testing.T, h *OveruseDetectorTestHelper) {
				packetSize := 1200
				packetPerFrame := 2
				frameDurationMs := 200
				driftPerFrameMs := 1
				sigmaMs := 3
				uniqueOveruse := h.Run100000Samples(packetPerFrame, packetSize, frameDurationMs, sigmaMs)
				if uniqueOveruse != 0 {
					t.Fatal("should be equal")
				}
				frameUntilOveruse := h.RunUntilOveruse(packetPerFrame, packetSize, frameDurationMs, sigmaMs, driftPerFrameMs)
				if frameUntilOveruse != 20 {
					t.Fatal("should be equal", frameUntilOveruse)
				}
			},
		},
		{
			name: "HighGaussianVariance100Kbit5fps",
			method: func(t *testing.T, h *OveruseDetectorTestHelper) {
				packetSize := 1200
				packetPerFrame := 2
				frameDurationMs := 200
				driftPerFrameMs := 1
				sigmaMs := 10
				uniqueOveruse := h.Run100000Samples(packetPerFrame, packetSize, frameDurationMs, sigmaMs)
				if uniqueOveruse != 0 {
					t.Fatal("should be equal")
				}
				frameUntilOveruse := h.RunUntilOveruse(packetPerFrame, packetSize, frameDurationMs, sigmaMs, driftPerFrameMs)
				if frameUntilOveruse != 44 {
					t.Fatal("should be equal", frameUntilOveruse)
				}
			},
		},
		{
			name: "LowGaussianVariance100Kbit10fps",
			method: func(t *testing.T, h *OveruseDetectorTestHelper) {
				packetSize := 1200
				packetPerFrame := 1
				frameDurationMs := 100
				driftPerFrameMs := 1
				sigmaMs := 3
				uniqueOveruse := h.Run100000Samples(packetPerFrame, packetSize, frameDurationMs, sigmaMs)
				if uniqueOveruse != 0 {
					t.Fatal("should be equal")
				}
				frameUntilOveruse := h.RunUntilOveruse(packetPerFrame, packetSize, frameDurationMs, sigmaMs, driftPerFrameMs)
				if frameUntilOveruse != 20 {
					t.Fatal("should be equal", frameUntilOveruse)
				}
			},
		},
		{
			name: "HighGaussianVariance100Kbit10fps",
			method: func(t *testing.T, h *OveruseDetectorTestHelper) {
				packetSize := 1200
				packetPerFrame := 1
				frameDurationMs := 100
				driftPerFrameMs := 1
				sigmaMs := 10
				uniqueOveruse := h.Run100000Samples(packetPerFrame, packetSize, frameDurationMs, sigmaMs)
				if uniqueOveruse != 0 {
					t.Fatal("should be equal")
				}
				frameUntilOveruse := h.RunUntilOveruse(packetPerFrame, packetSize, frameDurationMs, sigmaMs, driftPerFrameMs)
				if frameUntilOveruse != 44 {
					t.Fatal("should be equal", frameUntilOveruse)
				}
			},
		},
		{
			name: "LowGaussianVariance300Kbit30fps",
			method: func(t *testing.T, h *OveruseDetectorTestHelper) {
				packetSize := 1200
				packetPerFrame := 1
				frameDurationMs := 33
				driftPerFrameMs := 1
				sigmaMs := 3
				uniqueOveruse := h.Run100000Samples(packetPerFrame, packetSize, frameDurationMs, sigmaMs)
				if uniqueOveruse != 0 {
					t.Fatal("should be equal")
				}
				frameUntilOveruse := h.RunUntilOveruse(packetPerFrame, packetSize, frameDurationMs, sigmaMs, driftPerFrameMs)
				if frameUntilOveruse != 19 {
					t.Fatal("should be equal", frameUntilOveruse)
				}
			},
		},
		{
			name: "LowGaussianVarianceFastDrift300Kbit30fps",
			method: func(t *testing.T, h *OveruseDetectorTestHelper) {
				packetSize := 1200
				packetPerFrame := 1
				frameDurationMs := 33
				driftPerFrameMs := 10
				sigmaMs := 3
				uniqueOveruse := h.Run100000Samples(packetPerFrame, packetSize, frameDurationMs, sigmaMs)
				if uniqueOveruse != 0 {
					t.Fatal("should be equal")
				}
				frameUntilOveruse := h.RunUntilOveruse(packetPerFrame, packetSize, frameDurationMs, sigmaMs, driftPerFrameMs)
				if frameUntilOveruse != 5 {
					t.Fatal("should be equal", frameUntilOveruse)
				}
			},
		},
		{
			name: "HighGaussianVariance300Kbit30fps",
			method: func(t *testing.T, h *OveruseDetectorTestHelper) {
				packetSize := 1200
				packetPerFrame := 1
				frameDurationMs := 33
				driftPerFrameMs := 1
				sigmaMs := 10
				uniqueOveruse := h.Run100000Samples(packetPerFrame, packetSize, frameDurationMs, sigmaMs)
				if uniqueOveruse != 0 {
					t.Fatal("should be equal")
				}
				frameUntilOveruse := h.RunUntilOveruse(packetPerFrame, packetSize, frameDurationMs, sigmaMs, driftPerFrameMs)
				if frameUntilOveruse != 44 {
					t.Fatal("should be equal", frameUntilOveruse)
				}
			},
		},
		{
			name: "HighGaussianVarianceFastDrift300Kbit30fps",
			method: func(t *testing.T, h *OveruseDetectorTestHelper) {
				packetSize := 1200
				packetPerFrame := 1
				frameDurationMs := 33
				driftPerFrameMs := 10
				sigmaMs := 10
				uniqueOveruse := h.Run100000Samples(packetPerFrame, packetSize, frameDurationMs, sigmaMs)
				if uniqueOveruse != 0 {
					t.Fatal("should be equal")
				}
				frameUntilOveruse := h.RunUntilOveruse(packetPerFrame, packetSize, frameDurationMs, sigmaMs, driftPerFrameMs)
				if frameUntilOveruse != 10 {
					t.Fatal("should be equal", frameUntilOveruse)
				}
			},
		},
		{
			name: "LowGaussianVariance1000Kbit30fps",
			method: func(t *testing.T, h *OveruseDetectorTestHelper) {
				packetSize := 1200
				packetPerFrame := 3
				frameDurationMs := 33
				driftPerFrameMs := 1
				sigmaMs := 3
				uniqueOveruse := h.Run100000Samples(packetPerFrame, packetSize, frameDurationMs, sigmaMs)
				if uniqueOveruse != 0 {
					t.Fatal("should be equal")
				}
				frameUntilOveruse := h.RunUntilOveruse(packetPerFrame, packetSize, frameDurationMs, sigmaMs, driftPerFrameMs)
				if frameUntilOveruse != 19 {
					t.Fatal("should be equal", frameUntilOveruse)
				}
			},
		},
		{
			name: "LowGaussianVarianceFastDrift1000Kbit30fps",
			method: func(t *testing.T, h *OveruseDetectorTestHelper) {
				packetSize := 1200
				packetPerFrame := 3
				frameDurationMs := 33
				driftPerFrameMs := 10
				sigmaMs := 3
				uniqueOveruse := h.Run100000Samples(packetPerFrame, packetSize, frameDurationMs, sigmaMs)
				if uniqueOveruse != 0 {
					t.Fatal("should be equal")
				}
				frameUntilOveruse := h.RunUntilOveruse(packetPerFrame, packetSize, frameDurationMs, sigmaMs, driftPerFrameMs)
				if frameUntilOveruse != 5 {
					t.Fatal("should be equal", frameUntilOveruse)
				}
			},
		},
		{
			name: "HighGaussianVariance1000Kbit30fps",
			method: func(t *testing.T, h *OveruseDetectorTestHelper) {
				packetSize := 1200
				packetPerFrame := 3
				frameDurationMs := 33
				driftPerFrameMs := 1
				sigmaMs := 10
				uniqueOveruse := h.Run100000Samples(packetPerFrame, packetSize, frameDurationMs, sigmaMs)
				if uniqueOveruse != 0 {
					t.Fatal("should be equal")
				}
				frameUntilOveruse := h.RunUntilOveruse(packetPerFrame, packetSize, frameDurationMs, sigmaMs, driftPerFrameMs)
				if frameUntilOveruse != 44 {
					t.Fatal("should be equal", frameUntilOveruse)
				}
			},
		},
		{
			name: "HighGaussianVarianceFastDrift1000Kbit30fps",
			method: func(t *testing.T, h *OveruseDetectorTestHelper) {
				packetSize := 1200
				packetPerFrame := 3
				frameDurationMs := 33
				driftPerFrameMs := 10
				sigmaMs := 10
				uniqueOveruse := h.Run100000Samples(packetPerFrame, packetSize, frameDurationMs, sigmaMs)
				if uniqueOveruse != 0 {
					t.Fatal("should be equal")
				}
				frameUntilOveruse := h.RunUntilOveruse(packetPerFrame, packetSize, frameDurationMs, sigmaMs, driftPerFrameMs)
				if frameUntilOveruse != 10 {
					t.Fatal("should be equal", frameUntilOveruse)
				}
			},
		},
		{
			name: "LowGaussianVariance2000Kbit30fps",
			method: func(t *testing.T, h *OveruseDetectorTestHelper) {
				packetSize := 1200
				packetPerFrame := 6
				frameDurationMs := 33
				driftPerFrameMs := 1
				sigmaMs := 3
				uniqueOveruse := h.Run100000Samples(packetPerFrame, packetSize, frameDurationMs, sigmaMs)
				if uniqueOveruse != 0 {
					t.Fatal("should be equal")
				}
				frameUntilOveruse := h.RunUntilOveruse(packetPerFrame, packetSize, frameDurationMs, sigmaMs, driftPerFrameMs)
				if frameUntilOveruse != 19 {
					t.Fatal("should be equal", frameUntilOveruse)
				}
			},
		},
		{
			name: "LowGaussianVarianceFastDrift2000Kbit30fps",
			method: func(t *testing.T, h *OveruseDetectorTestHelper) {
				packetSize := 1200
				packetPerFrame := 6
				frameDurationMs := 33
				driftPerFrameMs := 10
				sigmaMs := 3
				uniqueOveruse := h.Run100000Samples(packetPerFrame, packetSize, frameDurationMs, sigmaMs)
				if uniqueOveruse != 0 {
					t.Fatal("should be equal")
				}
				frameUntilOveruse := h.RunUntilOveruse(packetPerFrame, packetSize, frameDurationMs, sigmaMs, driftPerFrameMs)
				if frameUntilOveruse != 5 {
					t.Fatal("should be equal", frameUntilOveruse)
				}
			},
		},
		{
			name: "HighGaussianVariance2000Kbit30fps",
			method: func(t *testing.T, h *OveruseDetectorTestHelper) {
				packetSize := 1200
				packetPerFrame := 6
				frameDurationMs := 33
				driftPerFrameMs := 1
				sigmaMs := 10
				uniqueOveruse := h.Run100000Samples(packetPerFrame, packetSize, frameDurationMs, sigmaMs)
				if uniqueOveruse != 0 {
					t.Fatal("should be equal")
				}
				frameUntilOveruse := h.RunUntilOveruse(packetPerFrame, packetSize, frameDurationMs, sigmaMs, driftPerFrameMs)
				if frameUntilOveruse != 44 {
					t.Fatal("should be equal", frameUntilOveruse)
				}
			},
		},
		{
			name: "HighGaussianVarianceFastDrift2000Kbit30fps",
			method: func(t *testing.T, h *OveruseDetectorTestHelper) {
				packetSize := 1200
				packetPerFrame := 6
				frameDurationMs := 33
				driftPerFrameMs := 10
				sigmaMs := 10
				uniqueOveruse := h.Run100000Samples(packetPerFrame, packetSize, frameDurationMs, sigmaMs)
				if uniqueOveruse != 0 {
					t.Fatal("should be equal")
				}
				frameUntilOveruse := h.RunUntilOveruse(packetPerFrame, packetSize, frameDurationMs, sigmaMs, driftPerFrameMs)
				if frameUntilOveruse != 10 {
					t.Fatal("should be equal", frameUntilOveruse)
				}
			},
		},
	}
	for _, test := range tests {
		h := &OveruseDetectorTestHelper{
			ia:               newInterArrival(5*90, 1.0/90, true),
			overuseEstimator: NewOveruseEstimator(),
			detector:         NewOveruseDetector(),
			nowMs:            0,
			receiveTimeMs:    0,
			rtpTimestamp:     10 * 90,
			state:            123456789,
		}
		t.Run(test.name, func(t *testing.T) {
			test.method(t, h)
		})
	}
}
