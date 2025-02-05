package remb

import (
	"math"
	"time"

	"github.com/gotolive/sfu/rtc"
	"github.com/gotolive/sfu/rtc/logger"
	"github.com/pion/rtcp"
)

const (
	AbsSendTimeInterArrivalUpshift = 8
	UnlimitedRembNumPackets        = 4
	StreamTimeOutMs                = 2000
	InitialProbingIntervalMs       = 2000
	AbsSendTimeFraction            = 18
	InterArrivalShift              = AbsSendTimeFraction + AbsSendTimeInterArrivalUpshift
	MaxProbePackets                = 15
	TimestampGroupLengthMs         = 5
	TimestampToMs                  = 1000.0 / (1 << InterArrivalShift)
)

// const TimestampToMs = 1000.0 / float64(1<<24)

const (
	BitrateWindowMs        = 1000
	NoUpdate               = 0
	Update                 = 1
	ExpectedNumberOfProbes = 3
	MinClusterDelta        = 1
	MinClusterSize         = 4
)

type Clock interface {
	NowMs() int64 // Return current timestamp in ms.
}

type defaultClock string

func (d *defaultClock) NowMs() int64 {
	return time.Now().UnixMilli()
}

// NewReceiver RemoteBitrate Estimator
// https://datatracker.ietf.org/doc/html/draft-alvestrand-rmcat-congestion-03
func NewReceiver(id uint8, sendRtcp func(packet rtcp.Packet)) *receiver {
	clock := new(defaultClock)
	r := &receiver{
		id:                id,
		incomingBitrate:   NewRateStatistics(BitrateWindowMs, 8000),
		interArrival:      newInterArrival((TimestampGroupLengthMs<<InterArrivalShift)/1000, TimestampToMs, true),
		detector:          NewOveruseDetector(),
		estimator:         NewOveruseEstimator(),
		remoteRate:        NewAimdRateControl(),
		ssrcs:             make(map[int]int64),
		sendRtcp:          sendRtcp,
		firstPacketTimeMs: -1,
		clock:             clock,
	}
	return r
}

type RateControllerInput struct {
	State               int
	EstimatedThroughput *Bitrate
}

type Probe struct {
	sendTime    uint32
	recvTime    int64
	payloadSize int
}

type receiver struct {
	sendRtcp             func(packet rtcp.Packet)
	maxIncomingBitrate   uint64
	minIncomingBitrate   uint64
	unlimitedRembCounter int
	lastUpdateMs         int64
	// should be guard by mutex
	ssrcs                      map[int]int64 // ssrc->latest timestamp
	remoteRate                 *AimdRateControl
	estimator                  *OveruseEstimator
	detector                   *OveruseDetector
	incomingBitrate            *RateStatistics
	incomingBitrateInitialized bool
	firstPacketTimeMs          int64
	interArrival               *interArrival
	probes                     []*Probe
	clock                      Clock
	id                         uint8
}

func (r *receiver) Close() {
}

func (r *receiver) IncomingPacket(ms int64, packet rtc.Packet) {
	abs, _ := packet.ReadAbsSendTime(r.id)
	r.incomingPacket(ms, packet.Size(), packet.SSRC(), abs)
}

// should not use packet, upper layer check it.
func (r *receiver) incomingPacket(ms int64, payloadSize int, ssrc uint32, abs uint32) {
	r.incomingPacketInfo(ms, payloadSize, int(ssrc), abs)
}

func (r *receiver) SetMaxIncomingBitrate(bitrate uint64) {
	p := r.maxIncomingBitrate
	r.maxIncomingBitrate = bitrate
	if p != 0 && bitrate == 0 {
		r.unlimitedRembCounter = UnlimitedRembNumPackets
		r.maySendLimitationRembFeedback()
	}
}

func (r *receiver) SetMinIncomingBitrate(bitrate uint64) {
	r.minIncomingBitrate = bitrate
}

