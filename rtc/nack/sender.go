package nack

import (
	"github.com/gotolive/sfu/rtc"
	"github.com/pion/rtcp"
)

type Sender interface {
	OnNack(*rtcp.TransportLayerNack)
	ReceivePacket(packet rtc.Packet)
}

func NewSender(buf *Buffer, sendRTP func(packet rtc.Packet)) Sender {
	return &sender{
		buffer:  buf,
		sendRTP: sendRTP,
	}
}

type sender struct {
	buffer  *Buffer
	sendRTP func(packet rtc.Packet)
}

func (r *sender) ReceivePacket(packet rtc.Packet) {
	r.buffer.Put(packet)
}

func (r *sender) OnNack(report *rtcp.TransportLayerNack) {
	for _, item := range report.Nacks {
		item.Range(func(seqno uint16) bool {
			if packet := r.buffer.Get(seqno); packet != nil {
				r.sendRTP(packet)
			}
			return true
		})
	}
}
