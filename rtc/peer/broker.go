package peer

import (
	"sync"

	"github.com/gotolive/sfu/rtc/dtls"
	"github.com/gotolive/sfu/rtc/ice"
	"github.com/gotolive/sfu/rtc/logger"
)

var _ connectionListener = new(Broker)

func NewBroker(option BrokerOption) (*Broker, error) {
	cm, err := dtls.NewCertManager(false)
	if err != nil {
		return nil, err
	}
	iceServer, err := ice.NewServer(option.ICE)
	if err != nil {
		return nil, err
	}

	w := &Broker{
		certManager: cm,
		options:     option,
		connections: map[string]*Connection{},
		iceServer:   iceServer,
	}

	return w, nil
}

type BrokerOption struct {
	ICE ice.Option
}

// Broker is a sfu node, with global setting in it.
type Broker struct {
	options   BrokerOption
	iceServer *ice.Server

	cm          sync.RWMutex
	connections map[string]*Connection
	certManager dtls.CertificateGenerator
}

func (b *Broker) removeConnection(id string) {
	b.cm.Lock()
	delete(b.connections, id)
	b.cm.Unlock()
}

func (b *Broker) NewWebRTCConnection(options *WebRTCOption) (*Connection, error) {
	t, err := NewWebRTCTransport(options, b.iceServer, b.certManager)
	if err != nil {
		return nil, err
	}
	if options.ID == "" {
		options.ID = RandomString(12)
	}
	connection := newConnection(options.ID, options.BweType, t, b)
	t.SetConnection(connection)

	b.cm.Lock()
	if _, ok := b.connections[connection.ID()]; ok {
		logger.Error("exists", connection.ID())
		return nil, ErrConnExist
	}
	b.connections[connection.ID()] = connection
	b.cm.Unlock()
	return connection, nil
}

func (b *Broker) NewConnection(id string, bweType string, transport Transport) (*Connection, error) {
	connection := newConnection(id, bweType, transport, b)
	b.cm.Lock()
	if _, ok := b.connections[connection.ID()]; ok {
		return nil, ErrConnExist
	}
	b.connections[connection.ID()] = connection
	b.cm.Unlock()
	return connection, nil
}

func (b *Broker) Connection(id string) *Connection {
	b.cm.Lock()
	defer b.cm.Unlock()
	return b.connections[id]
}

// Close clean before stop, if someone still call new connection, it's their fault, we don't care.
func (b *Broker) Close() {
	// copy it to avoid dead lock.
	connections := make([]*Connection, 0, len(b.connections))
	b.cm.Lock()
	// this may have error, but we don't care.
	for _, c := range b.connections {
		connections = append(connections, c)
	}
	b.cm.Unlock()
	for _, c := range connections {
		c.Close()
	}
}

func (b *Broker) Connections() []*Connection {
	b.cm.Lock()
	defer b.cm.Unlock()
	result := make([]*Connection, 0, len(b.connections))
	for _, c := range b.connections {
		result = append(result, c)
	}
	return result
}
