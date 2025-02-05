package ice

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/gotolive/sfu/rtc/logger"
	"github.com/pion/stun"
)

const (
	defaultWriteBufferSize = 100
)

func createTCPServer(ips []string, port uint16, onConnection onConnection) (*tcpServer, error) {
	listeners := make([]net.Listener, 0)
	for _, ip := range ips {
		var addr string
		ipAddr := net.ParseIP(ip)
		if ipAddr.To4() == nil {
			addr = fmt.Sprintf("[%s]:%d", ip, port)
		} else {
			addr = fmt.Sprintf("%s:%d", ip, port)
		}
		lis, err := net.Listen("tcp", addr)
		if err != nil {
			return nil, err
		}
		listeners = append(listeners, lis)
	}
	s := &tcpServer{
		onConnection: onConnection,
		listeners:    listeners,
	}
	s.start()
	return s, nil
}

type tcpServer struct {
	onConnection onConnection
	listeners    []net.Listener
}

// when we stop server, we only stop accept new connections.
// current connection is managed by transport.
func (s *tcpServer) stop() {
	for _, lis := range s.listeners {
		_ = lis.Close()
	}
}

func (s *tcpServer) start() {
	for _, lis := range s.listeners {
		//  accept will exit when lis close
		go s.accept(lis)
	}
}

func (s *tcpServer) accept(lis net.Listener) {
	for {
		conn, err := lis.Accept()
		if err != nil {
			// if accept fail, we stop the goroutine and try to stop the lis.
			// in most case it will trigger by lis.Close().
			// but if it's not a lis.Close() error, the continue will make it a dead loop.
			logger.Warnf("TCP %v accept fail %s", lis.Addr(), err)
			// it's ok if we close the lis twice.
			_ = lis.Close()
			break
		}
		s.handleConn(conn)
	}
}

func (s *tcpServer) handleConn(conn net.Conn) {
	// it will exit when createTCPConnection finished or reach firstTimeout
	go func() {
		if err := createTCPConnection(conn, s.onConnection); err != nil {
			logger.Error("new tcp conn fail:", err)
			_ = conn.Close()
		}
	}()
}

func createTCPConnection(conn net.Conn, onConnection onConnection) error {
	c := &tcpConnection{
		conn:         conn,
		firstDone:    make(chan error),
		writeBuf:     make(chan []byte, defaultWriteBufferSize),
		onConnection: onConnection,
		closeCh:      make(chan bool),
	}
	// it will exit when conn closed.
	go c.readLoop()
	if err := c.waitFirst(); err != nil {
		return err
	}
	// https://github.com/golang/go/issues/23842
	// it will exit when conn closed.
	go c.writeLoop()
	return nil
}

type tcpConnection struct {
	conn         net.Conn
	firstDone    chan error
	callback     connectionCallback
	onConnection onConnection
	writeBuf     chan []byte
	closeCh      chan bool
}

func (c *tcpConnection) Close() error {
	close(c.closeCh)
	err := c.conn.Close()
	return err
}

func (c *tcpConnection) Write(data []byte) (int, error) {
	select {
	case c.writeBuf <- data:
	default:
	}
	return len(data), nil
}

func (c *tcpConnection) Protocol() string {
	return TCP
}

func (c *tcpConnection) String() string {
	return fmt.Sprintf("%s: %s<->%s", TCP, c.conn.LocalAddr().String(), c.conn.RemoteAddr().String())
}

func (c *tcpConnection) setCallback(receive connectionCallback) {
	c.callback = receive
}

func (c *tcpConnection) RemoteAddr() net.Addr {
	return c.conn.RemoteAddr()
}

const readTimeout = 5

func (c *tcpConnection) waitFirst() error {
	select {
	case err := <-c.firstDone:
		return err
	case <-time.After(time.Second * readTimeout):
		return ErrTCPReadTimeout
	}
}

func (c *tcpConnection) readLoop() {
	ufrag, err := c.readUfrag()
	if err != nil {
		c.firstDone <- err
		return
	}

	c.firstDone <- nil
	// it will be break when conn closed
	for {
		buf := make([]byte, defaultMTU)
		n, err := readStreamingPacket(c.conn, buf)
		if err != nil {
			logger.Errorf("Read packet fail. Conn: %v, Ufrag: %s, Err: %v", c.conn, ufrag, err)
			break
		}
		c.callback(buf[:n], c)
	}
}

func (c *tcpConnection) readUfrag() (string, error) {
	buf := make([]byte, defaultMTU)

	n, err := readStreamingPacket(c.conn, buf)
	if err != nil {
		return "", err
	}

	if !stun.IsMessage(buf[:n]) {
		return "", ErrNoStunMessage
	}
	m := stun.New()
	err = stun.Decode(buf[:n], m)
	if err != nil {
		return "", err
	}
	// username:remoteUsername
	username, _ := getUsername(m)
	if username == "" {
		return "", ErrInvalidStun
	}
	if err = c.onConnection(username, c); err != nil {
		return "", err
	}
	c.callback(buf[:n], c)
	return username, nil
}

func (c *tcpConnection) writeLoop() {
	for {
		select {
		case <-c.closeCh:
			return
		case data := <-c.writeBuf:
			if _, err := writeStreamingPacket(c.conn, data); err != nil {
				logger.Errorf("Read packet fail. Conn: %v, Err:%v", c.conn, err)
				break
			}
		}
	}
}

const streamingPacketHeaderLen = 2

// readStreamingPacket reads 1 packet from stream
// read packet  bytes https://tools.ietf.org/html/rfc4571#section-2
// 2-byte length header prepends each packet:
//
//	 0                   1                   2                   3
//	 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
//	-----------------------------------------------------------------
//	|             LENGTH            |  RTP or RTCP packet ...       |
//	-----------------------------------------------------------------
func readStreamingPacket(conn net.Conn, buf []byte) (int, error) {
	header := make([]byte, streamingPacketHeaderLen)

	var (
		bytesRead, n int
		err          error
	)

	for bytesRead < streamingPacketHeaderLen {
		if n, err = conn.Read(header[bytesRead:streamingPacketHeaderLen]); err != nil {
			return 0, err
		}
		bytesRead += n
	}

	length := int(binary.BigEndian.Uint16(header))

	if length > cap(buf) {
		return length, io.ErrShortBuffer
	}

	bytesRead = 0
	for bytesRead < length {
		if n, err = conn.Read(buf[bytesRead:length]); err != nil {
			return 0, err
		}
		bytesRead += n
	}

	return bytesRead, nil
}

func writeStreamingPacket(conn net.Conn, buf []byte) (int, error) {
	bufferCopy := make([]byte, streamingPacketHeaderLen+len(buf))
	binary.BigEndian.PutUint16(bufferCopy, uint16(len(buf)))
	copy(bufferCopy[2:], buf)

	n, err := conn.Write(bufferCopy)
	if err != nil {
		return 0, err
	}

	return n - streamingPacketHeaderLen, nil
}
