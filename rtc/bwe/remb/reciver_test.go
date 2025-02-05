package remb

import (
	"fmt"
	"math"
	"testing"

	"github.com/pion/rtcp"
)

var (
	kNumInitialPackets       = 2
	kDefaultSsrc             = 1
	kMtu                     = 1200
	kAcceptedBitrateErrorBps = 50000.0
)

type receiverObserver struct {
	updated     bool
	lastBitrate uint64
}

func (o *receiverObserver) observe(packet rtcp.Packet) {
	p, ok := packet.(*rtcp.ReceiverEstimatedMaximumBitrate)
	if ok {
		o.lastBitrate = uint64(p.Bitrate)
		o.updated = true
	}
}

func (o *receiverObserver) reset() {
	o.updated = false
}

type streamGenerator struct {
	streams map[uint]struct {
		framePerSecond  int
		bitrate         uint64
		ssrc            uint
		rtpFrequency    int
		timestampOffset int
		rtcpReceiveTime int
	}
}

//
//func (g *streamGenerator) AddDefaultStream() {
//	g.addStream(30, 3e5, 1, 90000, 0xFFFFF000, 0)
//}

//func (g *streamGenerator) generateAndProcessFrame(r *receiver, o *receiverObserver, ssrc int, bps uint64) bool {
//	g.setBitrateBps(bps)
//	nextTimeUs, packets := g.generateFrame(r.clock)
//	var overuse bool
//	for i := range packets {
//		o.reset()
//		r.clock.Add()
//		r.incomingPacketInfo()
//		if o.updated && o.lastBitrate < bps {
//			overuse = true
//		}
//	}
//	r.clock.Add()
//	return overuse
//}
//
//func (g *streamGenerator) addStream(framePerSecond int, bitrate uint64, ssrc uint, rtpFrequency int, timestampOffset int, rtcpReceiveTime int) {
//	g.streams[ssrc] = struct {
//		framePerSecond  int
//		bitrate         uint64
//		ssrc            uint
//		rtpFrequency    int
//		timestampOffset int
//		rtcpReceiveTime int
//	}{framePerSecond: framePerSecond, bitrate: bitrate, ssrc: ssrc, rtpFrequency: rtpFrequency, timestampOffset: timestampOffset, rtcpReceiveTime: rtcpReceiveTime}
//}
//
//func (g *streamGenerator) setBitrateBps(bps uint64) {
//
//}

func AbsSendTime(t, denom int64) uint32 {
	return uint32(((t<<18)+(denom>>1))/denom) & 0x00ffffff
}

func AddAbsSendTime(t1, t2 uint32) uint32 {
	return (t1 + t2) & 0x00ffffff
}

func initialBehavior(t *testing.T) {
	clock := &SimulatedClock{
		currentUs: 100000000,
	}
	o := new(receiverObserver)
	receiver := NewReceiver(1, o.observe)
	receiver.clock = clock
	// expectedConvergeBitrate := 674840
	framerate := 50
	abs := uint32(0)
	// frameIntervalMs := 1000 / framerate
	frameIntervalAbsSendTime := AbsSendTime(1, int64(framerate))
	// var bitrateBps uint64 = 0
	ssrcs := make([]int, 0)
	var flag bool
	ssrcs, _, flag = receiver.LatestEstimate()
	if flag {
		t.Fatal("flag should be false")
	}
	if len(ssrcs) != 0 {
		t.Fatal("ssrcs should be empty")
	}
	o.reset()
	clock.Add(Millisecond * 2000)
	for i := 0; i < 5*framerate+1+kNumInitialPackets; i++ {
		// when it is 2, expect not updated
		if i == kNumInitialPackets {
			ssrcs, _, flag = receiver.LatestEstimate()
			if flag {
				t.Log("flag should be false")
			}
			if len(ssrcs) != 0 {
				t.Log("ssrcs should be empty")
			}
			o.reset()
		}
		receiver.incomingPacketInfo(clock.NowMs(), kMtu, kDefaultSsrc, abs)
		clock.Add(1000 * Millisecond / Duration(framerate))
		abs = AddAbsSendTime(abs, frameIntervalAbsSendTime)
	}
	var bps uint64
	ssrcs, bps, flag = receiver.LatestEstimate()
	if !flag {
		t.Fatal("flag should be true")
	}
	if len(ssrcs) != 1 || ssrcs[0] != kDefaultSsrc {
		t.Fatal("ssrcs should be empty")
	}
	if math.Abs(float64(bps-674840)) > kAcceptedBitrateErrorBps {
		t.Fatal("bps:", bps)
	}
	if !o.updated {
		t.Fatal("should be updated")
	}
	o.reset()
	if o.lastBitrate != bps {
		t.Fatal("should be equal")
	}
	receiver.RemoveStream(kDefaultSsrc)
	ssrcs, bps, flag = receiver.LatestEstimate()
	if !flag {
		t.Fatal("flag should be true")
	}
	if len(ssrcs) != 0 {
		t.Fatal("ssrcs should be empty")
	}
	if bps != 0 {
		t.Fatal("bps:", bps)
	}
}

