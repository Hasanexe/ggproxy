package main

import (
	"crypto/subtle"
	"encoding/base64"
	"io"
	"net"
	"strings"
)

// validateAuth validates HTTP Basic Authentication header value
// Returns true if credentials match configuration, false otherwise
func validateAuth(authHeader string) bool {
	// Use pre-computed flag for faster check
	if !cfg.AuthRequired {
		return true
	}

	// Check if header exists
	if authHeader == "" {
		return false
	}

	// Check "Basic " prefix
	if !strings.HasPrefix(authHeader, "Basic ") {
		return false
	}

	// Decode base64 (skip "Basic " prefix)
	encoded := authHeader[6:]
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return false
	}

	// Split username:password
	creds := string(decoded)
	colonIndex := strings.IndexByte(creds, ':')
	if colonIndex == -1 {
		return false
	}

	username := creds[:colonIndex]
	password := creds[colonIndex+1:]

	// Constant-time comparison (security)
	return subtle.ConstantTimeCompare([]byte(username), []byte(cfg.AuthUsername)) == 1 &&
		subtle.ConstantTimeCompare([]byte(password), []byte(cfg.AuthPassword)) == 1
}

// authenticateSocks performs SOCKS5 username/password authentication (RFC 1929)
func authenticateSocks(client net.Conn) bool {
	var buf [256]byte

	// Read version, username length
	if _, err := io.ReadFull(client, buf[:2]); err != nil {
		return false
	}
	version, ulen := buf[0], buf[1]

	if version != 0x01 || ulen > 255 {
		return false
	}

	// Read username
	if _, err := io.ReadFull(client, buf[:ulen]); err != nil {
		return false
	}
	username := string(buf[:ulen])

	// Read password length
	if _, err := io.ReadFull(client, buf[:1]); err != nil {
		return false
	}
	plen := buf[0]

	if plen > 255 {
		return false
	}

	// Read password
	if _, err := io.ReadFull(client, buf[:plen]); err != nil {
		return false
	}
	password := string(buf[:plen])

	// Verify credentials using constant-time comparison
	if subtle.ConstantTimeCompare([]byte(username), []byte(cfg.AuthUsername)) == 1 &&
		subtle.ConstantTimeCompare([]byte(password), []byte(cfg.AuthPassword)) == 1 {
		// Success
		client.Write([]byte{0x01, 0x00})
		return true
	}

	// Failure
	client.Write([]byte{0x01, 0x01})
	return false
}
