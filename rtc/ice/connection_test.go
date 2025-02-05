package ice

import (
	"net"
)

func newFaceConnection(protocol string, size int) *fakeConnection {
	return &fakeConnection{
		protocol: protocol,
		buf:      make(chan []byte, size),
	}
}

type fakeConnection struct {
	callback connectionCallback
	protocol string
	buf      chan []byte
}

func (f *fakeConnection) Close() error {
	return nil
}

func (f *fakeConnection) Write(data []byte) (int, error) {
	f.buf <- data
	return len(data), nil
}

func (f *fakeConnection) Protocol() string {
	return f.protocol
}

func (f *fakeConnection) setCallback(receive connectionCallback) {
	f.callback = receive
}

func (f *fakeConnection) RemoteAddr() net.Addr {
	switch f.protocol {
	case UDP:
		return &net.UDPAddr{
			IP:   net.IPv4(127, 0, 0, 1),
			Port: 30000,
		}
	case TCP:
		return &net.TCPAddr{
			IP:   net.IPv4(127, 0, 0, 1),
			Port: 30000,
		}
	}
	return nil
}

func (f *fakeConnection) input(data []byte) {
	if f.callback != nil {
		f.callback(data, f)
	}
}

func (f *fakeConnection) output(skip int) []byte {
	for i := 0; i < skip; i++ {
		<-f.buf
	}
	return <-f.buf
}
