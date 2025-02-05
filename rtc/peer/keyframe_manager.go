package peer

import (
	"sync/atomic"
	"time"
)

const keyframeWaitTimeout = time.Second

func newKeyframeManager(delay int, lis func(uint32)) *keyframeManager {
	k := &keyframeManager{
		closeCh:         make(chan struct{}),
		ssrcNeedCh:      make(chan uint32),
		ssrcReceived:    make(chan uint32),
		ssrcDelayCh:     make(chan uint32),
		ssrcTimeout:     make(chan uint32),
		pendingKeyframe: map[uint32]*pendingInfo{},
		delayKeyframe:   map[uint32]*delayInfo{},
		callback:        lis,
	}
	if delay != 0 {
		k.delay = time.Millisecond * time.Duration(delay)
	}
	go k.loop()
	return k
}

type keyframeManager struct {
	// delay could decrease key frame in mass senders, but it may cause delay for new sender.
	// so it a trade-off.
	delay           time.Duration
	closeCh         chan struct{}
	ssrcNeedCh      chan uint32
	ssrcReceived    chan uint32
	ssrcDelayCh     chan uint32
	pendingKeyframe map[uint32]*pendingInfo
	delayKeyframe   map[uint32]*delayInfo
	callback        func(uint32)
	ssrcTimeout     chan uint32
}

func (k *keyframeManager) keyFrameNeeded(ssrc uint32) {
	if k.delay != 0 {
		k.ssrcDelayCh <- ssrc
	} else {
		k.ssrcNeedCh <- ssrc
	}
}

func (k *keyframeManager) keyFrameReceived(ssrc uint32) {
	k.ssrcReceived <- ssrc
}

func (k *keyframeManager) close() {
	close(k.closeCh)
}

func (k *keyframeManager) loop() {
	for {
		select {
		case <-k.closeCh:
			return
		case ssrc := <-k.ssrcNeedCh:
			// if we received delay need, remove delay
			if info, ok := k.delayKeyframe[ssrc]; ok {
				info.Stop()
				delete(k.delayKeyframe, ssrc)
			}
			// we're already waiting the keyframe for given ssrc
			if info, ok := k.pendingKeyframe[ssrc]; ok {
				info.Retry()
				continue
			}
			k.pendingKeyframe[ssrc] = newPending(ssrc, k.callback, k.ssrcTimeout)
			k.callback(ssrc)
		case ssrc := <-k.ssrcDelayCh:
			// we already have one, skip.
			if _, ok := k.delayKeyframe[ssrc]; ok {
				continue
			}
			if _, ok := k.pendingKeyframe[ssrc]; ok {
				continue
			}
			k.delayKeyframe[ssrc] = newDelay(k.delay, ssrc, func(ssrc uint32) {
				k.ssrcNeedCh <- ssrc
			})
		case ssrc := <-k.ssrcReceived:
			if info, ok := k.delayKeyframe[ssrc]; ok {
				info.Stop()
				delete(k.delayKeyframe, ssrc)
			}
			if info, ok := k.pendingKeyframe[ssrc]; ok {
				info.Stop()
				delete(k.pendingKeyframe, ssrc)
			}
		case ssrc := <-k.ssrcTimeout:
			if info, ok := k.delayKeyframe[ssrc]; ok {
				info.Stop()
				delete(k.delayKeyframe, ssrc)
			}
			if info, ok := k.pendingKeyframe[ssrc]; ok {
				info.Stop()
				delete(k.pendingKeyframe, ssrc)
			}
		}
	}
}

func newPending(ssrc uint32, needed func(ssrc uint32), timeoutCh chan uint32) *pendingInfo {
	//  default retry once, but if we have more request, we will retry more
	info := pendingInfo{retry: 1, stopCh: make(chan struct{})}
	go func() {
		ticker := time.NewTicker(keyframeWaitTimeout)
		for {
			retry := atomic.AddInt32(&info.retry, -1)
			if retry < 0 {
				timeoutCh <- ssrc
				return
			}
			select {
			case <-info.stopCh:
				return
			case <-ticker.C:
				needed(ssrc)
			}
		}
	}()
	return &info
}

type pendingInfo struct {
	stopCh chan struct{}
	retry  int32
}

func (i *pendingInfo) Stop() {
	close(i.stopCh)
}

func (i *pendingInfo) Retry() {
	atomic.AddInt32(&i.retry, 1)
}

func newDelay(delay time.Duration, ssrc uint32, needed func(ssrc uint32)) *delayInfo {
	info := &delayInfo{
		ssrc:   ssrc,
		stopCh: make(chan struct{}),
	}
	go func() {
		select {
		case <-info.stopCh:
			return
		case <-time.After(delay):
			needed(ssrc)
		}
	}()
	return info
}

type delayInfo struct {
	ssrc   uint32
	stopCh chan struct{}
}

func (i *delayInfo) Stop() {
	close(i.stopCh)
}
