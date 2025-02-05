package peer

import (
	"time"

	"github.com/gotolive/sfu/rtc"
	"github.com/gotolive/sfu/rtc/codec"
	"github.com/pion/rtcp"
	"github.com/pion/rtp"
)

type ConsumerListener interface {
	sendRTPPacket(packet rtc.Packet)
	OnConsumerNeedBitrateChange(s Sender)
	getHeaderExtensions(headers []rtc.HeaderExtension) rtc.HeaderExtensionIDs
	getCodec(codecs *Codec) *Codec
	updateCodecs(codec *Codec) error
	updateHeaderExtensions(headers []rtc.HeaderExtension) error
	removeSender(id string)
}

type Sender interface {
	ID() string
	SendRTPPacket(packet rtc.Packet)
	SetExternallyManagedBitrate()
	TransportConnected()
	MediaType() string
	TransportDisconnected()
	ProducerRtcpSenderReport(stream ReceiverStream, first bool)
	ReceiveRtcpReceiverReport(report rtcp.ReceptionReport)
	GetRtcp(ms int64) rtcp.Packet
	ReceiveNack(report *rtcp.TransportLayerNack)
	RequestKeyframe()
	OnRTPStreamRetransmitRTPPacket(packet rtc.Packet)
	FractionLost() uint8
	UpdateLayer(layer int)
	HeaderExtensions() []rtc.HeaderExtension
	Codec() *Codec
	MID() string
	Stream() *StreamOption
	ReceiverID() string
	Close()
	Kind() string
	GetBitrate(layer int) int64
	updateTargetLayers(i int, force bool)
	increaseLayer(bitrate int64) int64
}

func NewSender(options *SenderOption, listener ConsumerListener, receiver *Receiver, stats *Stats) (Sender, error) {
	var s Sender
	var err error
	switch receiver.Kind() {
	case RTPTypeSimple:
		s, err = newSender(listener, receiver, options, stats)
	case RTPTypeSimulcast:
		s, err = newSimulcastConsumer(listener, receiver, options, stats)
	}
	if err != nil {
		return nil, err
	}
	receiver.AddSender(s)
	return s, nil
}

func newSender(listener ConsumerListener, receiver *Receiver, options *SenderOption, stats *Stats) (*sender, error) {
	c := &sender{
		mid:                   options.MID,
		id:                    options.ID,
		rtpHeaderExtensionIds: make(map[string]rtc.HeaderExtension),
		listener:              listener,
		receiverID:            options.ReceiverID,
		receiver:              receiver,
		headerMap:             make(map[rtc.HeaderExtensionID]rtc.HeaderExtensionID),
	}

	c.mediaType = receiver.MediaType()
	headers := options.HeaderExtensions
	if len(headers) != 0 {
		err := listener.updateHeaderExtensions(headers)
		if err != nil {
			return nil, err
		}
	} else {
		headers = receiver.HeaderExtensions()
	}
	c.rtpHeaderExtensionIds = listener.getHeaderExtensions(headers)
	for _, h := range headers {
		c.headerMap[h.ID] = c.rtpHeaderExtensionIds[h.URI].ID
	}
	codecs := options.Codec
	if codecs != nil {
		err := listener.updateCodecs(codecs)
		if err != nil {
			return nil, err
		}
	} else {
		codecs = receiver.Codec()
	}

	c.codec = listener.getCodec(codecs)
	streams := receiver.GetRTPStreams()

	c.stream = &StreamOption{
		SSRC:        rtc.GenerateSSRC(),
		PayloadType: c.codec.PayloadType,
	}
	for _, stream := range streams {
		if stream.RtxSSRC() != 0 || stream.RtxPayloadType() != 0 {
			c.stream.RTX = rtc.GenerateSSRC()
			c.stream.Cname = stream.Cname()
		}
	}

	if c.mediaType == "audio" {
		c.maxRtcpInterval = rtc.MaxRTCPAudioInterval
	} else {
		c.maxRtcpInterval = rtc.MaxRTCPVideoInterval
	}
	c.stats = stats
	err := c.CreateRTPStream()
	if err != nil {
		return nil, err
	}
	ps := receiver.GetRTPStreams()
	// reverse the stream, make it order from lower->higher.
	for i := len(ps) - 1; i >= 0; i-- {
		c.producerRTPStreams = append(c.producerRTPStreams, ps[i])
	}
	return c, nil
}

type sender struct {
	id                    string
	receiver              *Receiver
	receiverID            string
	listener              ConsumerListener
	mediaType             string
	mid                   string
	stream                *StreamOption
	rtpHeaderExtensionIds rtc.HeaderExtensionIDs
	codec                 *Codec
	rtpStream             SenderStream

	maxRtcpInterval          int
	transportConnected       bool
	producerClosed           bool
	externallyManagedBitrate bool
	stats                    *Stats
	lastRtcpSentTime         int64
	syncRequired             bool
	producerRTPStreams       []ReceiverStream
	headerMap                map[rtc.HeaderExtensionID]rtc.HeaderExtensionID
}

