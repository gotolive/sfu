package remb

import (
	"github.com/pion/rtcp"
)

func NewSender(initialBitrate uint64) *sender {
	return &sender{
		initialBitrate: Bitrate(initialBitrate),
	}
}

type sender struct {
	lastBitrate    Bitrate
	initialBitrate Bitrate
}

func (s *sender) ReceiveRTCP(report rtcp.Packet) {
	if r, ok := report.(*rtcp.ReceiverEstimatedMaximumBitrate); ok {
		s.lastBitrate = Bitrate(r.Bitrate)
	}
}

func (s *sender) BweType() string {
	return "remb"
}

func (s *sender) EstimateBitrate() uint64 {
	if s.lastBitrate == 0 {
		return uint64(s.initialBitrate)
	}
	return uint64(s.lastBitrate)
}
