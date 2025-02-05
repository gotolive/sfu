package rtc

type HeaderExtensionID uint8

// All support rtp header extensions
// see https://chromium.googlesource.com/external/webrtc/+/HEAD/modules/rtp_rtcp/include/rtp_rtcp_defines.h#51
const (
	HeaderExtensionIDNone HeaderExtensionID = iota
	HeaderExtensionIDTransmissionTimeOffset
	HeaderExtensionIDAudioLevel
	HeaderExtensionIDCsrcAudioLevel
	HeaderExtensionIDInbandComfortNoise
	HeaderExtensionIDAbsoluteSendTime
	HeaderExtensionIDAbsoluteCaptureTime
	HeaderExtensionIDVideoRotation
	HeaderExtensionIDTransportSequenceNumber
	HeaderExtensionIDTransportSequenceNumber02
	HeaderExtensionIDPlayoutDelay
	HeaderExtensionIDVideoContentType
	HeaderExtensionIDVideoLayersAllocation
	HeaderExtensionIDVideoTiming
	HeaderExtensionIDHeaderStreamID
	HeaderExtensionIDRepairedHeaderStreamID
	HeaderExtensionIDMid
	HeaderExtensionIDGenericFrameDescriptor00
	HeaderExtensionIDGenericFrameDescriptor02
	HeaderExtensionIDColorSpace
	HeaderExtensionIDVideoFrameTrackingID
)

// see https://chromium.googlesource.com/external/webrtc/+/HEAD/api/rtp_parameters.h#298
const (
	HeaderExtensionNone                      = ""
	HeaderExtensionTimestampOffset           = "urn:ietf:params:rtp-hdrext:toffset"
	HeaderExtensionAudioLevel                = "urn:ietf:params:rtp-hdrext:ssrc-audio-level"
	HeaderExtensionCsrcAudioLevels           = "urn:ietf:params:rtp-hdrext:csrc-audio-level"
	HeaderExtensionInbandCN                  = "http://www.webrtc.org/experiments/rtp-hdrext/inband-cn"
	HeaderExtensionAbsSendTime               = "http://www.webrtc.org/experiments/rtp-hdrext/abs-send-time"
	HeaderExtensionAbsoluteCaptureTime       = "http://www.webrtc.org/experiments/rtp-hdrext/abs-capture-time"
	HeaderExtensionVideoRotation             = "urn:3gpp:video-orientation"
	HeaderExtensionTransportSequenceNumber   = "http://www.ietf.org/id/draft-holmer-rmcat-transport-wide-cc-extensions-01"
	HeaderExtensionTransportSequenceNumberV2 = "http://www.webrtc.org/experiments/rtp-hdrext/transport-wide-cc-02"
	HeaderExtensionPlayoutDelay              = "http://www.webrtc.org/experiments/rtp-hdrext/playout-delay"
	HeaderExtensionVideoContentType          = "http://www.webrtc.org/experiments/rtp-hdrext/video-content-type"
	HeaderExtensionVideoLayersAllocation     = "http://www.webrtc.org/experiments/rtp-hdrext/video-layers-allocation00"
	HeaderExtensionVideoTiming               = "http://www.webrtc.org/experiments/rtp-hdrext/video-timing"
	HeaderExtensionRid                       = "urn:ietf:params:rtp-hdrext:sdes:rtp-stream-id"
	HeaderExtensionRepairedRid               = "urn:ietf:params:rtp-hdrext:sdes:repaired-rtp-stream-id"
	HeaderExtensionMid                       = "urn:ietf:params:rtp-hdrext:sdes:mid"
	HeaderExtensionGenericFrameDescriptor00  = "http://www.webrtc.org/experiments/rtp-hdrext/generic-frame-descriptor-00"
	HeaderExtensionColorSpace                = "http://www.webrtc.org/experiments/rtp-hdrext/color-space"
	HeaderExtensionVideoFrameTrackingID      = "http://www.webrtc.org/experiments/rtp-hdrext/video-frame-tracking-id"
	HeaderExtensionDependencyDescriptor      = "https://aomediacodec.github.io/av1-rtp-spec/#dependency-descriptor-rtp-header-extension"
)

