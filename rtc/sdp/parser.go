package sdp

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/gotolive/sfu/rtc"
)

var (
	ErrSDPParseFail   = errors.New("parse line fail")
	ErrUnknownSection = errors.New("unknown section")
	ErrEmptySDP       = errors.New("empty sdp")
	ErrInvalidFID     = errors.New("invalid fid ssrc-group")
)

type unmarshaler struct {
	index  int
	mindex int
	state  int
	lines  []string
}

func (u *unmarshaler) Unmarshal(raw string, sdp *SessionDescription) error {
	raw = strings.TrimSuffix(raw, "\r\n")
	raw = strings.TrimSuffix(raw, "\n")
	raw = strings.ReplaceAll(raw, "\r\n", "\n")
	if raw == "" {
		return ErrEmptySDP
	}
	u.lines = strings.Split(raw, "\n")
	for u.index < len(u.lines) {
		if err := u.parseSection(sdp); err != nil {
			return err
		}
	}
	for _, media := range sdp.MediaDescription {
		if err := media.updateSendStreams(); err != nil {
			return err
		}
		if err := media.updateCodec(); err != nil {
			return err
		}
	}
	return nil
}

func (u *unmarshaler) parseSection(sdp *SessionDescription) error {
	switch u.state {
	case sessionState:
		if err := u.parseSession(sdp); err != nil {
			return err
		}
		u.state++
		return nil
	case mediaState:
		if err := u.parseMedia(sdp); err != nil {
			return err
		}
		return nil
	}
	return ErrUnknownSection
}

// Do we validate the sdp or not?
func (u *unmarshaler) parseSession(sdp *SessionDescription) error {
	for {
		// we reach the EOF
		if u.index == len(u.lines) {
			return nil
		}
		if !isValidLine(u.lines[u.index]) {
			return failParse(u.lines[u.index])
		}
		lineType := u.lineType()
		// we reach the media section
		if lineType == lineTypeMedia {
			return nil
		}
		// we don't know the line type
		parser := u.sessionParser(lineType)
		err := parser(u.lines[u.index], sdp)
		if err != nil {
			return err
		}
		u.index++
	}
}

func (u *unmarshaler) parseMedia(sdp *SessionDescription) error {
	if err := u.parseMline(sdp); err != nil {
		return err
	}
	u.index++
	for {
		if u.index == len(u.lines) {
			return nil
		}
		if !isValidLine(u.lines[u.index]) {
			return failParse(u.lines[u.index])
		}
		lineType := u.lineType()
		// we reach the media section
		if lineType == lineTypeMedia {
			u.mindex++
			return nil
		}
		// we don't know the line type
		if parser := u.mediaParser(lineType); parser == nil {
			return failParse(u.lines[u.index])
		} else {
			err := parser(u.lines[u.index], sdp)
			if err != nil {
				return err
			}
		}
		u.index++
	}
}

type parseFunc func(string, *SessionDescription) error

func (u *unmarshaler) lineType() string {
	return string(u.lines[u.index][0])
}

func (u *unmarshaler) parseMline(sdp *SessionDescription) error {
	line := u.lines[u.index]
	fields := strings.Split(line[sdpLinePrefixLength:], sdpDelimiterSpace)
	if len(fields) < 4 {
		return failParse(line)
	}
	// m=audio 9 UDP/TLS/RTP/SAVPF 111 103 104 9 102 0 8 106 105 13 110 112 113 126
	mediaType := fields[0]

	desc := MediaDescription{
		MediaType: mediaType,
		Codecs:    map[uint8]*Codec{},
		ssrcInfo:  map[uint32]*ssrcInfo{},
	}

	if isRTPProtocol(fields[2]) {
		for j := 3; j < len(fields); j++ {
			pt, err := strconv.Atoi(fields[j])
			if err != nil {
				return err
			}
			desc.Codecs[uint8(pt)] = &Codec{PayloadType: uint8(pt)}
		}
	}
	sdp.MediaDescription = append(sdp.MediaDescription, &desc)
	return nil
}

func (u *unmarshaler) mediaParser(lineType string) parseFunc {
	switch lineType {
	case lineTypeConnection, lineTypeSessionBandwidth:
		return emptyParser
	case lineTypeAttributes:
		return mediaAttributeParser(u.lines[u.index])
	}

	return nil
}

