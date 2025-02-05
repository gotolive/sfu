package remb

import (
	"testing"
	"time"
)

func TestDataRate(t *testing.T) {
	b := Bitrate(90000)
	if b.For2(time.Second/30) != b.For(Second/30) {
		t.Fatal("fail, got")
	}
}
