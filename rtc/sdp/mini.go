package sdp

import (
	"bytes"
	"encoding/binary"
	"errors"
	"math"
)

var (
	ErrNoMiniSDP      = errors.New("not a mini sdp payload")
	ErrHeaderTooShort = errors.New("payload does not have enough header")
	ErrInvalidMiniSDP = errors.New("mini sdp is invalid")
	ErrExtOversize    = errors.New("ext oversize")
)

const (
	miniSdpMarker = "SDP"

	headerLength     = 12
	versionOffset    = 4
	subVersionOffset = 5
	flagOffset       = 7
	seqOffset        = 8
	statusOffset     = 10
)

type miniUnmarshaler struct {
	raw    []byte
	offset int
}

func (u *miniUnmarshaler) Unmarshal(raw []byte, m *ms) error {
	u.raw = raw
	if err := u.parseHeader(m); err != nil {
		return err
	}
	if err := u.parseSessionHeader(m); err != nil {
		return err
	}
	for i := uint8(0); i < m.sessionHeader.mediaNum; i++ {
		if err := u.parseMedia(m); err != nil {
			return err
		}
	}

	return nil
}

func (u *miniUnmarshaler) parseHeader(m *ms) error {
	if len(u.raw) < headerLength {
		return ErrHeaderTooShort
	}
	u.offset = headerLength
	m.msHeader.version = u.raw[versionOffset]
	m.msHeader.subVersion = uint16(u.raw[subVersionOffset])<<8 + uint16(u.raw[subVersionOffset+1])
	// fmt.Printf("%08b\n", b)
	m.msHeader.sdpType = u.raw[flagOffset] >> 6 & 0b11
	m.msHeader.planType = u.raw[flagOffset] >> 5 & 0b1
	m.msHeader.flag = u.raw[flagOffset] & 0b00011111
	m.msHeader.seq = uint16(u.raw[seqOffset])<<8 | uint16(u.raw[seqOffset+1])
	m.msHeader.status = uint16(u.raw[statusOffset])<<8 | uint16(u.raw[statusOffset+1])
	return nil
}

func (u *miniUnmarshaler) parseSessionHeader(m *ms) error {
	flagByte := u.raw[u.offset]
	m.sessionHeader.e = flagByte&0b10000000 > 0
	m.sessionHeader.c = flagByte&0b01000000 > 0
	m.sessionHeader.s = flagByte&0b00100000 > 0
	m.sessionHeader.immsend = flagByte&0b00010000 > 0
	m.sessionHeader.role = flagByte >> 2 & 0b11
	m.sessionHeader.direction = flagByte & 0b11
	u.offset++
	m.sessionHeader.candidateNum = u.raw[u.offset]
	u.offset++
	m.sessionHeader.reversed = u.raw[u.offset]
	u.offset++
	if (m.sessionHeader.c && m.sessionHeader.candidateNum == 0) || (!m.sessionHeader.c || m.sessionHeader.candidateNum != 0) {
		return ErrInvalidMiniSDP
	}
	u.parseCandidate(m)

	m.sessionHeader.sessionCustomExtension = u.parseCustomExt()

	return nil
}

func (u *miniUnmarshaler) parseCustomExt() customExtensions {
	c := customExtensions{}
	c.length = u.raw[u.offset]
	c.bitMapSize = u.raw[u.offset+1]
	u.offset += 2
	end := u.offset + int(c.length)
	for u.offset < end {
		ext := struct {
			strLen uint16
			id     uint8
			str    string
		}{
			strLen: uint16(u.raw[u.offset])<<8 + uint16(u.raw[u.offset+1]),
			id:     u.raw[u.offset+2],
		}
		ext.str = string(u.raw[u.offset+3 : u.offset+3+int(ext.strLen)])
		c.exts = append(c.exts, ext)
		u.offset += 3 + int(ext.strLen)
	}
	return c
}

func (u *miniUnmarshaler) parseCandidate(m *ms) {
	for i := uint8(0); i < m.sessionHeader.candidateNum; i++ {
		offset := u.offset
		c := struct {
			ipType        uint8
			candidateFlag uint8
			port          uint16
			ip            []byte
		}{
			ipType:        u.raw[u.offset] >> 7 & 0b1,
			candidateFlag: u.raw[u.offset] & 0b01111111,
			port:          uint16(u.raw[u.offset+1])<<8 + uint16(u.raw[u.offset+2]),
		}
		if c.ipType > 0 {
			c.ip = make([]byte, 16)
			u.offset += 19
		} else {
			c.ip = make([]byte, 4)
			u.offset += 7
		}
		m.sessionHeader.candidates = append(m.sessionHeader.candidates, c)
		copy(c.ip, u.raw[offset+3:])
	}
}

