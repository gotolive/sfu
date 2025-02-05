package peer

import (
	"github.com/gotolive/sfu/rtc"
)

type RTPSeqManager interface {
	Sync(seq uint16)
	Input(seq uint16) uint16
}

func NewSeqManager() RTPSeqManager {
	return &defaultRTPManager{}
}

type defaultRTPManager struct {
	base      uint16
	maxOutput uint16
	maxInput  uint16
}

func (d *defaultRTPManager) Sync(seq uint16) {
	d.base = d.maxOutput - seq
	d.maxInput = seq
}

func (d *defaultRTPManager) Input(seq uint16) uint16 {
	base := d.base
	output := seq + base
	if seq-d.maxInput < rtc.SeqNumberMaxValue/2 {
		d.maxInput = seq
	}
	if output-d.maxOutput < rtc.SeqNumberMaxValue/2 {
		d.maxOutput = output
	}
	return output
}
