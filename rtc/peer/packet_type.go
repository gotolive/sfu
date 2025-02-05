package peer

type PacketType string

const (
	RTCP    = "rtcp"
	RTP     = "rtp"
	DTLS    = "dtls"
	Unknown = "unknown"
)

func IsDtls(data []byte) bool {
	return len(data) > 13 && (data[0] > 19 && data[0] < 64)
}

func CheckPacket(data []byte) PacketType {
	if IsDtls(data) {
		return DTLS
	}
	if IsRtcp(data) {
		return RTCP
	}
	if MatchSRTP(data) {
		return RTP
	}
	return Unknown
}

// MatchSRTPOrSRTCP is a MatchFunc that accepts packets with the first byte in [128..191]
// as defied in RFC7983.
func MatchSRTPOrSRTCP(b []byte) bool {
	return MatchRange(128, 191)(b)
}

func MatchRange(lower, upper byte) MatchFunc {
	return func(buf []byte) bool {
		if len(buf) < 1 {
			return false
		}
		b := buf[0]
		return b >= lower && b <= upper
	}
}

type MatchFunc func([]byte) bool

func isRTCP(buf []byte) bool {
	// Not long enough to determine RTP/RTCP
	if len(buf) < 4 {
		return false
	}
	if buf[1] >= 192 && buf[1] <= 223 {
		return true
	}
	return false
}

// MatchSRTP is a MatchFunc that only matches SRTP and not SRTCP.
func MatchSRTP(buf []byte) bool {
	return MatchSRTPOrSRTCP(buf) && !isRTCP(buf)
}

// IsRtcp is a MatchFunc that only matches SRTCP and not SRTP.
func IsRtcp(buf []byte) bool {
	return MatchSRTPOrSRTCP(buf) && isRTCP(buf)
}
