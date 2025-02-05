package remb

import (
	"math"
	"sync"
)

// NewRateStatistics return a new RateStatistics.
// windowSizeMs: the calculate window.
// scale: the input size with bit and ms.
func NewRateStatistics(windowSizeMs int64, scale float64) *RateStatistics {
	r := &RateStatistics{
		maxWindowSizeMs: windowSizeMs,
		scale:           scale,
		buckets:         make([]*bucket, 0),
	}
	r.Reset()
	return r
}

type RateStatistics struct {
	m                   sync.Mutex
	accumulatedCount    int64     // byte count
	overflow            bool      // the bitrate has over max int64
	numSamples          int       // packet count
	firstTimestamp      int64     // current first packet timestamp
	currentWindowSizeMs int64     // current window size, default=maxWindowSizeMs
	buckets             []*bucket // group packet by timestamp
	scale               float64   // 8000 transfer byte to bitrate
	maxWindowSizeMs     int64     // default 1 sec
}

// The Rate will return bit per second based on arrived time.
// The logic is quite simple, allPayloadSize / windowTime is the bitrate
func (r *RateStatistics) Rate(nowMs int64) *Bitrate {
	r.m.Lock()
	defer r.m.Unlock()
	// we will only count 1 window size.
	r.eraseOld(nowMs)
	var activeWindowSize int64
	if r.firstTimestamp != -1 {
		if r.firstTimestamp <= nowMs-r.currentWindowSizeMs {
			activeWindowSize = r.currentWindowSizeMs
		} else {
			activeWindowSize = nowMs - r.firstTimestamp + 1
		}
	}
	if (r.numSamples == 0 || activeWindowSize <= 1 || (r.numSamples <= 1 && activeWindowSize < r.currentWindowSizeMs)) || r.overflow {
		return nil
	}
	scale := r.scale / float64(activeWindowSize)
	result := float64(r.accumulatedCount)*scale + 0.5

	if result > math.MaxInt64 {
		return nil
	}
	res := Bitrate(result)
	return &res
}

func (r *RateStatistics) Reset() {
	r.m.Lock()
	defer r.m.Unlock()
	r.accumulatedCount = 0
	r.overflow = false
	r.numSamples = 0
	r.firstTimestamp = -1
	r.currentWindowSizeMs = r.maxWindowSizeMs
	r.buckets = r.buckets[0:0]
}

func (r *RateStatistics) Update(payloadSize int64, nowMs int64) {
	r.m.Lock()
	defer r.m.Unlock()
	r.eraseOld(nowMs)
	if r.firstTimestamp == -1 || r.numSamples == 0 {
		r.firstTimestamp = nowMs
	}
	if len(r.buckets) == 0 || nowMs != r.buckets[len(r.buckets)-1].timestamp {
		if len(r.buckets) != 0 && nowMs < r.buckets[len(r.buckets)-1].timestamp {
			nowMs = r.buckets[len(r.buckets)-1].timestamp
		}
		r.buckets = append(r.buckets, newBucket(nowMs))
	}
	bucket := r.buckets[len(r.buckets)-1]
	bucket.sum += payloadSize
	bucket.numSamples++
	if math.MaxInt64-r.accumulatedCount > payloadSize {
		r.accumulatedCount += payloadSize
	} else {
		r.overflow = true
	}
	r.numSamples++
}

func (r *RateStatistics) eraseOld(nowMs int64) {
	newOldestTime := nowMs - r.currentWindowSizeMs + 1
	var i int
	for ; i < len(r.buckets); i++ {
		bucket := r.buckets[i]
		if bucket.timestamp < newOldestTime {
			r.accumulatedCount -= bucket.sum
			r.numSamples -= bucket.numSamples
		} else {
			break
		}
	}
	r.buckets = r.buckets[i:]
}

type bucket struct {
	timestamp  int64
	sum        int64
	numSamples int
}

func newBucket(timestamp int64) *bucket {
	return &bucket{
		timestamp: timestamp,
	}
}
