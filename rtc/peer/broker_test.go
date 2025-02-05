package peer

import (
	"testing"
)

func TestNewBroker(t *testing.T) {
	option := BrokerOption{}

	broker, err := NewBroker(option)
	if err != nil {
		t.Errorf("Failed to create broker: %v", err)
	}

	if broker == nil {
		t.Error("Broker is nil")
	}
	c, err := broker.NewWebRTCConnection(&WebRTCOption{
		ID: "test-connection",
	})
	if c == nil || err != nil {
		t.Error("Fail to create connection")
	}
	broker.Close()
}
