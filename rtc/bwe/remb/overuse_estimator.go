package remb

import (
	"math"
)

const (
	DeltaCounterMax             = 1000
	MinFramePeriodHistoryLength = 60
)

func NewOveruseEstimator() *OveruseEstimator {
	estimator := &OveruseEstimator{
		numOfDeltas: 0,
		offset:      0,
		e: [][]float64{
			{100, 0},
			{0, 1e-1},
		},
		processNoise: []float64{
			1e-13, 1e-3,
		},
		prevOffset:  0,
		slope:       8.0 / 512.0,
		varNoise:    50.0,
		tsDeltaHist: nil,
		avgNoise:    0,
	}
	estimator.e[0] = make([]float64, 2)
	estimator.e[1] = make([]float64, 2)
	return estimator
}

type OveruseEstimator struct {
	numOfDeltas  int
	offset       float64
	e            [][]float64
	processNoise []float64
	prevOffset   float64
	slope        float64
	varNoise     float64
	tsDeltaHist  []float64
	avgNoise     float64
}

func (e *OveruseEstimator) Update(tDelta int64, tsDelta uint32, sizeDelta int, currentHypothesis int, nowMs int64) {
	minFramePeriod := e.updateMinFramePeriod(tsDelta)
	tTSDelta := float64(tDelta) - float64(tsDelta)
	fsDelta := sizeDelta
	e.numOfDeltas++
	if e.numOfDeltas > DeltaCounterMax {
		e.numOfDeltas = DeltaCounterMax
	}
	e.e[0][0] += e.processNoise[0]
	e.e[1][1] += e.processNoise[1]

	if (currentHypothesis == Overusing && e.offset < e.prevOffset) || (currentHypothesis == Underusing && e.offset > e.prevOffset) {
		e.e[1][1] += 10 * e.processNoise[1]
	}
	h := []float64{float64(fsDelta), 1.0}
	eh := []float64{e.e[0][0]*h[0] + e.e[0][1]*h[1], e.e[1][0]*h[0] + e.e[1][1]*h[1]}

	residual := tTSDelta - e.slope*h[0] - e.offset
	inStabeState := currentHypothesis == Normal
	maxResidual := 3 * math.Sqrt(e.varNoise)
	if math.Abs(residual) < maxResidual {
		e.updateNoiseEstimate(residual, minFramePeriod, inStabeState)
	} else {
		if residual < 0 {
			e.updateNoiseEstimate(-maxResidual, minFramePeriod, inStabeState)
		} else {
			e.updateNoiseEstimate(maxResidual, minFramePeriod, inStabeState)
		}
	}
	denom := e.varNoise + h[0]*eh[0] + h[1]*eh[1]
	k := []float64{eh[0] / denom, eh[1] / denom}
	ikh := [][]float64{
		{1 - k[0]*h[0], -k[0] * h[1]},
		{-k[1] * h[0], 1 - k[1]*h[1]},
	}
	e00 := e.e[0][0]
	e01 := e.e[0][1]
	e.e[0][0] = e00*ikh[0][0] + e.e[1][0]*ikh[0][1]
	e.e[0][1] = e01*ikh[0][0] + e.e[1][1]*ikh[0][1]
	e.e[1][0] = e00*ikh[1][0] + e.e[1][0]*ikh[1][1]
	e.e[1][1] = e01*ikh[1][0] + e.e[1][1]*ikh[1][1]

	positiveSemiDefinite := e.e[0][0]+e.e[1][1] >= 0 && e.e[0][0]*e.e[1][1]-e.e[0][1]*e.e[1][0] >= 0 && e.e[0][0] >= 0
	if !positiveSemiDefinite {
		return
	}
	e.slope += k[0] * residual
	e.prevOffset = e.offset
	e.offset += k[1] * residual
}

func (e *OveruseEstimator) Offset() float64 {
	return e.offset
}

func (e *OveruseEstimator) NumOfDeltas() int {
	return e.numOfDeltas
}

func (e *OveruseEstimator) updateMinFramePeriod(tsDelta uint32) float64 {
	minFramePeriod := float64(tsDelta)
	if len(e.tsDeltaHist) >= MinFramePeriodHistoryLength {
		e.tsDeltaHist = e.tsDeltaHist[1:]
	}
	for _, v := range e.tsDeltaHist {
		if v < minFramePeriod {
			minFramePeriod = v
		}
	}
	e.tsDeltaHist = append(e.tsDeltaHist, float64(tsDelta))
	return minFramePeriod
}

func (e *OveruseEstimator) updateNoiseEstimate(residual float64, tsDelta float64, stableState bool) {
	if !stableState {
		return
	}
	alpha := 0.01
	if e.numOfDeltas > 10*30 {
		alpha = 0.002
	}
	beta := math.Pow(1-alpha, tsDelta*30/1000)
	e.avgNoise = beta*e.avgNoise + (1-beta)*residual
	e.varNoise = beta*e.varNoise + (1-beta)*(e.avgNoise-residual)*(e.avgNoise-residual)
	if e.varNoise < 1 {
		e.varNoise = 1
	}
}