type HeaderExtension struct {
	URI     string
	ID      HeaderExtensionID
	Encrypt bool
}

// DefaultHeaderExtensionID is used when we are the offer side
func DefaultHeaderExtensionID(uri string) HeaderExtensionID {
	headers := map[string]HeaderExtensionID{
		HeaderExtensionNone:                      HeaderExtensionIDNone,
		HeaderExtensionTimestampOffset:           HeaderExtensionIDTransmissionTimeOffset,
		HeaderExtensionAudioLevel:                HeaderExtensionIDAudioLevel,
		HeaderExtensionCsrcAudioLevels:           HeaderExtensionIDCsrcAudioLevel,
		HeaderExtensionInbandCN:                  HeaderExtensionIDInbandComfortNoise,
		HeaderExtensionAbsSendTime:               HeaderExtensionIDAbsoluteSendTime,
		HeaderExtensionAbsoluteCaptureTime:       HeaderExtensionIDAbsoluteCaptureTime,
		HeaderExtensionVideoRotation:             HeaderExtensionIDVideoRotation,
		HeaderExtensionTransportSequenceNumber:   HeaderExtensionIDTransportSequenceNumber,
		HeaderExtensionTransportSequenceNumberV2: HeaderExtensionIDTransportSequenceNumber02,
		HeaderExtensionPlayoutDelay:              HeaderExtensionIDPlayoutDelay,
		HeaderExtensionVideoContentType:          HeaderExtensionIDVideoContentType,
		HeaderExtensionVideoLayersAllocation:     HeaderExtensionIDVideoLayersAllocation,
		HeaderExtensionVideoTiming:               HeaderExtensionIDVideoTiming,
		HeaderExtensionRid:                       HeaderExtensionIDHeaderStreamID,
		HeaderExtensionRepairedRid:               HeaderExtensionIDRepairedHeaderStreamID,
		HeaderExtensionMid:                       HeaderExtensionIDMid,
		HeaderExtensionGenericFrameDescriptor00:  HeaderExtensionIDGenericFrameDescriptor00,
		HeaderExtensionColorSpace:                HeaderExtensionIDColorSpace,
		HeaderExtensionVideoFrameTrackingID:      HeaderExtensionIDVideoFrameTrackingID,
		HeaderExtensionDependencyDescriptor:      HeaderExtensionIDGenericFrameDescriptor02,
	}
	return headers[uri]
}

func NewHerderExtensionIDs(headers []HeaderExtension) HeaderExtensionIDs {
	header := map[string]HeaderExtension{}
	for _, v := range headers {
		header[v.URI] = v
	}
	return header
}

type HeaderExtensionIDs map[string]HeaderExtension

func (h HeaderExtensionIDs) HeaderExtensions() []HeaderExtension {
	headers := make([]HeaderExtension, 0, len(h))
	for _, v := range h {
		headers = append(headers, v)
	}

	return headers
}

func (h HeaderExtensionIDs) Mid() HeaderExtensionID {
	return h[HeaderExtensionMid].ID
}

func (h HeaderExtensionIDs) Rid() HeaderExtensionID {
	return h[HeaderExtensionRid].ID
}

func (h HeaderExtensionIDs) RRid() HeaderExtensionID {
	return h[HeaderExtensionRepairedRid].ID
}

func (h HeaderExtensionIDs) AbsSendTime() HeaderExtensionID {
	return h[HeaderExtensionAbsSendTime].ID
}

func (h HeaderExtensionIDs) TransportWideCC() HeaderExtensionID {
	return h[HeaderExtensionTransportSequenceNumber].ID
}