func (u *unmarshaler) sessionParser(lineType string) parseFunc {
	switch lineType {
	case lineTypeVersion,
		lineTypeOrigin,
		lineTypeSessionName,
		lineTypeSessionInfo,
		lineTypeSessionURI,
		lineTypeSessionPhone,
		lineTypeSessionEmail,
		lineTypeConnection,
		lineTypeSessionBandwidth,
		lineTypeTiming,
		lineTypeRepeatTimes,
		lineTypeTimeZone,
		lineTypeEncryptionKey:
		return emptyParser
	case lineTypeAttributes:
		return sessionAttributeParser(u.lines[u.index])
	default:
		return errorParser
	}
}

func sessionAttributeParser(line string) parseFunc {
	attr := getAttr(line)
	if attr == "" {
		return errorParser
	}
	switch attr {
	case attributeGroup:
		return emptyParser
	case attributeMsidSemantics:
		return emptyParser
	case attributeExtmapAllowMixed:
		return extmapAllowMixedParser
	case attributeIceUfrag:
		return iceUfragParser
	case attributeIcePwd:
		return icePwdParser
	case attributeIceLite:
		return iceLiteParser
	case attributeIceOption:
		return iceOptionParser
	case attributeCandidate:
		return candidateParser
	case attributeEOFCandidate:
		return emptyParser
	case attributeFingerprint:
		return fingerprintParser
	case attributeSetup:
		return dtlsRoleParser
	default:
		return errorParser
	}
}

func mediaAttributeParser(line string) parseFunc {
	attr := getAttr(line)
	if attr == "" {
		return errorParser
	}
	switch attr {
	case attributeExtmap:
		return extmapParser
	case attributeRtpmap:
		return rtpmapParser
	case attributeFmtp:
		return fmtpParser
	case attributeMid:
		return midParser
	case attributeRtcpMux:
		return rtcpMuxParser
	case attributeRtcpReducedSize:
		return rtcpRSizeParser
	case attributeIceUfrag:
		return iceUfragParser
	case attributeIcePwd:
		return icePwdParser
	case attributeIceLite:
		return iceLiteParser
	case attributeIceOption:
		return iceOptionParser
	case attributeCandidate:
		return candidateParser
	case attributeEOFCandidate:
		return emptyParser
	case attributeFingerprint:
		return fingerprintParser
	case attributeSetup:
		return dtlsRoleParser
	case attributeRecvOnly, attributeSendOnly, attributeSendRecv, attributeInactive:
		return directionParser
	case attributeRtcpFb:
		return rtcpFbParser
	case attributeSsrc:
		return ssrcParser
	case attributeSsrcGroup:
		return ssrcGroupParser
	case attributeSimulcast:
		return simulcastParser
	case attributeRid:
		return ridParser
	case attributeMsid:
		return msidParser
	case attributeRtcp:
		return emptyParser

	default:
		return errorParser
	}
}

// we do not parse simulcast, if the len(streams)>0, we consider it simulcast.
func simulcastParser(line string, description *SessionDescription) error {
	return nil
}

// a=rid:1 send pt=103;max-width=1280;max-height=720;max-fps=30
// we don't check the direction and restriction.
// as we said, this is only a helper method, sdp correctness should be promised.
// https://tools.ietf.org/html/draft-ietf-mmusic-rid-15
func ridParser(line string, description *SessionDescription) error {
	mediaDesc := description.MediaDescription[len(description.MediaDescription)-1]
	if len(line) == sdpLinePrefixLength+len(attributeRid) {
		return failParse(line)
	}
	tokens := strings.Split(line[sdpLinePrefixLength+len(attributeRid)+1:], sdpDelimiterSpace)
	if len(tokens) < 2 || len(tokens) > 3 {
		return failParse(line)
	}
	if len(tokens[0]) > 16 || tokens[0] == "" {
		return failParse(line)
	}
	if tokens[1] != sendDirection && tokens[1] != receiveDirection {
		return failParse(line)
	}
	rid := ridDescription{
		rid:          tokens[0],
		ridDirection: tokens[1],
	}
	mediaDesc.rids = append(mediaDesc.rids, rid)
	return nil
}

func msidParser(line string, description *SessionDescription) error {
	mediaDesc := description.MediaDescription[len(description.MediaDescription)-1]
	fields := strings.SplitN(line[sdpLinePrefixLength:], sdpDelimiterSpace, 2)
	if len(fields) != 2 {
		return failParse(line)
	}
	if fields[1] == "" {
		return failParse(line)
	}
	trackID := fields[1]
	description.MediaDescription[len(description.MediaDescription)-1].TrackID = trackID

	streamID, err := getValue(line, fields[0], attributeMsid)
	if err != nil {
		return err
	}
	if streamID == "" {
		return failParse(line)
	}
	if streamID != "-" {
		mediaDesc.streamIds = append(mediaDesc.streamIds, streamID)
	}
	return nil
}

