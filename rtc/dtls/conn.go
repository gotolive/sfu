package dtls

import (
	"io"
	"net"
	"time"
)

func NewConn(r io.Reader, w io.Writer) net.Conn {
	return &wrapperConn{
		reader: r,
		writer: w,
	}
}

type wrapperConn struct {
	reader io.Reader
	writer io.Writer
}

func (c *wrapperConn) Read(b []byte) (int, error) {
	return c.reader.Read(b)
}

func (c *wrapperConn) Write(b []byte) (int, error) {
	return c.writer.Write(b)
}

func (c *wrapperConn) Close() error {
	return nil
}

func (c *wrapperConn) LocalAddr() net.Addr {
	return nil
}

func (c *wrapperConn) RemoteAddr() net.Addr {
	return nil
}

func (c *wrapperConn) SetDeadline(t time.Time) error {
	return nil
}

func (c *wrapperConn) SetReadDeadline(t time.Time) error {
	return nil
}

func (c *wrapperConn) SetWriteDeadline(t time.Time) error {
	return nil
}