func (u *miniUnmarshaler) parseMedia(m *ms) error {
	media := mediaSection{}
	media.hasExt = u.raw[u.offset]&0b10000000 > 0
	media.trackNum = u.raw[u.offset] & 0b01111111
	u.offset++
	media.mediaType = u.raw[u.offset] >> 6 & 0b11
	media.codecNum = u.raw[u.offset] & 0b00111111
	u.offset++
	media.rtpExtNum = u.raw[u.offset]
	u.offset++
	if media.hasExt {
		media.mediaCustomExtensions = u.parseCustomExt()
	}
	for i := uint8(0); i < media.trackNum; i++ {
		media.tracks = append(media.tracks, track{
			ssrc:     binary.BigEndian.Uint32(u.raw[u.offset:]),
			order:    u.raw[u.offset+4],
			streamID: u.raw[u.offset+5],
		})
		u.offset += 6
	}
	for i := uint8(0); i < media.codecNum; i++ {
		c := codec{}
		c.encoder = u.raw[u.offset] >> 4
		c.frequency = u.raw[u.offset] & 0b00001111
		u.offset++
		c.payloadType = u.raw[u.offset] >> 1
		c.hasExt = u.raw[u.offset]&0b1 > 0
		c.channels = u.raw[u.offset] >> 6 & 0x11
		u.offset++
		if c.hasExt {
			c.extensions = u.parseCustomExt()
		}
		media.codecs = append(media.codecs, c)
	}

	for i := uint8(0); i < media.rtpExtNum; i++ {
		media.rtpExtensions = append(media.rtpExtensions, rtpExt{
			id:  u.raw[u.offset],
			url: u.raw[u.offset+1],
		})
		u.offset += 2
	}
	m.medias = append(m.medias, media)
	return nil
}

type customExtensions struct {
	length     uint8
	bitMapSize uint8
	exts       []struct {
		strLen uint16
		id     uint8
		str    string
	}
}

type codec struct {
	encoder     uint8
	frequency   uint8
	payloadType uint8
	hasExt      bool
	channels    uint8
	extensions  customExtensions
}

type rtpExt struct {
	id  uint8
	url uint8
}

type track struct {
	ssrc     uint32
	order    uint8
	streamID uint8
}

type mediaSection struct {
	hasExt                bool
	trackNum              uint8
	mediaType             uint8
	codecNum              uint8
	rtpExtNum             uint8
	mediaCustomExtensions customExtensions
	tracks                []track
	codecs                []codec
	rtpExtensions         []rtpExt
}

type ms struct {
	// fix 12 byte
	msHeader struct {
		version    uint8
		subVersion uint16
		sdpType    uint8
		planType   uint8
		flag       uint8
		seq        uint16
		status     uint16
	}
	sessionHeader struct {
		e            bool
		c            bool
		s            bool
		immsend      bool
		role         uint8
		direction    uint8
		candidateNum uint8
		reversed     uint8
		candidates   []struct {
			ipType        uint8
			candidateFlag uint8
			port          uint16
			ip            []byte
		}
		mediaNum               uint8
		sessionCustomExtension customExtensions
	}
	medias []mediaSection
}

type miniMarshaller struct {
	buf bytes.Buffer
}

func (m *miniMarshaller) Marshal(ms *ms) ([]byte, error) {
	m.buf.WriteByte(0xff)
	m.buf.WriteString(miniSdpMarker)
	if err := m.writeHeader(ms); err != nil {
		return nil, err
	}
	if err := m.writeSessionHeader(ms); err != nil {
		return nil, err
	}

	for _, media := range ms.medias {
		if err := m.writeMedia(media); err != nil {
			return nil, err
		}
	}
	return m.buf.Bytes(), nil
}

