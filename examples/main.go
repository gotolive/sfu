package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	_ "net/http/pprof"
	"strings"
	"sync"
	"time"

	"github.com/gotolive/sfu/examples/conference"
	"github.com/gotolive/sfu/rtc"
	"github.com/gotolive/sfu/rtc/bwe"
	"github.com/gotolive/sfu/rtc/dtls"
	"github.com/gotolive/sfu/rtc/ice"
	"github.com/gotolive/sfu/rtc/logger"
	"github.com/gotolive/sfu/rtc/peer"
	"github.com/gotolive/sfu/rtc/sdp"
)

var (
	listenAddress = ":8990"
)

// The order of m-line must match offer.
func generateSdp(kind string, t *peer.Connection) map[string]string {
	i := t.Transport().Info()
	bundleList := []string{}
	buf := new(bytes.Buffer)
	buf.WriteString("v=0\r\n" +
		"o=" + "-" + " " + fmt.Sprint(time.Now().UnixNano()) + " 0" + " IN IP4 127.0.0.1\r\n" +
		"s=-\r\n" +
		"t=0 0\r\n")
	buf.WriteString("a=extmap-allow-mixed\r\n" +
		"a=msid-semantic:WMS *\r\n" +
		"a=rtcp-mux\r\n" +
		"a=ice-lite\r\n" +
		"{{bundle-place-holder}}")

	for _, r := range t.Receivers() {
		bundleList = append(bundleList, r.MID())
		cp := []string{}
		cp = append(cp, fmt.Sprint(r.Codec().PayloadType))
		if r.Codec().RTX != 0 {
			cp = append(cp, fmt.Sprint(r.Codec().RTX))
		}
		buf.WriteString("m=" + r.MediaType() + " 9 UDP/TLS/RTP/SAVPF " + strings.Join(cp, " ") + "\r\n")
		buf.WriteString("c=IN IP4 0.0.0.0\r\n")
		buf.WriteString("a=rtcp:9 IN IP4 0.0.0.0\r\n")
		c := r.Codec()
		buf.WriteString(fmt.Sprintf("a=rtpmap:%d %s/%d/%d\r\n", c.PayloadType, c.EncoderName, c.ClockRate, c.Channels))

		tmp := []string{}
		for key, value := range c.Parameters {
			tmp = append(tmp, fmt.Sprintf("%s=%v", key, value))
		}
		buf.WriteString(fmt.Sprintf("a=fmtp:%d %s\r\n", c.PayloadType, strings.Join(tmp, ";")))
		for _, fb := range c.FeedbackParams {
			buf.WriteString(fmt.Sprintf("a=rtcp-fb:%d %s %v \r\n", c.PayloadType, fb.Type, fb.Parameter))
		}
		if c.RTX != 0 {
			buf.WriteString(fmt.Sprintf("a=rtpmap:%d %s/%d/%d\r\n", c.RTX, "RTX", c.ClockRate, c.Channels))
			buf.WriteString(fmt.Sprintf("a=fmtp:%d %s\r\n", c.RTX, fmt.Sprintf("apt=%d", c.PayloadType)))
		}

		buf.WriteString("a=rtcp-mux\r\n")
		for _, header := range r.HeaderExtensions() {
			buf.WriteString(fmt.Sprintf("a=extmap:%d %s\r\n", header.ID, header.URI))
		}
		buf.WriteString(fmt.Sprintf("a=mid:%s\r\n", r.MID()))
		buf.WriteString("a=recvonly\r\n")
		buf.WriteString(fmt.Sprintf("a=setup:%s\r\n", "active"))
		for _, f := range i.DtlsInfo.Fingerprints {
			buf.WriteString(fmt.Sprintf("a=fingerprint:%s %s\r\n", f.Algorithm, f.Value))
		}
		buf.WriteString(fmt.Sprintf("a=ice-ufrag:%s\r\n", i.IceInfo.Ufrag))
		buf.WriteString(fmt.Sprintf("a=ice-pwd:%s\r\n", i.IceInfo.Pwd))
		buf.WriteString("a=ice-lite\r\n")
		buf.WriteString("a=ice-options:trickle\r\n")
		for _, c := range i.IceInfo.Candidates {
			buf.WriteString(fmt.Sprintf("a=candidate:%s 1 %s %d %s %d typ %s \r\n", c.Foundation, c.Protocol, c.Priority, c.IP, c.Port, c.Type))
		}
		buf.WriteString("a=end-of-candidates\r\n")
		if len(r.GetRTPStreams()) > 1 {
			simulcast := []string{}
			for _, stream := range r.GetRTPStreams() {
				buf.WriteString(fmt.Sprintf("a=rid %s recv\r\n", stream.RID()))
				simulcast = append(simulcast, stream.RID())
			}
			buf.WriteString(fmt.Sprintf("a=simulcast:recv %s\r\n", strings.Join(simulcast, ";")))
		}

	}
	for _, r := range t.Senders() {

		bundleList = append(bundleList, r.MID())
		cp := []string{}

		codecs := r.Codec()
		cp = append(cp, fmt.Sprint(codecs.PayloadType))
		if codecs.RTX != 0 {
			cp = append(cp, fmt.Sprint(codecs.RTX))
		}
		buf.WriteString("m=" + r.MediaType() + " 9 UDP/TLS/RTP/SAVPF " + strings.Join(cp, " ") + "\r\n")
		buf.WriteString("c=IN IP4 0.0.0.0\r\n")
		buf.WriteString("a=rtcp:9 IN IP4 0.0.0.0\r\n")
		c := r.Codec()
		buf.WriteString(fmt.Sprintf("a=rtpmap:%d %s/%d/%d\r\n", c.PayloadType, c.EncoderName, c.ClockRate, c.Channels))

		tmp := []string{}
		for key, value := range c.Parameters {
			tmp = append(tmp, fmt.Sprintf("%s=%v", key, value))
		}
		buf.WriteString(fmt.Sprintf("a=fmtp:%d %s\r\n", c.PayloadType, strings.Join(tmp, ";")))
		for _, fb := range c.FeedbackParams {
			buf.WriteString(fmt.Sprintf("a=rtcp-fb:%d %s %v \r\n", c.PayloadType, fb.Type, fb.Parameter))
		}
		if c.RTX != 0 {
			buf.WriteString(fmt.Sprintf("a=rtpmap:%d %s/%d/%d\r\n", c.RTX, "RTX", c.ClockRate, c.Channels))
			buf.WriteString(fmt.Sprintf("a=fmtp:%d %s\r\n", c.RTX, fmt.Sprintf("apt=%d", c.PayloadType)))
		}
		buf.WriteString("a=rtcp-mux\r\n")
		for _, header := range r.HeaderExtensions() {
			buf.WriteString(fmt.Sprintf("a=extmap:%d %s\r\n", header.ID, header.URI))
		}
		buf.WriteString(fmt.Sprintf("a=mid:%s\r\n", r.MID()))
		buf.WriteString("a=sendonly\r\n")
		buf.WriteString(fmt.Sprintf("a=setup:%s\r\n", "active"))
		for _, f := range i.DtlsInfo.Fingerprints {
			buf.WriteString(fmt.Sprintf("a=fingerprint:%s %s\r\n", f.Algorithm, f.Value))
		}
		buf.WriteString(fmt.Sprintf("a=ice-ufrag:%s\r\n", i.IceInfo.Ufrag))
		buf.WriteString(fmt.Sprintf("a=ice-pwd:%s\r\n", i.IceInfo.Pwd))
		buf.WriteString("a=ice-lite\r\n")
		buf.WriteString("a=ice-options:trickle\r\n")
		for _, c := range i.IceInfo.Candidates {
			buf.WriteString(fmt.Sprintf("a=candidate:%s 1 %s %d %s %d typ %s \r\n", c.Foundation, c.Protocol, c.Priority, c.IP, c.Port, c.Type))
		}
		buf.WriteString("a=end-of-candidates\r\n")
		buf.WriteString(fmt.Sprintf("a=ssrc:%d cname:%s\r\n", r.Stream().SSRC, peer.RandomString(12)))

	}
	sdp := buf.String()
	sdp = strings.ReplaceAll(sdp, "{{bundle-place-holder}}", fmt.Sprintf("a=group:BUNDLE %s\r\n", strings.Join(bundleList, " ")))
	return map[string]string{
		"type": kind,
		"sdp":  sdp,
	}
}