func TestReceiver(t *testing.T) {
	tests := []struct {
		name   string
		method func(t2 *testing.T)
	}{

		{
			name:   "TestProbeDetection",
			method: testProbeDetection,
		},

		{
			name:   "TestProbeDetectionTooHighBitrate",
			method: testProbeDetectionTooHighBitrate,
		},
		{
			name:   "TestProbeDetectionSlightlyFasterArrival",
			method: testProbeDetectionSlightlyFasterArrival,
		},
		{
			name:   "TestProbeDetectionFasterArrival",
			method: testProbeDetectionFasterArrival,
		},
		{
			name:   "TestProbeDetectionSlowerArrival",
			method: testProbeDetectionSlowerArrival,
		},
		{
			name:   "TestProbeDetectionSlowerArrivalHighBitrate",
			method: testProbeDetectionSlowerArrivalHighBitrate,
		},
		{
			name:   "ProbingIgnoresSmallPackets",
			method: probingIgnoresSmallPackets,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.method(t)
		})
	}
}

func probingIgnoresSmallPackets(t *testing.T) {
	probeLength := 5
	clock := &SimulatedClock{
		currentUs: 100000000,
	}
	o := new(receiverObserver)
	var now int64
	receiver := NewReceiver(1, o.observe)
	receiver.clock = clock
	for i := 0; i < probeLength; i++ {
		clock.Add(Millisecond * 10)
		now = clock.NowMs()
		receiver.incomingPacketInfo(now, 200, 0, AbsSendTime(now, 1000))
	}
	if o.updated {
		t.Fatal("should not update")
	}
	for i := 0; i < probeLength; i++ {
		clock.Add(Millisecond * 10)
		now = clock.NowMs()
		receiver.incomingPacketInfo(now, 1000, 0, AbsSendTime(now, 1000))
	}
	if !o.updated {
		t.Fatal("should be updated")
	}
	if math.Abs(800000-float64(o.lastBitrate)) > 10000 {
		t.Fatal("should be great")
	}
}

func testProbeDetectionSlowerArrivalHighBitrate(t *testing.T) {
	probeLength := 5
	clock := &SimulatedClock{
		currentUs: 100000000,
	}
	o := new(receiverObserver)
	var now, sendTimeMs int64
	receiver := NewReceiver(1, o.observe)
	receiver.clock = clock
	for i := 0; i < probeLength; i++ {
		clock.Add(Millisecond * 2)
		now = clock.NowMs()
		sendTimeMs += 1
		receiver.incomingPacketInfo(now, 1000, 0, AbsSendTime(sendTimeMs, 1000))
	}

	if !o.updated {
		t.Fatal("should be updated")
	}
	if math.Abs(4000000-float64(o.lastBitrate)) > 10000 {
		t.Fatal("should be great")
	}
}