func failParse(line string) error {
	return fmt.Errorf("%w:%s", ErrSDPParseFail, line)
}

func errorParser(line string, description *SessionDescription) error {
	return failParse(line)
}

// a=extmap:<value>["/"<direction>] <URI> <extensionattributes>
// a=extmap:2 http://www.webrtc.org/experiments/rtp-hdrext/abs-send-time
func extmapParser(line string, description *SessionDescription) error {
	mediaDesc := description.MediaDescription[len(description.MediaDescription)-1]
	fields := strings.Split(line[sdpLinePrefixLength:], sdpDelimiterSpace)
	if len(fields) < 2 {
		return failParse(line)
	}
	uri := fields[1]
	valueDirection, err := getValue(line, fields[0], attributeExtmap)
	if err != nil {
		return err
	}
	subFields := strings.Split(valueDirection, sdpDelimiterSlashChar)
	if len(subFields) == 0 {
		return failParse(line)
	}

	value, err := strconv.Atoi(subFields[0])
	if err != nil {
		return err
	}
	var encrypt bool
	if uri == encryptHeaderExtensions {
		if len(fields) < 3 {
			return failParse(line)
		}
		encrypt = true
		uri = fields[2]
		if uri == encryptHeaderExtensions {
			return failParse(line)
		}
	}
	mediaDesc.HeaderExtensions = append(mediaDesc.HeaderExtensions, HeaderExtension{
		URI:     uri,
		ID:      uint8(value),
		Encrypt: encrypt,
	})

	return nil
}

// a=rtpmap:111 opus/48000/2
func rtpmapParser(line string, description *SessionDescription) error {
	fields := strings.Split(line[sdpLinePrefixLength:], sdpDelimiterSpace)
	if len(fields) < 2 {
		return failParse(line)
	}
	pt, err := getValue(line, fields[0], attributeRtpmap)
	if err != nil {
		return err
	}
	payloadType, err := strconv.Atoi(pt)
	if err != nil {
		return err
	}

	codec, ok := description.MediaDescription[len(description.MediaDescription)-1].Codecs[uint8(payloadType)]
	if !ok {
		// ignore it
		return failParse(line)
	}
	codecParams := strings.Split(fields[1], "/")
	if len(codecParams) < 2 || len(codecParams) > 3 {
		return failParse(line)
	}
	encoderName := codecParams[0]
	clockRate, err := strconv.Atoi(codecParams[1])
	if err != nil {
		return err
	}
	channel := 1

	if description.MediaDescription[len(description.MediaDescription)-1].MediaType == "audio" {
		if len(codecParams) == 3 {
			channel, err = strconv.Atoi(codecParams[2])
			if err != nil {
				return err
			}
		}
	}

	codec.EncoderName = encoderName
	codec.ClockRate = clockRate
	codec.Channel = channel

	return nil
}

// a=fmtp:99 apt=98
// a=fmtp:63 111/111
// a=fmtp:100 level-asymmetry-allowed=1;packetization-mode=1;profile-level-id=42e01f
func fmtpParser(line string, description *SessionDescription) error {
	fields := strings.SplitN(line[sdpLinePrefixLength:], sdpDelimiterSpace, 2)
	if len(fields) != 2 {
		return failParse(line)
	}
	payloadStr, err := getValue(line, fields[0], attributeFmtp)
	if err != nil {
		return err
	}
	payloadType, err := strconv.Atoi(payloadStr)
	if err != nil {
		return err
	}
	codec, ok := description.MediaDescription[len(description.MediaDescription)-1].Codecs[uint8(payloadType)]
	if !ok {
		// ignore it
		return failParse(line)
	}

	params := strings.Split(fields[1], sdpDelimiterSemicolon)
	if codec.Parameters == nil {
		codec.Parameters = make(map[string]string)
	}
	for _, v := range params {
		if c := strings.TrimSpace(v); c != "" {
			kv := strings.Split(c, sdpDelimiterEqual)
			// it should be less than 2
			switch len(kv) {
			case 1:
				codec.Parameters[kv[0]] = "" // mark it exist
			case 2:
				codec.Parameters[kv[0]] = kv[1]
			default:
				return failParse(line)
			}
		}
	}

	return nil
}

func midParser(line string, description *SessionDescription) error {
	v, err := getValue(line, line, attributeMid)
	if err != nil {
		return err
	}

	description.MediaDescription[len(description.MediaDescription)-1].MID = v
	return nil
}

func iceUfragParser(line string, description *SessionDescription) error {
	v, err := getValue(line, line, attributeIceUfrag)
	if err != nil {
		return err
	}

	description.TransportInfo.IceUfrag = v
	return nil
}

