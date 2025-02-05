package dtls

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"math/big"
	"strings"
	"time"

	"github.com/pion/dtls/v2/pkg/crypto/fingerprint"
)

type CertificateGenerator interface {
	GenerateCertificate() *Certificate
}

type defaultCertificateGenerator struct {
	cert *Certificate
}

func (d *defaultCertificateGenerator) GenerateCertificate() *Certificate {
	return d.cert
}

type uniqueCertificateGenerator struct{}

func (u *uniqueCertificateGenerator) GenerateCertificate() *Certificate {
	c, err := generateCertificate()
	if err != nil {
	}
	return c
}

// NewCertManager if unique is true, we generate a cert for every call,
// otherwise we will use only one cert for all requests.
func NewCertManager(unique bool) (CertificateGenerator, error) {
	var err error
	if unique {
		return &uniqueCertificateGenerator{}, nil
	}
	cert, err := generateCertificate()
	if err != nil {
		return nil, err
	}
	return &defaultCertificateGenerator{cert: cert}, nil
}

func generateCertificate() (*Certificate, error) {
	secretKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	origin := make([]byte, 16)
	/* #nosec */
	if _, err := rand.Read(origin); err != nil {
		return nil, err
	}

	// Max random value, a 130-bits integer, i.e 2^130 - 1
	maxBigInt := new(big.Int)
	/* #nosec */
	maxBigInt.Exp(big.NewInt(2), big.NewInt(130), nil).Sub(maxBigInt, big.NewInt(1))
	/* #nosec */
	serialNumber, err := rand.Int(rand.Reader, maxBigInt)
	if err != nil {
		return nil, err
	}

	return NewCertificate(secretKey, x509.Certificate{
		ExtKeyUsage: []x509.ExtKeyUsage{
			x509.ExtKeyUsageClientAuth,
			x509.ExtKeyUsageServerAuth,
		},
		BasicConstraintsValid: true,
		NotBefore:             time.Now(),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		NotAfter:              time.Now().AddDate(1, 0, 0),
		SerialNumber:          serialNumber,
		Version:               2,
		Subject:               pkix.Name{CommonName: hex.EncodeToString(origin)},
		IsCA:                  true,
	})
}

type Certificate struct {
	privateKey   crypto.PrivateKey
	x509Cert     *x509.Certificate
	fingerprints []Fingerprint
}

// NewCertificate generates a new x509 compliant Certificate to be used
// by DTLS for encrypting data sent over the wire. This method differs from
// generateCertificate by allowing to specify a template x509.Certificate to
// be used in order to define certificate parameters.
func NewCertificate(key crypto.PrivateKey, tpl x509.Certificate) (*Certificate, error) {
	var err error
	var certDER []byte
	switch sk := key.(type) {
	case *rsa.PrivateKey:
		pk := sk.Public()
		tpl.SignatureAlgorithm = x509.SHA256WithRSA
		certDER, err = x509.CreateCertificate(rand.Reader, &tpl, &tpl, pk, sk)
		if err != nil {
			return nil, err
		}
	case *ecdsa.PrivateKey:
		pk := sk.Public()
		tpl.SignatureAlgorithm = x509.ECDSAWithSHA256
		certDER, err = x509.CreateCertificate(rand.Reader, &tpl, &tpl, pk, sk)
		if err != nil {
			return nil, err
		}
	default:
		return nil, err
	}

	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		return nil, err
	}

	return &Certificate{privateKey: key, x509Cert: cert}, nil
}

func (c *Certificate) Fingerprints() []Fingerprint {
	if len(c.fingerprints) != 0 {
		return c.fingerprints
	}
	fingerprintAlgorithms := []crypto.Hash{crypto.SHA256}
	c.fingerprints = make([]Fingerprint, len(fingerprintAlgorithms))
	i := 0
	for _, algo := range fingerprintAlgorithms {
		name, err := fingerprint.StringFromHash(algo)
		if err != nil {
			panic(err)
		}
		value, err := fingerprint.Fingerprint(c.x509Cert, algo)
		if err != nil {
			panic(err)
		}
		c.fingerprints[i] = Fingerprint{
			Algorithm: name,
			Value:     strings.ToUpper(value), // firefox required up case.
		}
	}
	return c.fingerprints
}
