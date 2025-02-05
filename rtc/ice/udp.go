package ice

import (
	"fmt"
	"net"

	"github.com/gotolive/sfu/rtc/logger"
	"github.com/pion/stun"
)

const (
	defaultMTU = 1500
)

func createUDPServer(onConnection onConnection) *udpServer {
	return &udpServer{
		onConnection: onConnection,
	}
}

type udpServer struct {
	onConnection onConnection
	listeners    []*udpListener
}

func (s *udpServer) listen(ip string, minPort, maxPort uint16) (net.Addr, error) {
	conn, err := createUDPListener(ip, minPort, maxPort, s.onConnection)
	if err != nil {
		return nil, err
	}
	s.listeners = append(s.listeners, conn)
	return conn.LAddr(), nil
}

func (s *udpServer) stop() {
	for _, l := range s.listeners {
		l.close()
	}
}

func createUDPListener(ip string, minPort, maxPort uint16, onConnection onConnection) (*udpListener, error) {
	// if minPort is 0, use a random port
	for port := minPort; port <= maxPort; port++ {
		var addr string
		ipAddr := net.ParseIP(ip)
		if ipAddr.To4() == nil {
			addr = fmt.Sprintf("[%s]:%d", ip, port)
		} else {
			addr = fmt.Sprintf("%s:%d", ip, port)
		}
		conn, err := net.ListenPacket("udp", addr)
		if err == nil {
			return newUDPListener(conn, onConnection), nil
		}
	}
	return nil, fmt.Errorf("%w : %s", ErrNoAvailablePort, ip)
}

func newUDPListener(conn net.PacketConn, connection onConnection) *udpListener {
	lis := &udpListener{conn: conn, onConnection: connection, conns: map[string]*udpConnection{}}
	go lis.accept()
	return lis
}

type udpListener struct {
	conn         net.PacketConn
	onConnection onConnection
	conns        map[string]*udpConnection
}

func (l *udpListener) accept() {
	// select for exit
	// it will break when conn closed
	data := make([]byte, defaultMTU)
	for {
		n, addr, err := l.conn.ReadFrom(data)
		if err != nil {
			logger.Error("read from fail:", l.conn.LocalAddr(), err)
			break
		}

		if conn, ok := l.conns[addr.String()]; ok && conn != nil {
			conn.callback(data[:n], conn)
			continue
		}

		if stun.IsMessage(data) {
			m := stun.New()
			err = stun.Decode(data, m)
			if err != nil {
				logger.Error("decode stun message fail:", err)
				continue
			}
			username, _ := getUsername(m)
			if username == "" {
				logger.Error("received invalid username:", username)
				continue
			}
			conn := &udpConnection{
				conn:     l.conn,
				remote:   addr,
				listener: l,
			}
			// Even we received invalid message we won't stop listen on udp.
			if err := l.onConnection(username, conn); err != nil {
				logger.Warn("received unexpected stun")
				continue
			}
			l.conns[addr.String()] = conn
			conn.callback(data[:n], conn)
		} else {
			logger.Warn("receive non stun message before ufrag")
			continue
		}
	}
}

func (l *udpListener) LAddr() net.Addr {
	return l.conn.LocalAddr()
}

func (l *udpListener) close() {
	_ = l.conn.Close()
}

type udpConnection struct {
	conn     net.PacketConn
	remote   net.Addr
	callback connectionCallback
	listener *udpListener
}

func (c *udpConnection) String() string {
	return fmt.Sprintf("%s: %s<->%s", UDP, c.conn.LocalAddr().String(), c.RemoteAddr().String())
}

func (c *udpConnection) Close() error {
	c.listener.close()
	return nil
}

func (c *udpConnection) RemoteAddr() net.Addr {
	return c.remote
}

func (c *udpConnection) setCallback(receive connectionCallback) {
	c.callback = receive
}

func (c *udpConnection) Protocol() string {
	return UDP
}

func (c *udpConnection) Write(data []byte) (int, error) {
	return c.conn.WriteTo(data, c.remote)
}
