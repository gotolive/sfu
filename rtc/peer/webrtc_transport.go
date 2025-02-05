package peer

import (
	"io"
	"log"

	"github.com/gotolive/sfu/rtc"
	"github.com/gotolive/sfu/rtc/dtls"
	"github.com/gotolive/sfu/rtc/ice"
	"github.com/gotolive/sfu/rtc/logger"
	"github.com/pion/rtcp"
)

var _ Transport = new(webRTCTransport)

// NewWebRTCTransport is a webrtc implementation of peer.Transport, support ice, dtls, srtp.
func NewWebRTCTransport(options *WebRTCOption, iceServer *ice.Server, cm dtls.CertificateGenerator) (Transport, error) {
	transport := &webRTCTransport{
		buffer:       make(rtc.CowBuffer, 1500),
		sendBuffer:   make([]byte, 1500),
		sendChan:     make(chan []byte, 100),
		sendRtcpChan: make(chan []byte, 100),
		closeCh:      make(chan struct{}),
		dtlsRole:     dtls.Actpass,
		packet:       new(rtpPacket),
	}

	ufrag := RandomString(4)

	iceTransport, err := iceServer.NewTransport(ufrag, RandomString(24), options.ListenIPs, transport.onIceData, transport.onIceState)
	if err != nil {
		return nil, err
	}

	transport.iceTransport = iceTransport

	transport.pipeR, transport.pipeW = io.Pipe()

	transport.dtlsTransport = dtls.NewDtlsTransport(dtls.Option{
		Certificate:  cm.GenerateCertificate(),
		Reader:       transport.pipeR,
		Writer:       transport.iceTransport,
		Role:         options.DtlsOption.Role,
		OnState:      transport.OnState,
		Fingerprints: options.DtlsOption.Fingerprints,
	})
	go transport.sendInternal()

	return transport, nil
}

type webRTCTransport struct {
	pipeW         *io.PipeWriter
	pipeR         *io.PipeReader
	connection    *Connection
	dtlsTransport *dtls.Transport
	dtlsRole      string
	packet        rtc.Packet
	buffer        rtc.CowBuffer
	srtpSession   *dtls.SrtpSession
	iceTransport  ice.Transport
	sendChan      chan []byte
	sendBuffer    []byte
	sendRtcpChan  chan []byte
	closeCh       chan struct{}
}

func (t *webRTCTransport) Close() {
	t.pipeW.Close()
	t.iceTransport.Close()
}

func (t *webRTCTransport) SetConnection(connection *Connection) {
	t.connection = connection
}

func (t *webRTCTransport) SendRTPPacket(packet rtc.Packet) {
	if !t.IsConnected() || t.srtpSession == nil {
		return
	}
	raw, err := packet.Marshal()
	if err != nil {
		log.Println("marshal rtp fail:", err)
		return
	}
	t.sendChan <- raw
}

func (t *webRTCTransport) IsConnected() bool {
	return t.dtlsTransport.GetState() == dtls.Connected
}

func (t *webRTCTransport) onIceData(data []byte) {
	switch CheckPacket(data) {
	case RTP:
		t.onRTPDataReceived(data)
	case RTCP:
		t.onRtcpDataReceived(data)
	case DTLS:
		// it will read and process in another goroutine, we need copy it.
		c := rtc.CowBuffer(data).Copy()
		_, _ = t.pipeW.Write(c)
	default:
		// error
	}
}

func (t *webRTCTransport) onRTPDataReceived(data []byte) {
	if t.dtlsTransport.GetState() != dtls.Connected {
		log.Println("dtls connecting ignore rtp")
		return
	}
	if t.srtpSession == nil {
		log.Println("no srtp session")
		return
	}

	if d, err := t.srtpSession.DecryptSrtp(t.buffer, data); err != nil {
		log.Println("decode srtp fail:", err)
	} else {
		if e := t.packet.Parse(d); e != nil {
			log.Println("parse rtp fail:", err)
		} else {
			t.connection.receiveRTPPacket(t.packet)
		}
	}
}

