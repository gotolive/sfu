package ice

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gotolive/sfu/rtc/logger"
	"github.com/pion/stun"
)

// Transport is the interface for ICE transport.
type Transport interface {
	Write(data []byte) (int, error)
	Close()
	State() ConnectionState
	Parameters() Parameters
	// SetOnData(receive OnData)
}

// Constants for default ICE candidate priority, type preference, and component
const (
	Host = "host"

	CandidateFoundationUDP = "udpcandidate"
	CandidateFoundationTCP = "tcpcandidate"

	RoleControlled  = "controlled"
	RoleControlling = "controlling"

	UDP = "udp"
	TCP = "tcp"

	defaultDisconnectedTimeout = 5
	defaultFailedTimeout       = 30
)

// ConnectionState represents the state of the ICE connection
// See https://w3c.github.io/webrtc-pc/#dom-rtciceconnectionstate
// We simplify the state for our use case.
type ConnectionState int32

func (c ConnectionState) String() string {
	switch c {
	case ConnectionNew:
		return "new"
	case ConnectionConnected:
		return "connected"
	case ConnectionCompleted:
		return "completed"
	case ConnectionDisconnected:
		return "disconnected"
	case ConnectionFailed:
		return "failed"
	}
	return "unknown"
}

const (
	ConnectionNew          ConnectionState = iota // ConnectionNew indicates new connection
	ConnectionConnected                           // ConnectionConnected indicates connection connected
	ConnectionCompleted                           // ConnectionCompleted indicates connection completed
	ConnectionDisconnected                        // ConnectionDisconnected indicates connection disconnected
	ConnectionFailed                              // ConnectionFailed indicates connection failed
)

type (
	// OnData is the callback when data received.
	OnData func(data []byte)
	// OnState is the callback when state changed.
	OnState func(state ConnectionState)
)

type iceTransport struct {
	userFragment string
	password     string
	connections  []Connection
	connection   Connection
	candidates   []Candidate
	onData       OnData
	onState      OnState
	// if we don't use atomic, the race test will complain, in fact,
	//  no atomic is totally fine in here but anyway.
	state                       int32
	role                        string // for now, it's always controlled
	lastReceiveTimestamp        int64
	iceLocalPreferenceDecrement int

	once              sync.Once
	connected         chan bool
	disconnected      chan bool
	failTimeout       int64 // second
	disconnectTimeout int64 // second

}

func (t *iceTransport) addConnection(conn Connection) {
	conn.setCallback(t.onReceive)
	t.connections = append(t.connections, conn)
}

func (t *iceTransport) Write(data []byte) (int, error) {
	switch t.State() {
	case ConnectionConnected, ConnectionCompleted:
	default:
		return 0, fmt.Errorf("%w: %v", ErrInvalidState, t.state)
	}
	return t.connection.Write(data)
}

// Close closes the transport.
func (t *iceTransport) Close() {
	state := t.State()
	if state == ConnectionFailed || state == ConnectionDisconnected {
		return
	}
	t.connection = nil
	t.setState(ConnectionDisconnected)
	t.close()
	t.onState(ConnectionDisconnected)
}

func (t *iceTransport) State() ConnectionState {
	return ConnectionState(atomic.LoadInt32(&t.state))
}

// Parameters
func (t *iceTransport) Parameters() Parameters {
	return Parameters{
		UsernameFragment: t.userFragment,
		Password:         t.password,
		Candidates:       t.Candidates(),
		Role:             t.role,
		Lite:             true, // always true
	}
}

func (t *iceTransport) addCandidate(protocol, ip string, port uint16) {
	candidate := buildCandidate(protocol, ip, port, t.iceLocalPreferenceDecrement)
	t.candidates = append(t.candidates, candidate)
	t.iceLocalPreferenceDecrement += 100
}

func (t *iceTransport) Candidates() []Candidate {
	return t.candidates
}

func (t *iceTransport) onReceive(data []byte, conn Connection) {
	t.updateTimestamp()
	// if its stun we handle it.
	if stun.IsMessage(data) {
		t.processStun(data, conn)
		return
	}

	state := t.State()
	if state == ConnectionConnected || state == ConnectionCompleted {
		if t.onData != nil {
			t.onData(data)
		}
	}
}

func (t *iceTransport) processStun(data []byte, conn Connection) {
	var (
		err      error
		response *stun.Message
	)

	m := stun.New()
	err = stun.Decode(data, m)
	if err != nil {
		logger.Error("Decode stun message fail:", err, " drop it")
		return
	}

	code := validateBindingStun(m, t.userFragment, t.password)
	if code != 0 {
		logger.Error("validate stun fail:", code)
		response, err = createErrorResponse(m, code)
		if err != nil {
			logger.Error("create stun response fail:", err)
		}
	} else {
		response, err = createBindSuccessResponse(m, conn.Protocol(), conn.RemoteAddr(), t.password)
		if err != nil {
			logger.Error("create stun response fail:", err)
			return
		}
	}

	response.Encode()
	_, err = conn.Write(response.Raw)
	if err != nil {
		logger.Warnf("Send with conn %v fail: %v", conn, err)
		return
	}
	if code == 0 {
		// only update it with success response
		t.updateState(conn, m.Contains(stun.AttrUseCandidate))
	}
}

func (t *iceTransport) updateState(conn Connection, useCandidate bool) {
	switch t.State() {
	case ConnectionNew:
		t.connection = conn
		t.connect()
		if useCandidate {
			t.setState(ConnectionCompleted)
		} else {
			t.setState(ConnectionConnected)
		}
		t.onState(t.State())

	case ConnectionConnected, ConnectionCompleted:
		if useCandidate {
			// it could be same conn, but it's fine.
			t.connection = conn
			t.connect()
			if t.State() != ConnectionCompleted {
				t.setState(ConnectionCompleted)
				t.onState(ConnectionCompleted)
			}
		}
	default:
		logger.Warn("Receive connection after disconnected")
	}
}

func (t *iceTransport) connectiveCheck() {
	ticker := time.NewTicker(time.Second)
	for {
		select {
		case <-t.disconnected:
			return
		case <-ticker.C:
			timestamp := atomic.LoadInt64(&t.lastReceiveTimestamp)
			if timestamp == 0 {
				continue
			}
			// https://github.com/pion/webrtc/issues/2061
			now := time.Now().Unix()
			if now-timestamp > t.disconnectTimeout {
				logger.Warnf("ICE Timeout after %d seconds", t.disconnectTimeout)
				t.Close()
				return
			}
		}
	}
}

func (t *iceTransport) start() {
	// it will exit after connected, or timeout.
	go func() {
		select {
		case <-t.connected:
		case <-time.After(time.Duration(t.failTimeout) * time.Second):
			if t.State() == ConnectionNew {
				t.setState(ConnectionFailed)
				t.close()
				t.onState(ConnectionFailed)
			}
		}
	}()
}

func (t *iceTransport) updateTimestamp() {
	timestamp := time.Now().Unix()
	atomic.StoreInt64(&t.lastReceiveTimestamp, timestamp)
}

func (t *iceTransport) connect() {
	t.once.Do(func() {
		// it will exit when it closed or timeout
		go t.connectiveCheck()
		close(t.connected)
	})
}

func (t *iceTransport) setState(state ConnectionState) {
	atomic.StoreInt32(&t.state, int32(state))
}

// closed all connections
func (t *iceTransport) close() {
	close(t.disconnected)
	for _, c := range t.connections {
		_ = c.Close()
	}
}
