package peer

import (
	"math/rand"
	"sync"
	"time"

	"github.com/gotolive/sfu/rtc"
	"github.com/gotolive/sfu/rtc/bwe"
	"github.com/gotolive/sfu/rtc/dtls"
	"github.com/gotolive/sfu/rtc/ice"
	"github.com/gotolive/sfu/rtc/logger"
	"github.com/pion/rtcp"
)

var (
	_ ConsumerListener = new(Connection)
	_ ReceiverListener = new(Connection)
)

type TransportInfo struct {
	ID      string
	IceInfo struct {
		Role       string
		Candidates []ice.Candidate
		Ufrag      string
		Pwd        string
		Lite       bool
	}
	DtlsInfo struct {
		Fingerprints []dtls.Fingerprint
		Role         string
	}
}

// Transport should respond for read and write pkt.
type Transport interface {
	SetConnection(connection *Connection)
	IsConnected() bool
	SendRTPPacket(packet rtc.Packet)
	SendRtcpPacket(packet rtcp.Packet)
	Info() TransportInfo
	Close()
}

// connectionListener  is cross-connection communication.
type connectionListener interface {
	removeConnection(id string)
	Connection(id string) *Connection
}

func newConnection(id, bwe string, transport Transport, l connectionListener) *Connection {
	t := Connection{
		id:            id,
		listener:      l,
		bweType:       bwe,
		transport:     transport,
		senders:       []Sender{},
		ssrcSenders:   map[uint32]Sender{},
		rtxSsrcSender: map[uint32]Sender{},
		rtpTable:      newRTPTable(),
		rtpHeaders:    map[string]rtc.HeaderExtensionID{},
		codec:         map[rtc.PayloadType]*Codec{},
		stats:         newStats(),
		closeCh:       make(chan struct{}),
	}
	transport.SetConnection(&t)
	return &t
}

type Connection struct {
	id        string
	transport Transport          // send & receive data
	rtpTable  *rtpTable          // matching rtp->receiver
	listener  connectionListener // callback
	//
	mutex     sync.Mutex
	receivers []*Receiver
	senders   []Sender
	//senders       map[string]Sender
	ssrcSenders   map[uint32]Sender
	rtxSsrcSender map[uint32]Sender
	// this will replace the header in producer and connection
	// we must maintain a uri->id mapping in connection
	rtpHeaders map[string]rtc.HeaderExtensionID
	codec      map[rtc.PayloadType]*Codec

	// allow user custom receiver and sender bwe type
	bweType     string
	bweReceiver bwe.Receiver // bwe receiver
	bweSender   bwe.Sender   // bwe sender

	stats *Stats

	initialAvailableOutgoingBitrate uint64
	maxIncomingBitrate              uint64
	minIncomingBitrate              uint64

	transportWideCcSeq int
	connected          bool

	onStateChange func(int)
	closeCh       chan struct{}
}

func (c *Connection) ID() string {
	return c.id
}

// this is a tbd way to do it.
func (c *Connection) OnStateChange(callback func(state int)) {
	c.onStateChange = callback
}

func (c *Connection) Transport() Transport {
	return c.transport
}

const (
	payloadBottom = rtc.PayloadType(100)
	payloadTop    = rtc.PayloadType(150)
)

const (
	headerBottom = rtc.HeaderExtensionID(1)
	headerTop    = rtc.HeaderExtensionID(20)
)

func (c *Connection) getCodec(codec *Codec) *Codec {
	for _, tc := range c.codec {
		if tc.Equal(codec) {
			return tc
		}
	}

	pt := payloadBottom
	for ; pt <= payloadTop; pt++ {
		if c.codec[pt] == nil {
			break
		}
	}
	codec.PayloadType = pt
	c.codec[pt] = codec
	return codec
}

func (c *Connection) updateCodecs(codec *Codec) error {
	// check it first, avoid partial update
	if c, ok := c.codec[codec.PayloadType]; ok {
		if !c.Equal(codec) {
			return ErrPayloadNotMatch
		}
	}
	if _, ok := c.codec[codec.RTX]; ok {
		return ErrRTXPayloadNotMatch
	}
	for _, c := range c.codec {
		if c.Equal(codec) {
			return ErrPayloadNotMatch
		}
	}
	c.codec[codec.PayloadType] = codec

	return nil
}