func (s *sender) GetBitrate(_ int) int64 {
	return s.producerRTPStreams[0].Stats().ReceiveBPS(time.Now().UnixMilli())
}

func (s *sender) updateTargetLayers(i int, force bool) {
}

func (s *sender) increaseLayer(bitrate int64) int64 {
	return 0
}

func (s *sender) Kind() string {
	return RTPTypeSimple
}

func (s *sender) Codec() *Codec {
	return s.codec
}

func (s *sender) Close() {
	s.listener.removeSender(s.ID())
	s.receiver.removeSender(s.ID())
}

func (s *sender) MID() string {
	return s.mid
}

func (s *sender) Stream() *StreamOption {
	return s.stream
}

func (s *sender) HeaderExtensions() []rtc.HeaderExtension {
	return s.rtpHeaderExtensionIds.HeaderExtensions()
}

func (s *sender) SetExternallyManagedBitrate() {
	s.externallyManagedBitrate = true
}

func (s *sender) FractionLost() uint8 {
	if !s.IsActive() {
		return 0
	}
	return s.rtpStream.FractionLost()
}

func (s *sender) OnRTPStreamRetransmitRTPPacket(packet rtc.Packet) {
	s.listener.sendRTPPacket(packet)
}

func (s *sender) ReceiveNack(report *rtcp.TransportLayerNack) {
	if !s.IsActive() {
		return
	}
	s.rtpStream.ReceiveNack(report)
}

func (s *sender) GetRtcp(ms int64) rtcp.Packet {
	packet := rtcp.CompoundPacket{}
	if (float64(ms)-float64(s.lastRtcpSentTime))*1.15 < float64(s.maxRtcpInterval) {
		return nil
	}

	report := s.rtpStream.GetRtcpSenderReport(ms)
	if report == nil {
		return nil
	}
	packet = append(packet, report)
	sdes := s.rtpStream.GetRtcpSdesChunk()
	packet = append(packet, sdes)
	s.lastRtcpSentTime = ms
	return &packet
}

func (s *sender) ReceiveRtcpReceiverReport(report rtcp.ReceptionReport) {
	s.rtpStream.ReceiveRtcpReceiverReport(report)
}

func (s *sender) ReceiverID() string {
	return s.receiverID
}

func (s *sender) MediaType() string {
	return s.mediaType
}

func (s *sender) ID() string {
	return s.id
}

func (s *sender) CreateRTPStream() error {
	s.rtpStream = NewSenderStream(s, s.mediaType, *s.stream, *s.codec)
	return nil
}

func (s *sender) IsActive() bool {
	return s.transportConnected && !s.producerClosed
}

func (s *sender) UserOnTransportConnected() {
	s.syncRequired = true
	if s.IsActive() {
		s.RequestKeyframe()
	}
}

func (s *sender) RequestKeyframe() {
	if s.IsActive() {
		if s.mediaType != rtc.MediaTypeVideo {
			return
		}
		if s.producerRTPStreams[0].SSRC() != 0 {
			// if ssrc not exists yet.
			s.receiver.RequestKeyFrame(s.producerRTPStreams[0].SSRC())
		}
	}
}

func (s *sender) SendRTPPacket(packet rtc.Packet) {
	if !s.IsActive() {
		return
	}
	if s.syncRequired && codec.CanBeKeyFrame(s.codec.EncoderName) && !packet.IsKeyFrame() {
		return
	}

	if s.syncRequired {
		s.syncRequired = false
	}

	if err := s.rtpStream.ReceivePacket(packet); err != nil {
		return
	}

	originSsrc := packet.SSRC()
	originPayloadType := packet.PayloadType()
	originHeader := packet.HeaderExtensions()
	newHeaders := []rtp.Extension{}
	for _, e := range originHeader {
		newHeaders = append(newHeaders, rtp.Extension{
			ID:      uint8(s.headerMap[rtc.HeaderExtensionID(e.ID)]),
			Payload: e.Payload,
		})
	}

	packet.SetSsrc(s.stream.SSRC)
	packet.SetPayloadType(s.codec.PayloadType)
	packet.UpdateHeader(newHeaders)
	if s.mediaType == rtc.MediaTypeVideo {
		_ = packet
	}

	s.listener.sendRTPPacket(packet)
	packet.SetSsrc(originSsrc)
	packet.SetPayloadType(originPayloadType)
	packet.UpdateHeader(originHeader)
}

func (s *sender) TransportConnected() {
	s.transportConnected = true
	s.UserOnTransportConnected()
}

func (s *sender) TransportDisconnected() {
}

func (s *sender) ProducerRtcpSenderReport(stream ReceiverStream, first bool) {
	// do nothing
}

func (s *sender) UpdateLayer(layer int) {
	// do nothing
}
