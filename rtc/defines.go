package rtc

import (
	"errors"
	"math"
)

const SeqNumberMaxValue = math.MaxUint16

// Max RTCP Report Interval
const (
	MaxRTCPAudioInterval = 5000
	MaxRTCPVideoInterval = 1000
)

const (
	MediaTypeAudio = "audio"
	MediaTypeVideo = "video"
)

var ErrUnknownType = errors.New("unknown kind, only support MediaTypeAudio and MediaTypeVideo")

type SSRC uint32

type Seq uint16
