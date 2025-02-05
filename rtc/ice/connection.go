package ice

import (
	"net"
)

type connectionCallback func(data []byte, conn Connection)

// Connection represents a connection between two peers.
type Connection interface {
	Write(data []byte) (int, error)
	Protocol() string
	RemoteAddr() net.Addr
	Close() error
	// TBD make it a net.Conn if we use a Read method, it could be a net.Conn.
	// Read(data []byte) (int, error)

	setCallback(receive connectionCallback)
}
