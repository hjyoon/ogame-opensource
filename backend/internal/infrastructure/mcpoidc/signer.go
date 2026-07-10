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
	publicKey    ed25519.PublicKey
	privateKey   ed25519.PrivateKey
	keyID        string
	previousKeys []ed25519PublicKey
	now          func() time.Time
}

type ed25519PublicKey struct {
	publicKey ed25519.PublicKey
	keyID     string
}

func NewEphemeralEd25519Signer() (Ed25519Signer, error) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return Ed25519Signer{}, err
	}
	return NewEd25519Signer(publicKey, privateKey, time.Now), nil
}

func NewEd25519SignerFromBase64Seed(raw string, now func() time.Time) (Ed25519Signer, error) {
	return NewEd25519SignerFromBase64Seeds(raw, nil, now)
}

func NewEd25519SignerFromBase64Seeds(activeRaw string, previousRaw []string, now func() time.Time) (Ed25519Signer, error) {
	active, err := ed25519KeyFromBase64Seed(activeRaw)
	if err != nil {
		return Ed25519Signer{}, err
	}
	signer := NewEd25519Signer(active.publicKey, active.privateKey, now)
	seen := map[string]struct{}{signer.keyID: {}}
	for index, raw := range previousRaw {
		if strings.TrimSpace(raw) == "" {
			continue
		}
		previous, err := ed25519KeyFromBase64Seed(raw)
		if err != nil {
			return Ed25519Signer{}, fmt.Errorf("previous oidc ed25519 seed %d: %w", index+1, err)
		}
		if _, ok := seen[previous.keyID]; ok {
			continue
		}
		seen[previous.keyID] = struct{}{}
		signer.previousKeys = append(signer.previousKeys, ed25519PublicKey{
			publicKey: previous.publicKey,
			keyID:     previous.keyID,
		})
	}
	return signer, nil
}

type ed25519Key struct {
	publicKey  ed25519.PublicKey
	privateKey ed25519.PrivateKey
	keyID      string
}

func ed25519KeyFromBase64Seed(raw string) (ed25519Key, error) {
	seed, err := base64.StdEncoding.DecodeString(strings.TrimSpace(raw))
	if err != nil {
		return ed25519Key{}, fmt.Errorf("decode oidc ed25519 seed: %w", err)
	}
	if len(seed) != ed25519.SeedSize {
		return ed25519Key{}, fmt.Errorf("oidc ed25519 seed must be %d bytes", ed25519.SeedSize)
	}
	privateKey := ed25519.NewKeyFromSeed(seed)
	publicKey := privateKey.Public().(ed25519.PublicKey)
	sum := sha256.Sum256(publicKey)
	return ed25519Key{
		publicKey:  publicKey,
		privateKey: privateKey,
		keyID:      base64.RawURLEncoding.EncodeToString(sum[:8]),
	}, nil
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
	keys := []domainmcp.JSONWebKey{ed25519JWK(s.publicKey, s.keyID, s.Algorithm())}
	for _, previous := range s.previousKeys {
		if len(previous.publicKey) == 0 {
			continue
		}
		keys = append(keys, ed25519JWK(previous.publicKey, previous.keyID, s.Algorithm()))
	}
	return domainmcp.JSONWebKeySet{Keys: keys}
}

func ed25519JWK(publicKey ed25519.PublicKey, keyID string, algorithm string) domainmcp.JSONWebKey {
	return domainmcp.JSONWebKey{
		KeyType: "OKP",
		Use:     "sig",
		KeyID:   keyID,
		Alg:     algorithm,
		Curve:   "Ed25519",
		X:       base64.RawURLEncoding.EncodeToString(publicKey),
	}
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
