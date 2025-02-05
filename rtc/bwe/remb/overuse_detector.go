package remb

import (
	"math"
)

const (
	Normal     = 0
	Overusing  = 1
	Underusing = 2
)

const (
	MaxNumDeltas     = 60
	MaxAdaptOffsetMs = 15.0
)

func NewOveruseDetector() *OveruseDetector {
	return &OveruseDetector{
		hypothesis:             0,
		threshold:              12.5,
		lastUpdateMs:           -1,
		timeOverUsing:          -1,
		overuseCounter:         0,
		overusingTimeThreshold: 10,
		prevOffset:             0,
		inExperiment:           true,
		kDown:                  0.039,
		kUp:                    0.0087,
	}
}

type OveruseDetector struct {
	hypothesis             int
	threshold              float64
	timeOverUsing          int
	overuseCounter         int
	overusingTimeThreshold int
	prevOffset             float64
	inExperiment           bool
	lastUpdateMs           int64
	kDown                  float64
	kUp                    float64
}

func (d *OveruseDetector) State() int {
	return d.hypothesis
}

func (d *OveruseDetector) Detect(offset float64, tsDelta uint32, numOfDeltas int, nowMs int64) int {
	if numOfDeltas < 2 {
		return Normal
	}
	var T float64
	if numOfDeltas < MaxNumDeltas {
		T = float64(numOfDeltas) * offset
	} else {
		T = MaxNumDeltas * offset
	}
	if T > d.threshold {
		if d.timeOverUsing == -1 {
			d.timeOverUsing = int(tsDelta) / 2
		} else {
			d.timeOverUsing += int(tsDelta)
		}
		d.overuseCounter++
		if d.timeOverUsing > d.overusingTimeThreshold && d.overuseCounter > 1 {
			if offset >= d.prevOffset {
				d.timeOverUsing = 0
				d.overuseCounter = 0
				d.hypothesis = Overusing
			}
		}
	} else if T < (-d.threshold) {
		d.timeOverUsing = 0
		d.overuseCounter = 0
		d.hypothesis = Underusing
	} else {
		d.timeOverUsing = 0
		d.overuseCounter = 0
		d.hypothesis = Normal
	}
	d.prevOffset = offset
	d.updateThreshold(T, nowMs)
	return d.hypothesis
}

func (d *OveruseDetector) updateThreshold(modifiedOffset float64, ms int64) {
	if !d.inExperiment {
		return
	}
	if d.lastUpdateMs == -1 {
		d.lastUpdateMs = ms
	}
	if math.Abs(modifiedOffset) > d.threshold+MaxAdaptOffsetMs {
		d.lastUpdateMs = ms
		return
	}
	var k float64
	if math.Abs(modifiedOffset) < d.threshold {
		k = d.kDown
	} else {
		k = d.kUp
	}
	timeDeltaMs := ms - d.lastUpdateMs
	if timeDeltaMs > int64(100) {
		timeDeltaMs = int64(100)
	}
	d.threshold += k * (math.Abs(modifiedOffset) - d.threshold) * float64(timeDeltaMs)
	if d.threshold < 6 {
		d.threshold = 6
	}
	if d.threshold > 600 {
		d.threshold = 600
	}
	d.lastUpdateMs = ms
}
