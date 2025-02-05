package nack

import (
	"sync"

	"github.com/gotolive/sfu/rtc"
)

func NewBuffer(size int) *Buffer {
	ring := make([]int32, size)
	for i := range ring {
		ring[i] = -1
	}
	return &Buffer{
		ring: ring,
		buf:  map[int32]rtc.Packet{},
	}
}

// Buffer keeps a fix size rtp packet for consumer to nack.
type Buffer struct {
	m    sync.RWMutex
	buf  map[int32]rtc.Packet
	ring []int32
	pos  int
}

func (b *Buffer) Put(pkt rtc.Packet) {
	seq := int32(pkt.SequenceNumber())
	b.m.Lock()
	if b.ring[b.pos] >= 0 {
		delete(b.buf, b.ring[b.pos])
	}
	b.buf[seq] = pkt
	b.m.Unlock()
	b.ring[b.pos] = seq
	b.pos++
	if b.pos == len(b.ring) {
		b.pos = 0
	}
}

func (b *Buffer) Get(seq uint16) rtc.Packet {
	b.m.RLock()
	defer b.m.RUnlock()
	return b.buf[int32(seq)]
}
