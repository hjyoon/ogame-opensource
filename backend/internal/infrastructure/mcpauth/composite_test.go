package mcpauth

import (
	"context"
	"errors"
	"testing"

	domainmcp "github.com/hjyoon/ogame-opensource/backend/internal/domain/mcp"
)

func TestCompositeTokenVerifierFallsThroughUnauthorized(t *testing.T) {
	verifier := NewCompositeTokenVerifier(
		fakeCompositeVerifier{err: domainmcp.ErrUnauthorized},
		fakeCompositeVerifier{access: domainmcp.Access{Authenticated: true, PlayerID: 7, Scopes: []string{domainmcp.ScopeRead}}},
	)

	access, err := verifier.VerifyMCPToken(context.Background(), "token")
	if err != nil {
		t.Fatalf("VerifyMCPToken returned error: %v", err)
	}
	if access.PlayerID != 7 {
		t.Fatalf("unexpected access: %+v", access)
	}
}

func TestCompositeTokenVerifierStopsOnNonAuthError(t *testing.T) {
	wantErr := errors.New("database down")
	verifier := NewCompositeTokenVerifier(fakeCompositeVerifier{err: wantErr})

	if _, err := verifier.VerifyMCPToken(context.Background(), "token"); !errors.Is(err, wantErr) {
		t.Fatalf("expected non-auth error, got %v", err)
	}
}

func TestCompositeTokenVerifierRejectsWhenAllVerifiersReject(t *testing.T) {
	verifier := NewCompositeTokenVerifier(nil, fakeCompositeVerifier{err: domainmcp.ErrUnauthorized})

	if _, err := verifier.VerifyMCPToken(context.Background(), "token"); !errors.Is(err, domainmcp.ErrUnauthorized) {
		t.Fatalf("expected unauthorized after all verifiers reject, got %v", err)
	}
}

type fakeCompositeVerifier struct {
	access domainmcp.Access
	err    error
}

func (f fakeCompositeVerifier) VerifyMCPToken(context.Context, string) (domainmcp.Access, error) {
	if f.err != nil {
		return domainmcp.Access{}, f.err
	}
	return f.access, nil
}
