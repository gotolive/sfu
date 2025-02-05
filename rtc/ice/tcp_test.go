package ice

import (
	"fmt"
	"net"
	"sync"
	"testing"
	"time"
)

func TestTCPServer(t *testing.T) {

	tests := []testHelper{
		{
			name:        "connected_through_tcp",
			description: "",
			method: func(t *testing.T) {
				server, err := NewServer(Option{
					EnableTCP:  true,
					DisableUDP: true,
					IPs:        []string{"127.0.0.1"},
					TCPPort:    0,
				})
				assert(t, err, nil)
				defer server.Close()
				wait := make(chan bool)
				once := sync.Once{}
				transport, err := server.NewTransport(stunUsername, stunPwd, nil, nil, func(state ConnectionState) {
					once.Do(func() {
						if state != ConnectionConnected {
							t.Fail()
						}
						close(wait)
					})
				})
				if err != nil || transport == nil {
					t.FailNow()
				}
				assert(t, len(transport.Parameters().Candidates), 1)
				candidate := transport.Parameters().Candidates[0]
				conn, err := net.Dial("tcp", fmt.Sprintf("%s:%d", candidate.IP, candidate.Port))
				assert(t, err, nil)
				_, err = writeStreamingPacket(conn, stunBinding)
				assert(t, err, nil)
				<-wait
				assert(t, transport.State(), ConnectionConnected)
			},
		},

		{
			name:        "send_through_tcp",
			description: "",
			method: func(t *testing.T) {
				server, err := NewServer(Option{
					EnableTCP:  true,
					DisableUDP: true,
					IPs:        []string{"127.0.0.1"},
					TCPPort:    0,
				})
				assert(t, err, nil)
				defer server.Close()
				wait := make(chan bool)
				once := sync.Once{}
				transport, err := server.NewTransport(stunUsername, stunPwd, nil, nil, func(state ConnectionState) {
					once.Do(func() {
						if state != ConnectionConnected {
							t.Fail()
						}
						close(wait)
					})
				})
				if err != nil || transport == nil {
					t.FailNow()
				}
				assert(t, len(transport.Parameters().Candidates), 1)
				candidate := transport.Parameters().Candidates[0]
				conn, err := net.Dial("tcp", fmt.Sprintf("%s:%d", candidate.IP, candidate.Port))
				assert(t, err, nil)
				_, err = writeStreamingPacket(conn, stunBinding)
				assert(t, err, nil)
				<-wait
				assert(t, transport.State(), ConnectionConnected)
				_, err = transport.Write([]byte("OK"))
				assert(t, err, nil)
				b := make([]byte, 100)
				// skip stun message
				_, _ = readStreamingPacket(conn, b)
				n, err := readStreamingPacket(conn, b)
				assert(t, n, 2)
				assert(t, err, nil)
				assert(t, string(b[:n]), "OK")
			},
		},
		{
			name:        "received_through_tcp",
			description: "",
			method: func(t *testing.T) {
				server, err := NewServer(Option{
					EnableTCP:  true,
					DisableUDP: true,
					IPs:        []string{"127.0.0.1"},
					TCPPort:    0,
				})
				assert(t, err, nil)
				defer server.Close()
				wait := make(chan bool)
				waitData := make(chan []byte)
				once := sync.Once{}
				transport, err := server.NewTransport(stunUsername, stunPwd, nil, func(data []byte) {
					go func() {
						waitData <- data
					}()
				}, func(state ConnectionState) {
					once.Do(func() {
						if state != ConnectionConnected {
							t.Fail()
						}
						close(wait)
					})
				})
				if err != nil || transport == nil {
					t.FailNow()
				}
				assert(t, len(transport.Parameters().Candidates), 1)
				candidate := transport.Parameters().Candidates[0]
				conn, err := net.Dial("tcp", fmt.Sprintf("%s:%d", candidate.IP, candidate.Port))
				assert(t, err, nil)
				_, err = writeStreamingPacket(conn, stunBinding)
				assert(t, err, nil)
				<-wait
				assert(t, transport.State(), ConnectionConnected)
				_, err = writeStreamingPacket(conn, []byte("OK"))
				assert(t, err, nil)
				data := <-waitData
				assert(t, string(data), "OK")
			},
		},
		{
			name:        "close_should_make_tcp_closed",
			description: "",
			method: func(t *testing.T) {
				server, err := NewServer(Option{
					EnableTCP:  true,
					DisableUDP: true,
					IPs:        []string{"127.0.0.1"},
					TCPPort:    0,
				})
				assert(t, err, nil)
				defer server.Close()
				wait := make(chan bool)
				waitData := make(chan []byte)
				once := sync.Once{}
				transport, err := server.NewTransport(stunUsername, stunPwd, nil, func(data []byte) {
					go func() {
						waitData <- data
					}()
				}, func(state ConnectionState) {
					once.Do(func() {
						if state != ConnectionConnected {
							t.Fail()
						}
						close(wait)
					})
				})
				if err != nil || transport == nil {
					t.FailNow()
				}
				assert(t, len(transport.Parameters().Candidates), 1)
				candidate := transport.Parameters().Candidates[0]
				conn, err := net.Dial("tcp", fmt.Sprintf("%s:%d", candidate.IP, candidate.Port))
				assert(t, err, nil)
				_, err = writeStreamingPacket(conn, stunBinding)
				assert(t, err, nil)
				<-wait
				assert(t, transport.State(), ConnectionConnected)
				transport.Close()
				b := make([]byte, 100)
				// skip stun response
				_, _ = conn.Read(b)
				_, err = conn.Read(b)
				if err == nil {
					t.FailNow()
				}
			},
		},
		{
			name:        "connected_fail",
			description: "",
			method: func(t *testing.T) {
				server, err := NewServer(Option{
					EnableTCP:  true,
					DisableUDP: true,
					IPs:        []string{"127.0.0.1"},
					TCPPort:    0,
				})
				assert(t, err, nil)
				defer server.Close()
				wait := make(chan bool)
				transport, err := server.NewTransport(stunUsername+"invalid", stunPwd, nil, nil, func(state ConnectionState) {
				})
				if err != nil || transport == nil {
					t.FailNow()
				}
				assert(t, len(transport.Parameters().Candidates), 1)
				candidate := transport.Parameters().Candidates[0]
				conn, err := net.Dial("tcp", fmt.Sprintf("%s:%d", candidate.IP, candidate.Port))
				assert(t, err, nil)
				go func() {
					for {
						b := make([]byte, 100)
						if _, err := conn.Read(b); err != nil {
							close(wait)
							break
						}

					}
				}()
				_, err = writeStreamingPacket(conn, stunBinding)
				assert(t, err, nil)
				assert(t, transport.State(), ConnectionNew)
				// conn should close if stun validate fail
				select {
				case <-wait:
				case <-time.After(time.Second * 3):
					t.FailNow()
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
