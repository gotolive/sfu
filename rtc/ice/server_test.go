package ice

import (
	"errors"
	"net"
	"testing"
	"time"
)

func TestServer(t *testing.T) {
	tests := []testHelper{
		{
			name:        "create_server_with_invalid_ip",
			description: "NewServer should FailNow if we supply a invalid ip",
			method: func(t *testing.T) {
				server, err := NewServer(Option{
					MinPort:    0,
					MaxPort:    0,
					EnableTCP:  false,
					TCPPort:    0,
					DisableUDP: false,
					IPs:        []string{"1.1.1.1"},
				})
				if !errors.Is(err, ErrUnknownIP) || server != nil {
					t.FailNow()
				}
			},
		},
		{
			name:        "create_server_with_ipv6_enabled",
			description: "NewServer should success when ipv6 is enabled",
			method: func(t *testing.T) {
				server, err := NewServer(Option{
					EnableTCP:  true,
					EnableIPV6: true,
				})
				if err != nil || server == nil {
					t.Log(err)
					t.FailNow()
				}
			},
		},
		{
			name:        "create_server_with_given_ip",
			description: "NewServer should success if we supply invalid ip",
			method: func(t *testing.T) {
				server, err := NewServer(Option{
					MinPort:    0,
					MaxPort:    0,
					EnableTCP:  false,
					TCPPort:    0,
					DisableUDP: false,
					IPs:        []string{"127.0.0.1"},
				})
				if err != nil || server == nil {
					t.FailNow()
				}
			},
		},
		{
			name:        "create_server_with_tcp_support",
			description: "NewServer should success with tcp enable",
			method: func(t *testing.T) {
				server, err := NewServer(Option{
					MinPort:    0,
					MaxPort:    0,
					EnableTCP:  true,
					TCPPort:    0,
					DisableUDP: false,
					IPs:        []string{"127.0.0.1"},
				})
				if err != nil || server == nil {
					t.FailNow()
				}
			},
		},
		{
			name:        "create_server_with_tcp_port_inuse",
			description: "NewServer should FailNow with tcp inuse",
			method: func(t *testing.T) {
				lis, err := net.Listen("tcp", "127.0.0.1:0")
				if err != nil {
					t.FailNow()
				}
				server, err := NewServer(Option{
					MinPort:    0,
					MaxPort:    0,
					EnableTCP:  true,
					TCPPort:    uint16(lis.Addr().(*net.TCPAddr).Port),
					DisableUDP: false,
					IPs:        []string{"127.0.0.1"},
				})
				if err == nil || server != nil {
					t.FailNow()
				}
			},
		},
		{
			name:        "server_close_should_trigger_transport_state_change",
			description: "Server close should make transport state change to disconnect",
			method: func(t *testing.T) {
				server, err := NewServer(Option{
					MinPort:    0,
					MaxPort:    0,
					EnableTCP:  true,
					TCPPort:    0,
					DisableUDP: false,
					IPs:        nil,
				})
				if err != nil || server == nil {
					t.Log(err)
					t.FailNow()
				}
				ch := make(chan bool)
				transport, err := server.NewTransport("ufrag", "pwd", nil, nil, func(state ConnectionState) {
					if state != ConnectionDisconnected {
						t.FailNow()
					}
					close(ch)
				})
				if err != nil || transport == nil {
					t.FailNow()
				}
				server.Close()
				select {
				case <-time.After(time.Second):
					t.FailNow()
				case <-ch:
				}
			},
		},
		{
			name:        "create_transport_after_server_close",
			description: "NewTransport should FailNow after Server.Close called",
			method: func(t *testing.T) {
				server, err := NewServer(Option{
					MinPort:    0,
					MaxPort:    0,
					EnableTCP:  false,
					TCPPort:    0,
					DisableUDP: false,
					IPs:        nil,
				})
				if err != nil || server == nil {
					t.FailNow()
				}
				server.Close()
				transport, err := server.NewTransport("ufrag", "pwd", nil, nil, nil)
				if !errors.Is(err, ErrServerClosed) || transport != nil {
					t.FailNow()
				}
			},
		},
		{
			name:        "create_repeat_ufrag_transport",
			description: "NewTransport with already used ufrag should FailNow",
			method: func(t *testing.T) {
				server, err := NewServer(Option{
					MinPort:    0,
					MaxPort:    0,
					EnableTCP:  true,
					TCPPort:    0,
					DisableUDP: false,
					IPs:        nil,
				})
				if err != nil {
					t.FailNow()
				}
				_, err = server.NewTransport("ufrag", "pwd", nil, nil, nil)
				if err != nil {
					t.FailNow()
				}
				transport, err := server.NewTransport("ufrag", "pwd", nil, nil, nil)
				if !errors.Is(err, ErrTransportExist) || transport != nil {
					t.FailNow()
				}
			},
		},
		{
			name:        "create_transport_with_tcp_support",
			description: "NewTransport should success with tcp enable",
			method: func(t *testing.T) {
				server, err := NewServer(Option{
					MinPort:    0,
					MaxPort:    0,
					EnableTCP:  true,
					TCPPort:    0,
					DisableUDP: true,
					IPs:        []string{"127.0.0.1"},
				})
				if err != nil || server == nil {
					t.FailNow()
				}
				transport, err := server.NewTransport("ufrag", "pwd", nil, nil, nil)
				if err != nil || transport == nil {
					t.FailNow()
				}
			},
		},
		{
			name:        "create_transport_with_no_port",
			description: "NewTransport should fail when no available port",
			method: func(t *testing.T) {
				server, err := NewServer(Option{
					MinPort:    35000,
					MaxPort:    35000,
					EnableTCP:  false,
					TCPPort:    0,
					DisableUDP: false,
					IPs:        nil,
				})
				if err != nil {
					t.FailNow()
				}
				t1, err := server.NewTransport("ufrag", "pwd", nil, nil, nil)
				if err != nil {
					t.FailNow()
				}
				defer t1.Close()
				transport, err := server.NewTransport("ufrag1", "pwd", nil, nil, nil)
				if !errors.Is(err, ErrNoAvailablePort) || transport != nil {
					t.Log(err, transport)
					t.FailNow()
				}
			},
		},
		{
			name:        "set_fake_connection_success",
			description: "Fake a success connection",
			method: func(t *testing.T) {
				server, err := NewServer(Option{
					EnableTCP: true,
				})
				if err != nil {
					t.FailNow()
				}
				transport, err := server.NewTransport("ufrag", "pwd", nil, nil, nil)
				if err != nil || transport == nil {
					t.FailNow()
				}
				err = server.onConnection("ufrag", &fakeConnection{})
				if err != nil {
					t.FailNow()
				}
			},
		},
		{
			name:        "set_fake_connection_fail",
			description: "Fake a success connection",
			method: func(t *testing.T) {
				server, err := NewServer(Option{
					EnableTCP: true,
				})
				if err != nil {
					t.FailNow()
				}
				transport, err := server.NewTransport("ufrag", "pwd", nil, nil, nil)
				if err != nil || transport == nil {
					t.FailNow()
				}
				err = server.onConnection("non-ufrag", &fakeConnection{})
				if !errors.Is(err, ErrTransportNotExist) {
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
