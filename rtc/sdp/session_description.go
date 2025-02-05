package sdp

import (
	"strconv"
)

type HeaderExtension struct {
	URI     string
	ID      uint8
	Encrypt bool
}
type Fingerprint struct {
	Algorithm string
	Value     string
}

type Candidate struct {
	ID             string
	Component      int
	Protocol       string
	RelayProtocol  string
	Address        string
	Priority       uint32
	Username       string
	Password       string
	Type           string
	NetworkName    string
	Generation     uint32
	Foundation     string
	RelatedAddress string
	TCPType        string
	TransportName  string
	NetworkID      uint16
	NetworkCost    uint16
	URL            string
}

type TransportInfo struct {
	IceUfrag         string
	IcePwd           string
	IceMode          IceMode
	TransportOptions []string
	ConnectionRole   string
	FingerPrint      *Fingerprint
	Candidates       []Candidate
}

type SessionDescription struct {
	ExtmapAllowMixed bool
	MsidSupported    bool
	TransportInfo    TransportInfo
	MediaDescription []*MediaDescription
}

type FeedbackParams struct {
	ID     string
	Params string
}

type ssrcInfo struct {
	SSRC     uint32
	RTX      uint32
	Cname    string
	StreamID string
	TrackID  string

	Label   string
	MsLabel string
}

type Codec struct {
	PayloadType    uint8
	EncoderName    string
	ClockRate      int
	Channel        int // only worked for audio
	Parameters     map[string]string
	FeedbackParams []FeedbackParams
	RTX            uint8
}

type ssrcGroup struct {
	Semantics string
	SSRCs     []uint32
}

type ridDescription struct {
	rid          string
	ridDirection string
}

type StreamParams struct {
	SSRC  uint32
	RTX   uint32
	Cname string
	RID   string
}

type MediaDescription struct {
	MediaType        string
	MID              string
	RtcpMux          bool
	RtcpReducedSize  bool
	Direction        string
	HeaderExtensions []HeaderExtension
	Codecs           map[uint8]*Codec
	TrackID          string
	Streams          []StreamParams
	ssrcInfo         map[uint32]*ssrcInfo
	ssrcGroup        []ssrcGroup
	rids             []ridDescription
	streamIds        []string
}

func (d *MediaDescription) updateCodec() error {
	for pt, v := range d.Codecs {
		if v.EncoderName == "rtx" {
			for k, p := range v.Parameters {
				if k == "apt" {
					apt, err := strconv.Atoi(p)
					if err != nil {
						return err
					}
					d.Codecs[uint8(apt)].RTX = pt
				}
			}
		}
	}
	return nil
}

func (d *MediaDescription) updateSendStreams() error {
	if len(d.ssrcInfo) != 0 {
		// we are ssrc based
		// process rtx
		for _, sg := range d.ssrcGroup {
			if sg.Semantics == "FID" {
				if len(sg.SSRCs) != 2 {
					return ErrInvalidFID
				}
				d.ssrcInfo[sg.SSRCs[0]].RTX = sg.SSRCs[1]
				delete(d.ssrcInfo, sg.SSRCs[1])
			}
		}
		// create stream
		for _, ssrc := range d.ssrcInfo {
			d.Streams = append(d.Streams, StreamParams{
				SSRC:  ssrc.SSRC,
				RTX:   ssrc.RTX,
				Cname: ssrc.Cname,
			})
			// they should share the track id.
			d.TrackID = ssrc.TrackID
		}
	}
	if len(d.rids) != 0 {
		// we are rid based
		for _, rid := range d.rids {
			d.Streams = append(d.Streams, StreamParams{
				RID: rid.rid,
			})
		}
	}
	return nil
}

func (s *SessionDescription) Marshal() (string, error) {
	return "", nil
}

func (s *SessionDescription) Unmarshal(sdp string) error {
	um := new(unmarshaler)
	return um.Unmarshal(sdp, s)
}

func Unmarshal(sdp string) (*SessionDescription, error) {
	sd := new(SessionDescription)
	if err := sd.Unmarshal(sdp); err != nil {
		return nil, err
	}
	return sd, nil
}

// TODO not support yet
func Marshal(sdp *SessionDescription) (string, error) {
	return sdp.Marshal()
}