type SDP struct {
	Type string
	SDP  string
}

var broker *peer.Broker
var sessionMap sync.Map

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	var err error
	broker, err = peer.NewBroker(peer.BrokerOption{
		ICE: ice.Option{
			EnableIPV6: true,
			EnableTCP:  true,
		},
	})
	if err != nil {
		log.Fatal("broker start fail:", err)
	}

	{
		http.HandleFunc("/pub", func(writer http.ResponseWriter, request *http.Request) {
			request.ParseForm()
			sessionId := request.FormValue("sessionId")
			requestBody, _ := io.ReadAll(request.Body)
			defer request.Body.Close()
			offer := SDP{}
			json.Unmarshal(requestBody, &offer)
			jsdp, err := sdp.Unmarshal(offer.SDP)
			if err != nil {
				logger.Error("err:", err)
				return
			}

			t, err := broker.NewWebRTCConnection(&peer.WebRTCOption{
				ID: sessionId,
				DtlsOption: dtls.Option{
					Role: dtls.Active,
				},
				BweType: bwe.Remb,
			})
			if err != nil {
				logger.Error("create connection fail:", err, sessionId)
				return
			}
			t.OnStateChange(func(state int) {
				if state == 2 {
					t.Close()
				}
			})

			for _, v := range jsdp.MediaDescription {
				options := getOptionsFromSdp(v)
				_, err := t.NewReceiver(options)
				if err != nil {
					log.Println(err)
				}
			}

			answer := generateSdp("answer", t)
			r, _ := json.Marshal(answer)
			sessionMap.Store(sessionId, t)
			writer.Write(r)
		})

		http.HandleFunc("/sub", func(writer http.ResponseWriter, request *http.Request) {
			t, err := broker.NewWebRTCConnection(&peer.WebRTCOption{
				DtlsOption: dtls.Option{
					Role: dtls.Active,
				},
				BweType: bwe.Remb,
			})
			if err != nil {
				logger.Error("create connection fail:", err)
				return
			}
			t.OnStateChange(func(state int) {
				if state == 2 {
					t.Close()
				}
			})
			request.ParseForm()
			defer request.Body.Close()
			sessionId := request.FormValue("sessionId")
			v, ok := sessionMap.Load(sessionId)
			if !ok {
				return
			}
			publisher := v.(*peer.Connection)
			for _, v := range publisher.Receivers() {
				op := &peer.SenderOption{
					ID:           v.ID() + peer.RandomString(12),
					ReceiverID:   v.ID(),
					ConnectionID: publisher.ID(),
					MID:          v.MID(),
					SwitchMode:   peer.ManualSwitchLayer,
				}
				t.NewSender(op)
			}
			sessionMap.Store(sessionId+"-sub", t)
			offer := generateSdp("offer", t)
			r, _ := json.Marshal(offer)
			writer.Write(r)
		})

		http.HandleFunc("/change", func(writer http.ResponseWriter, request *http.Request) {
			request.ParseForm()
			sessionId := request.FormValue("sessionId")
			v, ok := sessionMap.Load(sessionId + "-sub")
			if !ok {
				return
			}
			subscriber := v.(*peer.Connection)
			for _, s := range subscriber.Senders() {
				if s.MediaType() == "video" {
					switch request.FormValue("profile") {
					case "high":
						logger.Error("up high")
						s.UpdateLayer(2)
					case "medium":
						logger.Error("up mi")
						s.UpdateLayer(1)
					case "low":
						logger.Error("up low")
						s.UpdateLayer(0)

					}
				}
			}
			writer.Write([]byte("OK"))
		})
	}
	{
		//go func() {
		//	ticker := time.NewTicker(time.Second)
		//	for range ticker.C {
		//		var sendbit, receivebit, recceiveByte, sendByte int64
		//		connections := broker.Connections()
		//		now := time.Now().UnixMilli()
		//		for _, c := range connections {
		//			stats := c.Stats()
		//			sendbit += stats.SentBPS(now)
		//			sendByte += stats.BytesSend()
		//			recceiveByte += stats.BytesReceived()
		//			receivebit += stats.ReceiveBPS(now)
		//		}
		//		logger.Infof("Total Connection %d, ReceiveBPS: %v, SendBPS: %v, TotalReceiveByte: %d, TotalSendByte: %d", len(connections), remb.Bitrate(receivebit), remb.Bitrate(sendbit), recceiveByte, sendByte)
		//	}
		//}()
	}
	http.HandleFunc("/whip", whip)
	http.HandleFunc("/whep", whep)
	http.HandleFunc("/ws", conference.WSHandleFunc)
	http.Handle("/", http.FileServer(http.Dir("./examples/static")))
	if err := http.ListenAndServe(listenAddress, nil); err != nil {
		log.Fatal("Listen HTTP fail:", err)
	}
}

