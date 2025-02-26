package conference

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/gotolive/sfu/rtc"
	"github.com/gotolive/sfu/rtc/bwe"
	"github.com/gotolive/sfu/rtc/dtls"
	"github.com/gotolive/sfu/rtc/ice"
	"github.com/gotolive/sfu/rtc/logger"
	"github.com/gotolive/sfu/rtc/peer"
	"github.com/gotolive/sfu/rtc/sdp"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type Room struct {
	id         string
	clients    map[*Client]bool
	broadcast  chan Message
	register   chan *Client
	unregister chan *Client
}

type Client struct {
	id         string
	conn       *websocket.Conn
	send       chan []byte
	connection *peer.Connection
	published  chan bool
}

type Message struct {
	ClientID string          `json:"clientId"`
	Type     string          `json:"type"`
	Data     json.RawMessage `json:"data"`
}

var rooms = make(map[string]*Room)
var roomsMutex sync.RWMutex
var broker, _ = peer.NewBroker(peer.BrokerOption{
	ICE: ice.Option{
		EnableIPV6: true,
		EnableTCP:  true,
	},
})

func newRoom(id string) *Room {
	return &Room{
		id:         id,
		clients:    make(map[*Client]bool),
		broadcast:  make(chan Message),
		register:   make(chan *Client),
		unregister: make(chan *Client),
	}
}

func (room *Room) run(closeFn func()) {
	ticker := time.NewTicker(time.Second * 30)
	for {
		select {
		case client := <-room.register:
			room.clients[client] = true
		case client := <-room.unregister:
			if _, ok := room.clients[client]; ok {
				delete(room.clients, client)
				close(client.send)
			}
		case message := <-room.broadcast:
			room.handleMessage(message)

		case <-ticker.C:
			if len(room.clients) == 0 {
				closeFn()
				return
			}
		}
	}
}

func (room *Room) handleMessage(message Message) {
	var response, broadcast *Message
	switch message.Type {
	case joinRoom:
		clients := make([]string, 0, len(room.clients))
		for client := range room.clients {
			clients = append(clients, client.id)
		}
		data, _ := json.Marshal(clients)
		response = &Message{
			Type: joinRoom + "-response",
			Data: data,
		}
		broadcast = &Message{
			Type:     joinRoom,
			ClientID: message.ClientID,
		}
	case publish:
		broadcast = &Message{
			Type:     publish,
			ClientID: message.ClientID,
		}
	case subscribe:
		response = &Message{
			Type:     subscribe + "-response",
			ClientID: message.ClientID,
		}
		var cid string
		json.Unmarshal(message.Data, &cid)
		if cid == "" {
			log.Println("client id is required")
			return
		}
		var client *Client
		for c := range room.clients {
			if c.id == cid {
				client = c
				break
			}
		}
		if client == nil {
			log.Println("client not found", cid)
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
		publisher := client.connection
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
		offer := generateSdp("offer", t)
		offer["clientId"] = cid
		r, _ := json.Marshal(offer)
		response.Data = r
	}
	for client := range room.clients {
		if client.id == message.ClientID && response != nil {
			data, _ := json.Marshal(response)
			select {
			case client.send <- data:
			default:
				close(client.send)
				delete(room.clients, client)
			}
		}
		if client.id != message.ClientID && broadcast != nil {
			data, _ := json.Marshal(broadcast)
			select {
			case client.send <- data:
			default:
				close(client.send)
				delete(room.clients, client)
			}
		}
	}
}

func (client *Client) readPump(room *Room) {
	defer func() {
		room.unregister <- client
	}()
	for {
		_, message, err := client.conn.ReadMessage()
		if err != nil {
			break
		}
		var msg Message
		err = json.Unmarshal(message, &msg)
		if err != nil {
			break
		}
		client.handleMessage(room, msg)
	}
}

func (client *Client) writePump() {
	defer client.conn.Close()
	for {
		select {
		case message, ok := <-client.send:
			if !ok {
				client.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			client.conn.WriteMessage(websocket.TextMessage, message)
		}
	}
}

const (
	joinRoom  = "join"
	publish   = "publish"
	subscribe = "subscribe"
)

func (client *Client) handleMessage(room *Room, msg Message) {
	msg.ClientID = client.id
	switch msg.Type {
	case publish:
		offer := sdp.SDP{}
		json.Unmarshal(msg.Data, &offer)
		jsdp, err := sdp.Unmarshal(offer.SDP)
		if err != nil {
			logger.Error("err:", err)
			return
		}

		t, err := broker.NewWebRTCConnection(&peer.WebRTCOption{
			ID: room.id + "-" + client.id + "-pub",
			DtlsOption: dtls.Option{
				Role: dtls.Active,
			},
			BweType: bwe.Remb,
		})
		if err != nil {
			logger.Error("create connection fail:", err, client.id)
			return
		}
		t.OnStateChange(func(state int) {
			if state == 1 {
				close(client.published)
				client.connection = t
				room.broadcast <- Message{
					ClientID: client.id,
					Type:     publish,
				}
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
		message := Message{
			Type: publish + "-response",
			Data: r,
		}
		data, _ := json.Marshal(message)
		client.send <- data
	default:
		room.broadcast <- msg
	}
}

func serveWs(clientID string, room *Room, w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}
	client := &Client{id: clientID, conn: conn, send: make(chan []byte, 256), published: make(chan bool)}
	room.register <- client

	go client.writePump()
	go client.readPump(room)
}

func WSHandleFunc(w http.ResponseWriter, r *http.Request) {
	roomID := r.URL.Query().Get("room")
	if roomID == "" {
		http.Error(w, "Room ID is required", http.StatusBadRequest)
		return
	}
	clientID := r.URL.Query().Get("client")
	if clientID == "" {
		http.Error(w, "Client ID is required", http.StatusBadRequest)
		return
	}

	roomsMutex.Lock()
	room, ok := rooms[roomID]
	if !ok {
		room = newRoom(roomID)
		go room.run(func() {
			roomsMutex.Lock()
			delete(rooms, roomID)
			roomsMutex.Unlock()
		})
		rooms[roomID] = room
	}
	roomsMutex.Unlock()

	serveWs(clientID, room, w, r)
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