func testProbeDetectionSlowerArrival(t *testing.T) {
	probeLength := 5
	clock := &SimulatedClock{
		currentUs: 100000000,
	}
	o := new(receiverObserver)
	var now, sendTimeMs int64
	receiver := NewReceiver(1, o.observe)
	receiver.clock = clock
	for i := 0; i < probeLength; i++ {
		clock.Add(Millisecond * 7)
		now = clock.NowMs()
		sendTimeMs += 5
		receiver.incomingPacketInfo(now, 1000, 0, AbsSendTime(sendTimeMs, 1000))
	}

	if !o.updated {
		t.Fatal("should be updated")
	}
	if math.Abs(1140000-float64(o.lastBitrate)) > 10000 {
		t.Fatal("should be great")
	}
}

func testProbeDetectionFasterArrival(t *testing.T) {
	probeLength := 5
	clock := &SimulatedClock{
		currentUs: 100000000,
	}
	o := new(receiverObserver)
	var now, sendTimeMs int64
	receiver := NewReceiver(1, o.observe)
	receiver.clock = clock
	for i := 0; i < probeLength; i++ {
		clock.Add(Millisecond * 1)
		now = clock.NowMs()
		sendTimeMs += 10
		receiver.incomingPacketInfo(now, 1000, 0, AbsSendTime(sendTimeMs, 1000))
	}

	if o.updated {
		t.Fatal("should be false")
	}
}

func testProbeDetectionSlightlyFasterArrival(t *testing.T) {
	probeLength := 5
	clock := &SimulatedClock{
		currentUs: 100000000,
	}
	o := new(receiverObserver)
	var now, sendTimeMs int64
	receiver := NewReceiver(1, o.observe)
	receiver.clock = clock
	for i := 0; i < probeLength; i++ {
		clock.Add(Millisecond * 5)
		now = clock.NowMs()
		sendTimeMs += 10
		receiver.incomingPacketInfo(now, 1000, 0, AbsSendTime(sendTimeMs, 1000))
	}

	if !o.updated {
		t.Fatal("should be updated")
	}
	if o.lastBitrate < 800000 {
		t.Fatal("should be great")
	}
}

func testProbeDetectionTooHighBitrate(t *testing.T) {
	probeLength := 5
	clock := &SimulatedClock{
		currentUs: 100000000,
	}
	o := new(receiverObserver)
	var now, sendTimeMs int64
	receiver := NewReceiver(1, o.observe)
	receiver.clock = clock
	for i := 0; i < probeLength; i++ {
		clock.Add(Millisecond * 10)
		now = clock.NowMs()
		sendTimeMs += 10
		receiver.incomingPacketInfo(now, 1000, 0, AbsSendTime(sendTimeMs, 1000))
	}
	for i := 0; i < probeLength; i++ {
		clock.Add(Millisecond * 8)
		now = clock.NowMs()
		sendTimeMs += 5
		receiver.incomingPacketInfo(now, 1000, 0, AbsSendTime(sendTimeMs, 1000))
	}
	if !o.updated {
		t.Fatal("should be updated")
	}
	fmt.Println(o.updated)
	fmt.Println(o.lastBitrate)
	if math.Abs(float64(800000-int64(o.lastBitrate))) > 10000 {
		t.Fatal("should be less")
	}
}

func testProbeDetectionNonPacedPackets(t *testing.T) {
	probeLength := 5
	clock := &SimulatedClock{
		currentUs: 100000000,
	}
	o := new(receiverObserver)
	var now int64
	receiver := NewReceiver(1, o.observe)
	receiver.clock = clock
	for i := 0; i < probeLength; i++ {
		clock.Add(Millisecond * 5)
		now = clock.NowMs()
		receiver.incomingPacketInfo(now, 1000, 0, AbsSendTime(now, 1000))
		clock.Add(Millisecond * 5)
		receiver.incomingPacketInfo(now, 100, 0, AbsSendTime(now, 1000))
	}
	if !o.updated {
		t.Fatal("should updated")
	}
	if o.lastBitrate <= 800000 {
		t.Fatal("should be great", o.lastBitrate)
	}
}

