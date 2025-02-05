package dtls

import (
	"github.com/pion/srtp/v2"
)

type SrtpSession struct {
	remoteContext *srtp.Context
	localContext  *srtp.Context
}

// DecryptSrtp is not concurrent-safe, but it won't be called in concurrent.
func (s *SrtpSession) DecryptSrtp(dst, data []byte) ([]byte, error) {
	decrypted, err := s.remoteContext.DecryptRTP(dst, data, nil)
	if err != nil {
		return nil, err
	}
	return decrypted, err
}

// EncryptRtp is not concurrent-safe, but it won't be called in concurrent.
func (s *SrtpSession) EncryptRtp(dst, packet []byte) ([]byte, int, error) {
	data, err := s.localContext.EncryptRTP(dst, packet, nil)
	return data, len(data), err
}

// DecryptSrtcp is not concurrent-safe, but it won't be called in concurrent.
func (s *SrtpSession) DecryptSrtcp(dst, data []byte) ([]byte, error) {
	return s.remoteContext.DecryptRTCP(dst, data, nil)
}

// EncryptRtcp is not concurrent-safe, but it won't be called in concurrent.
func (s *SrtpSession) EncryptRtcp(dst, packet []byte) ([]byte, int, error) {
	data, err := s.localContext.EncryptRTCP(dst, packet, nil)
	return data, len(data), err
}

// NewSrtpSession Start a new srtp session from dtls transport key.
func NewSrtpSession(transport *Transport) (*SrtpSession, error) {
	config := srtp.Config{
		Profile: transport.srtpProtectionProfile,
	}
	state := transport.dtlsConn.ConnectionState()
	err := config.ExtractSessionKeysFromDTLS(&state, transport.role == Active)
	if err != nil {
		return nil, err
	}
	remoteContext, err := srtp.CreateContext(config.Keys.RemoteMasterKey, config.Keys.RemoteMasterSalt, transport.srtpProtectionProfile, config.RemoteOptions...)
	if err != nil {
		return nil, err
	}
	localContext, err := srtp.CreateContext(config.Keys.LocalMasterKey, config.Keys.LocalMasterSalt, transport.srtpProtectionProfile, config.RemoteOptions...)
	if err != nil {
		return nil, err
	}
	return &SrtpSession{remoteContext: remoteContext, localContext: localContext}, nil
}
