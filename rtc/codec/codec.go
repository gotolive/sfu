package codec

import (
	"github.com/gotolive/sfu/rtc"
)

func Register(codec *Codec) {
	// we do not check the duplicated codec
	allCodecs[codec.EncoderName] = codec
}

var allCodecs = map[string]*Codec{}

type Codec struct {
	MediaType        string
	EncoderName      string
	SupportKeyFrame  bool
	SupportSimulcast bool
	SupportSVC       bool
	Process          func([]byte) rtc.PayloadDescriptor
}

// ProcessRTPPacket try to update packet according the encoder name
func ProcessRTPPacket(packet rtc.Packet, encoderName string) {
	if codec, ok := allCodecs[encoderName]; ok {
		if codec.Process != nil {
			if pd := codec.Process(packet.Payload()); pd != nil {
				packet.SetPayloadDescriptor(pd)
			}
		}
	}
}

func CanBeKeyFrame(encoderName string) bool {
	if codec, ok := allCodecs[encoderName]; ok {
		return codec.SupportKeyFrame
	}
	return false
}
