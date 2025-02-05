package remb

import "math"

func NewLinkCapacityEstimator() *LinkCapacityEstimator {
	return &LinkCapacityEstimator{}
}

type LinkCapacityEstimator struct {
	estimateKbps   Bitrate
	deviationKbps  uint64
	capacitySample Bitrate
}

func (l *LinkCapacityEstimator) UpperBound() Bitrate {
	if l.estimateKbps > 0 {
		return l.estimateKbps + 3*l.deviationEstimateKbps()
	}
	return 0
}

func (l *LinkCapacityEstimator) Reset() {
	l.estimateKbps = 0
}

func (l *LinkCapacityEstimator) HasEstimate() bool {
	return l.estimateKbps > 0
}

func (l *LinkCapacityEstimator) Estimate() Bitrate {
	return l.estimateKbps
}

func (l *LinkCapacityEstimator) LowerBound() Bitrate {
	if l.estimateKbps > 0 && l.estimateKbps-3*l.deviationEstimateKbps() > 0 {
		return l.estimateKbps - 3*l.deviationEstimateKbps()
	}
	return 0
}

func (l *LinkCapacityEstimator) OnOveruseDetected(throughput Bitrate) {
	l.update(throughput, 0.5)
}

func (l *LinkCapacityEstimator) deviationEstimateKbps() Bitrate {
	return Bitrate(math.Sqrt(float64(Bitrate(l.deviationKbps) * l.estimateKbps)))
}

func (l *LinkCapacityEstimator) update(_ Bitrate, alpha float64) {
	sampleKbps := l.capacitySample
	if l.estimateKbps <= 0 {
		l.estimateKbps = sampleKbps
	} else {
		l.estimateKbps = Bitrate((1-alpha)*float64(l.estimateKbps) + alpha*float64(sampleKbps))
	}
}
