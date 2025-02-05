package remb

import (
	"fmt"
	"time"
)

// Bitrate bits per second
type Bitrate uint64

func (b Bitrate) String() string {
	var out float64
	switch {
	case b > 1000*1000*1000:
		out = float64(b) / (1000 * 1000 * 1000)
		return fmt.Sprintf("%.2f Gb/s", out)
	case b > 1000*1000:
		out = float64(b) / (1000 * 1000)
		return fmt.Sprintf("%.2f Mb/s", out)
	case b > 1000:
		out = float64(b) / 1000
		return fmt.Sprintf("%.2f Kb/s", out)
	}
	return fmt.Sprintf("%d bit/s", b)
}

const (
	BitPerSecond      Bitrate = 1
	BytePerSecond             = 8 * BitPerSecond
	KilobitsPerSecond         = 1000 * BitPerSecond
)

func (b Bitrate) For2(duration time.Duration) DataSize {
	// add 1/2s bitrate for float round to 1
	return DataSize((float64(b)*float64(duration) + float64(time.Second/2)) / float64(time.Second) / 8)
}

func (b Bitrate) For(duration Duration) DataSize {
	microBits := int64(b) * int64(duration)
	// the 400000 is for float +1, eg 374 375
	return DataSize((microBits + 4000000) / 8000000)
}

// Byte count
func NewBitrate(byteSize int64, duration Duration) Bitrate {
	return Bitrate(float64(byteSize*8) * float64(Second) / float64(duration))
}

type DataSize int64
