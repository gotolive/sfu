package sdp

const (
	ConnectionRoleNone     = ""
	ConnectionRoleActive   = "active"
	ConnectionRolePassive  = "passive"
	ConnectionRoleActpass  = "actpass"
	ConnectionRoleHoldconn = "holdconn"
)

var connectionRoles = []string{
	ConnectionRoleNone, ConnectionRoleActive, ConnectionRolePassive, ConnectionRoleActpass, ConnectionRoleHoldconn,
}

const (
	UDPProtocolName    = "udp"
	TCPProtocolName    = "tcp"
	SsltcpProtocolName = "ssltcp"
)

var protoNames = map[string]bool{
	UDPProtocolName: true, TCPProtocolName: true, SsltcpProtocolName: true,
}

const (
	// Candidate
	candidateHost  = "host"
	candidateSrflx = "srflx"
	candidatePrflx = "prflx"
	candidateRelay = "relay"

	tcpCandidateType = "tcptype"
)

var candidateTypes = map[string]bool{
	candidateHost:  true,
	candidateSrflx: true,
	candidatePrflx: true,
	candidateRelay: true,
}

const (
	sessionState = 0
	mediaState   = 1

	sdpLinePrefixLength   = 2
	sdpDelimiterSpace     = " "
	sdpDelimiterEqual     = "="
	sdpDelimiterColon     = ":"
	sdpDelimiterSlashChar = "/"
	sdpDelimiterSemicolon = ";"

	lineTypeVersion          = "v"
	lineTypeOrigin           = "o"
	lineTypeSessionName      = "s"
	lineTypeSessionInfo      = "i"
	lineTypeSessionURI       = "u"
	lineTypeSessionEmail     = "e"
	lineTypeSessionPhone     = "p"
	lineTypeSessionBandwidth = "b"
	lineTypeTiming           = "t"
	lineTypeRepeatTimes      = "r"
	lineTypeTimeZone         = "z"
	lineTypeEncryptionKey    = "k"
	lineTypeMedia            = "m"
	lineTypeConnection       = "c"
	lineTypeAttributes       = "a"

	attributeGroup            = "group"
	attributeMid              = "mid"
	attributeMsid             = "msid"
	attributeRtcpMux          = "rtcp-mux"
	attributeRtcpReducedSize  = "rtcp-rsize"
	attributeSsrc             = "ssrc"
	ssrcAttributeCname        = "cname"
	attributeExtmapAllowMixed = "extmap-allow-mixed"
	attributeExtmap           = "extmap"

	attributeMsidSemantics = "msid-semantic"
	ssrcAttributeMslabel   = "mslabel"
	ssrcAttributeLabel     = "label"
	attributeSsrcGroup     = "ssrc-group"
	attributeCandidate     = "candidate"
	attributeEOFCandidate  = "end-of-candidates"
	attributeCandidateTyp  = "typ"
	attributeFingerprint   = "fingerprint"
	attributeSetup         = "setup"
	attributeFmtp          = "fmtp"
	attributeRtpmap        = "rtpmap"
	attributeRtcp          = "rtcp"
	attributeIceUfrag      = "ice-ufrag"
	attributeIcePwd        = "ice-pwd"
	attributeIceLite       = "ice-lite"
	attributeIceOption     = "ice-options"
	attributeSendOnly      = "sendonly"
	attributeRecvOnly      = "recvonly"
	attributeRtcpFb        = "rtcp-fb"
	attributeSendRecv      = "sendrecv"
	attributeInactive      = "inactive"

	// draft-ietf-mmusic-rid-15
	// a=rid
	attributeRid = "rid"
	// draft-ietf-mmusic-sdp-simulcast-13
	// a=simulcast
	attributeSimulcast = "simulcast"
)

const (
	sendDirection    = "send"
	receiveDirection = "recv"
)

const (
	encryptHeaderExtensions = "urn:ietf:params:rtp-hdrext:encrypt"
)

const (
	mediaProtocolRTPPrefix = "RTP/"
)

type IceMode int

const (
	IceModeLite = IceMode(1)
)
