package ice

import (
	"errors"
)

var (
	ErrUnknownIP         = errors.New("unknown ip")                     // ErrUnknownIP will raise if a given listen ip not in local interfaces.
	ErrTransportNotExist = errors.New("transport not exist")            // ErrTransportNotExist will raise if transport not exist.
	ErrNoAvailablePort   = errors.New("no available port")              // ErrNoAvailablePort will raise if no available port.
	ErrNoStunMessage     = errors.New("no stun message")                // ErrNoStunMessage will raise if no stun message.
	ErrUnsupportedStun   = errors.New("unsupported stun type received") // ErrUnsupportedStun will raise if unsupported stun type received.
	ErrInvalidStun       = errors.New("invalid stun message received")  // ErrInvalidStun will raise if invalid stun message received.
	ErrStunRoleConflict  = errors.New("stun role conflict")             // ErrStunRoleConflict will raise if stun role conflict.
	ErrNoAvailableIP     = errors.New("no available ip")                // ErrNoAvailableIP will raise if no available ip.
	ErrNoUDPAddr         = errors.New("not a udp addr")                 // ErrNoUDPAddr will raise if not a udp addr.
	ErrNoTCPAddr         = errors.New("not a tcp addr")                 // ErrNoTCPAddr will raise if not a tcp addr.
	ErrUnknownProtocol   = errors.New("unknown protocol")               // ErrUnknownProtocol will raise if unknown protocol.

	ErrServerClosed   = errors.New("server has closed")          // ErrServerClosed will raise if server has closed.
	ErrTransportExist = errors.New("transport already exist")    // ErrTransportExist will raise if transport already exist.
	ErrInvalidState   = errors.New("transport state is invalid") // ErrInvalidState will raise if transport state is invalid.

	ErrTCPReadTimeout = errors.New("tcp conn read timeout") // ErrTCPReadTimeout will raise if tcp conn read timeout.
)
