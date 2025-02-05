package ice

const (
	iceCandidateDefaultLocalPriority = 10000
	iceHostTypePreference            = 64
	iceRTPComponent                  = 1 // RTP=1 RTCP=2
	iceUDPPrefer                     = 1000
)

// Parameters only used for return ice info to generate sdp.
type Parameters struct {
	UsernameFragment string
	Password         string
	Candidates       []Candidate
	Role             string
	Lite             bool
}

// Candidate represents an ICE candidate
type Candidate struct {
	Type       string
	Protocol   string
	IP         string
	Port       uint16
	Priority   int
	Foundation string
}

func buildCandidate(protocol string, ip string, port uint16, iceLocalPreferenceDecrement int) Candidate {
	var (
		foundation  string
		icePriority int
	)

	switch protocol {
	case UDP:
		// we prefer udp so we add 1000 for the cal
		foundation = CandidateFoundationUDP
		iceLocalPreference := iceCandidateDefaultLocalPriority - iceLocalPreferenceDecrement + iceUDPPrefer
		icePriority = generateIceCandidatePriority(iceLocalPreference)
	case TCP:
		foundation = CandidateFoundationTCP
		iceLocalPreference := iceCandidateDefaultLocalPriority - iceLocalPreferenceDecrement
		icePriority = generateIceCandidatePriority(iceLocalPreference)
	}
	return Candidate{
		Type:       Host,
		Protocol:   protocol,
		IP:         ip,
		Port:       port,
		Priority:   icePriority,
		Foundation: foundation,
	}
}

// generateIceCandidatePriority
// priority = (2^24)*(type preference) + (2^8)*(local preference) + (2^0)*(256 - component ID)
//
//nolint:gomnd
func generateIceCandidatePriority(localPreference int) int {
	return 2<<24*iceHostTypePreference + 2<<8*localPreference + 2*(256-iceRTPComponent)
}
