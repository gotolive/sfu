package peer

import (
	"github.com/gotolive/sfu/rtc"
)

// StreamOption basically one-ssrc map to one stream.
// but the rtx is different, they consider to be same stream
// if there is only one stream, ssrc and rid could both be empty, but mid can't
// otherwise it must exist at least 1
type StreamOption struct {
	SSRC            uint32
	Cname           string
	RID             string
	RTX             uint32
	Dtx             bool
	PayloadType     rtc.PayloadType
	ScalabilityMode string
	MaxBitrate      int
	MaxFramerate    float64
}

type Codec struct {
	PayloadType    rtc.PayloadType
	EncoderName    string
	ClockRate      int
	Channels       int
	Parameters     map[string]string
	FeedbackParams []RtcpFeedback
	RTX            rtc.PayloadType
}

// Equal if two codec has name encoder name and encoder parameters, we consider they are equal.
func (c Codec) Equal(c2 *Codec) bool {
	if c.EncoderName != c2.EncoderName {
		return false
	}
	if len(c.Parameters) != len(c2.Parameters) {
		return false
	}
	for k, v := range c.Parameters {
		if c2.Parameters[k] != v {
			return false
		}
	}
	return true
}

type RtcpFeedback struct {
	Type      string
	Parameter string
}

const (
	RTPTypeSVC       = "svc"
	RTPTypeSimple    = "simple"
	RTPTypeSimulcast = "simulcast"
	RTPTypeNone      = "none"
)
