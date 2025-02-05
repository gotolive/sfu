package dtls

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"io"
	"net"
	"strings"

	"github.com/gotolive/sfu/rtc/logger"
	"github.com/pion/dtls/v2"
	"github.com/pion/dtls/v2/pkg/crypto/fingerprint"
	"github.com/pion/logging"
	"github.com/pion/srtp/v2"
)

const (
	New        = 1
	Connecting = 2
	Connected  = 3
	Failed     = 4
	Closed     = 5

	Actpass = "actpass" // both fine.
	Passive = "passive" // server
	Active  = "active"  // client
)

type Transport struct {
	state                 int
	dtlsConn              *dtls.Conn
	role                  string
	remoteFingerprint     *Fingerprint
	srtpProtectionProfile srtp.ProtectionProfile
	conn                  net.Conn
	onState               func(int)
	cert                  *Certificate
}

// it could be called more than once, that is the reason try.
// but clearly we did not handle it yet.
func (t *Transport) TryRun() {
	if t.state != New {
		return
	}
	t.state = Connecting
	t.onState(Connecting)

	// we can not wait handshake done or make handshake sync.
	// This method will be called in read packet goroutine, it will block next dtls data read.
	go func() {
		err := t.handshake()
		if err != nil {
			logger.Error("Something wrong:", err)
			t.onState(Failed)
		} else {
			t.state = Connected
			t.onState(Connected)
		}
	}()
}

func (t *Transport) handshake() error {
	var (
		dtlsConn *dtls.Conn
		err      error
	)
	switch t.role {
	case Active, Actpass:
		dtlsConn, err = dtls.Client(t.conn, &dtls.Config{
			Certificates: []tls.Certificate{
				{
					Certificate: [][]byte{t.cert.x509Cert.Raw},
					PrivateKey:  t.cert.privateKey,
				},
			},
			SRTPProtectionProfiles: func() []dtls.SRTPProtectionProfile {
				return []dtls.SRTPProtectionProfile{dtls.SRTP_AEAD_AES_128_GCM, dtls.SRTP_AES128_CM_HMAC_SHA1_80}
			}(),
			ClientAuth:         dtls.RequireAnyClientCert,
			LoggerFactory:      logging.NewDefaultLoggerFactory(),
			InsecureSkipVerify: true,
		})
	case Passive:
		dtlsConn, err = dtls.Server(t.conn, &dtls.Config{
			Certificates: []tls.Certificate{
				{
					Certificate: [][]byte{t.cert.x509Cert.Raw},
					PrivateKey:  t.cert.privateKey,
				},
			},
			SRTPProtectionProfiles: func() []dtls.SRTPProtectionProfile {
				return []dtls.SRTPProtectionProfile{dtls.SRTP_AEAD_AES_128_GCM, dtls.SRTP_AES128_CM_HMAC_SHA1_80}
			}(),
			ExtendedMasterSecret: dtls.RequireExtendedMasterSecret,
			ClientAuth:           dtls.RequireAnyClientCert,
			LoggerFactory:        logging.NewDefaultLoggerFactory(),
		})
	}
	if err != nil {
		logger.Error("something wrong:", err)
		return err
	}
	t.dtlsConn = dtlsConn
	// to here handshake already done
	srtpProfile, ok := dtlsConn.SelectedSRTPProtectionProfile()
	if !ok {
		return errors.New("no profile")
	}

	switch srtpProfile {
	case dtls.SRTP_AEAD_AES_128_GCM:
		t.srtpProtectionProfile = srtp.ProtectionProfileAeadAes128Gcm
	case dtls.SRTP_AES128_CM_HMAC_SHA1_80:
		t.srtpProtectionProfile = srtp.ProtectionProfileAes128CmHmacSha1_80
	default:
		return errors.New("no valid profile")
	}

	remoteCerts := dtlsConn.ConnectionState().PeerCertificates
	if len(remoteCerts) == 0 {
		return errors.New("no valid cert")
	}

	if err = t.validateFingerPrint(remoteCerts[0]); err != nil {
		return err
	}

	t.dtlsConn = dtlsConn

	go func() {
		// it will be called when dtls receive close notify, but not always,
		buf := make([]byte, 2048)
		_, err := t.dtlsConn.Read(buf)
		if err != nil {
			t.onState(Closed)
		}
	}()

	return nil
}

// we consider it's optional, if the fingerprints exist, we validate it.
// if not, it's fine.
// we only keep the code to fulfill the logic of dtls.
// consider we are the offer side, we won't know answer's fingerprints unless we do another exchange.
func (t *Transport) validateFingerPrint(remoteCert []byte) error {
	if t.remoteFingerprint == nil {
		return nil
	}
	parsedRemoteCert, err := x509.ParseCertificate(remoteCert)
	if err != nil {
		return err
	}
	hashAlgo, err := fingerprint.HashFromString(t.remoteFingerprint.Algorithm)
	if err != nil {
		return err
	}

	remoteValue, err := fingerprint.Fingerprint(parsedRemoteCert, hashAlgo)
	if err != nil {
		return err
	}

	if strings.EqualFold(remoteValue, t.remoteFingerprint.Value) {
		return nil
	}

	return errors.New("invalid fingerprints")
}

func (t *Transport) GetLocalFingerprints() []Fingerprint {
	return t.cert.Fingerprints()
}

func (t *Transport) GetState() int {
	return t.state
}

func (t *Transport) DtlsConn() *dtls.Conn {
	return t.dtlsConn
}

func (t *Transport) Role() string {
	return t.role
}

type Option struct {
	Reader       io.Reader
	Writer       io.Writer
	Role         string
	OnState      func(int)
	Fingerprints *Fingerprint
	Certificate  *Certificate
}

// normally if chrome generate offer it will be actpass
// if they are both fine, we prefer client, we could send client hello asap, zero rtt.
// but the problem is, without ice completed, the dtls client could fail, not verify could it be wait.
// client could fail, but fast.
func NewDtlsTransport(option Option) *Transport {
	t := &Transport{
		onState:           option.OnState,
		cert:              option.Certificate,
		conn:              NewConn(option.Reader, option.Writer),
		state:             New,
		role:              option.Role,
		remoteFingerprint: option.Fingerprints,
	}
	return t
}
