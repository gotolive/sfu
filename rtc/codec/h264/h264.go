package h264

import (
	"encoding/binary"

	"github.com/gotolive/sfu/rtc"
	"github.com/gotolive/sfu/rtc/codec"
)

func init() {
	codec.Register(Codec())
}

const CodecName = "H264"

func Codec() *codec.Codec {
	return &codec.Codec{
		MediaType:        rtc.MediaTypeVideo,
		EncoderName:      CodecName,
		SupportKeyFrame:  true,
		SupportSimulcast: true,
		SupportSVC:       true,
		Process:          parsePayload,
	}
}

type h264PayloadDescriptor struct {
	isKeyFrame bool
}

func (a *h264PayloadDescriptor) IsKeyFrame() bool {
	return a.isKeyFrame
}

func parsePayload(data []byte) rtc.PayloadDescriptor {
	if len(data) < 2 {
		return nil
	}
	p := new(h264PayloadDescriptor)
	nal := data[0] & 0x1f
	switch nal {
	// sps packet
	case 7:
		p.isKeyFrame = true
	case 24:
		// STAP-A
		offset := 1
		last := len(data) // 10
		for offset <= last-3 {
			subnal := data[offset+2] & 0x1f
			if subnal == 7 {
				p.isKeyFrame = true
				break
			}
			offset += 2 + int(binary.BigEndian.Uint16(data[offset:]))
		}
	case 28, 29:
		subnal := data[1] & 0x1f
		startBit := data[1] & 0x80
		if subnal == 7 && startBit == 128 {
			p.isKeyFrame = true
		}
	}

	return p
}