func (t *webRTCTransport) onRtcpDataReceived(data []byte) {
	if t.dtlsTransport.GetState() != dtls.Connected {
		log.Println("dtls connecting ignore rtp")
		return
	}
	if t.srtpSession == nil {
		log.Println("no srtp session")
		return
	}

	if d, err := t.srtpSession.DecryptSrtcp(t.buffer, data); err != nil {
		log.Println("decode srtp fail:", err, err)
	} else {
		if p, e := rtcp.Unmarshal(d); e != nil {
			log.Println("parse rtcp fail:", err)
		} else {
			t.connection.receiveRtcpPacket(p)
		}
	}
}

type WebRTCOption struct {
	ID         string
	ListenIPs  []string
	DtlsOption dtls.Option
	BweType    string
}

func (t *webRTCTransport) Info() TransportInfo {
	p := t.iceTransport.Parameters()
	return TransportInfo{
		ID: t.connection.ID(),
		IceInfo: struct {
			Role       string
			Candidates []ice.Candidate
			Ufrag      string
			Pwd        string
			Lite       bool
		}{
			Role:       p.Role,
			Candidates: p.Candidates,
			Ufrag:      p.UsernameFragment,
			Pwd:        p.Password,
			Lite:       p.Lite,
		},
		DtlsInfo: struct {
			Fingerprints []dtls.Fingerprint
			Role         string
		}{
			Fingerprints: t.dtlsTransport.GetLocalFingerprints(),
			Role:         t.dtlsTransport.Role(),
		},
	}
}

func (t *webRTCTransport) SendRtcpPacket(packet rtcp.Packet) {
	if !t.IsConnected() || t.srtpSession == nil {
		return
	}

	raw, err := packet.Marshal()
	if err != nil {
		return
	}
	t.sendRtcpChan <- raw
}

func (t *webRTCTransport) sendInternal() {
	for {
		select {
		case raw := <-t.sendChan:
			data, _, err := t.srtpSession.EncryptRtp(t.sendBuffer, raw)
			if err != nil {
				log.Println("encrypt rtp fail:", err)
			}
			_, err = t.iceTransport.Write(data)
			if err != nil {
				log.Println("write rtp fail:", err)
				return
			}
		case raw := <-t.sendRtcpChan:
			data, _, err := t.srtpSession.EncryptRtcp(t.sendBuffer, raw)
			if err != nil {
				return
			}
			_, err = t.iceTransport.Write(data)
			if err != nil {
				logger.Error("write rtcp fail:", err)
				return
			}
		case <-t.closeCh:
			return
		}
	}
}

func (t *webRTCTransport) OnState(state int) {
	logger.Debug("dtls state change:", t.connection.ID(), state)
	switch state {
	case dtls.Connecting:
	case dtls.Connected:
		var err error
		t.srtpSession, err = dtls.NewSrtpSession(t.dtlsTransport)
		if err != nil {
			logger.Error("create srtp fail:", err)
		}
		t.connection.Connected()
	case dtls.Failed:
		t.iceTransport.Close()
	}
}

func (t *webRTCTransport) onIceState(state ice.ConnectionState) {
	logger.Debug("ice state change:", t.connection.ID(), state)
	switch state {
	case ice.ConnectionConnected:
		t.dtlsTransport.TryRun()
		if t.dtlsTransport.GetState() == dtls.Connected {
			t.connection.Connected()
		}
	case ice.ConnectionCompleted:
		t.dtlsTransport.TryRun()
		if t.dtlsTransport.GetState() == dtls.Connected {
			t.connection.Connected()
		}
	case ice.ConnectionDisconnected:
		if t.dtlsTransport.GetState() == dtls.Connected {
			t.pipeW.Close()
			t.connection.Disconnected()
		}
		close(t.closeCh)
	case ice.ConnectionFailed:
		if t.dtlsTransport != nil && t.dtlsTransport.GetState() == dtls.Connected {
			t.pipeW.Close()
			t.connection.Disconnected()
		}
		close(t.closeCh)
	}
}
