package peer

import (
	"github.com/gotolive/sfu/rtc"
)

type ReceiverOption struct {
	ID                   string
	MID                  string
	MediaType            string // video/audio
	Codec                *Codec
	HeaderExtensions     []rtc.HeaderExtension // required
	Streams              []StreamOption
	KeyFrameRequestDelay int
}

func (o ReceiverOption) Validate() error {
	if len(o.Streams) == 0 {
		return ErrStreamCantBeEmpty
	}
	if o.Codec == nil {
		return ErrCodecCantBeNil
	}
	for _, s := range o.Streams {
		if s.PayloadType != o.Codec.PayloadType {
			return ErrCodecNotMatch
		}
	}
	if o.MediaType != rtc.MediaTypeVideo && o.MediaType != rtc.MediaTypeAudio {
		return rtc.ErrUnknownType
	}
	return nil
}

type SenderOption struct {
	ID               string
	MID              string
	ConnectionID     string
	ReceiverID       string
	Codec            *Codec                // Optional
	HeaderExtensions []rtc.HeaderExtension // Optional
	// only worked in simulcast
	SwitchMode SwitchLayerMode
}
