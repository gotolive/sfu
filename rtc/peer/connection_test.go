package peer

import (
	"errors"
	"testing"

	"github.com/gotolive/sfu/rtc"
	"github.com/gotolive/sfu/rtc/bwe"
	"github.com/pion/rtcp"
)

func TestConnection(t *testing.T) {
	transport := &MockTransport{}
	listener := &MockConnectionListener{
		conns: map[string]*Connection{},
	}
	tests := []testHelper{
		{
			name:        "default header value",
			description: "",
			method: func(t *testing.T) {
				conn := newConnection("test-id", bwe.Remb, transport, listener)

				// if we don't have headers yet, we should get default ids
				headers := conn.getHeaderExtensions([]rtc.HeaderExtension{
					{
						URI: rtc.HeaderExtensionAbsSendTime,
						ID:  10,
					},
				})
				assert(t, len(headers), 1)
				if headers.AbsSendTime() == 0 {
					t.Error("should get header id")
				}
				// if we don't have headers yet, we should get default ids
				headers2 := conn.getHeaderExtensions([]rtc.HeaderExtension{
					{
						URI: rtc.HeaderExtensionAbsSendTime,
						ID:  1,
					},
				})
				assert(t, headers.AbsSendTime(), headers2.AbsSendTime())
			},
		},
		{
			name:        "preset header value",
			description: "",
			method: func(t *testing.T) {
				conn := newConnection("test-id", bwe.Remb, transport, listener)

				conn.updateHeaderExtensions([]rtc.HeaderExtension{
					{
						URI: rtc.HeaderExtensionAbsSendTime,
						ID:  10,
					},
				})

				// if we don't have headers yet, we should get default ids
				headers := conn.getHeaderExtensions([]rtc.HeaderExtension{
					{
						URI: rtc.HeaderExtensionAbsSendTime,
						ID:  rtc.HeaderExtensionIDAbsoluteSendTime,
					},
				})
				assert(t, len(headers), 1)
				if headers.AbsSendTime() != 10 {
					t.Errorf("expected 10")
				}
			},
		},
		{
			name:        "default codec",
			description: "",
			method: func(t *testing.T) {
				conn := newConnection("test-id", "", transport, listener)

				// if we don't have headers yet, we should get default ids
				codec := conn.getCodec(&Codec{
					PayloadType: 0,
					EncoderName: "H264",
					ClockRate:   90000,
				})
				if codec.PayloadType == 0 {
					t.Errorf("should not be zero")
				}
			},
		},
		{
			name:        "codec exists",
			description: "",
			method: func(t *testing.T) {
				conn := newConnection("test-id", "", transport, listener)

				// if we don't have headers yet, we should get default ids
				codec := conn.getCodec(&Codec{
					PayloadType: 0,
					EncoderName: "H264",
					ClockRate:   90000,
				})
				codec2 := conn.getCodec(&Codec{
					PayloadType: 0,
					EncoderName: "H264",
					ClockRate:   90000,
				})
				if codec.PayloadType != codec2.PayloadType {
					t.Errorf("should be same")
				}
			},
		},
		{
			name:        "preset codec ",
			description: "",
			method: func(t *testing.T) {
				conn := newConnection("test-id", "", transport, listener)

				// if we don't have headers yet, we should get default ids
				err := conn.updateCodecs(&Codec{
					PayloadType: 100,
					EncoderName: "H264",
				})
				codec := conn.getCodec(&Codec{
					PayloadType: 0,
					EncoderName: "H264",
					ClockRate:   90000,
				})
				if err != nil || codec.PayloadType != 100 {
					t.Errorf("should be same")
				}
				err = conn.updateCodecs(&Codec{
					PayloadType: 101,
					EncoderName: "H264",
				})
				if !errors.Is(err, ErrPayloadNotMatch) {
					t.Errorf("should not allow")
				}
				err = conn.updateCodecs(&Codec{
					PayloadType: 100,
					EncoderName: "VP9",
				})
				if !errors.Is(err, ErrPayloadNotMatch) {
					t.Errorf("should not allow")
				}
				assert(t, len(conn.codec), 1)
				err = conn.updateCodecs(&Codec{
					PayloadType: 101,
					EncoderName: "VP9",
				})
				assert(t, err, nil)
				assert(t, len(conn.codec), 2)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.method(t)
		})
	}
}

func TestConnectionReceiver(t *testing.T) {
	tests := []testHelper{
		{
			name:        "create a new receiver",
			description: "",
			method: func(t *testing.T) {
				transport := &MockTransport{}
				listener := &MockConnectionListener{
					conns: map[string]*Connection{},
				}
				conn := newConnection("test-id", "", transport, listener)
				_, err := conn.NewReceiver(&ReceiverOption{
					ID:        "test-receiver",
					MID:       "1",
					MediaType: rtc.MediaTypeVideo,
					Codec: &Codec{
						PayloadType: 100,
						EncoderName: "H264",
						ClockRate:   90000,
					},
					HeaderExtensions: make([]rtc.HeaderExtension, 0),
					Streams: []StreamOption{
						{
							SSRC:        1000,
							PayloadType: 100,
						},
					},
					KeyFrameRequestDelay: 0,
				})
				assert(t, err, nil)
				assert(t, len(conn.Receivers()), 1)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.method(t)
		})
	}
}

