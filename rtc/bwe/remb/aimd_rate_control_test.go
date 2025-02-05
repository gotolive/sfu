package remb

import (
	"testing"
)

var (
	DefaultPeriodMs      = Millisecond * 3000
	MinBwePeriodMs       = Millisecond * 2000
	MaxBwePeriodMs       = Millisecond * 50000
	ClockInitialTime     = Duration(123456) // us
	FractionAfterOveruse = 0.85
)

type aimdRateControlStates struct {
	arc   *AimdRateControl
	clock *SimulatedClock
}

func CreateAimdRateControlStates(sendSide bool) *aimdRateControlStates {
	return &aimdRateControlStates{
		arc:   NewAimdRateControl(),
		clock: &SimulatedClock{currentUs: ClockInitialTime},
	}
}

func setEstimate(states *aimdRateControlStates, bitrate Bitrate) {
	states.arc.SetEstimate(bitrate, states.clock.NowMs())
}

func UpdateRateControl(state *aimdRateControlStates, usage int, throughputEstimate Bitrate, nowMs int64) {
	input := RateControllerInput{
		State:               usage,
		EstimatedThroughput: &throughputEstimate,
	}
	state.arc.Update(input, nowMs)
}

func TestAimdRateControl(t *testing.T) {
	tests := []struct {
		name   string
		method func(*testing.T)
	}{
		{
			name: "MinNearMaxIncreaseRateOnLowBandwidth",
			method: func(t *testing.T) {
				states := CreateAimdRateControlStates(false)
				bitrate := Bitrate(30000) // Start an 3k bitrate
				setEstimate(states, bitrate)
				// after set current should be 30000
				if states.arc.getNearMaxIncreaseRateBpsPerSecond() != 4000 {
					t.Fatal("should be 4000, but got:", states.arc.getNearMaxIncreaseRateBpsPerSecond())
				}
			},
		},
		{
			name: "NearMaxIncreaseRateIs5kbpsOn90kbpsAnd200msRtt",
			method: func(t *testing.T) {
				states := CreateAimdRateControlStates(false)
				bitrate := Bitrate(90000)
				setEstimate(states, bitrate)
				if states.arc.getNearMaxIncreaseRateBpsPerSecond() != 5000 {
					t.Fatal("should be 5000, but got:", states.arc.getNearMaxIncreaseRateBpsPerSecond())
				}
			},
		},
		{
			name: "NearMaxIncreaseRateIs5kbpsOn60kbpsAnd100msRtt",
			method: func(t *testing.T) {
				states := CreateAimdRateControlStates(false)
				bitrate := Bitrate(60000)

				setEstimate(states, bitrate)
				states.arc.rtt = 100 * Millisecond
				if states.arc.getNearMaxIncreaseRateBpsPerSecond() != 5000 {
					t.Fatal("should be 5000, but got:", states.arc.getNearMaxIncreaseRateBpsPerSecond())
				}
			},
		},
		{
			name: "GetIncreaseRateAndBandwidthPeriod",
			method: func(t *testing.T) {
				states := CreateAimdRateControlStates(false)
				bitrate := Bitrate(300000)
				setEstimate(states, bitrate)
				UpdateRateControl(states, Overusing, bitrate, states.clock.NowMs())
				if states.arc.getNearMaxIncreaseRateBpsPerSecond()-14000 >= 1000 || states.arc.getNearMaxIncreaseRateBpsPerSecond()-14000 <= -1000 {
					t.Fatal("should near 14000, but got:", states.arc.getNearMaxIncreaseRateBpsPerSecond())
				}
				if states.arc.GetExpectedBandwidthPeriod() != 3000*Millisecond {
					t.Fatal("should be 3000, but got:", states.arc.GetExpectedBandwidthPeriod())
				}
			},
		},
		{
			name: "BweLimitedByAckedBitrate",
			method: func(t *testing.T) {
				states := CreateAimdRateControlStates(false)
				bitrate := Bitrate(10000)
				setEstimate(states, bitrate)
				for states.clock.NowMs()-123456 < 20000 {
					UpdateRateControl(states, Normal, bitrate, states.clock.NowMs())
					states.clock.Add(Millisecond * 100)
				}
				if !states.arc.ValidEstimate() {
					t.Fatal("should be true")
				}
				if Bitrate(1.5*float64(bitrate)+10000) != states.arc.LatestEstimate() {
					t.Fatal("should be equeal, but got:", Bitrate(1.5*float64(bitrate)+10000), states.arc.LatestEstimate())
				}
			},
		},
		{
			name: "BweNotLimitedByDecreasingAckedBitrate",
			method: func(t *testing.T) {
				states := CreateAimdRateControlStates(false)
				bitrate := Bitrate(100000)
				setEstimate(states, bitrate)
				for states.clock.NowMs()/1000-123456 < 20000 {
					UpdateRateControl(states, Normal, bitrate, states.clock.NowMs())
					states.clock.Add(Millisecond * 100)
				}
				if !states.arc.ValidEstimate() {
					t.Fatal("should be true")
				}
				prevEstimate := states.arc.LatestEstimate()
				UpdateRateControl(states, Normal, bitrate/2, states.clock.NowMs())
				newEstimate := states.arc.LatestEstimate()
				if newEstimate-Bitrate(1.5*float64(bitrate)+10000) > 2000 || Bitrate(1.5*float64(bitrate)+10000)-newEstimate > 2000 {
					t.Fatal("should be near")
				}
				if prevEstimate != newEstimate {
					t.Fatal("should be equal")
				}
			},
		},
		{
			name: "DefaultPeriodUntilFirstOveruse",
			method: func(t *testing.T) {
				states := CreateAimdRateControlStates(false)
				states.arc.SetStartBitrate(300) // kbps
				if states.arc.GetExpectedBandwidthPeriod() != DefaultPeriodMs {
					t.Fatal("should be equal")
				}
				// microsecond us

				states.clock.Add(Millisecond * 100)
				UpdateRateControl(states, Overusing, 280000, states.clock.NowMs())
				if states.arc.GetExpectedBandwidthPeriod() < DefaultPeriodMs {
					t.Fatal("should be great")
				}
			},
		},
		{
			name: "ExpectedPeriodAfter20kbpsDropAnd5kbpsIncrease",
			method: func(t *testing.T) {
				states := CreateAimdRateControlStates(false)
				bitrate := Bitrate(110000)
				setEstimate(states, bitrate)
				states.clock.Add(Millisecond * 100)
				ackBitrate := float64(bitrate-20000) / FractionAfterOveruse
				UpdateRateControl(states, Overusing, Bitrate(ackBitrate), states.clock.NowMs())
				if states.arc.getNearMaxIncreaseRateBpsPerSecond() != 5000 {
					t.Fatal("should be equal", states.arc.getNearMaxIncreaseRateBpsPerSecond())
				}
				if states.arc.GetExpectedBandwidthPeriod() != 4000*Millisecond {
					t.Fatal("should be equal", states.arc.GetExpectedBandwidthPeriod())
				}
			},
		},
		{
			name: "BandwidthPeriodIsNotBelowMin",
			method: func(t *testing.T) {
				states := CreateAimdRateControlStates(false)
				bitrate := Bitrate(10000)
				setEstimate(states, bitrate)
				states.clock.Add(Millisecond * 100)
				UpdateRateControl(states, Overusing, bitrate-1, states.clock.NowMs())

				if states.arc.GetExpectedBandwidthPeriod() != MinBwePeriodMs {
					t.Fatal("should be equal", states.arc.GetExpectedBandwidthPeriod())
				}
			},
		},
		{
			name: "BandwidthPeriodIsNotAboveMaxNoSmoothingExp",
			method: func(t *testing.T) {
				states := CreateAimdRateControlStates(false)
				bitrate := Bitrate(10010000)
				setEstimate(states, bitrate)
				states.clock.Add(Millisecond * 100)
				ackedBitrate := 10000 / FractionAfterOveruse

				UpdateRateControl(states, Overusing, Bitrate(ackedBitrate), states.clock.NowMs())

				if states.arc.GetExpectedBandwidthPeriod() != MaxBwePeriodMs {
					t.Fatal("should be equal")
				}
			},
		},
		{
			name: "SendingRateBoundedWhenThroughputNotEstimated",
			method: func(t *testing.T) {
				states := CreateAimdRateControlStates(false)
				bitrate := Bitrate(123000)
				UpdateRateControl(states, Normal, bitrate, states.clock.NowMs())
				timeMs := 5000
				states.clock.Add(Millisecond * Duration(timeMs+1))
				UpdateRateControl(states, Normal, bitrate, states.clock.NowMs())
				for i := 0; i < 100; i++ {
					UpdateRateControl(states, Normal, 0, states.clock.NowMs())
					states.clock.Add(Millisecond * 100)
				}
				if states.arc.LatestEstimate() > Bitrate(float64(bitrate)*1.5)+10000 {
					t.Fatal("should be le")
				}
			},
		},
		// The next three test based on WebRTC-DontIncreaseDelayBasedBweInAlr/Enabled
		{
			name: "EstimateDoesNotIncreaseInAlr",
			method: func(t *testing.T) {
			},
		},
		{
			name: "SetEstimateIncreaseBweInAlr",
			method: func(t *testing.T) {
			},
		},
		{
			name: "EstimateIncreaseWhileNotInAlr",
			method: func(t *testing.T) {
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.method(t)
		})
	}
}
