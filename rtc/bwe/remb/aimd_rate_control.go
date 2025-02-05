package remb

import (
	"math"
)

const (
	InitializationTime          = 5000 // 5s
	FrameInterval               = 1000 / 30
	PacketSize                  = 1200
	MinIncreaseRateBpsPerSecond = 4000
	DefaultRTT                  = 200
)

const (
	RcHold     = 0
	RcIncrease = 1
	RcDecrease = 2

	MinPeriod     = Second * 2
	DefaultPeriod = Second * 3
	MaxPeriod     = Second * 50
)

func NewAimdRateControl() *AimdRateControl {
	return &AimdRateControl{
		inExperiment:                true,
		beta:                        0.85,
		rtt:                         DefaultRTT * Millisecond,
		timeFirstThroughputEstimate: -1,
		linkCapacity:                NewLinkCapacityEstimator(),
		maxConfiguredBitrate:        30000 * 1000,
		currentBitrate:              30000 * 1000, // default value
	}
}

type AimdRateControl struct {
	bitrateIsInitialized        bool
	timeFirstThroughputEstimate int64
	currentBitrate              Bitrate
	latestEstimatedThroughput   Bitrate
	rateControlState            int
	timeLastBitrateChange       int64
	linkCapacity                *LinkCapacityEstimator
	sendSide                    bool
	inAlr                       bool
	noBitrateIncreaseInAlr      bool
	beta                        float64
	linkCapacityFix             bool
	estimateBoundedBackoff      bool
	networkEstimate             *NetworkStateEstimate
	lastDecrease                Bitrate
	timeLastBitrateDecrease     int64
	inExperiment                bool
	rtt                         Duration
	minConfiguredBitrate        Bitrate
	maxConfiguredBitrate        Bitrate
}

func (c *AimdRateControl) Update(input RateControllerInput, now int64) Bitrate {
	if !c.bitrateIsInitialized {
		if c.timeFirstThroughputEstimate == -1 {
			// Review !=0? or !=nil?
			if input.EstimatedThroughput != nil {
				c.timeFirstThroughputEstimate = now
			}
		} else if now-c.timeFirstThroughputEstimate > InitializationTime && input.EstimatedThroughput != nil {
			c.currentBitrate = *input.EstimatedThroughput
			c.bitrateIsInitialized = true
		}
	}
	c.changeBitrate(input, now)
	return c.currentBitrate
}

func (c *AimdRateControl) ValidEstimate() bool {
	return c.bitrateIsInitialized
}

func (c *AimdRateControl) TimeToReduceFurther(ms int64, rate Bitrate) bool {
	panic("implements me")
}

func (c *AimdRateControl) FeedbackInterval() Duration {
	// 100000 bit per second ~= 100000/8 byte per second
	// %5 means 100000/20 = 5000 bit per second
	// 640/5000
	// 5000/8 = 625 byte per second
	// every packet 80 byte
	// 625/80~=8packet every second
	// 1000/8 ~= 125ms
	//   (80*8)/(100000 * 0.05) 0.128s~=128ms
	rtcpSize := 80.0                             // every rtcp packet
	rtcpRate := float64(c.currentBitrate) * 0.05 // how much bitrate we should use?
	//
	interval := Duration((rtcpSize * 8 / rtcpRate) * float64(Second))
	// fmt.Println(interval, c.currentBitrate)
	if interval < 200*Millisecond {
		// fmt.Println(interval, c.currentBitrate)
		return 200 * Millisecond
	}
	if interval > Second {
		return Second
	}
	return interval
}

func (c *AimdRateControl) changeBitrate(input RateControllerInput, now int64) {
	var newBitrate Bitrate
	estimatedThroughput := c.latestEstimatedThroughput
	if input.EstimatedThroughput != nil {
		estimatedThroughput = *input.EstimatedThroughput
		c.latestEstimatedThroughput = *input.EstimatedThroughput
	}

	if !c.bitrateIsInitialized && input.State != Overusing {
		return
	}

	c.changeState(input, now)

	switch c.rateControlState {
	case RcHold:
		return
	case RcIncrease:
		newBitrate = c.increase(estimatedThroughput, now)
	case RcDecrease:
		newBitrate = c.decrease(estimatedThroughput, now)
	}
	if newBitrate != 0 {
		c.currentBitrate = newBitrate
	}
}

func (c *AimdRateControl) increase(estimatedThroughput Bitrate, now int64) Bitrate {
	var newBitrate Bitrate
	throughputBasedLimit := Bitrate(1.5*float64(estimatedThroughput) + 0.5 + 10*1000)
	if estimatedThroughput > c.linkCapacity.UpperBound() {
		c.linkCapacity.Reset()
	}
	if c.currentBitrate < throughputBasedLimit && !(c.sendSide && c.inAlr && c.noBitrateIncreaseInAlr) {
		var increasedBitrate Bitrate
		if c.linkCapacity.HasEstimate() {
			addictiveIncrease := c.additiveRateIncrease(now, c.timeLastBitrateChange)
			increasedBitrate = c.currentBitrate + addictiveIncrease
		} else {
			multiplicativeIncrease := c.multiplicativeRateIncrease(now, c.timeLastBitrateChange, c.currentBitrate)
			increasedBitrate = c.currentBitrate + multiplicativeIncrease
		}
		newBitrate = throughputBasedLimit
		if increasedBitrate < throughputBasedLimit {
			newBitrate = increasedBitrate
		}
	}
	c.timeLastBitrateChange = now
	return newBitrate
}

