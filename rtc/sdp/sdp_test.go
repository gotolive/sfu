package sdp

import (
	"os"
	"testing"
)

func TestUnmarshal(t *testing.T) {
	// read all file under testdata
	// unmarshal and marshal
	files, err := os.ReadDir("../../testdata/sdp")
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		if f.IsDir() {
			continue
		}
		if f.Name()[0] == '.' {
			continue
		}
		b, err := os.ReadFile("../../testdata/sdp/" + f.Name())
		if err != nil {
			t.Fatal(err)
		}
		s, err := Unmarshal(string(b))
		if err != nil {
			t.Fatal(f.Name(), err)
		}
		_, err = Marshal(s)
		if err != nil {
			t.Fatal(err)
		}
		//if string(b) != string(b2) {
		//	t.Fatal("not equal")
		//}
	}
}

func TestUnmarshalNoPanic(t *testing.T) {
	sdps := []string{
		"",
		"a",
		"1",
		"a00",
		"b0",
		"o=-\nm=",
	}
	for _, sdp := range sdps {
		sd, err := Unmarshal(sdp)
		if err == nil || sd != nil {
			t.Fatal("should be error")
		}
	}
}

// Validate it's no panic
func FuzzUnmarshal(f *testing.F) {
	f.Fuzz(func(t *testing.T, sdp string) {
		_, _ = Unmarshal(sdp)
	})
}

func TestSDPBasic(t *testing.T) {
	b, err := os.ReadFile("../../testdata/sdp/sdp-1")

	s, err := Unmarshal(string(b))
	if err != nil {
		t.Fatal("err:", err)
	}
	if len(s.MediaDescription) != 2 {
		t.Fatal("unmarshal media section fail:", len(s.MediaDescription))
	}

	if len(s.MediaDescription[0].HeaderExtensions) != 2 || len(s.MediaDescription[1].HeaderExtensions) != 9 {
		t.Fatal("should be 9")
	}

	if len(s.MediaDescription[0].Codecs) == 0 || len(s.MediaDescription[1].Codecs) == 0 {
		t.Fatal("should ok")
	}

	if s.MediaDescription[0].RtcpReducedSize || !s.MediaDescription[1].RtcpReducedSize {
		t.Fatal("should ok", s.MediaDescription[0].RtcpReducedSize, s.MediaDescription[1].RtcpReducedSize)
	}

	for _, media := range s.MediaDescription {
		if media.MediaType == "" || !media.RtcpMux || media.MID == "" {
			t.Fatal("should ok")
		}

		for pt, c := range media.Codecs {
			if pt != c.PayloadType || c.EncoderName == "" || c.ClockRate == 0 || c.Channel == 0 {
				t.Fatal("should ok")
			}

			if pt == 100 && len(c.Parameters) == 0 {
				t.Fatal("should ok")
			}
			if pt == 98 && c.RTX != 99 {
				t.Fatal("should ok")
			}

		}
	}
}

func TestSDPSSRCFid(t *testing.T) {
	b, err := os.ReadFile("../../testdata/sdp/sdp-2")

	s, err := Unmarshal(string(b))
	if err != nil {
		t.Fatal("err:", err)
	}

	if len(s.MediaDescription) != 2 {
		t.Fatal("not expected")
	}

	media := s.MediaDescription[1]
	if len(media.Streams) != 1 {
		t.Fatal("should be 1")
	}
	for _, stream := range media.Streams {
		if stream.RTX == 0 {
			t.Fatal("should not be zero")
		}
	}
}

func TestSDPRID(t *testing.T) {
	b, err := os.ReadFile("../../testdata/sdp/sdp-rid")

	s, err := Unmarshal(string(b))
	if err != nil {
		t.Fatal("err:", err)
	}

	if len(s.MediaDescription) != 3 {
		t.Fatal("not expected")
	}

	media := s.MediaDescription[2]

	if len(media.Streams) != 3 {
		t.Fatal("should be 3")
	}
}

func TestSDPSIM(t *testing.T) {
	b, err := os.ReadFile("../../testdata/sdp/sdp-sim")

	s, err := Unmarshal(string(b))
	if err != nil {
		t.Fatal("err:", err)
	}

	if len(s.MediaDescription) != 3 {
		t.Fatal("not expected")
	}

	media := s.MediaDescription[2]

	if len(media.Streams) != 3 {
		t.Fatal("should be 3", len(media.Streams))
	}
}
