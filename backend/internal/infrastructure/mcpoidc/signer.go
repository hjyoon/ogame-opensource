package mcpoidc

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	domainmcp "github.com/hjyoon/ogame-opensource/backend/internal/domain/mcp"
)

type Ed25519Signer struct {
	publicKey  ed25519.PublicKey
	privateKey ed25519.PrivateKey
	keyID      string
	now        func() time.Time
}

func NewEphemeralEd25519Signer() (Ed25519Signer, error) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return Ed25519Signer{}, err
	}
	return NewEd25519Signer(publicKey, privateKey, time.Now), nil
}

func NewEd25519Signer(publicKey ed25519.PublicKey, privateKey ed25519.PrivateKey, now func() time.Time) Ed25519Signer {
	if now == nil {
		now = time.Now
	}
	sum := sha256.Sum256(publicKey)
	return Ed25519Signer{
		publicKey:  publicKey,
		privateKey: privateKey,
		keyID:      base64.RawURLEncoding.EncodeToString(sum[:8]),
		now:        now,
	}
}

func (s Ed25519Signer) Algorithm() string {
	return "EdDSA"
}

func (s Ed25519Signer) JWKS(context.Context) domainmcp.JSONWebKeySet {
	if len(s.publicKey) == 0 {
		return domainmcp.JSONWebKeySet{}
	}
	return domainmcp.JSONWebKeySet{Keys: []domainmcp.JSONWebKey{{
		KeyType: "OKP",
		Use:     "sig",
		KeyID:   s.keyID,
		Alg:     s.Algorithm(),
		Curve:   "Ed25519",
		X:       base64.RawURLEncoding.EncodeToString(s.publicKey),
	}}}
}

func (s Ed25519Signer) SignIDToken(_ context.Context, command domainmcp.IDTokenCommand) (string, error) {
	if len(s.privateKey) == 0 {
		return "", fmt.Errorf("oidc signer private key unavailable")
	}
	now := s.now().Unix()
	header := map[string]any{
		"alg": s.Algorithm(),
		"kid": s.keyID,
		"typ": "JWT",
	}
	claims := map[string]any{
		"iss":   strings.TrimRight(command.Issuer, "/"),
		"sub":   fmt.Sprintf("player:%d", command.PlayerID),
		"aud":   command.Audience,
		"iat":   now,
		"exp":   now + 600,
		"scope": strings.Join(command.Scopes, " "),
	}
	unsigned, err := encodedJWTParts(header, claims)
	if err != nil {
		return "", err
	}
	signature := ed25519.Sign(s.privateKey, []byte(unsigned))
	return unsigned + "." + base64.RawURLEncoding.EncodeToString(signature), nil
}

func encodedJWTParts(header map[string]any, claims map[string]any) (string, error) {
	headerJSON, err := json.Marshal(header)
	if err != nil {
		return "", err
	}
	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(headerJSON) + "." + base64.RawURLEncoding.EncodeToString(claimsJSON), nil
}
