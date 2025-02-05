package bwe

import (
	"github.com/gotolive/sfu/rtc/bwe/remb"
	"github.com/pion/rtcp"
)

type SenderListener interface{}

type Sender interface {
	BweType() string
	ReceiveRTCP(report rtcp.Packet)
	EstimateBitrate() uint64
}

func NewSender(bweType string, initialBitrate uint64) Sender {
	switch bweType {
	case TransportCC:
		panic("not implemented")
	case Remb:
		return remb.NewSender(initialBitrate)
	}
	return nil
}
