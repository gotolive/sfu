package av1

import (
	"github.com/gotolive/sfu/rtc"
	"github.com/gotolive/sfu/rtc/codec"
)

const CodecName = "AV1"

func init() {
	codec.Register(Codec())
}

func Codec() *codec.Codec {
	return &codec.Codec{
		MediaType:        rtc.MediaTypeVideo,
		EncoderName:      CodecName,
		SupportKeyFrame:  true,
		SupportSimulcast: false,
		SupportSVC:       true,
		Process:          parsePayload,
	}
}

type av1PayloadDescriptor struct {
	isKeyFrame bool
}

func (a *av1PayloadDescriptor) IsKeyFrame() bool {
	return a.isKeyFrame
}

func parsePayload(data []byte) rtc.PayloadDescriptor {
	if len(data) < 2 {
		return nil
	}
	p := new(av1PayloadDescriptor)
	if data[0]&8 == 8 {
		p.isKeyFrame = true
	}

	return p
}
