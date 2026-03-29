package jira

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestHasCustomTLS(t *testing.T) {
	tests := []struct {
		name string
		cfg  TLSConfig
		want bool
	}{
		{"empty", TLSConfig{}, false},
		{"cert only", TLSConfig{CertFile: "x"}, true},
		{"CA only", TLSConfig{CAFile: "x"}, true},
		{"insecure only", TLSConfig{Insecure: true}, true},
		{"key alone is not enough", TLSConfig{KeyFile: "x"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.cfg.HasCustomTLS(); got != tt.want {
				t.Errorf("HasCustomTLS() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBuildHTTPClient_Insecure(t *testing.T) {
	c, err := TLSConfig{Insecure: true}.BuildHTTPClient()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	tr := c.Transport.(*http.Transport)
	if !tr.TLSClientConfig.InsecureSkipVerify {
		t.Error("InsecureSkipVerify should be true")
	}
}

func TestBuildHTTPClient_CertAndCA(t *testing.T) {
	certFile, keyFile, caFile := writeSelfSignedCert(t)

	c, err := TLSConfig{CertFile: certFile, KeyFile: keyFile, CAFile: caFile}.BuildHTTPClient()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	tr := c.Transport.(*http.Transport)
	if len(tr.TLSClientConfig.Certificates) == 0 {
		t.Error("expected client certificate")
	}
	if tr.TLSClientConfig.RootCAs == nil {
		t.Error("expected CA pool")
	}
}

func TestBuildHTTPClient_CAOnly(t *testing.T) {
	_, _, caFile := writeSelfSignedCert(t)

	c, err := TLSConfig{CAFile: caFile}.BuildHTTPClient()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	tr := c.Transport.(*http.Transport)
	if tr.TLSClientConfig.RootCAs == nil {
		t.Error("expected CA pool")
	}
	if len(tr.TLSClientConfig.Certificates) != 0 {
		t.Error("should have no client certs")
	}
}

func TestBuildHTTPClient_MissingFiles(t *testing.T) {
	if _, err := (TLSConfig{CertFile: "/no.crt", KeyFile: "/no.key"}).BuildHTTPClient(); err == nil {
		t.Error("expected error for missing cert")
	}
	if _, err := (TLSConfig{CAFile: "/no.pem"}).BuildHTTPClient(); err == nil {
		t.Error("expected error for missing CA")
	}
}

func TestBuildHTTPClient_InvalidCA(t *testing.T) {
	dir := t.TempDir()
	bad := filepath.Join(dir, "bad.pem")
	_ = os.WriteFile(bad, []byte("not a certificate"), 0o600)

	if _, err := (TLSConfig{CAFile: bad}).BuildHTTPClient(); err == nil {
		t.Error("expected error for invalid PEM")
	}
}

// writeSelfSignedCert generates a self-signed cert+key PEM pair in a temp dir.
func writeSelfSignedCert(t *testing.T) (certFile, keyFile, caFile string) {
	t.Helper()
	dir := t.TempDir()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "test"},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(time.Hour),
		IsCA:         true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyDER, _ := x509.MarshalECPrivateKey(key)
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})

	certFile = filepath.Join(dir, "cert.pem")
	keyFile = filepath.Join(dir, "key.pem")
	caFile = filepath.Join(dir, "ca.pem")
	_ = os.WriteFile(certFile, certPEM, 0o600)
	_ = os.WriteFile(keyFile, keyPEM, 0o600)
	_ = os.WriteFile(caFile, certPEM, 0o600) // self-signed, same as CA
	return certFile, keyFile, caFile
}
