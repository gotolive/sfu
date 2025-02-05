package peer

import (
	"testing"
)

func TestNewSeqManager(t *testing.T) {
	seq := NewSeqManager()
	seq.Sync(10)
	var res uint16
	res = seq.Input(11)
	if res != 1 {
		t.Fatal("seq_manager:first number wrong")
	}
	res = seq.Input(12)
	if res != 2 {
		t.Fatal("seq_manager:seq number wrong")
	}
	res = seq.Input(14)
	if res != 4 {
		t.Fatal("seq_manager:misorder number wrong")
	}
}
