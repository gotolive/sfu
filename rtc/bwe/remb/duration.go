package remb

type (
	Duration       int64 // μs
	SimulatedClock struct {
		currentUs Duration
	}
)

func (c *SimulatedClock) Add(duration Duration) {
	c.currentUs += duration
}

func (c *SimulatedClock) NowMs() int64 {
	return int64(c.currentUs / Millisecond)
}

const (
	Microsecond Duration = 1 // 1μs
	Millisecond          = 1000 * Microsecond
	Second               = 1000 * Millisecond
	Minute               = 60 * Second
)