func (m *miniMarshaller) writeHeader(ms *ms) error {
	m.buf.WriteByte(ms.msHeader.version)
	if err := binary.Write(&m.buf, binary.BigEndian, ms.msHeader.subVersion); err != nil {
		return err
	}
	m.buf.WriteByte(ms.msHeader.sdpType<<6 | ms.msHeader.planType<<5 | ms.msHeader.flag)

	if err := binary.Write(&m.buf, binary.BigEndian, ms.msHeader.seq); err != nil {
		return err
	}
	if err := binary.Write(&m.buf, binary.BigEndian, ms.msHeader.status); err != nil {
		return err
	}
	if m.buf.Len() != headerLength {
		return ErrHeaderTooShort
	}
	return nil
}

func (m *miniMarshaller) writeSessionHeader(ms *ms) error {
	var flag byte
	if ms.sessionHeader.e {
		flag |= 0b10000000
	}
	if ms.sessionHeader.c {
		flag |= 0b01000000
	}
	if ms.sessionHeader.s {
		flag |= 0b00100000
	}
	if ms.sessionHeader.immsend {
		flag |= 0b00010000
	}
	flag |= ms.sessionHeader.role << 2
	flag |= ms.sessionHeader.direction
	m.buf.WriteByte(flag)
	m.buf.WriteByte(ms.sessionHeader.candidateNum)
	m.buf.WriteByte(ms.sessionHeader.reversed) // reversed

	for _, c := range ms.sessionHeader.candidates {
		m.buf.WriteByte(c.ipType<<7 | c.candidateFlag)
		if err := binary.Write(&m.buf, binary.BigEndian, c.port); err != nil {
			return err
		}
		m.buf.Write(c.ip)
	}
	return m.writeCustomExtension(ms.sessionHeader.sessionCustomExtension)
}

func (m *miniMarshaller) writeCustomExtension(extension customExtensions) error {
	length := 1
	for _, e := range extension.exts {
		length += len(e.str) + 3
	}
	if length > math.MaxUint8 {
		return ErrExtOversize
	}
	m.buf.WriteByte(byte(length))
	m.buf.WriteByte(extension.bitMapSize)
	for _, e := range extension.exts {
		if err := binary.Write(&m.buf, binary.BigEndian, e.strLen); err != nil {
			return err
		}
		m.buf.WriteByte(e.id)
		m.buf.WriteString(e.str)
	}
	return nil
}

func (m *miniMarshaller) writeMedia(media mediaSection) error {
	flag := media.trackNum
	if media.hasExt {
		flag |= 0b10000000
	}
	m.buf.WriteByte(flag)
	m.buf.WriteByte(media.mediaType<<6 | media.codecNum)
	m.buf.WriteByte(media.rtpExtNum)
	if media.hasExt {
		if err := m.writeCustomExtension(media.mediaCustomExtensions); err != nil {
			return err
		}
	}
	for _, t := range media.tracks {
		if err := binary.Write(&m.buf, binary.BigEndian, t.ssrc); err != nil {
			return err
		}
		m.buf.WriteByte(t.order)
		m.buf.WriteByte(t.streamID)
	}
	for _, c := range media.codecs {
		m.buf.WriteByte(c.encoder<<4 | c.frequency)
		flag = c.payloadType << 1
		if c.hasExt {
			flag |= 0b1
		}
		m.buf.WriteByte(flag)
		if c.hasExt {
			if err := m.writeCustomExtension(c.extensions); err != nil {
				return err
			}
		}
	}
	for _, r := range media.rtpExtensions {
		m.buf.WriteByte(r.id)
		m.buf.WriteByte(r.url)
	}
	return nil
}

func (m *ms) MarshalMiniSDP() ([]byte, error) {
	if m == nil {
		return nil, ErrInvalidMiniSDP
	}
	marshaller := new(miniMarshaller)
	return marshaller.Marshal(m)
}

func (m *ms) Unmarshal(raw []byte) error {
	um := new(miniUnmarshaler)
	return um.Unmarshal(raw, m)
}

func UnmarshalMiniSDP(raw []byte) (*ms, error) {
	if !IsMiniSDP(raw) {
		return nil, ErrNoMiniSDP
	}
	sd := new(ms)
	if err := sd.Unmarshal(raw); err != nil {
		return nil, err
	}
	return sd, nil
}

func IsMiniSDP(payload []byte) bool {
	return len(payload) > 4 && string(payload[1:4]) == miniSdpMarker
}