func (r *receiver) incomingPacketInfo(arrivalTimeMs int64, payloadSize int, ssrc int, sendTime uint32) {
	// bitrate
	var updateEstimate bool
	var bitrate uint64

	if payloadSize > 200 {
		_ = payloadSize
	}
	timestamp := sendTime << 8
	sendTimeMs := toTimestamp(sendTime)
	nowMs := arrivalTimeMs

	incomingRate := r.incomingBitrate.Rate(arrivalTimeMs)

	if incomingRate != nil {
		r.incomingBitrateInitialized = true
	} else if r.incomingBitrateInitialized {
		r.incomingBitrate.Reset()
		r.incomingBitrateInitialized = false
	}

	// Update new bitrate
	r.incomingBitrate.Update(int64(payloadSize), arrivalTimeMs)

	if r.firstPacketTimeMs == -1 {
		r.firstPacketTimeMs = nowMs
	}

	r.timeoutStreams(nowMs)
	r.ssrcs[ssrc] = nowMs

	// first 2 second we try to response quickly.
	if payloadSize > 200 && (!r.remoteRate.ValidEstimate() || nowMs-r.firstPacketTimeMs < InitialProbingIntervalMs) {
		r.probes = append(r.probes, &Probe{
			sendTime:    sendTimeMs,
			recvTime:    arrivalTimeMs,
			payloadSize: payloadSize,
		})
		updateEstimate = r.ProcessClusters(nowMs) == Update
	}

	if delta := r.interArrival.computeDeltas(timestamp, arrivalTimeMs, nowMs, payloadSize); delta != nil {
		tsDeltaMs := 1000 * delta.tsDelta / 1 << InterArrivalShift
		r.estimator.Update(delta.tDelta, delta.tsDelta, delta.sizeDelta, r.detector.State(), arrivalTimeMs)
		r.detector.Detect(r.estimator.Offset(), tsDeltaMs, r.estimator.NumOfDeltas(), arrivalTimeMs)
	}

	if !updateEstimate {
		if r.lastUpdateMs == 0 || arrivalTimeMs-r.lastUpdateMs > int64(r.remoteRate.FeedbackInterval()/Millisecond) {
			updateEstimate = true
		} else if r.detector.State() == Overusing {
			ir := r.incomingBitrate.Rate(arrivalTimeMs)
			if ir != nil && r.remoteRate.TimeToReduceFurther(arrivalTimeMs, *ir) {
				updateEstimate = true
			}
		}
	}

	if updateEstimate {
		input := RateControllerInput{
			State:               r.detector.State(),
			EstimatedThroughput: r.incomingBitrate.Rate(arrivalTimeMs),
		}

		bitrate = uint64(r.remoteRate.Update(input, nowMs))
		updateEstimate = r.remoteRate.ValidEstimate()
	}

	if updateEstimate {
		r.sendFeedback(arrivalTimeMs, bitrate)
	}
}

func (r *receiver) sendFeedback(arrivalTimeMs int64, bitrate uint64) {
	if r.maxIncomingBitrate != 0 && bitrate > r.maxIncomingBitrate {
		bitrate = r.maxIncomingBitrate
	}
	if r.minIncomingBitrate != 0 && bitrate < r.minIncomingBitrate {
		bitrate = r.minIncomingBitrate
	}
	r.lastUpdateMs = arrivalTimeMs
	p := &rtcp.ReceiverEstimatedMaximumBitrate{
		Bitrate: float32(bitrate),
	}
	for k := range r.ssrcs {
		p.SSRCs = append(p.SSRCs, uint32(k))
	}
	r.sendRtcp(p)
}

func toTimestamp(time uint32) uint32 {
	timestamp := time << AbsSendTimeInterArrivalUpshift
	return uint32(float64(timestamp) * (TimestampToMs))
}

func (r *receiver) maySendLimitationRembFeedback() {
}

func (r *receiver) timeoutStreams(ms int64) {
	for k, v := range r.ssrcs {
		if ms-v > StreamTimeOutMs {
			delete(r.ssrcs, k)
		}
	}
	// if no ssrc left, reset.
	if len(r.ssrcs) == 0 {
		r.interArrival = newInterArrival((TimestampGroupLengthMs<<InterArrivalShift)/1000, TimestampToMs, true)
		r.estimator = NewOveruseEstimator()
	}
}

func (r *receiver) ProcessClusters(now int64) int {
	clusters := r.ComputeClusters()
	if len(clusters) == 0 {
		if len(r.probes) >= MaxProbePackets {
			r.probes = r.probes[1:]
		}
		return NoUpdate
	}
	if best := r.findBestProbe(clusters); best != nil {
		probeBitrate := uint64(math.Min(float64(best.SendBitrate()), float64(best.RecvBitrate())))
		if r.isBitrateImproving(probeBitrate) {
			logger.Info("Probe successful, sent at ", best.SendBitrate(), " bps, received at ", best.RecvBitrate(),
				" bps. Mean send groupDelta: ", best.sendMean,
				" ms, mean recv groupDelta: ", best.recvMean,
				" ms, num probes: ", best.count)
			r.remoteRate.SetEstimate(Bitrate(probeBitrate), now)
			return Update
		}
	}
	if len(clusters) >= ExpectedNumberOfProbes {
		r.probes = r.probes[0:0]
	}
	return NoUpdate
}

