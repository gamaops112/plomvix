// Package server provides a PostgreSQL Wire Protocol v3.0 compatible server.
// ssl.go handles TLS/SSL negotiation and encrypted connections.
package server

import (
	"crypto/tls"
)

// SSLConfig holds TLS certificate configuration.
type SSLConfig struct {
	CertFile string // Path to PEM certificate
	KeyFile  string // Path to private key
}

// tlsConfig creates a tls.Config from the SSLConfig.
func (cfg SSLConfig) tlsConfig() (*tls.Config, error) {
	if cfg.CertFile == "" || cfg.KeyFile == "" {
		return nil, nil
	}
	cert, err := tls.LoadX509KeyPair(cfg.CertFile, cfg.KeyFile)
	if err != nil {
		return nil, err
	}
	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	}, nil
}
