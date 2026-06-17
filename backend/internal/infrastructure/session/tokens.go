package session

import (
	"crypto/rand"
	"encoding/hex"
)

type TokenGenerator struct{}

func (TokenGenerator) NewPublicSession() (string, error) {
	return randomHex(6)
}

func (TokenGenerator) NewPrivateSession() (string, error) {
	return randomHex(16)
}

func randomHex(byteLength int) (string, error) {
	value := make([]byte, byteLength)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return hex.EncodeToString(value), nil
}
