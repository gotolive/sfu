package peer

import (
	"log"
	"sync"

	"github.com/gotolive/sfu/rtc"
	"github.com/gotolive/sfu/rtc/logger"
	"github.com/pion/rtcp"
)

var _ ReceiverStreamListener = new(Receiver)

type ReceiverListener interface {
	sendRtcpPacket(packet rtcp.Packet)
	removeReceiver(id string)
}

func newReceiver(options *ReceiverOption, l ReceiverListener, stats *Stats) (*Receiver, error) {
	if err := options.Validate(); err != nil {
		return nil, err
	}
	receiver := &Receiver{
		stats:             stats,
		mid:               options.MID,
		id:                options.ID,
		mediaType:         options.MediaType,
		ssrcRTPStreams:    map[uint32]ReceiverStream{},
		rtxSsrcRTPStreams: map[uint32]ReceiverStream{},
		// ridStreams: map[string]ReceiverStream,
		listener: l,
	}
	receiver.init(options)
	return receiver, nil
}

type Receiver struct {
	id                     string // it should be track-id from sdp, but any non-duplicated string is ok.
	mid                    string
	mediaType              string // video audio
	kind                   string // simple,simulcast,svc,pipe
	listener               ReceiverListener
	currentRTPPacket       rtc.Packet // used in NotifyNewRTPStream
	maxRtcpInterval        int64
	keyframeManager        *keyframeManager
	rtpStreamByEncodingIdx []ReceiverStream
	ssrcRTPStreams         map[uint32]ReceiverStream
	rtxSsrcRTPStreams      map[uint32]ReceiverStream
	lastRtcpSentTime       int64

	// codec will be used for create rtp_stream
	// in fact, there is no need to be more than one codec, only one codec is enough for sfu.
	// but another codec could used when set sender payload i guess?
	codec                 *Codec
	rtpHeaderExtensionIds rtc.HeaderExtensionIDs
	senders               sync.Map
	stats                 *Stats
}

func (r *Receiver) init(options *ReceiverOption) {
	// with receiver, it must supply rtp header extensions
	r.codec = options.Codec
	r.rtpHeaderExtensionIds = rtc.NewHerderExtensionIDs(options.HeaderExtensions)

	// simple,simulcast,svc
	r.kind = r.calType(options.Streams) // nobody read it. assume it should be used in conu
	r.rtpStreamByEncodingIdx = make([]ReceiverStream, len(options.Streams))
	// TBD inactive m-section

	if r.mediaType == rtc.MediaTypeAudio {
		r.maxRtcpInterval = rtc.MaxRTCPAudioInterval
	} else {
		r.maxRtcpInterval = rtc.MaxRTCPVideoInterval
	}
	if r.mediaType == rtc.MediaTypeVideo {
		r.keyframeManager = newKeyframeManager(options.KeyFrameRequestDelay, r.keyFrameNeeded)
	}

	for i, v := range options.Streams {
		r.rtpStreamByEncodingIdx[i] = r.createStream(v)
	}
}

func (r *Receiver) AddSender(s Sender) {
	r.senders.Store(s.ID(), s)
}

func (r *Receiver) OnRTPStreamNeedWorstRemoteFractionLost(ssrc uint32) uint8 {
	var wl uint8
	r.senders.Range(func(key, value any) bool {
		l := value.(Sender).FractionLost()
		if l > wl {
			wl = l
		}
		return true
	})
	return wl
}

func (r *Receiver) sendRtcp(packet rtcp.Packet) {
	r.listener.sendRtcpPacket(packet)
}

func (r *Receiver) keyFrameNeeded(ssrc uint32) {
	if stream, ok := r.ssrcRTPStreams[ssrc]; ok {
		stream.RequestKeyFrame()
	}
}

func (r *Receiver) GetRTPHeaderExtensionIds() rtc.HeaderExtensionIDs {
	return r.rtpHeaderExtensionIds
}

func (r *Receiver) ReceiveRTPPacket(packet rtc.Packet) string {
	if packet.PayloadLength() == 0 {
		// Some rtx probe rtpPacket will have zero payload, and it could before rtp
		// We simply drop it.
		return ReceiveRTPPacketDiscarded
	}
	r.currentRTPPacket = nil
	numRTPStreamsBefore := len(r.ssrcRTPStreams)

	rtpStream := r.GetRTPStream(packet)
	if rtpStream == nil {
		log.Println("cant find stream for rtpPacket:", packet.SSRC())
		return ReceiveRTPPacketDiscarded
	}
	packet.SetHeaderExtensionIDs(r.rtpHeaderExtensionIds)
	var result string
	// var isRtx bool
	switch packet.SSRC() {
	case rtpStream.SSRC():
		result = ReceiveRTPPacketMedia
		if err := rtpStream.ReceivePacket(packet); err != nil {
			if len(r.ssrcRTPStreams) > numRTPStreamsBefore {
				return result
			}
		}
	case rtpStream.RtxSSRC():
		// log.Println("we received a rtx package")
		result = ReceiveRTPPacketRetransmission
		packet.SetRTX(true)
		if err := rtpStream.ReceivePacket(packet); err != nil {
			log.Println(err)
			return result
		}
	default:
		log.Println("we could not find the ssrc ")
		return ReceiveRTPPacketDiscarded
	}

	if packet.IsKeyFrame() {
		if r.keyframeManager != nil {
			r.keyframeManager.keyFrameReceived(packet.SSRC())
		}
	}
	if len(r.ssrcRTPStreams) > numRTPStreamsBefore {
		if r.keyframeManager != nil && !packet.IsKeyFrame() {
			r.keyframeManager.keyFrameNeeded(packet.SSRC())
		}
		r.currentRTPPacket = packet
	}
	r.sendPacket(packet)

	return result
}

