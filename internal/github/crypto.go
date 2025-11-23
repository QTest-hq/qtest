package github

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"strings"
)

// base64URLEncode encodes data using URL-safe base64 without padding
func base64URLEncode(data []byte) string {
	const base64URLChars = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_"
	var result strings.Builder

	for i := 0; i < len(data); i += 3 {
		var n uint32
		remaining := len(data) - i

		n = uint32(data[i]) << 16
		if remaining > 1 {
			n |= uint32(data[i+1]) << 8
		}
		if remaining > 2 {
			n |= uint32(data[i+2])
		}

		result.WriteByte(base64URLChars[(n>>18)&0x3F])
		result.WriteByte(base64URLChars[(n>>12)&0x3F])

		if remaining > 1 {
			result.WriteByte(base64URLChars[(n>>6)&0x3F])
		}

		if remaining > 2 {
			result.WriteByte(base64URLChars[n&0x3F])
		}
	}

	return result.String()
}

// signRS256 signs data using RSA-SHA256
func signRS256(data []byte, key *rsa.PrivateKey) ([]byte, error) {
	hash := sha256.Sum256(data)
	return rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, hash[:])
}
