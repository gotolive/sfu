package ice

import (
	"fmt"
	"net"
	"sync"
)

type onConnection func(ufrag string, conn Connection) error

// Option is the option for create ice server.
type Option struct {
	MinPort uint16
	MaxPort uint16

	// MuxPort    uint16 // we do not support udp port mux for now, using a turn server instead
	EnableIPV6 bool // we do not support ipv6 yet.
	EnableTCP  bool // default disable
	TCPPort    uint16
	DisableUDP bool     // default enable
	IPs        []string // listen ips, if empty or nil, will use all ips available

	FailTimeout       int64 // how many seconds will a transport wait before transaction to fail, default is 30s.
	DisconnectTimeout int64 // how many seconds will a transport wait before transaction to disconnected, default is 5s.
}

// NewServer creates a new ice server.
func NewServer(option Option) (*Server, error) {
	ips, err := validateIps(option.IPs, option.EnableIPV6)
	if err != nil {
		return nil, err
	}

	if option.FailTimeout == 0 {
		option.FailTimeout = defaultFailedTimeout
	}
	if option.DisconnectTimeout == 0 {
		option.DisconnectTimeout = defaultDisconnectedTimeout
	}

	server := &Server{
		minPort:           option.MinPort,
		maxPort:           option.MaxPort,
		tcpPort:           option.TCPPort,
		enableTCP:         option.EnableTCP,
		disableUDP:        option.DisableUDP,
		ips:               ips,
		failTimeout:       option.FailTimeout,
		disconnectTimeout: option.DisconnectTimeout,
		closeCh:           make(chan bool),
		transports:        map[string]*iceTransport{},
	}

	if option.EnableTCP {
		ts, err := createTCPServer(ips, option.TCPPort, server.onConnection)
		if err != nil {
			return nil, err
		}
		server.tcpServer = ts
	}

	if !option.DisableUDP {
		us := createUDPServer(server.onConnection)
		server.udpServer = us
	}

	return server, nil
}

func validateIps(ips []string, ipv6 bool) ([]string, error) {
	localIps, err := getInterfaceIps(ipv6)
	if err != nil {
		return nil, err
	}

	if len(ips) == 0 {
		return localIps, nil
	}
	result := make([]string, 0, len(ips))
	// make sure the ips we have.
	for _, ip := range ips {
		var found bool
		for _, interfaceIP := range localIps {
			if ip == interfaceIP {
				found = true
				result = append(result, ip)
				break
			}
		}
		if !found {
			return nil, unknownIP(ip)
		}
	}
	return result, nil
}

// Server is ice server side, could be used as HOST type candidate
type Server struct {
	minPort    uint16
	maxPort    uint16
	tcpPort    uint16
	enableTCP  bool
	disableUDP bool
	ips        []string
	tcpServer  *tcpServer
	udpServer  *udpServer

	closeCh chan bool

	transportsMutex   sync.Mutex
	transports        map[string]*iceTransport
	failTimeout       int64
	disconnectTimeout int64
}

// NewTransport return new transport with given params.
// If ips is nil or empty,  we will allocate on every ip available in server.
func (s *Server) NewTransport(ufrag, password string, ips []string, onData OnData, onState OnState) (Transport, error) {
	select {
	case <-s.closeCh:
		return nil, ErrServerClosed
	default:
	}
	if len(ips) == 0 {
		ips = s.ips
	}

	transport := &iceTransport{
		userFragment:      ufrag,
		password:          password,
		onData:            onData,
		role:              RoleControlled,
		onState:           s.wrapOnState(ufrag, onState),
		connected:         make(chan bool),
		disconnected:      make(chan bool),
		failTimeout:       s.failTimeout,
		disconnectTimeout: s.disconnectTimeout,
	}

	if !s.disableUDP {
		for _, ip := range ips {
			addr, err := s.udpServer.listen(ip, s.minPort, s.maxPort)
			if err != nil {
				return nil, err
			}

			if udpAddr, ok := addr.(*net.UDPAddr); ok {
				transport.addCandidate(UDP, ip, uint16(udpAddr.Port))
			}
		}
	}

	if s.enableTCP {
		for _, lis := range s.tcpServer.listeners {
			if addr, ok := lis.Addr().(*net.TCPAddr); ok {
				for _, ip := range ips {
					if addr.IP.String() == ip {
						transport.addCandidate(TCP, ip, uint16(addr.Port))
						break
					}
				}
			}
		}
	}

	transport.start()

	s.transportsMutex.Lock()
	if _, ok := s.transports[ufrag]; ok {
		s.transportsMutex.Unlock()
		return nil, ErrTransportExist
	}
	s.transports[ufrag] = transport
	s.transportsMutex.Unlock()

	// the server transport only wrap
	return transport, nil
}

// Close stop the ice server
func (s *Server) Close() {
	// stop new transport
	close(s.closeCh)

	s.transportsMutex.Lock()
	transports := make([]Transport, 0, len(s.transports))
	for _, t := range s.transports {
		transports = append(transports, t)
	}
	s.transportsMutex.Unlock()

	// avoid deadlock.
	for _, t := range transports {
		t.Close()
	}

	if s.tcpServer != nil {
		s.tcpServer.stop()
	}
	if s.udpServer != nil {
		s.udpServer.stop()
	}
}

func (s *Server) onConnection(ufrag string, conn Connection) error {
	s.transportsMutex.Lock()
	defer s.transportsMutex.Unlock()
	if t, ok := s.transports[ufrag]; ok && t != nil {
		t.addConnection(conn)
		return nil
	}
	return ErrTransportNotExist
}

func (s *Server) wrapOnState(ufrag string, onState OnState) func(state ConnectionState) {
	return func(state ConnectionState) {
		if state == ConnectionDisconnected || state == ConnectionFailed {
			s.transportsMutex.Lock()
			delete(s.transports, ufrag)
			s.transportsMutex.Unlock()
		}
		if onState != nil {
			onState(state)
		}
	}
}

func unknownIP(ip string) error {
	return fmt.Errorf("%w: %s", ErrUnknownIP, ip)
}