func whip(writer http.ResponseWriter, request *http.Request) {
	requestBody, _ := io.ReadAll(request.Body)
	defer request.Body.Close()
	jsdp, err := sdp.Unmarshal(string(requestBody))
	if err != nil {
		logger.Error("err:", err)
		return
	}

	t, err := broker.NewWebRTCConnection(&peer.WebRTCOption{
		ID: "whip",
		DtlsOption: dtls.Option{
			Role: dtls.Active,
		},
		BweType: bwe.Remb,
	})
	if err != nil {
		logger.Error("create connection fail:", err, "whip")
		return
	}
	t.OnStateChange(func(state int) {
		if state == 2 {
			t.Close()
		}
	})

	for _, v := range jsdp.MediaDescription {
		options := getOptionsFromSdp(v)
		_, err := t.NewReceiver(options)
		if err != nil {
			log.Println(err)
		}
	}

	answer := generateSdp("answer", t)
	sessionMap.Store("whip", t)
	writer.Write([]byte(answer["sdp"]))
}

func whep(writer http.ResponseWriter, request *http.Request) {
	requestBody, _ := io.ReadAll(request.Body)
	defer request.Body.Close()
	jsdp, err := sdp.Unmarshal(string(requestBody))
	if err != nil {
		logger.Error("err:", err)
		return
	}

	t, err := broker.NewWebRTCConnection(&peer.WebRTCOption{
		DtlsOption: dtls.Option{
			Role: dtls.Active,
		},
		BweType: bwe.Remb,
	})
	if err != nil {
		logger.Error("create connection fail:", err)
		return
	}
	t.OnStateChange(func(state int) {
		if state == 2 {
			t.Close()
		}
	})
	v, ok := sessionMap.Load("whip")
	if !ok {
		return
	}
	publisher := v.(*peer.Connection)
	var videoOptions, audioOptions *sdp.MediaDescription
	for _, v := range jsdp.MediaDescription {
		if v.MediaType == rtc.MediaTypeVideo {
			videoOptions = v
		} else {
			audioOptions = v
		}
	}

	for _, v := range publisher.Receivers() {
		var op *peer.SenderOption
		if v.MediaType() == rtc.MediaTypeVideo {
			op = getSenderOptionsFromSdp(publisher.ID(), v, videoOptions)
		} else {
			op = getSenderOptionsFromSdp(publisher.ID(), v, audioOptions)
		}
		t.NewSender(op)
	}
	sessionMap.Store("whep", t)
	offer := generateSdp("offer", t)
	writer.Write([]byte(offer["sdp"]))
}

