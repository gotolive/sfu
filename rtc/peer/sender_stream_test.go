package peer

import (
	"testing"
	"time"

	"github.com/gotolive/sfu/rtc"
	"github.com/pion/rtcp"
)

type mockSenderListener struct {
	nack bool
}

func (m *mockSenderListener) OnRTPStreamRetransmitRTPPacket(packet rtc.Packet) {
	m.nack = true
}

func TestSenderStream(t *testing.T) {
	// Create a new sender internalStream with mock parameters.
	sender := NewSenderStream(&mockSenderListener{}, "video", StreamOption{}, Codec{})
	stream := sender.(*senderStream)

	// Test the ReceivePacket method.
	err := stream.ReceivePacket(&pkt{})
	if err != nil {
		t.Errorf("ReceivePacket returned an error: %v", err)
	}

	// Test the ReceiveNack method.
	stream.ReceiveNack(&rtcp.TransportLayerNack{})

	// Test the FractionLost method.
	fractionLost := stream.FractionLost()
	if fractionLost != 0 {
		t.Errorf("FractionLost returned %d, expected 0", fractionLost)
	}

	// Test the GetRtcpSenderReport method.
	report := stream.GetRtcpSenderReport(time.Now().UnixMilli())
	if report == nil {
		t.Errorf("GetRtcpSenderReport returned nil")
	}

	// Test the GetRtcpSdesChunk method.
	chunk := stream.GetRtcpSdesChunk()
	if chunk == nil {
		t.Errorf("GetRtcpSdesChunk returned nil")
	}

	// Test the ReceiveRtcpReceiverReport method.
	stream.ReceiveRtcpReceiverReport(rtcp.ReceptionReport{})
}

func TestSenderStream_ReceiveNack(t *testing.T) {
	lis := &mockSenderListener{}
	sender := NewSenderStream(lis, "video", StreamOption{
		SSRC: 1234,
		RTX:  1234,
	}, Codec{
		RTX: 100,
		FeedbackParams: []RtcpFeedback{
			{
				Type: "nack",
			},
		},
	})
	err := sender.ReceivePacket(&pkt{
		seq: 1000,
	})
	rtxSeq := sender.(*senderStream).rtxSeq
	assert(t, err, nil)
	report := &rtcp.TransportLayerNack{
		Nacks: rtcp.NackPairsFromSequenceNumbers([]uint16{1000}),
	}
	sender.ReceiveNack(report)
	if lis.nack != true {
		t.Fatal("nack fail")
	}
	assert(t, rtxSeq+1, sender.(*senderStream).rtxSeq)
}
