package jira

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"
)

// TLSConfig holds client certificate and CA settings
type TLSConfig struct {
	CertFile string
	KeyFile  string
	CAFile   string
	Insecure bool
}

// HasCustomTLS returns true if any TLS option is set
func (t TLSConfig) HasCustomTLS() bool {
	return t.CertFile != "" || t.CAFile != "" || t.Insecure
}

// BuildHTTPClient creates an *http.Client with the configured TLS settings
func (t TLSConfig) BuildHTTPClient() (*http.Client, error) {
	tlsCfg := &tls.Config{} //nolint:gosec

	if t.CertFile != "" && t.KeyFile != "" {
		cert, err := tls.LoadX509KeyPair(t.CertFile, t.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("load client certificate: %w", err)
		}
		tlsCfg.Certificates = []tls.Certificate{cert}
	}

	if t.CAFile != "" {
		caCert, err := os.ReadFile(t.CAFile)
		if err != nil {
			return nil, fmt.Errorf("load CA file: %w", err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(caCert) {
			return nil, errors.New("CA file contains no valid certificates")
		}
		tlsCfg.RootCAs = pool
	}

	tlsCfg.InsecureSkipVerify = t.Insecure

	return &http.Client{
		Timeout:   30 * time.Second,
		Transport: &http.Transport{TLSClientConfig: tlsCfg},
	}, nil
}
