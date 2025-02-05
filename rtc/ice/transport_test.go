package ice

import (
	"errors"
	"fmt"
	"sync"
	"testing"
)

func TestIceTransport(t *testing.T) {
	server, err := NewServer(Option{
		DisconnectTimeout: 300,
		FailTimeout:       300,
	})
	assert(t, err, nil)
	defer server.Close()
	tests := []testHelper{
		{
			name:        "connection_status_string",
			description: "",
			method: func(t *testing.T) {
				assert(t, fmt.Sprint(ConnectionNew), "new")
				assert(t, fmt.Sprint(ConnectionConnected), "connected")
				assert(t, fmt.Sprint(ConnectionCompleted), "completed")
				assert(t, fmt.Sprint(ConnectionDisconnected), "disconnected")
				assert(t, fmt.Sprint(ConnectionFailed), "failed")
				assert(t, fmt.Sprint(ConnectionState(10)), "unknown")
			},
		},
		{
			name:        "transport_parameters_should_return_correct",
			description: "",
			method: func(t *testing.T) {
				transport, err := server.NewTransport(stunUsername, stunPwd, nil, nil, nil)
				if err != nil || transport == nil {
					t.FailNow()
				}
				defer transport.Close()
				p := transport.Parameters()
				if !p.Lite || p.Role != RoleControlled || len(p.Candidates) == 0 || p.UsernameFragment != stunUsername || p.Password != stunPwd {
					t.FailNow()
				}
			},
		},
		{
			name:        "send_should_fail_when_transport_not_ready",
			description: "",
			method: func(t *testing.T) {
				transport := &iceTransport{}
				_, err := transport.Write([]byte{})
				if !errors.Is(err, ErrInvalidState) {
					t.FailNow()
				}
			},
		},
		{
			name:        "send_should_success_when_connected",
			description: "",
			method: func(t *testing.T) {
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
				defer transport.Close()
				conn := newFaceConnection(UDP, 100)
				err = server.onConnection(stunUsername, conn)
				if err != nil {
					t.FailNow()
				}
				conn.input(stunBinding)
				<-wait
				assert(t, transport.State(), ConnectionConnected)
				n, err := transport.Write([]byte("OK"))
				if n != 2 || err != nil {
					t.FailNow()
				}
				b := conn.output(1)
				if string(b) != "OK" {
					t.FailNow()
				}
			},
		},
		{
			name:        "send_should_success_when_completed",
			description: "",
			method: func(t *testing.T) {
				wait := make(chan bool)
				once := sync.Once{}
				transport, err := server.NewTransport(stunUsername, stunPwd, nil, nil, func(state ConnectionState) {
					once.Do(func() {
						if state != ConnectionCompleted {
							t.Fail()
						}
						close(wait)
					})
				})
				if err != nil || transport == nil {
					t.FailNow()
				}
				defer transport.Close()
				conn := newFaceConnection(UDP, 100)
				err = server.onConnection(stunUsername, conn)
				if err != nil {
					t.FailNow()
				}
				conn.input(stunBindingUse)
				<-wait
				assert(t, transport.State(), ConnectionCompleted)
				n, err := transport.Write([]byte("OK"))
				if n != 2 || err != nil {
					t.FailNow()
				}
				b := conn.output(1)
				if string(b) != "OK" {
					t.FailNow()
				}
			},
		},
		{
			name:        "should_drop_invalid_stun",
			description: "",
			method: func(t *testing.T) {
				transport, err := server.NewTransport(stunUsername+"invalid", stunPwd, nil, nil, func(state ConnectionState) {
				})
				if err != nil || transport == nil {
					t.FailNow()
				}
				defer transport.Close()
				conn := newFaceConnection(UDP, 100)
				err = server.onConnection(stunUsername+"invalid", conn)
				if err != nil {
					t.Log("fail here?")
					t.FailNow()
				}
				conn.input(stunBinding)
				assert(t, transport.State(), ConnectionNew)
			},
		},
		{
			name: "transport_should_connected_when_receive_binding",
			method: func(t *testing.T) {
				wait := make(chan bool)
				once := sync.Once{}
				transport, err := server.NewTransport(stunUsername, stunPwd, nil, nil, func(state ConnectionState) {
					once.Do(func() {
						if state != ConnectionConnected {
							t.FailNow()
						}
						close(wait)
					})
				})
				if err != nil || transport == nil {
					t.FailNow()
				}
				defer transport.Close()
				conn := newFaceConnection(UDP, 100)
				err = server.onConnection(stunUsername, conn)
				if err != nil {
					t.FailNow()
				}
				conn.input(stunBinding)
				<-wait
				assert(t, transport.State(), ConnectionConnected)
				t.Log("assert succes")
			},
		},
		{
			name: "transport_should_complete_when_use",
			method: func(t *testing.T) {
				waitConnect := make(chan bool)
				waitComplete := make(chan bool)
				transport, err := server.NewTransport(stunUsername, stunPwd, nil, nil, func(state ConnectionState) {
					if state == ConnectionConnected {
						close(waitConnect)
						return
					}
					if state == ConnectionCompleted {
						close(waitComplete)
						return
					}
				})
				if err != nil || transport == nil {
					t.FailNow()
				}
				defer transport.Close()
				conn := newFaceConnection(UDP, 100)
				err = server.onConnection(stunUsername, conn)
				if err != nil {
					t.FailNow()
				}
				conn.input(stunBinding)
				assert(t, transport.State(), ConnectionConnected)
				<-waitConnect
				conn.input(stunBindingUse)
				<-waitComplete
				assert(t, transport.State(), ConnectionCompleted)
			},
		},
		{
			name: "transport_should_disconnected_when_close_called",
			method: func(t *testing.T) {
				waitConnect := make(chan bool)
				waitClose := make(chan bool)
				transport, err := server.NewTransport(stunUsername, stunPwd, nil, nil, func(state ConnectionState) {
					if state == ConnectionConnected {
						close(waitConnect)
						return
					}
					if state == ConnectionDisconnected {
						close(waitClose)
						return
					}
					t.FailNow()
				})
				if err != nil || transport == nil {
					t.FailNow()
				}
				defer transport.Close()
				conn := newFaceConnection(UDP, 100)
				err = server.onConnection(stunUsername, conn)
				if err != nil {
					t.FailNow()
				}
				conn.input(stunBinding)
				assert(t, transport.State(), ConnectionConnected)
				<-waitConnect
				transport.Close()
				<-waitClose
				assert(t, transport.State(), ConnectionDisconnected)
			},
		},
		{
			name: "transport_should_disconnected_when_timeout",
			method: func(t *testing.T) {
				timeoutServer, err := NewServer(Option{
					DisconnectTimeout: 1,
				})
				if err != nil {
					t.FailNow()
				}
				defer timeoutServer.Close()
				waitConnect := make(chan bool)
				waitClose := make(chan bool)
				transport, err := timeoutServer.NewTransport(stunUsername, stunPwd, nil, nil, func(state ConnectionState) {
					if state == ConnectionConnected {
						close(waitConnect)
						return
					}
					if state == ConnectionDisconnected {
						close(waitClose)
						return
					}
				})
				if err != nil || transport == nil {
					t.FailNow()
				}
				defer transport.Close()
				conn := newFaceConnection(UDP, 100)
				err = timeoutServer.onConnection(stunUsername, conn)
				if err != nil {
					t.FailNow()
				}
				conn.input(stunBinding)
				assert(t, transport.State(), ConnectionConnected)
				<-waitConnect
				<-waitClose
				assert(t, transport.State(), ConnectionDisconnected)
			},
		},
		{
			name: "transport_should_fail_when_timeout",
			method: func(t *testing.T) {
				timeoutServer, err := NewServer(Option{
					FailTimeout: 1,
				})
				if err != nil {
					t.FailNow()
				}
				defer timeoutServer.Close()
				waitClose := make(chan bool)
				transport, err := timeoutServer.NewTransport(stunUsername, stunPwd, nil, nil, func(state ConnectionState) {
					if state == ConnectionFailed {
						close(waitClose)
						return
					}
					t.FailNow()
				})
				if err != nil || transport == nil {
					t.FailNow()
				}
				defer transport.Close()
				<-waitClose
				assert(t, transport.State(), ConnectionFailed)
			},
		},
		{
			name:        "should_receive_data",
			description: "",
			method: func(t *testing.T) {
				server, err := NewServer(Option{
					EnableTCP: true,
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
							close(wait)
						}
						close(wait)
					})
				})
				if err != nil || transport == nil {
					t.FailNow()
				}
				defer transport.Close()
				conn := newFaceConnection(UDP, 100)
				err = server.onConnection(stunUsername, conn)
				if err != nil {
					t.FailNow()
				}
				conn.input(stunBinding)
				<-wait
				assert(t, transport.State(), ConnectionConnected)
				conn.input([]byte("OK"))
				data := <-waitData
				if string(data) != "OK" {
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
