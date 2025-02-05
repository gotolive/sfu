package peer

import (
	"time"

	"github.com/gotolive/sfu/rtc"
	"github.com/gotolive/sfu/rtc/codec"
	"github.com/gotolive/sfu/rtc/logger"
)

const (
	MaxExtraOffsetMs = 75
	MsOffset         = 33
)

type SwitchLayerMode int

const (
	// AutoSwitchLayer we will auto switch layer based on bandwidth, from lowest to preferred
	AutoSwitchLayer = SwitchLayerMode(1) // TODO buggy, can't switch for now.
	// ManualSwitchLayer we won't change the layer unless UpdateLayer() has been called, default highest layer.
	ManualSwitchLayer = SwitchLayerMode(0)
)

func newSimulcastConsumer(listener ConsumerListener, receiver *Receiver, options *SenderOption, stats *Stats) (Sender, error) {
	basicConsumer, err := newSender(listener, receiver, options, stats)
	if err != nil {
		return nil, err
	}
	basicConsumer.stats = stats
	consumer := &SimulcastConsumer{
		mode:                   options.SwitchMode,
		sender:                 basicConsumer,
		rtpSeqManager:          NewSeqManager(),
		currentSpatialLayer:    -1,
		targetSpatialLayer:     -1,
		mappedSsrcSpatialLayer: map[uint32]int{},
	}

	for _, v := range consumer.producerRTPStreams {
		logger.Error("stream order:", v.RID())
	}
	// default is the best layer
	consumer.preferredSpatialLayer = len(consumer.producerRTPStreams) - 1
	return consumer, nil
}

type SimulcastConsumer struct {
	*sender
	mode                         SwitchLayerMode
	mappedSsrcSpatialLayer       map[uint32]int // ssrc->layer index
	preferredSpatialLayer        int            // user selected, we try our best to fix it.
	rtpSeqManager                RTPSeqManager
	syncRequired                 bool
	keyFrameForTSOffsetRequested bool
	targetSpatialLayer           int // during change, target layer
	currentSpatialLayer          int // current layer index
	tsReferenceSpatialLayer      int // current layer index
	tsOffset                     uint32
	lastBweDowngradeAtMs         int64
}

func (s *SimulcastConsumer) UpdateLayer(layer int) {
	s.preferredSpatialLayer = layer
	s.mayChangeLayers(true)
}

func (s *SimulcastConsumer) ProducerRtcpSenderReport(stream ReceiverStream, first bool) {
	if first {
		currentStream := s.getProducerCurrentRTPStream()
		if currentStream == nil || currentStream.GetSenderReportNtpMs() <= 0 {
			return
		}
		if s.IsActive() {
			s.mayChangeLayers(false)
		}
	}
}

func (s *SimulcastConsumer) Kind() string {
	return RTPTypeSimulcast
}

func (s *SimulcastConsumer) SendRTPPacket(packet rtc.Packet) {
	if !s.IsActive() {
		return
	}
	if s.targetSpatialLayer == -1 {
		return
	}
	layer := s.getLayer(packet.SSRC())

	shouldSwitchLayer := false
	if s.currentSpatialLayer != s.targetSpatialLayer && layer == s.targetSpatialLayer {
		shouldSwitchLayer = true
		s.syncRequired = true
	} else if layer != s.currentSpatialLayer {
		return
	}

	if s.syncRequired && codec.CanBeKeyFrame(s.codec.EncoderName) && !packet.IsKeyFrame() {
		logger.Warn("drop packet for wait key frame", layer, s.currentSpatialLayer)
		return
	}
	if s.syncRequired {
		tsOffset, ok := s.calTimestampOffset(packet, layer, shouldSwitchLayer)
		if !ok {
			return
		}
		s.tsOffset = tsOffset
		s.rtpSeqManager.Sync(packet.SequenceNumber() - 1)
		s.syncRequired = false
		s.keyFrameForTSOffsetRequested = false
	}

	if shouldSwitchLayer {
		logger.Infof("switch layer from %d to %d", s.currentSpatialLayer, s.targetSpatialLayer)
		s.currentSpatialLayer = s.targetSpatialLayer
	}
	timestamp := packet.Timestamp() - s.tsOffset
	seq := s.rtpSeqManager.Input(packet.SequenceNumber())
	originSsrc := packet.SSRC()
	originSeq := packet.SequenceNumber()
	originTimestamp := packet.Timestamp()
	originPayloadType := packet.PayloadType()
	packet.SetSsrc(s.stream.SSRC)
	packet.SetSequenceNumber(seq)
	packet.SetTimestamp(timestamp)
	packet.SetPayloadType(s.codec.PayloadType)

	s.listener.sendRTPPacket(packet)
	packet.SetSsrc(originSsrc)
	packet.SetSequenceNumber(originSeq)
	packet.SetTimestamp(originTimestamp)
	packet.SetPayloadType(originPayloadType)
}

