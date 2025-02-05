package peer

import (
	"testing"

	"github.com/gotolive/sfu/rtc"
	"github.com/pion/rtcp"
)

type mockReceiverListener struct {
	firCalled bool
	pliCalled bool
}

func (m *mockReceiverListener) OnRTPStreamNeedWorstRemoteFractionLost(ssrc uint32) uint8 {
	// TODO implement me
	panic("implement me")
}

func (m *mockReceiverListener) sendRtcp(packet rtcp.Packet) {
	switch packet.(type) {
	case *rtcp.PictureLossIndication:
		m.pliCalled = true
	case *rtcp.FullIntraRequest:
		m.firCalled = true
	}
}

func TestReceiverStream(t *testing.T) {
	receiver := NewReceiverStream(&mockReceiverListener{}, rtc.MediaTypeVideo, StreamOption{
		SSRC:            0,
		Cname:           "",
		RID:             "",
		RTX:             0,
		Dtx:             false,
		PayloadType:     0,
		ScalabilityMode: "",
		MaxBitrate:      0,
		MaxFramerate:    0,
	}, Codec{
		PayloadType: 0,
		EncoderName: "",
		ClockRate:   0,
		Channels:    0,
		Parameters:  nil,
		FeedbackParams: []RtcpFeedback{
			{
				Type: "nack",
			},
			{
				Type:      "nack",
				Parameter: "pli",
			},
		},
		RTX: 0,
	})
	receiver.SetRtx(0, 0)
	p := &pkt{}
	err := receiver.ReceivePacket(p)
	assert(t, err, nil)
	report := receiver.GetRtcpReceiverReport()
	if report == nil {
		t.Fatal("fail to get rr")
	}
	rtxReport := receiver.GetRtxReceiverReport()
	if rtxReport == nil {
		t.Fatal("fail to get rr")
	}

	receiver.ReceiveRtcpSenderReport(&rtcp.SenderReport{})

	receiver.ReceivePacket(&pkt{
		seq: 1,
		rtx: true,
	})
}

func TestReceiver_RequestKeyFrame(t *testing.T) {
	tests := []testHelper{
		{
			name:        "Use PLI",
			description: "",
			method: func(t *testing.T) {
				// Create a new receiver with a mock callback
				lis := &mockReceiverListener{}
				r := NewReceiverStream(lis, "video", StreamOption{}, Codec{
					FeedbackParams: []RtcpFeedback{
						{
							Type:      "nack",
							Parameter: "pli",
						},
					},
				})

				// Call RequestKeyframe and check that the mock callback's OnRequestKeyFrame method was called
				r.RequestKeyFrame()
				if !lis.pliCalled {
					t.Errorf("OnRequestKeyFrame was not called")
				}
			},
		},
		{
			name:        "Use FIR",
			description: "",
			method: func(t *testing.T) {
				// Create a new receiver with a mock callback
				lis := &mockReceiverListener{}
				r := NewReceiverStream(lis, "video", StreamOption{}, Codec{
					FeedbackParams: []RtcpFeedback{
						{
							Type:      "ccm",
							Parameter: "fir",
						},
					},
				})

				// Call RequestKeyframe and check that the mock callback's OnRequestKeyFrame method was called
				r.RequestKeyFrame()
				if !lis.firCalled {
					t.Errorf("OnRequestKeyFrame was not called")
				}
			},
		},
		{
			name:        "Nack",
			description: "",
			method: func(t *testing.T) {
				// Create a new receiver with a mock callback
				lis := &mockReceiverListener{}
				r := NewReceiverStream(lis, "video", StreamOption{}, Codec{
					FeedbackParams: []RtcpFeedback{
						{
							Type:      "ccm",
							Parameter: "fir",
						},
					},
				})

				// Call RequestKeyframe and check that the mock callback's OnRequestKeyFrame method was called
				r.(*receiverStream).onNackRTCP(&rtcp.PictureLossIndication{})
				if !lis.firCalled {
					t.Errorf("OnRequestKeyFrame was not called")
				}
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.method(t)
		})
	}
}

func TestReceiverStream_GetRtcpReceiverReport(t *testing.T) {
	tests := []testHelper{
		{
			name:        "Test Nack",
			description: "",
			method: func(t *testing.T) {
				// Create a new receiver with a mock callback
				lis := &mockReceiverListener{}
				r := NewReceiverStream(lis, "video", StreamOption{}, Codec{ClockRate: 90000})
				pkts := []*pkt{
					{seq: 1000},
					{seq: 1001},
					{seq: 1002},
					{seq: 1003},
					{seq: 1005},
					{seq: 1006},
					{seq: 1007},
					{seq: 1008},
					{seq: 1009},
				}
				for _, p := range pkts {
					err := r.ReceivePacket(p)
					if err != nil {
						t.Fatal("reveive rtpPacket fail")
					}
				}
				report := r.GetRtcpReceiverReport()
				if report == nil {
					t.Fatal("get report fail")
				}
				// 10%
				if report.FractionLost != 25 {
					t.Fatal("fraction lost fail", report.FractionLost)
				}
				if report.TotalLost != 1 {
					t.Fatal("total lost fail")
				}
				if report.LastSequenceNumber != 1009 {
					t.Fatal("LastSequenceNumber fail")
				}
				//if report.Jitter == 0 {
				//	t.Fatal("LastSequenceNumber fail")
				//}
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.method(t)
		})
	}
}
