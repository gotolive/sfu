package peer

import (
	"sync/atomic"
	"testing"
	"time"
)

func TestKeyframeManager(t *testing.T) {
	tests := []testHelper{
		{
			name:        "create a new keyframe manager",
			description: "",
			method: func(t *testing.T) {
				var called int32
				k := newKeyframeManager(0, func(u uint32) {
					atomic.AddInt32(&called, 1)
				})
				defer k.close()
				k.keyFrameNeeded(1234)
				k.keyFrameNeeded(1234)
				k.keyFrameReceived(1234)
				if atomic.LoadInt32(&called) == 0 {
					t.Error("should called")
				}
			},
		},
		{
			name:        "create a new keyframe manager with delay",
			description: "",
			method: func(t *testing.T) {
				var called int32
				k := newKeyframeManager(1000, func(u uint32) {
					atomic.AddInt32(&called, 1)
				})
				defer k.close()
				k.keyFrameNeeded(1234)
				k.keyFrameNeeded(1234)
				k.keyFrameReceived(1234)
				if atomic.LoadInt32(&called) != 0 {
					t.Error("should not called")
				}
			},
		},
		{
			name:        "create a new keyframe manager with delay",
			description: "",
			method: func(t *testing.T) {
				var called int32
				k := newKeyframeManager(1, func(u uint32) {
					atomic.AddInt32(&called, 1)
				})
				defer k.close()
				k.keyFrameNeeded(1234)
				// make sure it called
				time.Sleep(time.Millisecond * 10)
				k.keyFrameNeeded(1234)
				k.keyFrameReceived(1234)
				if atomic.LoadInt32(&called) == 0 {
					t.Error("should called")
				}
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.method(t)
		})
	}
}
