package rtc

import (
	"math/rand"
)

const (
	ssrcLowerBound uint32 = 800000000
	ssrcUpperBound uint32 = 900000000
)

func GenerateSSRC() uint32 {
	return ssrcLowerBound + uint32(rand.Int31n(int32(ssrcUpperBound-ssrcLowerBound)))
}
