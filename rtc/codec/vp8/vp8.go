package vp8

import (
	"github.com/gotolive/sfu/rtc"
	"github.com/gotolive/sfu/rtc/codec"
)

const CodecName = "VP8"

func init() {
	codec.Register(Codec())
}

func Codec() *codec.Codec {
	return &codec.Codec{
		MediaType:        rtc.MediaTypeVideo,
		EncoderName:      CodecName,
		SupportKeyFrame:  true,
		SupportSimulcast: true,
		SupportSVC:       false,
		Process:          parsePayload,
	}
}

type vp8PayloadDescriptor struct {
	isFirstPacketInFrame bool

	isKeyFrame bool
}

func (a *vp8PayloadDescriptor) IsKeyFrame() bool {
	return a.isKeyFrame
}

func parsePayload(data []byte) rtc.PayloadDescriptor {
	if len(data) < 2 {
		return nil
	}
	p := new(vp8PayloadDescriptor)
	index := 0
	b := data[index]
	extension := (b & 0x80) > 0
	// nonReference := (b & 0x20) > 0
	beginningOfPartition := (b & 0x10) > 0
	partitionID := b & 0x07
	index++
	if extension {
		hasPictureID := (data[index] & 0x80) > 0
		hasTl0PicIdx := (data[index] & 0x40) > 0
		hasTID := (data[index] & 0x20) > 0
		hasKeyIdx := (data[index] & 0x10) > 0

		index++
		if hasPictureID {
			// pictureId := uint16(data[index] & 0x7f)
			if data[index]&0x80 > 0 {
				index++
				// pictureId = (pictureId << 8) | data[index]
			}
			index++
		}
		if hasTl0PicIdx {
			index++
		}

		if hasTID || hasKeyIdx {
			index++
		}
	}

	p.isFirstPacketInFrame = beginningOfPartition && partitionID == 0

	// check the vp8 payload is key frame or not
	if p.isFirstPacketInFrame && data[index]&0x01 == 0 {
		p.isKeyFrame = true
	}

	return p
}