func (c *Connection) getHeaderExtensions(headers []rtc.HeaderExtension) rtc.HeaderExtensionIDs {
	var result []rtc.HeaderExtension
	bweHeader := false
	preHeader := make([]rtc.HeaderExtension, 0, len(headers)+1)
	for _, h := range headers {
		if (h.URI == rtc.HeaderExtensionAbsSendTime && c.bweType != bwe.Remb) ||
			(h.URI == rtc.HeaderExtensionTransportSequenceNumber && c.bweType != bwe.TransportCC) {
			continue
		}
		if h.URI == rtc.HeaderExtensionAbsSendTime || h.URI == rtc.HeaderExtensionTransportSequenceNumber {
			bweHeader = true
		}
		preHeader = append(preHeader, h)
	}
	if !bweHeader {
		switch c.bweType {
		case bwe.Remb:
			preHeader = append(preHeader, rtc.HeaderExtension{URI: rtc.HeaderExtensionAbsSendTime})
		case bwe.TransportCC:
			preHeader = append(preHeader, rtc.HeaderExtension{URI: rtc.HeaderExtensionTransportSequenceNumber})
		}
	}

	for _, h := range preHeader {
		// we already know the header
		if id, ok := c.rtpHeaders[h.URI]; ok {
			result = append(result, rtc.HeaderExtension{
				URI: h.URI,
				ID:  id,
			})
		} else {
			// default id may duplicate with other
			defaultID := c.generateHeaderID()
			result = append(result, rtc.HeaderExtension{
				URI: h.URI,
				ID:  defaultID,
			})
			c.rtpHeaders[h.URI] = defaultID
		}
	}
	return rtc.NewHerderExtensionIDs(result)
}

func (c *Connection) generateHeaderID() rtc.HeaderExtensionID {
	var defaultID rtc.HeaderExtensionID
	for i := headerBottom; i <= headerTop; i++ {
		exist := false
		for _, v := range c.rtpHeaders {
			if v == i {
				exist = true
				break
			}
		}
		if !exist {
			defaultID = i
			break
		}
	}
	return defaultID
}

func (c *Connection) updateHeaderExtensions(headers []rtc.HeaderExtension) error {
	for _, h := range headers {
		if id, ok := c.rtpHeaders[h.URI]; ok && id != h.ID {
			return ErrHeaderIDNotMatch
		}
		c.rtpHeaders[h.URI] = h.ID
	}

	return nil
}

func (c *Connection) NewReceiver(req *ReceiverOption) (*Receiver, error) {
	c.mutex.Lock()
	headers := make([]rtc.HeaderExtension, 0, len(req.HeaderExtensions))
	for _, h := range req.HeaderExtensions {
		if h.URI == rtc.HeaderExtensionAbsSendTime && c.bweType != bwe.Remb {
			continue
		}
		if h.URI == rtc.HeaderExtensionTransportSequenceNumber && c.bweType != bwe.TransportCC {
			continue
		}
		headers = append(headers, h)
	}
	req.HeaderExtensions = headers
	receiver, err := newReceiver(req, c, c.stats)
	if err != nil {
		return nil, err
	}
	// we need to make sure payload type in connection don't get duplicated.
	if err = c.updateCodecs(req.Codec); err != nil {
		return nil, err
	}

	if err = c.updateHeaderExtensions(req.HeaderExtensions); err != nil {
		return nil, err
	}

	err = c.rtpTable.AddProducer(receiver)
	if err != nil {
		return nil, err
	}
	// The SDP required order of contents, so we use slice rather than map.
	c.receivers = append(c.receivers, receiver)

	if c.bweReceiver == nil {
		switch c.bweType {
		case bwe.Remb:
			if c.rtpHeaders[rtc.HeaderExtensionAbsSendTime] != 0 {
				c.bweReceiver = bwe.NewReceiver(bwe.Remb, uint8(c.rtpHeaders[rtc.HeaderExtensionAbsSendTime]), c.sendRtcpPacket)
			}
		case bwe.TransportCC:
			if c.rtpHeaders[rtc.HeaderExtensionTransportSequenceNumber] != 0 {
				c.bweReceiver = bwe.NewReceiver(bwe.TransportCC, uint8(c.rtpHeaders[rtc.HeaderExtensionTransportSequenceNumber]), c.sendRtcpPacket)
			}
		}
	}

	if c.bweReceiver != nil {
		// t.bweReceiver.SetInitialOutgoingBitrate(t.initialAvailableOutgoingBitrate)
		c.bweReceiver.SetMaxIncomingBitrate(c.maxIncomingBitrate)
		c.bweReceiver.SetMinIncomingBitrate(c.minIncomingBitrate)
	}
	c.mutex.Unlock()

	return receiver, nil
}

func (c *Connection) Receivers() []*Receiver {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	s := make([]*Receiver, 0)
	s = append(s, c.receivers...)
	return s
}

func (c *Connection) removeReceiver(id string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	for i, v := range c.receivers {
		if v.id == id {
			c.rtpTable.RemoveProducer(v)
			c.receivers = append(c.receivers[:i], c.receivers[i+1:]...)
			return
		}
	}
}