func icePwdParser(line string, description *SessionDescription) error {
	v, err := getValue(line, line, attributeIcePwd)
	if err != nil {
		return err
	}

	description.TransportInfo.IcePwd = v
	return nil
}

func iceLiteParser(_ string, description *SessionDescription) error {
	description.TransportInfo.IceMode = IceModeLite
	return nil
}

func iceOptionParser(line string, description *SessionDescription) error {
	v, err := getValue(line, line, attributeIceOption)
	if err != nil {
		return err
	}
	options := strings.Split(v, sdpDelimiterSpace)
	description.TransportInfo.TransportOptions = append(description.TransportInfo.TransportOptions, options...)
	return nil
}

// a=candidate:udpcandidate 1 udp 1076302079 1.1.1.1 30002 typ host
// a=end-of-candidates
func candidateParser(line string, description *SessionDescription) error {
	if strings.HasPrefix(line, "a=") {
		line = line[sdpLinePrefixLength:]
	}
	attr := strings.SplitN(line, sdpDelimiterColon, 2)
	if len(attr) != 2 || attr[0] != attributeCandidate {
		return failParse(line)
	}

	fields := strings.Split(attr[1], sdpDelimiterSpace)
	expectedMinFields := 8
	if len(fields) < expectedMinFields || fields[6] != attributeCandidateTyp {
		return failParse(line)
	}
	foundation := fields[0]
	componentID, err := strconv.Atoi(fields[1])
	if err != nil {
		return err
	}
	protocol := fields[2]
	priority, err := strconv.Atoi(fields[3])
	if err != nil {
		return err
	}
	connectionAddr := fields[4]
	port, err := strconv.Atoi(fields[5])
	if err != nil || !isValidPort(port) {
		return failParse(line)
	}
	if !protoNames[protocol] {
		return failParse(line)
	}

	tcp := false

	if protocol == TCPProtocolName || protocol == SsltcpProtocolName {
		tcp = true
	}
	candidateType := fields[7]
	if !candidateTypes[candidateType] {
		return failParse(line)
	}
	currentPosition := expectedMinFields

	var tcpType string
	if len(fields) >= currentPosition+2 && fields[currentPosition] == tcpCandidateType {
		tcpType = fields[currentPosition+1]
	} else if tcp {
		tcpType = "passive"
	}

	candidate := Candidate{
		Component:  componentID,
		Protocol:   protocol,
		Address:    connectionAddr + ":" + strconv.Itoa(port),
		Priority:   uint32(priority),
		Type:       candidateType,
		Foundation: foundation,
		TCPType:    tcpType,
	}

	description.TransportInfo.Candidates = append(description.TransportInfo.Candidates, candidate)

	return nil
}

func fingerprintParser(line string, description *SessionDescription) error {
	fields := strings.Split(line[sdpLinePrefixLength:], sdpDelimiterSpace)
	if len(fields) != 2 {
		return failParse(line)
	}
	alg, err := getValue(line, fields[0], attributeFingerprint)
	if err != nil {
		return err
	}

	description.TransportInfo.FingerPrint = &Fingerprint{
		Algorithm: strings.ToLower(alg),
		Value:     fields[1],
	}
	return nil
}

func dtlsRoleParser(line string, description *SessionDescription) error {
	parts := strings.Split(line[2:], ":")
	if len(parts) != 2 {
		return failParse(line)
	}

	for _, v := range connectionRoles {
		if strings.ToLower(parts[1]) == v {
			description.TransportInfo.ConnectionRole = v
			return nil
		}
	}

	return failParse(line)
}

func directionParser(line string, description *SessionDescription) error {
	description.MediaDescription[len(description.MediaDescription)-1].Direction = line[2:]
	return nil
}

func rtcpMuxParser(_ string, description *SessionDescription) error {
	description.MediaDescription[len(description.MediaDescription)-1].RtcpMux = true
	return nil
}

func rtcpRSizeParser(_ string, description *SessionDescription) error {
	description.MediaDescription[len(description.MediaDescription)-1].RtcpReducedSize = true
	return nil
}

func extmapAllowMixedParser(_ string, description *SessionDescription) error {
	description.ExtmapAllowMixed = true
	return nil
}