func getOptionsFromSdp(v *sdp.MediaDescription) *peer.ReceiverOption {
	codecMap := make(map[rtc.PayloadType]*peer.Codec)
	var encoderPayload rtc.PayloadType

	for _, codec := range v.Codecs {
		c := peer.Codec{
			EncoderName: codec.EncoderName,
			PayloadType: rtc.PayloadType(codec.PayloadType),
			ClockRate:   codec.ClockRate,
			Channels:    codec.Channel,
			Parameters:  codec.Parameters,
			RTX:         rtc.PayloadType(codec.RTX),
		}
		for _, f := range codec.FeedbackParams {
			if f.ID == "transport-cc" {
				continue
			}
			c.FeedbackParams = append(c.FeedbackParams, peer.RtcpFeedback{
				Type:      f.ID,
				Parameter: f.Params,
			})
		}
		codecMap[rtc.PayloadType(codec.PayloadType)] = &c

		if v.MediaType == "audio" && codec.EncoderName == "opus" {
			encoderPayload = rtc.PayloadType(codec.PayloadType)
		}
		if v.MediaType == "video" && codec.EncoderName == "VP8" {
			encoderPayload = rtc.PayloadType(codec.PayloadType)
		}
	}

	o := &peer.ReceiverOption{
		ID:        v.TrackID,
		MID:       v.MID,
		MediaType: v.MediaType,
		// KeyFrameRequestDelay: 100,
	}
	for _, h := range v.HeaderExtensions {
		// skip tcc for test
		if h.URI == rtc.HeaderExtensionTransportSequenceNumber {
			continue
		}
		o.HeaderExtensions = append(o.HeaderExtensions, rtc.HeaderExtension{
			URI:     h.URI,
			ID:      rtc.HeaderExtensionID(h.ID),
			Encrypt: h.Encrypt,
		})
	}
	for _, s := range v.Streams {
		e := peer.StreamOption{
			SSRC:        s.SSRC,
			RID:         s.RID,
			RTX:         s.RTX,
			Dtx:         false,
			PayloadType: encoderPayload,
			Cname:       s.Cname,
		}

		o.Streams = append(o.Streams, e)
	}
	o.Codec = codecMap[o.Streams[0].PayloadType]
	return o
}

