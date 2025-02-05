package peer

import (
	"errors"
	"math/rand"
	"sync"
	"sync/atomic"
	"unsafe"

	"github.com/gotolive/sfu/rtc"
)

var (
	ErrNoMoreData = errors.New("no more data")
	ErrTooOld     = errors.New("seq too old")
)

// NewBuffer create a single buffer with given cap.
// How Much Cap should it be? cap = 4*pkts/s
func NewBuffer(cap int) *Buffer {
	b := &Buffer{
		cap:     cap,
		c:       sync.NewCond(&sync.Mutex{}),
		packets: make([]rtc.Packet, 0, 0),
		idx:     rand.Uint32(),
	}
	b.removed = b.idx
	return b
}

// Buffer is lock-free but not gc friendly
type Buffer struct {
	cap          int
	packets      []rtc.Packet // packet buffer
	lastKeyFrame uint32
	idx          uint32
	removed      uint32
	c            *sync.Cond
	snap         unsafe.Pointer
}

type BufferSnap struct {
	idx          uint32
	removed      uint32
	lastKeyFrame uint32
	packets      []rtc.Packet
}

func (s *BufferSnap) Get(seq uint32) ([]rtc.Packet, uint32, error) {
	if seq >= s.idx {
		return nil, s.idx, ErrNoMoreData
	}

	if seq < s.removed {
		return s.packets, s.idx, ErrTooOld
	}

	return s.packets[int(seq-s.removed):], s.idx, nil
}

// SetIdx should be called only once.
func (b *Buffer) SetIdx(idx uint32) {
	b.idx = idx
	b.removed = idx
}

func (b *Buffer) Put(packet rtc.Packet) {
	b.idx++
	if len(b.packets) >= b.cap {
		b.clear(b.cap / 3) // every time try to clear at least 1/3 cap
	}
	b.packets = append(b.packets, packet)

	if packet.IsKeyFrame() {
		b.lastKeyFrame = b.idx
	}
	snap := BufferSnap{idx: b.idx, lastKeyFrame: b.lastKeyFrame, removed: b.removed, packets: b.packets}
	atomic.StorePointer(&b.snap, unsafe.Pointer(&snap))
	b.c.Broadcast()
	return
}

func (b *Buffer) Snap() *BufferSnap {
	return (*BufferSnap)(atomic.LoadPointer(&b.snap))
}

func (b *Buffer) Latest() uint32 {
	return b.idx
}

func (b *Buffer) clear(i int) {
	b.removed = b.removed + uint32(i)
	b.packets = b.packets[i:]
}

func (b *Buffer) Wait() {
	b.c.L.Lock()
	b.c.Wait()
	b.c.L.Unlock()
}