func rtcpFbParser(line string, description *SessionDescription) error {
	media := description.MediaDescription[len(description.MediaDescription)-1]
	if media.MediaType != rtc.MediaTypeAudio && media.MediaType != rtc.MediaTypeVideo {
		return failParse(line)
	}
	fields := strings.Split(line, sdpDelimiterSpace)
	if len(fields) < 2 {
		return failParse(line)
	}
	// it could be * wildcard
	payloadTypeStr, err := getValue(line, fields[0], attributeRtcpFb)
	if err != nil {
		return err
	}

	payloadType := -1
	if payloadTypeStr != "*" {
		payloadType, err = strconv.Atoi(payloadTypeStr)
		if err != nil {
			return err
		}
	}

	codec, ok := description.MediaDescription[len(description.MediaDescription)-1].Codecs[uint8(payloadType)]
	if !ok {
		// ignore it
		return failParse(line)
	}

	params := ""
	if len(fields) > 2 {
		params = strings.Join(fields[2:], " ")
	}
	codec.FeedbackParams = append(codec.FeedbackParams, FeedbackParams{
		ID:     fields[1],
		Params: params,
	})
	return nil
}

// a=ssrc:1687778075 cname:/BVOQPCIu5+FZpRn
// a=ssrc:1687778075 msid:92864685-BC0C-49A8-AF2A-D3625BF2C51D 92864685-BC0C-49A8-AF2A-D3625BF2C51D
// a=ssrc:1687778075 mslabel:92864685-BC0C-49A8-AF2A-D3625BF2C51D
// a=ssrc:1687778075 label:92864685-BC0C-49A8-AF2A-D3625BF2C51D
func ssrcParser(line string, description *SessionDescription) error {
	fields := strings.SplitN(line[sdpLinePrefixLength:], sdpDelimiterSpace, 2)
	if len(fields) != 2 {
		return failParse(line)
	}
	v, err := getValue(line, fields[0], attributeSsrc)
	if err != nil {
		return err
	}
	ssrcInt, err := strconv.Atoi(v)
	if err != nil {
		return err
	}
	ssrc := uint32(ssrcInt)
	attr := strings.SplitN(fields[1], sdpDelimiterColon, 2)
	if len(attr) != 2 {
		return failParse(line)
	}

	ssrcInfos := description.MediaDescription[len(description.MediaDescription)-1].ssrcInfo

	if ssrcInfos[ssrc] == nil {
		ssrcInfos[ssrc] = &ssrcInfo{
			SSRC: ssrc,
		}
	}
	switch attr[0] {
	case ssrcAttributeCname:
		ssrcInfos[ssrc].Cname = attr[1]
	case attributeMsid:
		msidFields := strings.Split(attr[1], sdpDelimiterSpace)
		if len(msidFields) < 1 || len(fields) > 2 {
			return failParse(line)
		}
		ssrcInfos[ssrc].StreamID = msidFields[0]
		if len(msidFields) == 2 {
			ssrcInfos[ssrc].TrackID = fields[1]
		}
	case ssrcAttributeMslabel:
		ssrcInfos[ssrc].MsLabel = attr[1]
	case ssrcAttributeLabel:
		ssrcInfos[ssrc].Label = attr[1]
	}

	return nil
}

// a=ssrc-group:FID 4180466998 3681735331
func ssrcGroupParser(line string, description *SessionDescription) error {
	mediaDesc := description.MediaDescription[len(description.MediaDescription)-1]
	fields := strings.Split(line[sdpLinePrefixLength:], sdpDelimiterSpace)
	if len(fields) < 2 {
		return failParse(line)
	}
	semantics, err := getValue(line, fields[0], attributeSsrcGroup)
	if err != nil {
		return err
	}
	sg := ssrcGroup{
		Semantics: semantics,
	}
	for i := 1; i < len(fields); i++ {
		v, e := strconv.Atoi(fields[i])
		if e != nil {
			return e
		}
		sg.SSRCs = append(sg.SSRCs, uint32(v))
	}

	mediaDesc.ssrcGroup = append(mediaDesc.ssrcGroup, sg)

	return nil
}

func getAttr(line string) string {
	if len(line) < 3 {
		return ""
	}
	index := strings.Index(line, ":")
	if index == -1 {
		return line[2:]
	}
	return line[2:index]
}

func getValue(line, kv, attr string) (string, error) {
	if parts := strings.SplitN(kv, ":", 2); len(parts) == 2 && strings.HasSuffix(parts[0], attr) {
		return parts[1], nil
	}
	return "", failParse(line)
}

func emptyParser(line string, description *SessionDescription) error {
	return nil
}

func isRTPProtocol(protocol string) bool {
	if len(protocol) == 0 {
		return true
	}
	return strings.Contains(protocol, mediaProtocolRTPPrefix)
}

func isValidPort(port int) bool {
	return port >= 0 && port <= 65535
}

func isValidLine(line string) bool {
	return len(line) >= 3 && line[1] == '='
}
