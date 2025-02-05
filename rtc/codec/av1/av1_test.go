package av1

import (
	"bufio"
	"encoding/hex"
	"io"
	"os"
	"testing"
)

func TestAV1PayloadDescriptor_IsKeyFrame(t *testing.T) {
	// we only kept two frames.
	keys := []bool{true, false}
	file, err := os.Open("../../../testdata/codec/av1/stream")
	if err != nil {
		t.Fatal("read file fail", err)
	}
	scanner := bufio.NewScanner(file)
	var n int
	codec := Codec()
	if codec.EncoderName != CodecName {
		t.Fatal("wrong codec")
	}
	for scanner.Scan() {
		line := scanner.Text()
		payload, err := hex.DecodeString(line)
		if err != nil {
			t.Fatal("decode line fail", err)
		}
		pd := codec.Process(payload)
		if pd.IsKeyFrame() != keys[n] {
			t.Fatal("key frame detect fail:", n)
		}
		n++
		if err = scanner.Err(); err != nil && err != io.EOF {
			t.Fatal("readline fail:", err)
		}
	}
}
