package dtls

import (
	"testing"
)

func TestCertificateGenerator(t *testing.T) {
	tests := []struct {
		name   string
		method func(*testing.T)
	}{
		{
			name: "default certificate generator",
			method: func(t *testing.T) {
				cg, err := NewCertManager(false)
				if err != nil {
					t.Error("err should be nil:", err)
				}
				c1 := cg.GenerateCertificate()
				c2 := cg.GenerateCertificate()
				if len(c1.Fingerprints()) != 1 || len(c2.Fingerprints()) != 1 {
					t.Error("we expect 1 fingerprint")
				}
				if c1.Fingerprints()[0] != c2.Fingerprints()[0] {
					t.Error("we expected they have same fingerprint")
				}
			},
		},
		{
			name: "unique certificate generator",
			method: func(t *testing.T) {
				cg, err := NewCertManager(true)
				if err != nil {
					t.Error("err should be nil:", err)
				}
				c1 := cg.GenerateCertificate()
				c2 := cg.GenerateCertificate()
				if len(c1.Fingerprints()) != 1 || len(c2.Fingerprints()) != 1 {
					t.Error("we expect 1 fingerprint")
				}
				if c1.Fingerprints()[0] == c2.Fingerprints()[0] {
					t.Error("we expected they have different fingerprint")
				}
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, test.method)
	}
}
