package vp9

import (
	"github.com/gotolive/sfu/rtc"
	"github.com/gotolive/sfu/rtc/codec"
)

const CodecName = "VP9"

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

/**
https://datatracker.ietf.org/doc/html/draft-ietf-payload-vp9#section-4.2

flexible mode

        0 1 2 3 4 5 6 7
       +-+-+-+-+-+-+-+-+
       |i|P|L|F|B|E|V|Z| (REQUIRED)
       +-+-+-+-+-+-+-+-+
  i:   |M| PICTURE ID  | (REQUIRED)
       +-+-+-+-+-+-+-+-+
  M:   | EXTENDED PID  | (RECOMMENDED)
       +-+-+-+-+-+-+-+-+
  L:   | TID |U| SID |D| (Conditionally RECOMMENDED)
       +-+-+-+-+-+-+-+-+                             -\
  P,F: | P_DIFF      |N| (Conditionally REQUIRED)    - up to 3 times
       +-+-+-+-+-+-+-+-+                             -/
  V:   | SS            |
       | ..            |
       +-+-+-+-+-+-+-+-+

non-flexible mode

         0 1 2 3 4 5 6 7
        +-+-+-+-+-+-+-+-+
        |i|P|L|F|B|E|V|Z| (REQUIRED)
        +-+-+-+-+-+-+-+-+
   i:   |M| PICTURE ID  | (RECOMMENDED)
        +-+-+-+-+-+-+-+-+
   M:   | EXTENDED PID  | (RECOMMENDED)
        +-+-+-+-+-+-+-+-+
   L:   | TID |U| SID |D| (Conditionally RECOMMENDED)
        +-+-+-+-+-+-+-+-+
        |   TL0PICIDX   | (Conditionally REQUIRED)
        +-+-+-+-+-+-+-+-+
   V:   | SS            |
        | ..            |
        +-+-+-+-+-+-+-+-+

*/

type vp9PayloadDescriptor struct {
	i bool // picture ID
	p bool // key frame
	l bool // layer indices
	f bool // flexible mode
	b bool // start of frame
	e bool // end of frame
	v bool // Scalability structure
	z bool

	isKeyFrame bool
}

func (v *vp9PayloadDescriptor) IsKeyFrame() bool {
	return v.isKeyFrame
}

func parsePayload(data []byte) rtc.PayloadDescriptor {
	pd := new(vp9PayloadDescriptor)
	firstByte := data[0]
	pd.i = (firstByte>>7)&0x01 > 0
	pd.p = (firstByte>>6)&0x01 > 0
	pd.l = (firstByte>>5)&0x01 > 0
	pd.f = (firstByte>>4)&0x01 > 0
	pd.b = (firstByte>>3)&0x01 > 0
	pd.e = (firstByte>>2)&0x01 > 0
	pd.v = (firstByte>>1)&0x01 > 0
	pd.z = (firstByte)&0x01 > 0
	index := 0
	if (firstByte>>7)&0x01 > 0 {
		index++
		if (data[index]>>7)&0x01 > 0 {
			// pd.HasTwoBytesPictureId=true
			index++
		}
	}
	var slIndex byte
	if pd.l {
		index++
		slIndex = (data[index] >> 1) & 0x07
	}
	if !pd.p && pd.b && slIndex == 0 {
		pd.isKeyFrame = true
	}
	return pd
}
