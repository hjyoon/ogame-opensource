package mcpoidc

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"

	domainmcp "github.com/hjyoon/ogame-opensource/backend/internal/domain/mcp"
)

func TestEd25519SignerSignsIDTokenAndExposesJWKS(t *testing.T) {
	ephemeral, err := NewEphemeralEd25519Signer()
	if err != nil {
		t.Fatalf("NewEphemeralEd25519Signer returned error: %v", err)
	}
	if len(ephemeral.JWKS(context.Background()).Keys) != 1 {
		t.Fatalf("expected ephemeral signer JWKS")
	}

	seed := base64.StdEncoding.EncodeToString([]byte("12345678901234567890123456789012"))
	seedSigner, err := NewEd25519SignerFromBase64Seed(seed, func() time.Time { return time.Unix(1700, 0) })
	if err != nil {
		t.Fatalf("NewEd25519SignerFromBase64Seed returned error: %v", err)
	}
	seedSignerAgain, err := NewEd25519SignerFromBase64Seed(seed, nil)
	if err != nil {
		t.Fatalf("NewEd25519SignerFromBase64Seed second call returned error: %v", err)
	}
	if seedSigner.JWKS(context.Background()).Keys[0].KeyID != seedSignerAgain.JWKS(context.Background()).Keys[0].KeyID {
		t.Fatalf("expected stable key id from same seed")
	}

	privateKey := ed25519.NewKeyFromSeed([]byte("12345678901234567890123456789012"))
	publicKey := privateKey.Public().(ed25519.PublicKey)
	signer := NewEd25519Signer(publicKey, privateKey, func() time.Time { return time.Unix(1700, 0) })
	if NewEd25519Signer(publicKey, privateKey, nil).now == nil {
		t.Fatalf("expected default clock")
	}

	jwks := signer.JWKS(context.Background())
	if len(jwks.Keys) != 1 || jwks.Keys[0].KeyType != "OKP" || jwks.Keys[0].Curve != "Ed25519" || jwks.Keys[0].Alg != "EdDSA" {
		t.Fatalf("unexpected JWKS: %+v", jwks)
	}

	token, err := signer.SignIDToken(context.Background(), domainmcp.IDTokenCommand{
		Issuer:   "https://game.example/",
		Audience: "desktop-client",
		PlayerID: 42,
		Scopes:   []string{"openid", domainmcp.ScopeRead},
	})
	if err != nil {
		t.Fatalf("SignIDToken returned error: %v", err)
	}
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Fatalf("expected compact JWT, got %q", token)
	}
	unsigned := parts[0] + "." + parts[1]
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		t.Fatalf("decode signature: %v", err)
	}
	if !ed25519.Verify(publicKey, []byte(unsigned), signature) {
		t.Fatalf("expected signature to verify")
	}
	payloadJSON, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	var claims map[string]any
	if err := json.Unmarshal(payloadJSON, &claims); err != nil {
		t.Fatalf("decode claims: %v", err)
	}
	if claims["iss"] != "https://game.example" || claims["sub"] != "player:42" || claims["aud"] != "desktop-client" || claims["scope"] != "openid mcp:read" {
		t.Fatalf("unexpected claims: %+v", claims)
	}
	if claims["iat"] != float64(1700) || claims["exp"] != float64(2300) {
		t.Fatalf("unexpected token times: %+v", claims)
	}
}

func TestEd25519SignerSupportsPreviousJWKSKeys(t *testing.T) {
	activeSeed := base64.StdEncoding.EncodeToString([]byte("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"))
	previousSeed := base64.StdEncoding.EncodeToString([]byte("bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"))

	signer, err := NewEd25519SignerFromBase64Seeds(activeSeed, []string{"", previousSeed, activeSeed}, func() time.Time { return time.Unix(1700, 0) })
	if err != nil {
		t.Fatalf("NewEd25519SignerFromBase64Seeds returned error: %v", err)
	}
	jwks := signer.JWKS(context.Background())
	if len(jwks.Keys) != 2 {
		t.Fatalf("expected active plus previous JWKS keys, got %+v", jwks.Keys)
	}
	if jwks.Keys[0].KeyID == jwks.Keys[1].KeyID {
		t.Fatalf("expected duplicate active seed to be skipped: %+v", jwks.Keys)
	}

	token, err := signer.SignIDToken(context.Background(), domainmcp.IDTokenCommand{
		Issuer:   "https://game.example",
		Audience: "desktop-client",
		PlayerID: 42,
		Scopes:   []string{"openid"},
	})
	if err != nil {
		t.Fatalf("SignIDToken returned error: %v", err)
	}
	parts := strings.Split(token, ".")
	headerJSON, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		t.Fatalf("decode header: %v", err)
	}
	var header map[string]any
	if err := json.Unmarshal(headerJSON, &header); err != nil {
		t.Fatalf("decode header json: %v", err)
	}
	if header["kid"] != jwks.Keys[0].KeyID {
		t.Fatalf("expected active key id in JWT header, header=%+v jwks=%+v", header, jwks.Keys)
	}
}

func TestEd25519SignerHandlesMissingKeys(t *testing.T) {
	signer := Ed25519Signer{}
	if keys := signer.JWKS(context.Background()).Keys; len(keys) != 0 {
		t.Fatalf("expected empty JWKS for missing public key, got %+v", keys)
	}
	if _, err := signer.SignIDToken(context.Background(), domainmcp.IDTokenCommand{}); err == nil {
		t.Fatalf("expected missing private key error")
	}
	privateKey := ed25519.NewKeyFromSeed([]byte("cccccccccccccccccccccccccccccccc"))
	publicKey := privateKey.Public().(ed25519.PublicKey)
	signer = NewEd25519Signer(publicKey, privateKey, time.Now)
	signer.previousKeys = []ed25519PublicKey{{}}
	if keys := signer.JWKS(context.Background()).Keys; len(keys) != 1 {
		t.Fatalf("expected empty previous key to be skipped, got %+v", keys)
	}
	if _, err := encodedJWTParts(map[string]any{"bad": make(chan int)}, map[string]any{}); err == nil {
		t.Fatalf("expected header marshal error")
	}
	if _, err := encodedJWTParts(map[string]any{}, map[string]any{"bad": make(chan int)}); err == nil {
		t.Fatalf("expected claims marshal error")
	}
	if _, err := NewEd25519SignerFromBase64Seed("not-base64", nil); err == nil {
		t.Fatalf("expected invalid seed base64 error")
	}
	if _, err := NewEd25519SignerFromBase64Seed(base64.StdEncoding.EncodeToString([]byte("short")), nil); err == nil {
		t.Fatalf("expected invalid seed length error")
	}
	if _, err := NewEd25519SignerFromBase64Seeds(base64.StdEncoding.EncodeToString([]byte("12345678901234567890123456789012")), []string{"short"}, nil); err == nil {
		t.Fatalf("expected invalid previous seed error")
	}
}