func TestConnectionSender(t *testing.T) {
	tests := []testHelper{
		{
			name:        "create a new sender",
			description: "",
			method: func(t *testing.T) {
				transport := &MockTransport{}
				listener := &MockConnectionListener{
					conns: map[string]*Connection{},
				}
				conn := newConnection("test-id", "", transport, listener)
				conn2 := newConnection("test-id-2", "", transport, listener)
				listener.conns[conn.id] = conn
				listener.conns[conn2.id] = conn2
				_, err := conn.NewReceiver(&ReceiverOption{
					ID:        "test-receiver",
					MID:       "1",
					MediaType: rtc.MediaTypeVideo,
					Codec: &Codec{
						PayloadType: 100,
						EncoderName: "H264",
						ClockRate:   90000,
					},
					HeaderExtensions: make([]rtc.HeaderExtension, 0),
					Streams: []StreamOption{
						{
							SSRC:        1000,
							PayloadType: 100,
						},
					},
					KeyFrameRequestDelay: 0,
				})
				assert(t, err, nil)
				assert(t, len(conn.Receivers()), 1)
				s, err := conn2.NewSender(&SenderOption{
					ID:           "test-sender",
					MID:          "1",
					ConnectionID: conn.id,
					ReceiverID:   "test-receiver",
				})
				assert(t, err, nil)
				assert(t, len(conn2.Senders()), 1)

				_, err = conn2.NewSender(&SenderOption{
					ID:           "test-sender",
					MID:          "1",
					ConnectionID: conn.id,
					ReceiverID:   "no-receiver",
				})
				assert(t, err, ErrReceiverNotExist)
				assert(t, len(conn2.Senders()), 1)
				s.Close()
				assert(t, len(conn2.Senders()), 0)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.method(t)
		})
	}
}

func TestConnectionReceiveRTP(t *testing.T) {
	tests := []testHelper{
		{
			name:        "create a new receiver",
			description: "",
			method: func(t *testing.T) {
				transport := &MockTransport{}
				listener := &MockConnectionListener{
					conns: map[string]*Connection{},
				}
				conn := newConnection("test-id", "", transport, listener)
				_, err := conn.NewReceiver(&ReceiverOption{
					ID:        "test-receiver",
					MID:       "1",
					MediaType: rtc.MediaTypeVideo,
					Codec: &Codec{
						PayloadType: 100,
						EncoderName: "H264",
						ClockRate:   90000,
					},
					HeaderExtensions: make([]rtc.HeaderExtension, 0),
					Streams: []StreamOption{
						{
							SSRC:        1000,
							PayloadType: 100,
						},
					},
					KeyFrameRequestDelay: 0,
				})
				assert(t, err, nil)
				pkts := []*pkt{
					{receiveMs: 1, size: 1000},
					{receiveMs: 100, size: 1000},
					{receiveMs: 200, size: 1000},
					{receiveMs: 300, size: 1000},
					{receiveMs: 400, size: 1000},
					{receiveMs: 500, size: 1000},
					{receiveMs: 600, size: 1000},
					{receiveMs: 700, size: 1000},
					{receiveMs: 800, size: 1000},
					{receiveMs: 900, size: 1000},
				}
				for _, v := range pkts {
					conn.receiveRTPPacket(v)
				}

				stats := conn.Stats()
				if stats.PacketsReceived() != 10 {
					t.Error("packets count wrong", stats.packetsReceived)
				}
				if stats.BytesReceived() != 10*1000 {
					t.Error("bytes count wrong", stats.BytesReceived())
				}
				if stats.ReceiveBPS(1000) != 80000 {
					t.Error("bps wrong", stats.ReceiveBPS(1000))
				}
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.method(t)
		})
	}
}

type MockTransport struct{}

func (t *MockTransport) SetConnection(connection *Connection) {
}

func (t *MockTransport) Close() {
	// TODO implement me
	panic("implement me")
}

func (t *MockTransport) IsConnected() bool {
	return false
}

func (t *MockTransport) SendRTPPacket(packet rtc.Packet) {
	// do nothing
}

func (t *MockTransport) SendRtcpPacket(packet rtcp.Packet) {
	// do nothing
}

func (t *MockTransport) Info() TransportInfo {
	return TransportInfo{}
}

type MockConnectionListener struct {
	conns map[string]*Connection
}

func (l *MockConnectionListener) Connection(id string) *Connection {
	return l.conns[id]
}

func (l *MockConnectionListener) removeConnection(id string) {
	// do nothing
}