func (s *SimulcastConsumer) calTimestampOffset(packet rtc.Packet, layer int, shouldSwitchLayer bool) (uint32, bool) {
	var tsOffset uint32
	if layer != s.tsReferenceSpatialLayer {
		producerTSRTPStream := s.getProducerTSReferenceRTPStream()
		producerTargetRTPStream := s.getProducerTargetRTPStream()

		if producerTargetRTPStream.GetSenderReportNtpMs() <= 0 || producerTSRTPStream.GetSenderReportNtpMs() <= 0 {
			logger.Error("we could switch to a layer without ntp timestamp")
			return 0, false
		}

		ntpMs1 := producerTSRTPStream.GetSenderReportNtpMs()
		ts1 := producerTSRTPStream.GetSenderReportTS()
		ntpMs2 := producerTargetRTPStream.GetSenderReportNtpMs()
		ts2 := producerTargetRTPStream.GetSenderReportTS()

		diffMs := ntpMs2 - ntpMs1

		diffTS := diffMs * uint64(s.rtpStream.GetClockRate()/1000)
		newTS2 := ts2 - int64(diffTS)
		tsOffset = uint32(newTS2 - ts1)
	}
	if shouldSwitchLayer && packet.Timestamp()-tsOffset <= s.rtpStream.GetMaxPacketTS() {
		maxTSExtraOffset := MaxExtraOffsetMs * int64(s.rtpStream.GetClockRate()) / 1000
		tsExtraOffset := s.rtpStream.GetMaxPacketTS() - packet.Timestamp() + tsOffset
		if s.keyFrameForTSOffsetRequested {
			if int64(tsExtraOffset) > maxTSExtraOffset {
				tsExtraOffset = 1
			}
		} else if int64(tsExtraOffset) > maxTSExtraOffset {
			s.RequestKeyframe()
			s.keyFrameForTSOffsetRequested = true
			logger.Warn("drop packet for layer offset too much", layer, s.currentSpatialLayer)
			return 0, false
		} else if tsExtraOffset == 0 {
			tsExtraOffset = uint32(MsOffset * int64(s.rtpStream.GetClockRate()) / 1000)
		}
		if tsExtraOffset > 0 {
			tsOffset -= tsExtraOffset
		}
	}
	return tsOffset, true
}

func (s *SimulcastConsumer) TransportConnected() {
	s.transportConnected = true
	s.UserOnTransportConnected()
}

func (s *SimulcastConsumer) TransportDisconnected() {
	panic("implement me")
}

func (s *SimulcastConsumer) UserOnTransportConnected() {
	s.syncRequired = true
	s.keyFrameForTSOffsetRequested = false
	if s.IsActive() {
		s.mayChangeLayers(false)
	}
}

func (s *SimulcastConsumer) mayChangeLayers(force bool) {
	if spatialLayer, ok := s.recalculateTargetLayers(); ok {
		if s.mode == AutoSwitchLayer {
			if force {
				s.updateTargetLayers(spatialLayer, true)
			}
			if spatialLayer != s.targetSpatialLayer {
				logger.Error("try f", spatialLayer, s.targetSpatialLayer, force)
				s.listener.OnConsumerNeedBitrateChange(s)
			}
		} else {
			s.updateTargetLayers(spatialLayer, false)
		}
	}
}

