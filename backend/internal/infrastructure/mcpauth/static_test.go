package mcpauth

import (
	"context"
	"errors"
	"testing"

	domainmcp "github.com/hjyoon/ogame-opensource/backend/internal/domain/mcp"
)

func TestStaticTokenVerifierParsesAndVerifiesScopedTokens(t *testing.T) {
	verifier := NewStaticTokenVerifier(" read-token : 42 : mcp:read,mcp:messages,mcp:read ; bad ; no-scopes:7:")

	access, err := verifier.VerifyMCPToken(context.Background(), "read-token")
	if err != nil {
		t.Fatalf("VerifyMCPToken returned error: %v", err)
	}
	if !access.Authenticated || access.PlayerID != 42 || len(access.Scopes) != 2 ||
		!access.HasScope(domainmcp.ScopeRead) || !access.HasScope(domainmcp.ScopeMessages) {
		t.Fatalf("unexpected access: %+v", access)
	}

	if _, err := verifier.VerifyMCPToken(context.Background(), "missing"); !errors.Is(err, domainmcp.ErrUnauthorized) {
		t.Fatalf("expected unauthorized for missing token, got %v", err)
	}
	if _, err := verifier.VerifyMCPToken(context.Background(), "no-scopes"); !errors.Is(err, domainmcp.ErrUnauthorized) {
		t.Fatalf("expected invalid no-scope token to be ignored, got %v", err)
	}
}