func (r *Receiver) GetRTPStream(packet rtc.Packet) ReceiverStream {
	if s, ok := r.ssrcRTPStreams[packet.SSRC()]; ok {
		return s
	}
	if s, ok := r.rtxSsrcRTPStreams[packet.SSRC()]; ok {
		return s
	}
	s := r.findStreamForPacket(packet)
	if s != nil {
		if s.RtxSSRC() == packet.SSRC() || s.RtxPayloadType() == packet.PayloadType() {
			s.UpdateRtxSSRC(packet.SSRC())
			r.rtxSsrcRTPStreams[packet.SSRC()] = s
		} else {
			s.UpdateSSRC(packet.SSRC())
			r.ssrcRTPStreams[packet.SSRC()] = s
		}
	}
	return s
}

func (r *Receiver) findStreamForPacket(packet rtc.Packet) ReceiverStream {
	var (
		rid  = packet.Rid(r.rtpHeaderExtensionIds.Rid())
		rrid = packet.RRid(r.rtpHeaderExtensionIds.RRid())
		s    ReceiverStream
	)

	for _, r := range r.rtpStreamByEncodingIdx {
		if (r.SSRC() == packet.SSRC() || r.RtxSSRC() == packet.SSRC()) ||
			((rid != "" && r.RID() == rid) || (rrid != "" && r.RID() == rrid)) {
			s = r
			break
		}
	}
	// MID Basid
	if s == nil && len(r.rtpStreamByEncodingIdx) == 1 {
		if r.rtpStreamByEncodingIdx[0].SSRC() == 0 && r.rtpStreamByEncodingIdx[0].RID() == "" {
			if r.rtpStreamByEncodingIdx[0].PayloadType() == packet.PayloadType() || r.rtpStreamByEncodingIdx[0].RtxPayloadType() == packet.PayloadType() {
				s = r.rtpStreamByEncodingIdx[0]
			}
		}
	}

	return s
}

func (r *Receiver) GetRTPStreams() []ReceiverStream {
	streams := make([]ReceiverStream, 0, len(r.rtpStreamByEncodingIdx))
	streams = append(streams, r.rtpStreamByEncodingIdx...)
	return streams
}

func (r *Receiver) RequestKeyFrame(ssrc uint32) {
	if r.keyframeManager == nil {
		logger.Error(1)
		return
	}
	if r.currentRTPPacket != nil && r.currentRTPPacket.SSRC() == ssrc && r.currentRTPPacket.IsKeyFrame() {
		logger.Error(2)
		return
	}
	r.keyframeManager.keyFrameNeeded(ssrc)
}

func (r *Receiver) ReceiveRtcpSenderReport(report *rtcp.SenderReport) {
	stream := r.ssrcRTPStreams[report.SSRC]
	if stream != nil {
		first := stream.GetSenderReportNtpMs() == 0
		stream.ReceiveRtcpSenderReport(report)
		r.senders.Range(func(key, value any) bool {
			value.(Sender).ProducerRtcpSenderReport(stream, first)
			return true
		})
	}
}

func (r *Receiver) GetRtcp(ms int64) rtcp.Packet {
	result := new(rtcp.ReceiverReport)
	if float64(ms-r.lastRtcpSentTime)*1.15 < float64(r.maxRtcpInterval) {
		return nil
	}

	for _, v := range r.ssrcRTPStreams {
		report := v.GetRtcpReceiverReport()
		if report != nil {
			result.Reports = append(result.Reports, *report)
		}
		rtxReport := v.GetRtxReceiverReport()
		if rtxReport != nil {
			result.Reports = append(result.Reports, *rtxReport)
		}
	}
	return result
}

func (r *Receiver) HeaderExtensions() []rtc.HeaderExtension {
	return r.rtpHeaderExtensionIds.HeaderExtensions()
}

func (r *Receiver) Codec() *Codec {
	return r.codec
}

func (r *Receiver) calType(streams []StreamOption) string {
	if len(streams) == 0 {
		return RTPTypeNone
	}
	if len(streams) == 1 {
		if streams[0].ScalabilityMode != "" {
			return RTPTypeSVC
		}
	}
	if len(streams) > 1 {
		return RTPTypeSimulcast
	}
	return RTPTypeSimple
}

func (r *Receiver) MID() string {
	return r.mid
}

func (r *Receiver) MediaType() string {
	return r.mediaType
}

func (r *Receiver) ID() string {
	return r.id
}

func (r *Receiver) Kind() string {
	return r.kind
}

func (r *Receiver) createStream(s StreamOption) ReceiverStream {
	codec := r.codec
	stream := NewReceiverStream(r, r.mediaType, s, *r.codec)
	stream.SetRtx(codec.RTX, s.RTX)
	return stream
}

func (r *Receiver) sendPacket(packet rtc.Packet) {
	r.senders.Range(func(key, value any) bool {
		value.(Sender).SendRTPPacket(packet)
		return true
	})
}

func (r *Receiver) removeSender(id string) {
	r.senders.Delete(id)
}

func (r *Receiver) Close() {
	if r.keyframeManager != nil {
		r.keyframeManager.close()
	}
	for _, s := range r.rtpStreamByEncodingIdx {
		s.Close()
	}
	r.listener.removeReceiver(r.id)
	r.senders.Range(func(key, value any) bool {
		value.(Sender).Close()
		return true
	})
}