func (s *SimulcastConsumer) recalculateTargetLayers() (int, bool) {
	// if we are in manual mode, just return the target layers
	newTargetSpatialLayer := -1
	for i := 0; i < len(s.producerRTPStreams); i++ {
		// not sure yet
		if !s.canSwitchToSpatialLayer(i) {
			continue
		}
		newTargetSpatialLayer = i
		if i >= s.preferredSpatialLayer {
			break
		}
	}

	return newTargetSpatialLayer,
		newTargetSpatialLayer != s.targetSpatialLayer
}

func (s *SimulcastConsumer) updateTargetLayers(spatialLayer int, force bool) {
	// only once, set the first layer as ts reference layer.
	if spatialLayer != -1 && s.tsReferenceSpatialLayer == -1 {
		s.tsReferenceSpatialLayer = spatialLayer
	}
	if spatialLayer == -1 {
		s.targetSpatialLayer = -1
		s.currentSpatialLayer = -1
		return
	}

	s.targetSpatialLayer = spatialLayer

	// we got downgrade for bwe reason
	if force && s.targetSpatialLayer < s.currentSpatialLayer {
		logger.Info("down layer due to bwe control", s.targetSpatialLayer, s.currentSpatialLayer)
		s.lastBweDowngradeAtMs = time.Now().UnixMilli()
	}

	if s.targetSpatialLayer != s.currentSpatialLayer {
		s.requestKeyframeForTargetLayer()
	}
}

// unless we don't have ts refer layer or the target layer must have a sender report for sync.
func (s *SimulcastConsumer) canSwitchToSpatialLayer(i int) bool {
	return s.tsReferenceSpatialLayer == -1 ||
		i == s.tsReferenceSpatialLayer ||
		(s.getProducerTSReferenceRTPStream().GetSenderReportNtpMs() > 0 && s.producerRTPStreams[i].GetSenderReportNtpMs() > 0)
}

func (s *SimulcastConsumer) getProducerTSReferenceRTPStream() ReceiverStream {
	if s.tsReferenceSpatialLayer == -1 {
		return nil
	}
	return s.producerRTPStreams[s.tsReferenceSpatialLayer]
}

func (s *SimulcastConsumer) getProducerTargetRTPStream() ReceiverStream {
	if s.targetSpatialLayer == -1 {
		return nil
	}
	return s.producerRTPStreams[s.targetSpatialLayer]
}

func (s *SimulcastConsumer) getProducerCurrentRTPStream() ReceiverStream {
	if s.currentSpatialLayer == -1 {
		return nil
	}
	return s.producerRTPStreams[s.currentSpatialLayer]
}

func (s *SimulcastConsumer) RequestKeyframe() {
	if s.IsActive() {
		if s.mediaType != rtc.MediaTypeVideo {
			return
		}
		currentRTPStream := s.getProducerCurrentRTPStream()
		if currentRTPStream == nil {
			return
		}
		s.receiver.RequestKeyFrame(currentRTPStream.SSRC())
	}
}

func (s *SimulcastConsumer) requestKeyframeForTargetLayer() {
	if s.IsActive() {
		if s.mediaType != rtc.MediaTypeVideo {
			return
		}
		stream := s.getProducerTargetRTPStream()
		s.receiver.RequestKeyFrame(stream.SSRC())
	}
}

func (s *SimulcastConsumer) getLayer(ssrc uint32) int {
	if _, ok := s.mappedSsrcSpatialLayer[ssrc]; !ok {
		for i, v := range s.producerRTPStreams {
			if ssrc == v.SSRC() {
				s.mappedSsrcSpatialLayer[ssrc] = i
				break
			}
		}
	}
	return s.mappedSsrcSpatialLayer[ssrc]
}

func (s *SimulcastConsumer) GetBitrate(layer int) int64 {
	now := time.Now().UnixMilli()
	if layer > s.preferredSpatialLayer {
		// we dont need bitrate anymore
		return 0
	}

	if layer == 0 {
		return s.producerRTPStreams[0].Stats().ReceiveBPS(now)
	}

	return s.producerRTPStreams[layer].Stats().ReceiveBPS(now) - s.producerRTPStreams[layer-1].Stats().ReceiveBPS(now)
}
