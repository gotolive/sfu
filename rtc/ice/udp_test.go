package ice

import (
	"fmt"
	"net"
	"sync"
	"testing"
)

func TestUDPServer(t *testing.T) {
	tests := []testHelper{
		{
			name:        "connected_through_udp",
			description: "",
			method: func(t *testing.T) {
				server, err := NewServer(Option{
					IPs: []string{"127.0.0.1"},
				})
				if err != nil {
					t.FailNow()
				}
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
				conn, err := net.Dial("udp", fmt.Sprintf("%s:%d", candidate.IP, candidate.Port))
				assert(t, err, nil)
				_, err = conn.Write(stunBinding)
				assert(t, err, nil)
				<-wait
				assert(t, transport.State(), ConnectionConnected)
			},
		},
		{
			name:        "send_through_udp",
			description: "",
			method: func(t *testing.T) {
				server, err := NewServer(Option{
					IPs: []string{"127.0.0.1"},
				})
				if err != nil {
					t.FailNow()
				}
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
				conn, err := net.Dial("udp", fmt.Sprintf("%s:%d", candidate.IP, candidate.Port))
				assert(t, err, nil)
				_, err = conn.Write(stunBinding)
				assert(t, err, nil)
				<-wait
				assert(t, transport.State(), ConnectionConnected)
				_, err = transport.Write([]byte("OK"))
				assert(t, err, nil)
				b := make([]byte, 100)
				// skip stun message
				_, _ = conn.Read(b)
				n, err := conn.Read(b)
				assert(t, n, 2)
				assert(t, err, nil)
				assert(t, string(b[:n]), "OK")
			},
		},
		{
			name:        "received_through_udp",
			description: "",
			method: func(t *testing.T) {
				server, err := NewServer(Option{
					IPs: []string{"127.0.0.1"},
				})
				if err != nil {
					t.FailNow()
				}
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
				conn, err := net.Dial("udp", fmt.Sprintf("%s:%d", candidate.IP, candidate.Port))
				assert(t, err, nil)
				_, err = conn.Write(stunBinding)
				assert(t, err, nil)
				<-wait
				assert(t, transport.State(), ConnectionConnected)
				_, err = conn.Write([]byte("OK"))
				assert(t, err, nil)
				data := <-waitData
				assert(t, string(data), "OK")
			},
		},
		{
			name:        "close_should_make_udp_closed",
			description: "",
			method: func(t *testing.T) {
				server, err := NewServer(Option{
					IPs: []string{"127.0.0.1"},
				})
				if err != nil {
					t.FailNow()
				}
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
				conn, err := net.Dial("udp", fmt.Sprintf("%s:%d", candidate.IP, candidate.Port))
				assert(t, err, nil)
				_, err = conn.Write(stunBinding)
				assert(t, err, nil)
				<-wait
				assert(t, transport.State(), ConnectionConnected)
				transport.Close()
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.method(t)
		})
	}
}