func (r *receiver) ComputeClusters() []Cluster {
	clusters := make([]Cluster, 0)
	var prevSendTime int64 = -1
	var prevRecvTime int64 = -1
	var cluster Cluster
	for _, probe := range r.probes {
		if prevSendTime >= 0 {
			sendDelta := probe.sendTime - uint32(prevSendTime) // 5
			recvDelta := probe.recvTime - prevRecvTime         // 5
			if sendDelta >= MinClusterDelta && recvDelta >= MinClusterDelta {
				cluster.numAboveMinDelta++
			}
			if !r.isWithinClusterBounds(sendDelta, cluster) {
				clusters = r.maybeAddCluster(cluster, clusters)
				cluster = Cluster{}
			}
			cluster.sendMean += int64(sendDelta)
			cluster.recvMean += recvDelta
			cluster.meanSize += probe.payloadSize
			cluster.count++
		}
		prevSendTime = int64(probe.sendTime)
		prevRecvTime = probe.recvTime
	}
	clusters = r.maybeAddCluster(cluster, clusters)
	return clusters
}

func (r *receiver) findBestProbe(clusters []Cluster) *Cluster {
	var highestProbeBitrate Bitrate = 0
	var best *Cluster
	for i := range clusters {
		cluster := clusters[i]
		if cluster.sendMean == 0 || cluster.recvMean == 0 {
			continue
		}
		if cluster.numAboveMinDelta > cluster.count/2 && cluster.recvMean-cluster.sendMean <= 2 && cluster.sendMean-cluster.recvMean <= 5 {
			probeBitrate := cluster.SendBitrate()
			if probeBitrate > cluster.RecvBitrate() {
				probeBitrate = cluster.RecvBitrate()
			}
			if probeBitrate > highestProbeBitrate {
				highestProbeBitrate = probeBitrate
				best = &cluster
			}
		} else {
			break
		}
	}
	return best
}

func (r *receiver) isBitrateImproving(bitrate uint64) bool {
	initialProbe := !r.remoteRate.ValidEstimate() && bitrate > 0
	bitrateAboveEstimate := r.remoteRate.ValidEstimate() && bitrate > uint64(r.remoteRate.LatestEstimate())
	return initialProbe || bitrateAboveEstimate
}

func (r *receiver) isWithinClusterBounds(delta uint32, cluster Cluster) bool {
	if cluster.count == 0 {
		return true
	}
	clusterMean := cluster.sendMean / int64(cluster.count)
	return math.Abs(float64(int64(delta)-clusterMean)) < 3
}

func (r *receiver) maybeAddCluster(cluster Cluster, clusters []Cluster) []Cluster {
	if cluster.count < MinClusterSize || cluster.sendMean <= 0 || cluster.recvMean <= 0 {
		return clusters
	}
	newCluster := Cluster{
		numAboveMinDelta: cluster.numAboveMinDelta,
		recvMean:         cluster.recvMean / int64(cluster.count),
		sendMean:         cluster.sendMean / int64(cluster.count),
		meanSize:         cluster.meanSize / cluster.count,
		count:            cluster.count,
	}
	clusters = append(clusters, newCluster)
	return clusters
}

func (r *receiver) LatestEstimate() ([]int, uint64, bool) {
	ssrcs := make([]int, 0)
	var bitrateBps uint64
	var ok bool
	if !r.remoteRate.ValidEstimate() {
		return ssrcs, bitrateBps, ok
	}
	ok = true
	for k := range r.ssrcs {
		ssrcs = append(ssrcs, k)
	}
	if len(r.ssrcs) != 0 {
		bitrateBps = uint64(r.remoteRate.LatestEstimate())
	}
	return ssrcs, bitrateBps, ok
}

func (r *receiver) RemoveStream(ssrc int) {
	delete(r.ssrcs, ssrc)
}

func (r *receiver) Process() {
	// extend method, not found yet
}

type Cluster struct {
	numAboveMinDelta int
	recvMean         int64 // ms
	sendMean         int64 // ms
	meanSize         int   // byte
	count            int
}

func (c *Cluster) SendBitrate() Bitrate {
	return NewBitrate(int64(c.meanSize), Duration(c.sendMean)*Millisecond)
}

func (c *Cluster) RecvBitrate() Bitrate {
	return NewBitrate(int64(c.meanSize), Duration(c.recvMean)*Millisecond)
}