func testProbeDetection(t *testing.T) {
	probeLength := 5
	clock := &SimulatedClock{
		currentUs: 100000000,
	}
	o := new(receiverObserver)
	var now int64
	receiver := NewReceiver(1, o.observe)
	receiver.clock = clock
	for i := 0; i < probeLength; i++ {
		clock.Add(Millisecond * 10)
		now = clock.NowMs()
		receiver.incomingPacketInfo(now, 1000, 0, AbsSendTime(now, 1000))
	}
	for i := 0; i < probeLength; i++ {
		clock.Add(Millisecond * 5)
		now = clock.NowMs()
		receiver.incomingPacketInfo(now, 1000, 0, AbsSendTime(now, 1000))
	}
	if !o.updated {
		t.Fatal("should updated")
	}
	if o.lastBitrate <= 1500000 {
		t.Fatal("should be great", o.lastBitrate)
	}
}

func TestNewReceiver(t *testing.T) {
	//r := NewReceiver(0, func(packet rtcp.Packet) {
	//	fmt.Println(packet)
	//})
	//r.incomingPacket(0, 0, 0, 0)
	//file, err := os.Open("../../../testdata/remb")
	//if err != nil {
	//	t.Fatal("read file fail", err)
	//}
	//scanner := bufio.NewScanner(file)
	//data := struct {
	//	ArrivalTime int64  `json:"arrivalTime"`
	//	PayloadSize int    `json:"payloadSize"`
	//	SendTime    uint32 `json:"sendTime"`
	//	SSRC        uint32 `json:"ssrc"`
	//}{}
	//i := int64(0)
	//delta := int64(0)
	//for scanner.Scan() {
	//	if i%100 == 0 {
	//		delta += 20
	//	}
	//	i++
	//	line := scanner.Text()
	//	json.Unmarshal([]byte(line), &data)
	//	r.incomingPacket(data.ArrivalTime+delta, data.PayloadSize, data.SSRC, data.SendTime)
	//}
	//fmt.Println(i)
}

func rateIncreaseReordering(t *testing.T) {
	clock := &SimulatedClock{
		currentUs: 100000000,
	}
	o := new(receiverObserver)
	receiver := NewReceiver(1, o.observe)
	receiver.clock = clock
	framerate := 50
	abs := uint32(0)
	frameIntervalAbsSendTime := AbsSendTime(1, int64(framerate))
	for i := 0; i < 5*framerate+1+kNumInitialPackets; i++ {
		// when it is 2, expect not updated
		if i == kNumInitialPackets {
			receiver.Process()
			if o.updated {
				t.Fatal("should be false")
			}
		}
		receiver.incomingPacketInfo(clock.NowMs(), kMtu, kDefaultSsrc, abs)
		clock.Add(1000 * Millisecond / Duration(framerate))
		abs = AddAbsSendTime(abs, frameIntervalAbsSendTime)
	}
	receiver.Process()
	if !o.updated {
		t.Fatal("should be true")
	}
	if math.Abs(float64(o.lastBitrate-674840)) > kAcceptedBitrateErrorBps {
		t.Fatal("bps:", o.lastBitrate)
	}
	for i := 0; i < 10; i++ {
		clock.Add(2 * 1000 * Millisecond / Duration(framerate))
		abs = AddAbsSendTime(abs, 2*frameIntervalAbsSendTime)
		receiver.incomingPacketInfo(clock.NowMs(), 1000, kDefaultSsrc, abs)
		receiver.incomingPacketInfo(clock.NowMs(), 1000, kDefaultSsrc, AddAbsSendTime(abs, -frameIntervalAbsSendTime))
	}
	receiver.Process()
	if !o.updated {
		t.Fatal("should be true")
	}
	if math.Abs(float64(o.lastBitrate-674840)) > kAcceptedBitrateErrorBps {
		t.Fatal("bps:", o.lastBitrate)
	}
}

func TestReceiver_IncomingPacket(t *testing.T) {
	//r := NewReceiver(func(packet rtcp.packet) {
	//	fmt.Println(packet)
	//})
	//r.incomingPacketInfo(time.Now().UnixNano()/1000, 500, 3, 1000)
}
