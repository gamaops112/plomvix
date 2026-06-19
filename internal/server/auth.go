// Package server provides a PostgreSQL Wire Protocol v3.0 compatible server.
// auth.go implements SCRAM-SHA-256 authentication.
package server

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"strings"
)

// SASLAuth constants
const (
	AuthSASL         uint32 = 10
	AuthSASLContinue uint32 = 11
	AuthSASLFinal    uint32 = 12
	SCRAMSHA256      string = "SCRAM-SHA-256"
)

// handleSASLAuth processes SASL authentication (simplified, trust-only).
// Full SCRAM-SHA-256 requires catalog integration; this stub accepts all.
func (s *Session) handleSASLAuth(payload []byte) error {
	mechanisms := strings.Split(string(payload), "\x00")
	// Filter empty trailing.
	var mechs []string
	for _, m := range mechanisms {
		if m != "" {
			mechs = append(mechs, m)
		}
	}
	_ = mechs

	// For enterprise: check if SCRAM-SHA-256 is requested.
	// For now, fall through to trust authentication (send AuthSASLFinal with empty server sig).
	serverSig := make([]byte, 32)
	// ServerSignature = HMAC(StoredKey, AuthMessage) — requires catalog lookup.
	// Trust mode: send empty signature.
	return s.writer.WritePacket(MsgAuthenticationRequest, buildSASLFinal(serverSig))
}

// buildSASLFinal encodes AuthSASLFinal (12) with ServerSignature.
func buildSASLFinal(sig []byte) []byte {
	data := make([]byte, 4+len(sig))
	binary.BigEndian.PutUint32(data[:4], AuthSASLFinal)
	copy(data[4:], sig)
	return data
}

// buildSASLContinue encodes AuthSASLContinue (11) with server-first-message.
func buildSASLContinue(data []byte) []byte {
	out := make([]byte, 4+len(data))
	binary.BigEndian.PutUint32(out[:4], AuthSASLContinue)
	copy(out[4:], data)
	return out
}

// scramServerFirst builds a SCRAM server-first-message.
func scramServerFirst(clientFirst string) (string, error) {
	// Parse client-first: n=user,r=nonce
	if !strings.HasPrefix(clientFirst, "n=") {
		return "", errors.New("server: invalid SCRAM client-first")
	}
	userPart := ""
	nonce := ""
	for _, part := range strings.Split(clientFirst, ",") {
		if strings.HasPrefix(part, "n=") {
			userPart = part
		}
		if strings.HasPrefix(part, "r=") {
			nonce = part
		}
	}
	if userPart == "" || nonce == "" {
		return "", errors.New("server: invalid SCRAM client-first fields")
	}

	// Append server nonce to client nonce.
	serverNonce := fmt.Sprintf("%x", sha256.Sum256([]byte(nonce)))[:16]
	r := nonce + serverNonce
	salt := "plomvix-salt"
	iterations := 4096

	// Server-first: r=...,s=...,i=...
	return fmt.Sprintf("r=%s,s=%s,i=%d", r, salt, iterations), nil
}