func (c *Connection) NewSender(req *SenderOption) (Sender, error) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	if req.ID == "" {
		req.ID = RandomString(12)
	}
	conn := c.listener.Connection(req.ConnectionID)
	if conn == nil {
		return nil, ErrReceiverNotExist
	}
	receiver := conn.receiver(req.ReceiverID)
	if receiver == nil {
		return nil, ErrReceiverNotExist
	}
	sender, err := NewSender(req, c, receiver, c.stats)
	if err != nil {
		return nil, err
	}
	c.senders = append(c.senders, sender)
	stream := sender.Stream()
	c.ssrcSenders[stream.SSRC] = sender
	c.rtxSsrcSender[stream.RTX] = sender
	if c.bweSender == nil {
		switch c.bweType {
		case bwe.Remb:
			if c.rtpHeaders[rtc.HeaderExtensionAbsSendTime] != 0 {
				c.bweSender = bwe.NewSender(bwe.Remb, c.initialAvailableOutgoingBitrate)
			}
		case bwe.TransportCC:
			if c.rtpHeaders[rtc.HeaderExtensionTransportSequenceNumber] != 0 {
				c.bweSender = bwe.NewSender(bwe.TransportCC, c.initialAvailableOutgoingBitrate)
			}
		}
	}
	if c.bweSender != nil {
		sender.SetExternallyManagedBitrate()
	}

	if c.transport.IsConnected() {
		sender.TransportConnected()
	}
	return sender, nil
}

func (c *Connection) Senders() []Sender {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	s := make([]Sender, 0)
	for _, v := range c.senders {
		s = append(s, v)
	}
	return s
}

func (c *Connection) removeSender(id string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	for i, v := range c.senders {
		if v.ID() == id {
			c.senders = append(c.senders[:i], c.senders[i+1:]...)
			delete(c.ssrcSenders, v.Stream().SSRC)
			delete(c.rtxSsrcSender, v.Stream().RTX)
			break
		}

	}
}

func (c *Connection) getSenderBySSRC(ssrc uint32) Sender {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	return c.ssrcSenders[ssrc]
}

// receiveRTPPacket process incoming rtp packet and update stats.
func (c *Connection) receiveRTPPacket(packet rtc.Packet) {
	if c.bweReceiver != nil {
		c.bweReceiver.IncomingPacket(time.Now().UnixMilli(), packet)
	}
	c.stats.IncomingRTP(packet)
	producer := c.rtpTable.GetProducer(packet, c.rtpHeaders)
	if producer == nil {
		logger.Warn("cant find producer for rtpPacket:", packet)
		return
	}
	result := producer.ReceiveRTPPacket(packet)
	switch result {
	case ReceiveRTPPacketDiscarded:
	case ReceiveRTPPacketMedia:
	case ReceiveRTPPacketRetransmission:
	}
}

// receiveRtcpPacket process incoming rtcp packet and dispatch it.
func (c *Connection) receiveRtcpPacket(p []rtcp.Packet) {
	for _, packet := range p {
		c.handleRtcpPacket(packet)
	}
}

func (c *Connection) handleRtcpPacket(p rtcp.Packet) {
	p.DestinationSSRC()
	switch report := p.(type) {
	// sender
	case *rtcp.ReceiverReport:
		c.handleRtcpRR(report)
	case *rtcp.PictureLossIndication, *rtcp.FullIntraRequest:
		c.handleRtcpKeyframe(report)
	case *rtcp.TransportLayerNack:
		c.handleRtcpNack(report)
	case *rtcp.TransportLayerCC:
		if c.bweSender != nil && c.bweSender.BweType() == bwe.TransportCC {
			c.bweSender.ReceiveRTCP(report)
		}
	case *rtcp.ReceiverEstimatedMaximumBitrate:
		if c.bweSender != nil && c.bweSender.BweType() == bwe.Remb {
			c.bweSender.ReceiveRTCP(report)
		}
	// receiver
	case *rtcp.SenderReport:
		producer := c.rtpTable.GetProducerBySsrc(report.SSRC)
		if producer != nil {
			producer.ReceiveRtcpSenderReport(report)
		}
	case *rtcp.SourceDescription:
	// do nothing
	default:
		// It may receive goodbye
		logger.Warn("unknown rtcp packet:", p)
	}
}

func (c *Connection) handleRtcpNack(report *rtcp.TransportLayerNack) {
	consumer := c.getSenderBySSRC(report.MediaSSRC)
	if consumer != nil && report.MediaSSRC != RTPProbationSsrc {
		consumer.ReceiveNack(report)
	}
}