func (c *AimdRateControl) decrease(estimatedThroughput Bitrate, now int64) Bitrate {
	var decreasedBitrate Bitrate
	var newBitrate Bitrate
	decreasedBitrate = Bitrate(float64(estimatedThroughput) * c.beta)
	if decreasedBitrate > c.currentBitrate && !c.linkCapacityFix {
		if c.linkCapacity.HasEstimate() {
			decreasedBitrate = Bitrate(c.beta * float64(c.linkCapacity.Estimate()))
		}
	}
	if c.estimateBoundedBackoff && c.networkEstimate != nil {
		if decreasedBitrate < Bitrate(float64(c.networkEstimate.LinkCapacityLower())*c.beta) {
			decreasedBitrate = Bitrate(float64(c.networkEstimate.LinkCapacityLower()) * c.beta)
		}
	}

	if decreasedBitrate < c.currentBitrate {
		newBitrate = decreasedBitrate
	}
	if c.bitrateIsInitialized && estimatedThroughput < c.currentBitrate {
		if newBitrate == 0 {
			c.lastDecrease = 0
		} else {
			c.lastDecrease = c.currentBitrate - newBitrate
		}
	}
	if estimatedThroughput < c.linkCapacity.LowerBound() {
		c.linkCapacity.Reset()
	}
	c.bitrateIsInitialized = true
	c.linkCapacity.OnOveruseDetected(estimatedThroughput)
	c.rateControlState = RcHold
	c.timeLastBitrateChange = now
	c.timeLastBitrateDecrease = now
	return newBitrate
}

func (c *AimdRateControl) changeState(input RateControllerInput, now int64) {
	switch input.State {
	case Normal:
		if c.rateControlState == RcHold {
			c.timeLastBitrateChange = now
			c.rateControlState = RcIncrease
		}
	case Overusing:
		if c.rateControlState != RcDecrease {
			c.rateControlState = RcDecrease
		}
	case Underusing:
		c.rateControlState = RcHold
	}
}

func (c *AimdRateControl) additiveRateIncrease(now int64, lastTimestamp int64) Bitrate {
	timePeriodSecond := (now - lastTimestamp) / 1000
	return Bitrate(c.getNearMaxIncreaseRateBpsPerSecond() * timePeriodSecond)
}

const (
	msPerSecond = 1000
	maxPow      = 1.0
	maxIncrease = 1000
)

func (c *AimdRateControl) multiplicativeRateIncrease(now int64, lastTime int64, currentBitrate Bitrate) Bitrate {
	alpha := 1.08
	if lastTime > -1 {
		timeSinceLastUpdate := now - lastTime
		alpha = math.Pow(alpha, math.Min(float64(timeSinceLastUpdate)/msPerSecond, maxPow))
	}
	return Bitrate(math.Max(float64(currentBitrate)*(alpha-1), maxIncrease))
}

func (c *AimdRateControl) getNearMaxIncreaseRateBpsPerSecond() int64 {
	if c.currentBitrate == 0 {
		return 0
	}
	// 1s 30 frame
	duration := Second / 30
	frameSize := c.currentBitrate.For(duration)
	// 125 bytes
	// 1
	packetPerFrame := math.Ceil(float64(frameSize) / PacketSize)
	avgPacketSize := float64(frameSize) / packetPerFrame
	responseTime := c.rtt + Millisecond*100
	if c.inExperiment {
		responseTime *= 2
	}
	// 1666
	increaseRateBpsPerSecond := NewBitrate(int64(avgPacketSize), responseTime)
	if increaseRateBpsPerSecond > MinIncreaseRateBpsPerSecond {
		return int64(increaseRateBpsPerSecond)
	}
	return MinIncreaseRateBpsPerSecond
}

func (c *AimdRateControl) SetEstimate(bitrate Bitrate, now int64) {
	c.bitrateIsInitialized = true
	prevBitrate := c.currentBitrate
	// buggy
	c.currentBitrate = c.clampBitrate(bitrate)
	c.timeLastBitrateChange = now
	if c.currentBitrate < prevBitrate {
		c.timeLastBitrateDecrease = now
	}
}

func (c *AimdRateControl) GetExpectedBandwidthPeriod() Duration {
	bps := c.getNearMaxIncreaseRateBpsPerSecond()
	if c.lastDecrease == 0 {
		return DefaultPeriod
	}
	timeToRecoverSeconds := float64(c.lastDecrease) / float64(bps)
	r := Second * Duration(timeToRecoverSeconds)
	if r < MinPeriod {
		return MinPeriod
	}
	if r > MaxPeriod {
		return MaxPeriod
	}
	return r
}

func (c *AimdRateControl) LatestEstimate() Bitrate {
	return c.currentBitrate
}

func (c *AimdRateControl) clampBitrate(bitrate Bitrate) Bitrate {
	if bitrate > c.minConfiguredBitrate {
		return bitrate
	}
	return c.minConfiguredBitrate
}

func (c *AimdRateControl) SetStartBitrate(startBitrate Bitrate) {
	c.currentBitrate = startBitrate
	c.latestEstimatedThroughput = c.currentBitrate
	c.bitrateIsInitialized = true
}
