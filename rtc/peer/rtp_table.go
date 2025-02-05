package peer

import (
	"errors"

	"github.com/gotolive/sfu/rtc"
)

var (
	ErrMidExist  = errors.New("mid already in use")
	ErrRidExist  = errors.New("rid already in use")
	ErrSsrcExist = errors.New("ssrc already in use")
)

type rtpTable struct {
	SsrcTable map[uint32]*Receiver
	MidTable  map[string]*Receiver
	RidTable  map[string]*Receiver
}

func newRTPTable() *rtpTable {
	return &rtpTable{
		SsrcTable: map[uint32]*Receiver{},
		MidTable:  map[string]*Receiver{},
		RidTable:  map[string]*Receiver{},
	}
}

func (r *rtpTable) AddProducer(producer *Receiver) error {
	for _, v := range producer.GetRTPStreams() {
		if v.SSRC() != 0 {
			if _, ok := r.SsrcTable[v.SSRC()]; ok {
				r.RemoveProducer(producer)
				return ErrSsrcExist
			} else {
				r.SsrcTable[v.SSRC()] = producer
			}
		}
		if v.RtxSSRC() != 0 {
			if _, ok := r.SsrcTable[v.RtxSSRC()]; ok {
				r.RemoveProducer(producer)
				return ErrSsrcExist
			} else {
				r.SsrcTable[v.RtxSSRC()] = producer
			}
		}
		if v.RID() != "" {
			if _, ok := r.RidTable[v.RID()]; ok && producer.MID() == "" {
				r.RemoveProducer(producer)
				return ErrRidExist
			} else {
				r.RidTable[v.RID()] = producer
			}
		}
	}
	if producer.MID() != "" {
		if _, ok := r.MidTable[producer.MID()]; ok {
			r.RemoveProducer(producer)
			return ErrMidExist
		} else {
			r.MidTable[producer.MID()] = producer
		}
	}

	return nil
}

func (r *rtpTable) GetProducer(packet rtc.Packet, rtpHeader map[string]rtc.HeaderExtensionID) *Receiver {
	if p, ok := r.SsrcTable[packet.SSRC()]; ok {
		return p
	}
	if mid, ok := rtpHeader[rtc.HeaderExtensionMid]; ok && mid != 0 {
		if p, ok := r.MidTable[packet.Mid(mid)]; ok {
			r.SsrcTable[packet.SSRC()] = p
			return p
		}
	}
	if rid, ok := rtpHeader[rtc.HeaderExtensionRid]; ok && rid != 0 {
		if p, ok := r.RidTable[packet.Rid(rid)]; ok {
			r.SsrcTable[packet.SSRC()] = p
			return p
		}
	}

	return nil
}

func (r *rtpTable) GetProducerBySsrc(ssrc uint32) *Receiver {
	return r.SsrcTable[ssrc]
}

func (r *rtpTable) RemoveProducer(producer *Receiver) {
	for k, v := range r.SsrcTable {
		if v == producer {
			delete(r.SsrcTable, k)
		}
	}
	for k, v := range r.MidTable {
		if v == producer {
			delete(r.MidTable, k)
		}
	}
	for k, v := range r.RidTable {
		if v == producer {
			delete(r.RidTable, k)
		}
	}
}