func (c *Connection) handleRtcpKeyframe(report rtcp.Packet) {
	for _, ssrc := range report.DestinationSSRC() {
		consumer := c.getSenderBySSRC(ssrc)

		if consumer == nil && ssrc != RTPProbationSsrc {
			logger.Warn("unknown key frame request:", ssrc, report)
		} else {
			consumer.RequestKeyframe()
		}
	}
}

func (c *Connection) handleRtcpRR(report *rtcp.ReceiverReport) {
	for _, rr := range report.Reports {
		consumer := c.getSenderBySSRC(rr.SSRC)
		if consumer == nil && rr.SSRC != RTPProbationSsrc {
			// this could be rtx's RR, which we don't care
			// log.Println("err: consumer is nil", p)
		} else {
			consumer.ReceiveRtcpReceiverReport(rr)
		}
	}
}

func (c *Connection) sendRTPPacket(packet rtc.Packet) {
	packet.UpdateAbsSendTime(c.rtpHeaders[rtc.HeaderExtensionAbsSendTime], time.Now())
	if !packet.IsRTX() {
		if c.bweSender != nil && c.bweSender.BweType() == bwe.TransportCC && packet.UpdateTransportWideCc01(c.transportWideCcSeq+1) {
			c.transportWideCcSeq++
		}
	}
	c.stats.OutcomePacket(packet)
	c.transport.SendRTPPacket(packet)
}

func (c *Connection) sendRtcpPacket(packet rtcp.Packet) {
	c.transport.SendRtcpPacket(packet)
}

func (c *Connection) OnConsumerNeedBitrateChange(s Sender) {
	var bitrate int64
	bitrate = int64(c.bweSender.EstimateBitrate())
	simulcast := map[Sender]int{}

	for _, c := range c.Senders() {
		if c.Kind() == RTPTypeSimulcast {
			simulcast[c] = -1
		} else {
			bitrate -= c.GetBitrate(0)
		}
	}
	logger.Info("left for simulcast bitrate:", bitrate)
	if len(simulcast) == 0 {
		return
	}

	for bitrate > 0 {
		before := bitrate
		for c, l := range simulcast {
			l++
			expected := c.GetBitrate(l)
			logger.Error("expected:", expected, "bitrate:", bitrate)
			// IT will never update to higher, the expected is always higher than current.
			if expected >= bitrate {
				simulcast[c] = l
				bitrate = 0
				break
			}
			bitrate -= expected
			simulcast[c] = l
		}
		if before == bitrate {
			// nobody want bitrate anymore
			break
		}
	}
	for c, l := range simulcast {
		if l < 0 {
			l = 0
		}
		logger.Info("update sender layer to ", c, l)
		c.updateTargetLayers(l, true)
	}
}

func (c *Connection) Close() {
	c.listener.removeConnection(c.id)
	for _, r := range c.Receivers() {
		r.Close()
	}
	for _, s := range c.Senders() {
		s.Close()
	}
	c.transport.Close()
}

func (c *Connection) Connected() {
	if !c.connected {
		c.connected = true
		// it will be called more than once, make sure its ok
		for _, s := range c.Senders() {
			s.TransportConnected()
		}
		go c.loop()
		if c.onStateChange != nil {
			c.onStateChange(1)
		}
	}
}

// Disconnected We could restart ice and keep the connection, that's the disconnected do,
// but we do not support it yet.
func (c *Connection) Disconnected() {
	if c.onStateChange != nil {
		c.onStateChange(2)
	}
	close(c.closeCh)
	for _, r := range c.Receivers() {
		r.Close()
	}
	for _, s := range c.senders {
		s.TransportDisconnected()
	}
}

func (c *Connection) loop() {
	ticker := time.NewTicker(time.Millisecond * 200)
	lastSent := time.Now().UnixMilli()
	// we expected a range [200,2000] ms to send rtcp.
	for {
		select {
		case now := <-ticker.C:
			if now.UnixMilli()-lastSent > 2000 {
				c.onTimeSendRTCP(now.UnixMilli())
			}
			if rand.Int()%10 == 0 {
				c.onTimeSendRTCP(now.UnixMilli())
			}
		case <-c.closeCh:
			return
		}
	}
}

func (c *Connection) onTimeSendRTCP(ms int64) {
	// We sent it separately, avoid MTU size.
	for _, s := range c.senders {
		packet := s.GetRtcp(ms)
		if packet != nil {
			c.sendRtcpPacket(packet)
		}
	}
	for _, p := range c.receivers {
		packet := p.GetRtcp(ms)
		if packet != nil {
			c.sendRtcpPacket(packet)
		}
	}
}

func (c *Connection) Stats() *Stats {
	return c.stats
}

func (c *Connection) receiver(id string) *Receiver {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	for _, v := range c.receivers {
		if v.id == id {
			return v
		}
	}
	return nil
}