func getSenderOptionsFromSdp(id string, r *peer.Receiver, v *sdp.MediaDescription) *peer.SenderOption {
	var headerExtensions []rtc.HeaderExtension

	codecMap := make(map[rtc.PayloadType]*peer.Codec)
	var encoderPayload rtc.PayloadType

	for _, codec := range v.Codecs {
		c := peer.Codec{
			EncoderName: codec.EncoderName,
			PayloadType: rtc.PayloadType(codec.PayloadType),
			ClockRate:   codec.ClockRate,
			Channels:    codec.Channel,
			Parameters:  codec.Parameters,
			RTX:         rtc.PayloadType(codec.RTX),
		}
		for _, f := range codec.FeedbackParams {
			if f.ID == "transport-cc" {
				continue
			}
			c.FeedbackParams = append(c.FeedbackParams, peer.RtcpFeedback{
				Type:      f.ID,
				Parameter: f.Params,
			})
		}
		codecMap[rtc.PayloadType(codec.PayloadType)] = &c

		if v.MediaType == "audio" && codec.EncoderName == "opus" {
			encoderPayload = rtc.PayloadType(codec.PayloadType)
		}
		if v.MediaType == "video" && codec.EncoderName == "VP8" {
			encoderPayload = rtc.PayloadType(codec.PayloadType)
		}
	}

	o := &peer.SenderOption{
		ID:               r.ID() + peer.RandomString(12),
		ReceiverID:       r.ID(),
		ConnectionID:     id,
		Codec:            codecMap[encoderPayload],
		HeaderExtensions: headerExtensions,
		MID:              r.MID(),
		SwitchMode:       peer.ManualSwitchLayer,
	}
	return o
}
