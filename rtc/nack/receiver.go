package nack

import (
	"sync"
	"time"

	"github.com/gotolive/sfu/rtc"
	"github.com/pion/rtcp"
)

const (
	// packet seq max range
	maxPacketAge = 10000
	// the max packet list we hold
	maxNackPackets = 1000
	// every packet max retry times
	maxNackRetries = 10
	// retry interval
	defaultRtt = 100 // every 100ms
	// check interval
	defaultInterval = 40
)

type Receiver interface {
	IncomingPacket(ms int64, packet rtc.Packet)
	UpdateRTT(int64) // allow update rtt dynamic.
	Close()
}

type packetInfo struct {
	seq       uint16
	sentAtMs  int64
	sentAtSeq uint16
	retries   int
	keyframe  bool
	rtx       bool
}

func NewReceiver(sendRtcp rtc.SendRTCP) Receiver {
	r := &receiver{
		packetChan: make(chan packetInfo),
		interval:   defaultInterval * time.Millisecond,
		closeCh:    make(chan struct{}),
		sendRtcp:   sendRtcp,
		rtt:        defaultRtt,
	}
	r.wg.Add(1)
	go r.loop()
	return r
}

type receiver struct {
	wg          sync.WaitGroup
	packetChan  chan packetInfo
	interval    time.Duration
	closeCh     chan struct{}
	lastSeq     uint16
	nackList    []*packetInfo
	keyframes   []uint16
	recoverList []uint16
	mediaSSRC   uint32
	sendRtcp    rtc.SendRTCP
	rtt         int64
}

func (r *receiver) UpdateRTT(rtt int64) {
	r.rtt = rtt
}

func (r *receiver) IncomingPacket(ms int64, packet rtc.Packet) {
	r.incomingPacket(packet.SequenceNumber(), packet.IsKeyFrame(), packet.IsRTX())
}

func (r *receiver) incomingPacket(seq uint16, keyFrame bool, rtx bool) {
	packet := packetInfo{
		seq:      seq,
		keyframe: keyFrame,
		rtx:      rtx,
	}
	r.packetChan <- packet
}

func (r *receiver) Close() {
	close(r.closeCh)
}

func (r *receiver) loop() {
	defer r.wg.Done()
	// wait first pkt
	select {
	case <-r.closeCh:
		return
	case p := <-r.packetChan:
		r.lastSeq = p.seq
		if p.keyframe {
			r.keyframes = append(r.keyframes, p.seq)
		}
	}
	ticker := time.NewTicker(r.interval)
	for {
		select {
		case <-r.closeCh:
			return
		case p := <-r.packetChan:
			r.receive(p)
		case <-ticker.C:
			if batch := r.getNackBatch(true); len(batch) != 0 {
				packet := &rtcp.TransportLayerNack{
					MediaSSRC: r.mediaSSRC,
					Nacks:     rtcp.NackPairsFromSequenceNumbers(batch),
				}
				r.sendRtcp(packet)
			}
		}
	}
}

func IsSeqLowerThan(seq uint16, seq2 uint16) bool {
	return seq2 > seq && seq2-seq <= rtc.SeqNumberMaxValue/2 || seq > seq2 && seq-seq2 > rtc.SeqNumberMaxValue/2
}

func (r *receiver) receive(p packetInfo) {
	if p.seq == r.lastSeq {
		return
	}
	// we received a small seq pkt, could be dis-order or re-sent.
	if IsSeqLowerThan(p.seq, r.lastSeq) {
		for i, v := range r.nackList {
			// found it in  nacklist
			if v.seq == p.seq {
				r.nackList = append(r.nackList[:i], r.nackList[i+1:]...)
			}
		}
		// if it not in nack list, could be dis-order, and we received both normal and re-sent pkt.
		return
	}

	if p.keyframe {
		r.keyframes = append(r.keyframes, p.seq)
	}
	// remove old keyframe
	for i, v := range r.keyframes {
		if v >= p.seq-maxPacketAge {
			r.keyframes = r.keyframes[i:]
			break
		}
	}

	if p.rtx {
		// we received an rtx before rtp
		r.recoverList = append(r.recoverList, p.seq)
		for i, v := range r.recoverList {
			if v >= p.seq-maxPacketAge {
				r.recoverList = r.recoverList[i:]
			}
		}
		return
	}

	r.addPacketsToNackList(r.lastSeq+1, p)
	r.lastSeq = p.seq

	if batch := r.getNackBatch(false); len(batch) != 0 {
		packet := &rtcp.TransportLayerNack{
			MediaSSRC: r.mediaSSRC,
			Nacks:     rtcp.NackPairsFromSequenceNumbers(batch),
		}
		r.sendRtcp(packet)
	}
}

func (r *receiver) addPacketsToNackList(expected uint16, pkt packetInfo) {
	// remove lower than seq-maxAge
	index := 0
	for i, v := range r.nackList {
		if pkt.seq-v.seq > maxPacketAge {
			continue
		} else {
			index = i
			break
		}
	}
	r.nackList = r.nackList[index:]
	newSeq := pkt.seq - expected
	// the nackList is full
	if len(r.nackList)+int(newSeq) > maxNackPackets {
		for {
			if !r.removeNackItemsUntilKeyFrame() || len(r.nackList)+int(newSeq) <= maxNackPackets {
				break
			}
		}
		// after clean, we still have a huge nack list, clear the list and request a new Keyframe
		if len(r.nackList)+int(newSeq) > maxNackPackets {
			r.nackList = r.nackList[0:0]
			// we only emit pli, let callback decide pli or fir.
			r.sendRtcp(&rtcp.PictureLossIndication{
				MediaSSRC: r.mediaSSRC,
			})
			return
		}
	}

	for i := expected; i != pkt.seq; i++ {
		found := false
		// lost it, but recovered.
		for _, v := range r.recoverList {
			if v == i {
				found = true
				break
			}
		}
		if found {
			continue
		}
		r.nackList = append(r.nackList, &packetInfo{
			seq:       i,
			sentAtSeq: i,
		})
	}
}

func (r *receiver) getNackBatch(onTimer bool) []uint16 {
	var batch []uint16
	nowMs := time.Now().UnixMilli()
	nackList := make([]*packetInfo, 0, len(r.nackList))
	for _, seq := range r.nackList {
		if (onTimer && nowMs-seq.sentAtMs >= r.rtt) || (seq.sentAtMs == 0 && (seq.sentAtSeq == r.lastSeq || IsSeqLowerThan(seq.sentAtSeq, r.lastSeq))) {
			batch = append(batch, seq.seq)
			seq.retries++
			seq.sentAtMs = nowMs
			seq.sentAtSeq = r.lastSeq

			if seq.retries < maxNackRetries {
				nackList = append(nackList, seq)
			}
		}
	}
	r.nackList = nackList
	return batch
}

func (r *receiver) removeNackItemsUntilKeyFrame() bool {
	for i, v := range r.keyframes {
		var index int
		for index = range r.nackList {
			if r.nackList[index].seq >= v {
				break
			}
		}
		if index > 0 {
			if i != 0 {
				r.keyframes = r.keyframes[i:]
			}
			r.nackList = r.nackList[index:]
			return true
		}
	}
	return false
}
