package peer

import (
	"time"

	"github.com/gotolive/sfu/rtc"
	"github.com/gotolive/sfu/rtc/bwe/remb"
)

func newStats() *Stats {
	return &Stats{
		sendBps:    remb.NewRateStatistics(1000, 8000),
		receiveBps: remb.NewRateStatistics(1000, 8000),
		streams:    map[uint32]*StreamStats{},
	}
}

// Stats connection.GetStats()
// it design to be access from everywhere.
type Stats struct {
	// This four stats should be transport level.
	packetsSent     int64
	bytesSent       int64
	packetsReceived int64
	bytesReceived   int64
	sendBps         *remb.RateStatistics
	receiveBps      *remb.RateStatistics
	streams         map[uint32]*StreamStats
}

func (s *Stats) IncomingRTP(packet rtc.Packet) {
	s.bytesReceived += int64(packet.Size())
	s.packetsReceived++
	s.receiveBps.Update(int64(packet.Size()), packet.ReceiveMS())
	stream := s.stream(packet.SSRC())
	stream.incomingRTP(packet)
}

func (s *Stats) OutcomePacket(packet rtc.Packet) {
	s.bytesSent += int64(packet.Size())
	s.packetsSent++
	s.sendBps.Update(int64(packet.Size()), time.Now().UnixMilli())
}

func (s *Stats) PacketsReceived() int64 {
	return s.packetsReceived
}

func (s *Stats) BytesSend() int64 {
	return s.bytesSent
}

func (s *Stats) BytesReceived() int64 {
	return s.bytesReceived
}

func (s *Stats) ReceiveBPS(nowMs int64) int64 {
	bps := s.receiveBps.Rate(nowMs)
	if bps == nil {
		return 0
	}
	return int64(*bps)
}

func (s *Stats) SentBPS(nowMs int64) int64 {
	bps := s.sendBps.Rate(nowMs)
	if bps == nil {
		return 0
	}
	return int64(*bps)
}

func (s *Stats) stream(ssrc uint32) *StreamStats {
	if s.streams[ssrc] == nil {
		s.streams[ssrc] = &StreamStats{
			sendBps:    remb.NewRateStatistics(1000, 8000),
			receiveBps: remb.NewRateStatistics(1000, 8000),
		}
	}
	return s.streams[ssrc]
}

func newStreamStats(ssrc uint32) *StreamStats {
	return &StreamStats{
		ssrc:       ssrc,
		sendBps:    remb.NewRateStatistics(1000, 8000),
		receiveBps: remb.NewRateStatistics(1000, 8000),
	}
}

type StreamStats struct {
	packetsSent     int64
	bytesSent       int64
	packetsReceived int64
	bytesReceived   int64
	sendBps         *remb.RateStatistics
	receiveBps      *remb.RateStatistics

	packetRepaired      int64 // successful nack, we found it in nack list
	packetRetransmitted int64 // retransmit rtpPacket, could be large then repaired, if use rtx, rtx count, if not, this equal packetRepaired
	firCount            int64 // in receiver how much we sent, in sender, how much we received
	pliCount            int64 // in receiver how much we sent, in sender, how much we received
	videoFrameCount     int64 // only used for video
	rtt                 int64
	packetLost          int64
	fractionLost        int64
	nackCount           int64
	nackPacket          int64
	ssrc                uint32
}

func (s *StreamStats) incomingRTP(packet rtc.Packet) {
	s.bytesReceived += int64(packet.Size())
	s.packetsReceived++
	s.receiveBps.Update(int64(packet.Size()), packet.ReceiveMS())
}

func (s *StreamStats) outcomingRTP(packet rtc.Packet) {
	s.bytesSent += int64(packet.Size())
	s.packetsSent++
	s.sendBps.Update(int64(packet.Size()), packet.ReceiveMS())
}

func (s *StreamStats) PacketsReceived() int64 {
	return s.packetsReceived
}

func (s *StreamStats) BytesReceived() int64 {
	return s.bytesReceived
}

func (s *StreamStats) ReceiveBPS(nowMs int64) int64 {
	bps := s.receiveBps.Rate(nowMs)
	if bps == nil {
		return 0
	}
	return int64(*bps)
}

func (s *StreamStats) PacketsSent() int64 {
	return s.packetsSent
}

func (s *StreamStats) BytesSent() int64 {
	return s.bytesSent
}

func (s *StreamStats) SentBPS(nowMs int64) int64 {
	bps := s.sendBps.Rate(nowMs)
	if bps == nil {
		return 0
	}
	return int64(*bps)
}
