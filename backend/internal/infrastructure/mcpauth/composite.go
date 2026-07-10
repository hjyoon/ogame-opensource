package mcpauth

import (
	"context"
	"errors"

	domainmcp "github.com/hjyoon/ogame-opensource/backend/internal/domain/mcp"
)

type TokenVerifier interface {
	VerifyMCPToken(context.Context, string) (domainmcp.Access, error)
}

type CompositeTokenVerifier struct {
	verifiers []TokenVerifier
}

func NewCompositeTokenVerifier(verifiers ...TokenVerifier) CompositeTokenVerifier {
	clean := make([]TokenVerifier, 0, len(verifiers))
	for _, verifier := range verifiers {
		if verifier != nil {
			clean = append(clean, verifier)
		}
	}
	return CompositeTokenVerifier{verifiers: clean}
}

func (v CompositeTokenVerifier) VerifyMCPToken(ctx context.Context, token string) (domainmcp.Access, error) {
	for _, verifier := range v.verifiers {
		access, err := verifier.VerifyMCPToken(ctx, token)
		if err == nil {
			return access, nil
		}
		if !errors.Is(err, domainmcp.ErrUnauthorized) {
			return domainmcp.Access{}, err
		}
	}
	return domainmcp.Access{}, domainmcp.ErrUnauthorized
}
