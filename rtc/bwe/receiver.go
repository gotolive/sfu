package bwe

import (
	"github.com/gotolive/sfu/rtc"
	"github.com/gotolive/sfu/rtc/bwe/remb"
)

const (
	TransportCC = "tcc"
	Remb        = "remb"
)

type Receiver interface {
	IncomingPacket(ms int64, packet rtc.Packet)
	SetMaxIncomingBitrate(bitrate uint64)
	SetMinIncomingBitrate(bitrate uint64)
	Close()
}

func NewReceiver(bweType string, id uint8, sendRtcp rtc.SendRTCP) Receiver {
	switch bweType {
	case TransportCC:
		panic("not implemented")
	case Remb:
		return remb.NewReceiver(id, sendRtcp)
	}
	return nil
}
