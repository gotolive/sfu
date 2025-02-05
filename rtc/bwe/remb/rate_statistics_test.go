package remb

import (
	"math"
	"testing"
)

type RateStatisticsTestHelper struct {
	stats *RateStatistics
}

func TestRateStatistics(t *testing.T) {
	tests := []struct {
		name   string
		method func(*testing.T, *RateStatisticsTestHelper)
	}{
		{
			name: "StrictMode",
			method: func(t *testing.T, h *RateStatisticsTestHelper) {
				var nowMs int64 = 0
				if b := h.stats.Rate(nowMs); b != nil {
					t.Fatal("should be empty", b)
				}
				packetSize := int64(1500)                         // 1500 byte in one ms
				expectedRateBps := Bitrate(packetSize * 1000 * 8) // bitrate / per second
				h.stats.Update(packetSize, nowMs)
				nowMs++
				if b := h.stats.Rate(nowMs); b != nil {
					t.Fatal("should be empty", b)
				}
				h.stats.Update(packetSize, nowMs)
				if *h.stats.Rate(nowMs) != expectedRateBps {
					t.Fatal("should eq expect:", expectedRateBps, "real:", h.stats.Rate(nowMs))
				}
				h.stats.Reset()
				if h.stats.Rate(nowMs) != nil {
					t.Fatal("should be empty")
				}
				interval := 10
				for i := 0; i < 100000; i++ {
					// update stats every 10ms
					if i%interval == 0 {
						h.stats.Update(packetSize, nowMs)
					}
					if i > interval {
						rate := h.stats.Rate(nowMs)
						if rate == nil {
							t.Fatal("should gt zero")
						}
						samples := int64(i/interval + 1)
						totalBits := samples * packetSize * 8
						rateBps := (1000 * totalBits) / (int64(i) + 1)
						if math.Abs(float64(int64(*rate)-rateBps)) > 22000.0 {
							t.Fatal("wrong rateBps, rate:", rate, " rateBps", rateBps)
						}
					}
					nowMs++
				}
				// our window size is 500ms.
				nowMs += 500
				if h.stats.Rate(nowMs) != nil {
					t.Fatal("should be zero")
				}
			},
		},
		{
			name: "IncreasingThenDecreasingBitrate",
			method: func(t *testing.T, h *RateStatisticsTestHelper) {
				var nowMs int64 = 0
				if h.stats.Rate(nowMs) != nil {
					t.Fatal("should be empty")
				}
				nowMs++
				h.stats.Update(1000, nowMs)
				var expectedBitrate int64 = 8000000
				prevError := expectedBitrate
				nowMs++
				var bitrate Bitrate
				for ; nowMs < 10000; nowMs++ {
					h.stats.Update(1000, nowMs)
					bitrate = *h.stats.Rate(nowMs)
					if bitrate == 0 {
						t.Fatal("should not be zero on step:", nowMs)
					}
					errorValue := expectedBitrate - int64(bitrate)
					if errorValue < 0 {
						errorValue = -errorValue
					}
					if errorValue > prevError+1 {
						t.Fatal("wrong")
					}
					prevError = errorValue
				}
				if int64(bitrate) != expectedBitrate {
					t.Fatal("should be eq")
				}
				nowMs++
				for ; nowMs < 10000; nowMs++ {
					h.stats.Update(1000, nowMs)
					bitrate = *h.stats.Rate(nowMs)
					if int64(bitrate) != expectedBitrate {
						t.Fatal("should be eq")
					}
				}
				nowMs++
				for ; nowMs < 20000; nowMs++ {
					h.stats.Update(0, nowMs)
					newBitrate := h.stats.Rate(nowMs)
					if newBitrate != nil && *newBitrate != bitrate {
						if *newBitrate > bitrate {
							t.Fatal("should not gt")
						}
					} else {
						if *newBitrate != 0 {
							t.Fatal("new bitrate should be zero")
						}
						break
					}
					bitrate = *newBitrate
				}
				nowMs++
				for ; nowMs < 20000; nowMs++ {
					h.stats.Update(0, nowMs)
					if *h.stats.Rate(nowMs) != 0 {
						t.Fatal("should be zero")
					}
				}
			},
		},
		{
			name: "ResetAfterSilence",
			method: func(t *testing.T, h *RateStatisticsTestHelper) {
				var nowMs int64 = 0
				if h.stats.Rate(nowMs) != nil {
					t.Fatal("should be empty")
				}
				expectedBitrate := Bitrate(8000000)
				prevError := expectedBitrate
				var bitrate *Bitrate
				nowMs++
				for ; nowMs < 10000; nowMs++ {
					h.stats.Update(1000, nowMs)
					bitrate = h.stats.Rate(nowMs)
					if bitrate != nil {
						errorValue := expectedBitrate - *bitrate
						if errorValue < 0 {
							errorValue = -errorValue
						}
						if errorValue > prevError+1 {
							t.Fatal("should be le")
						}
						prevError = errorValue
					}
				}
				if expectedBitrate != *bitrate {
					t.Fatal("should be eq")
				}
				nowMs += 500 + 1
				if h.stats.Rate(nowMs) != nil {
					t.Fatal("should be zero")
				}
				h.stats.Update(1000, nowMs)
				nowMs++
				h.stats.Update(1000, nowMs)
				if *h.stats.Rate(nowMs) != 32000 {
					// TODO
					// t.Fatal("should be equal")
				}
				h.stats.Reset()
				if h.stats.Rate(nowMs) != nil {
					t.Fatal("should be zero")
				}
				h.stats.Update(1000, nowMs)
				nowMs++
				h.stats.Update(1000, nowMs)
				if *h.stats.Rate(nowMs) != expectedBitrate {
					t.Fatal("should be eq")
				}
			},
		},

		{
			name: "RespectsWindowSizeEdges",
			method: func(t *testing.T, h *RateStatisticsTestHelper) {
				var nowMs int64 = 0
				if h.stats.Rate(nowMs) != nil {
					t.Fatal("should be zero")
				}
				h.stats.Update(500, nowMs)
				nowMs += 500 - 2
				if h.stats.Rate(nowMs) != nil {
					t.Fatal("should be zero")
				}
				nowMs++
				bitrate := h.stats.Rate(nowMs)
				if bitrate == nil {
					t.Fatal("should be bigger")
				}
				if *bitrate != 8000 {
					t.Fatal("should be eq")
				}
				h.stats.Update(500, nowMs)
				bitrate = h.stats.Rate(nowMs)
				if bitrate == nil {
					t.Fatal("should be bigger")
				}
				if *bitrate != 8000*2 {
					t.Fatal("should be eq")
				}
				nowMs += 1
				bitrate = h.stats.Rate(nowMs)
				if bitrate == nil {
					t.Fatal("should be bigger")
				}
				if *bitrate != 8000 {
					t.Fatal("should be eq")
				}
			},
		},
		{
			name: "HandlesZeroCounts",
			method: func(t *testing.T, h *RateStatisticsTestHelper) {
				var nowMs int64 = 0
				if h.stats.Rate(nowMs) != nil {
					t.Fatal("should be zero")
				}

				h.stats.Update(500, nowMs)
				nowMs += 500 - 1

				h.stats.Update(0, nowMs)
				bitrate := h.stats.Rate(nowMs)
				if bitrate == nil || *bitrate != 8000 {
					t.Fatal("wrong bitrate")
				}
				nowMs++
				bitrate = h.stats.Rate(nowMs)
				if bitrate == nil || *bitrate != 0 {
					t.Fatal("should be zero")
				}
				nowMs += 500
				if h.stats.Rate(nowMs) != nil {
					t.Fatal("should be zero")
				}
			},
		},
		{
			name: "HandlesQuietPeriods",
			method: func(t *testing.T, h *RateStatisticsTestHelper) {
				var nowMs int64 = 0
				if h.stats.Rate(nowMs) != nil {
					t.Fatal("should be zero")
				}

				h.stats.Update(0, nowMs)
				nowMs += 500 - 1

				bitrate := h.stats.Rate(nowMs)
				if bitrate == nil || *bitrate != 0 {
					t.Fatal("wrong bitrate")
				}
				nowMs++
				bitrate = h.stats.Rate(nowMs)
				if bitrate != nil {
					t.Fatal("should be zero")
				}
				nowMs += 2 * 500
				h.stats.Update(0, nowMs)
				if rate := h.stats.Rate(nowMs); rate == nil || *rate != 0 {
					// TODO
					// t.Fatal("should be zero")
				}
			},
		},
		{
			name: "HandlesBigNumbers",
			method: func(t *testing.T, h *RateStatisticsTestHelper) {
				var largeNumber int64 = 0x100000000
				var nowMs int64 = 0
				h.stats.Update(largeNumber, nowMs)
				nowMs++
				h.stats.Update(largeNumber, nowMs)
				if bitrate := h.stats.Rate(nowMs); *bitrate != Bitrate(largeNumber*8000) {
					t.Fatal("should be large but got:", bitrate)
				}
			},
		},
		{
			name: "HandlesTooLargeNumbers",
			method: func(t *testing.T, h *RateStatisticsTestHelper) {
				var largeNumber int64 = math.MaxInt64
				var nowMs int64 = 0
				h.stats.Update(largeNumber, nowMs)
				nowMs++
				h.stats.Update(largeNumber, nowMs)
				if bitrate := h.stats.Rate(nowMs); bitrate != nil {
					t.Fatal("should be zero but got:", bitrate)
				}
			},
		},
		{
			name: "HandlesSomewhatLargeNumbers",
			method: func(t *testing.T, h *RateStatisticsTestHelper) {
				var largeNumber int64 = math.MaxInt64
				var nowMs int64 = 0
				h.stats.Update(largeNumber/4, nowMs)
				nowMs++
				h.stats.Update(largeNumber/4, nowMs)
				if bitrate := h.stats.Rate(nowMs); bitrate != nil {
					t.Fatal("should be zero but got:", bitrate)
				}
			},
		},
		{
			name: "GoValidInputOutputTest",
			method: func(t *testing.T, h *RateStatisticsTestHelper) {
				h.stats = NewRateStatistics(1000, 8000)
				input := int64(102000)
				payloadSize := int64(1200)
				h.stats.Update(payloadSize, input)
				input += 20
				// Only one samples won't have rate
				h.stats.Update(payloadSize, input)
				// if a != nil {
				//	t.Fatal("should be nil",*a)
				// }
				input += 20
				b := h.stats.Rate(input)
				if b == nil {
					t.Fatal("should not be nil")
				}
				if *b != 468293 {
					t.Fatal("should be 468293")
				}
			},
		},
	}

	for _, test := range tests {
		h := &RateStatisticsTestHelper{stats: NewRateStatistics(500, 8000)}
		t.Run(test.name, func(t *testing.T) {
			test.method(t, h)
		})
	}
}
