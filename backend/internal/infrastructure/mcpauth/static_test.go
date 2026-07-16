package mcpauth

import (
	"context"
	"errors"
	"testing"

	domainmcp "github.com/hjyoon/ogame-opensource/backend/internal/domain/mcp"
)

func TestStaticTokenVerifierParsesAndVerifiesScopedTokens(t *testing.T) {
	verifier := NewStaticTokenVerifier(" read-token : 42 : mcp:read,mcp:messages,mcp:read ; operator-token:43:mcp:operator;admin-token:44:mcp:admin ; bad ; no-scopes:7:")

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
	operator, err := verifier.VerifyMCPToken(context.Background(), "operator-token")
	if err != nil || operator.UserType != 1 || operator.Role != "operator" {
		t.Fatalf("unexpected operator access=%+v err=%v", operator, err)
	}
	admin, err := verifier.VerifyMCPToken(context.Background(), "admin-token")
	if err != nil || admin.UserType != 2 || admin.Role != "admin" {
		t.Fatalf("unexpected admin access=%+v err=%v", admin, err)
	}
}
