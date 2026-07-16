package mcpauth

import (
	"context"
	"strconv"
	"strings"

	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
	domainmcp "github.com/hjyoon/ogame-opensource/backend/internal/domain/mcp"
)

type StaticTokenVerifier struct {
	accessByToken map[string]domainmcp.Access
}

func NewStaticTokenVerifier(raw string) StaticTokenVerifier {
	accessByToken := map[string]domainmcp.Access{}
	for _, record := range strings.Split(raw, ";") {
		token, access, ok := parseStaticTokenRecord(record)
		if ok {
			accessByToken[token] = access
		}
	}
	return StaticTokenVerifier{accessByToken: accessByToken}
}

func (v StaticTokenVerifier) VerifyMCPToken(_ context.Context, token string) (domainmcp.Access, error) {
	access, ok := v.accessByToken[strings.TrimSpace(token)]
	if !ok {
		return domainmcp.Access{}, domainmcp.ErrUnauthorized
	}
	return access, nil
}

func parseStaticTokenRecord(record string) (string, domainmcp.Access, bool) {
	parts := strings.SplitN(strings.TrimSpace(record), ":", 3)
	if len(parts) != 3 {
		return "", domainmcp.Access{}, false
	}
	token := strings.TrimSpace(parts[0])
	playerID, err := strconv.Atoi(strings.TrimSpace(parts[1]))
	if token == "" || err != nil || playerID <= 0 {
		return "", domainmcp.Access{}, false
	}
	scopes := parseScopes(parts[2])
	if len(scopes) == 0 {
		return "", domainmcp.Access{}, false
	}
	userType := domaingame.AdminLevelPlayer
	for _, scope := range scopes {
		if scope == domainmcp.ScopeAdmin {
			userType = domaingame.AdminLevelAdmin
			break
		}
		if scope == domainmcp.ScopeOperator {
			userType = domaingame.AdminLevelOperator
		}
	}
	return token, domainmcp.Access{
		Authenticated: true,
		PlayerID:      playerID,
		UserType:      userType,
		Role:          domaingame.AdminRoleName(userType),
		Scopes:        scopes,
	}, true
}

func parseScopes(raw string) []string {
	seen := map[string]bool{}
	scopes := make([]string, 0)
	for _, scope := range strings.Split(raw, ",") {
		scope = strings.TrimSpace(scope)
		if scope == "" || seen[scope] {
			continue
		}
		seen[scope] = true
		scopes = append(scopes, scope)
	}
	return scopes
}
