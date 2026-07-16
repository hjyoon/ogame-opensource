package mysqlgame

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"

	domainmcp "github.com/hjyoon/ogame-opensource/backend/internal/domain/mcp"
)

func TestMCPTokenRepositoryEnsuresSchema(t *testing.T) {
	runner := &fakeMCPTokenRunner{
		fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{0})}}},
	}
	repository := NewMCPTokenRepositoryWithRunner(runner, runner, "uni1_")

	if err := repository.EnsureMCPTokenSchema(context.Background()); err != nil {
		t.Fatalf("EnsureMCPTokenSchema returned error: %v", err)
	}
	if !strings.Contains(runner.execCalls[0].sql, "CREATE TABLE IF NOT EXISTS `uni1_mcp_tokens`") || !strings.Contains(runner.execCalls[0].sql, "expires_at INT NOT NULL DEFAULT 0") {
		t.Fatalf("unexpected schema SQL: %s", runner.execCalls[0].sql)
	}
	if !strings.Contains(runner.execCalls[1].sql, "ALTER TABLE `uni1_mcp_tokens` ADD COLUMN expires_at") {
		t.Fatalf("expected token expiry migration, got %+v", runner.execCalls)
	}

	runner = &fakeMCPTokenRunner{
		fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{1})}}},
	}
	repository = NewMCPTokenRepositoryWithRunner(runner, runner, "uni1_")
	if err := repository.EnsureMCPTokenSchema(context.Background()); err != nil {
		t.Fatalf("EnsureMCPTokenSchema existing expires column returned error: %v", err)
	}
	if len(runner.execCalls) != 1 {
		t.Fatalf("expected no expiry migration when column exists, got %+v", runner.execCalls)
	}

	repository = NewMCPTokenRepositoryWithRunner(runner, runner, "uni1_;DROP")
	if err := repository.EnsureMCPTokenSchema(context.Background()); err == nil {
		t.Fatalf("expected unsafe prefix to be rejected")
	}
}

func TestMCPTokenRepositoryEnsuresOAuthCodeSchema(t *testing.T) {
	runner := &fakeMCPTokenRunner{
		fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{0})}}},
	}
	repository := NewMCPTokenRepositoryWithRunner(runner, runner, "uni1_")

	if err := repository.EnsureMCPOAuthCodeSchema(context.Background()); err != nil {
		t.Fatalf("EnsureMCPOAuthCodeSchema returned error: %v", err)
	}
	if !strings.Contains(runner.execCalls[0].sql, "CREATE TABLE IF NOT EXISTS `uni1_mcp_oauth_codes`") || !strings.Contains(runner.execCalls[0].sql, "resource TEXT NOT NULL") {
		t.Fatalf("unexpected oauth code schema SQL: %s", runner.execCalls[0].sql)
	}
	if !strings.Contains(runner.execCalls[1].sql, "ALTER TABLE `uni1_mcp_oauth_codes` ADD COLUMN resource") {
		t.Fatalf("expected oauth code resource migration, got %+v", runner.execCalls)
	}

	runner = &fakeMCPTokenRunner{
		fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{1})}}},
	}
	repository = NewMCPTokenRepositoryWithRunner(runner, runner, "uni1_")
	if err := repository.EnsureMCPOAuthCodeSchema(context.Background()); err != nil {
		t.Fatalf("EnsureMCPOAuthCodeSchema existing resource returned error: %v", err)
	}
	if len(runner.execCalls) != 1 {
		t.Fatalf("expected no resource migration when column exists, got %+v", runner.execCalls)
	}

	repository = NewMCPTokenRepositoryWithRunner(runner, nil, "uni1_")
	if err := repository.EnsureMCPOAuthCodeSchema(context.Background()); err == nil {
		t.Fatalf("expected nil execer oauth schema error")
	}
	repository = NewMCPTokenRepositoryWithRunner(nil, runner, "uni1_")
	if err := repository.EnsureMCPOAuthCodeSchema(context.Background()); err == nil {
		t.Fatalf("expected nil queryer oauth schema error")
	}
	repository = NewMCPTokenRepositoryWithRunner(runner, runner, "uni1_;DROP")
	if err := repository.EnsureMCPOAuthCodeSchema(context.Background()); err == nil {
		t.Fatalf("expected unsafe oauth schema prefix error")
	}
}

func TestMCPTokenRepositorySQLiteSchemaErrorsAndDialectOverride(t *testing.T) {
	wantErr := errors.New("sqlite index failed")
	for _, test := range []struct {
		name string
		run  func(MCPTokenRepository) error
	}{
		{name: "tokens", run: func(repository MCPTokenRepository) error {
			return repository.EnsureMCPTokenSchema(context.Background())
		}},
		{name: "oauth", run: func(repository MCPTokenRepository) error {
			return repository.EnsureMCPOAuthCodeSchema(context.Background())
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			runner := &sequencedMCPExecer{failAt: 2, err: wantErr}
			repository := NewMCPTokenRepositoryWithRunner(&fakeMCPTokenRunner{}, runner, "uni1_").WithDialect(DialectSQLite)
			if err := test.run(repository); !errors.Is(err, wantErr) {
				t.Fatalf("expected SQLite schema index error, got %v", err)
			}
			if len(runner.calls) != 2 {
				t.Fatalf("expected create and first index calls, got %d", len(runner.calls))
			}
		})
	}
	if got := (MCPTokenRepository{}).WithDialect("unknown").dialect; got != DialectMySQL {
		t.Fatalf("expected unknown dialect to normalize to MySQL, got %q", got)
	}
	for _, test := range []struct {
		name string
		run  func(MCPTokenRepository) error
	}{
		{name: "sqlite token create", run: func(repository MCPTokenRepository) error {
			return repository.EnsureMCPTokenSchema(context.Background())
		}},
		{name: "sqlite oauth create", run: func(repository MCPTokenRepository) error {
			return repository.EnsureMCPOAuthCodeSchema(context.Background())
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			runner := &sequencedMCPExecer{failAt: 1, err: wantErr}
			repository := NewMCPTokenRepositoryWithRunner(&fakeMCPTokenRunner{}, runner, "uni1_").WithDialect(DialectSQLite)
			if err := test.run(repository); !errors.Is(err, wantErr) {
				t.Fatalf("expected SQLite create error, got %v", err)
			}
		})
	}
	for _, test := range []struct {
		name string
		run  func(MCPTokenRepository) error
	}{
		{name: "mysql token create", run: func(repository MCPTokenRepository) error {
			return repository.EnsureMCPTokenSchema(context.Background())
		}},
		{name: "mysql oauth create", run: func(repository MCPTokenRepository) error {
			return repository.EnsureMCPOAuthCodeSchema(context.Background())
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			runner := &fakeMCPTokenRunner{err: wantErr}
			repository := NewMCPTokenRepositoryWithRunner(runner, runner, "uni1_")
			if err := test.run(repository); !errors.Is(err, wantErr) {
				t.Fatalf("expected MySQL create error, got %v", err)
			}
		})
	}
}

func TestMCPTokenRepositoryRejectsUnsafePrefixesForEveryOperation(t *testing.T) {
	runner := &fakeMCPTokenRunner{}
	repository := NewMCPTokenRepositoryWithRunner(runner, runner, "bad-prefix")
	checks := []func() error{
		func() error { _, err := repository.ListMCPTokens(context.Background(), 1); return err },
		func() error {
			_, err := repository.CreateMCPToken(context.Background(), domainmcp.Token{}, "hash", 5, 1)
			return err
		},
		func() error { _, err := repository.RevokeMCPToken(context.Background(), 1, 1, 1); return err },
		func() error {
			_, err := repository.CreateMCPOAuthCode(context.Background(), domainmcp.OAuthAuthorizationCode{})
			return err
		},
		func() error { _, err := repository.ConsumeMCPOAuthCode(context.Background(), "hash", 1); return err },
		func() error { _, err := repository.VerifyMCPToken(context.Background(), "secret"); return err },
	}
	for index, check := range checks {
		if err := check(); err == nil {
			t.Fatalf("expected unsafe prefix error for operation %d", index)
		}
	}
}

func TestMCPTokenRepositoryEnsuresTokenExpiresColumnErrors(t *testing.T) {
	wantErr := errors.New("expires column failed")
	for _, tt := range []struct {
		name   string
		runner *fakeMCPTokenRunner
	}{
		{name: "query", runner: &fakeMCPTokenRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{err: wantErr}}}}},
		{name: "missing", runner: &fakeMCPTokenRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues()}}}}},
		{name: "scan", runner: &fakeMCPTokenRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{"bad"})}}}}},
		{name: "rows", runner: &fakeMCPTokenRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValuesWithErr(wantErr, []any{0})}}}}},
		{name: "alter", runner: &fakeMCPTokenRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{0})}}}, err: wantErr}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			repository := NewMCPTokenRepositoryWithRunner(tt.runner, tt.runner, "uni1_")
			if err := repository.ensureMCPTokenExpiresColumn(context.Background(), "`uni1_mcp_tokens`", "uni1_mcp_tokens"); err == nil {
				t.Fatalf("expected expires column error")
			}
		})
	}
}

func TestMCPTokenRepositoryEnsuresOAuthCodeResourceColumnErrors(t *testing.T) {
	wantErr := errors.New("resource column failed")
	for _, tt := range []struct {
		name   string
		runner *fakeMCPTokenRunner
	}{
		{name: "query", runner: &fakeMCPTokenRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{err: wantErr}}}}},
		{name: "missing", runner: &fakeMCPTokenRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues()}}}}},
		{name: "scan", runner: &fakeMCPTokenRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{"bad"})}}}}},
		{name: "rows", runner: &fakeMCPTokenRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValuesWithErr(wantErr, []any{0})}}}}},
		{name: "alter", runner: &fakeMCPTokenRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{0})}}}, err: wantErr}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			repository := NewMCPTokenRepositoryWithRunner(tt.runner, tt.runner, "uni1_")
			if err := repository.ensureOAuthCodeResourceColumn(context.Background(), "`uni1_mcp_oauth_codes`", "uni1_mcp_oauth_codes"); err == nil {
				t.Fatalf("expected resource column error")
			}
		})
	}
}

func TestMCPTokenRepositoryCreatesListsAndRevokesTokens(t *testing.T) {
	runner := &fakeMCPTokenRunner{
		fakeQueryer: fakeQueryer{results: []fakeQueryResult{{
			rows: fakeRowsFromValues([]any{7, "Desktop", "mcp:read,mcp:messages", int64(1700), int64(2300), int64(0), int64(0)}),
		}}},
		result: fakeSQLResult(7),
	}
	repository := NewMCPTokenRepositoryWithRunner(runner, runner, "uni1_")

	token, err := repository.CreateMCPToken(context.Background(), domainmcp.Token{
		PlayerID:  42,
		Name:      "Desktop",
		Scopes:    []string{domainmcp.ScopeRead, domainmcp.ScopeMessages, domainmcp.ScopeRead},
		CreatedAt: 1700,
		ExpiresAt: 2300,
	}, "hash", 5, 1700)
	if err != nil {
		t.Fatalf("CreateMCPToken returned error: %v", err)
	}
	if token.ID != 7 || token.ExpiresAt != 2300 || !strings.Contains(runner.execCalls[0].sql, "INSERT INTO `uni1_mcp_tokens`") || runner.execCalls[0].args[3] != "mcp:read,mcp:messages" || runner.execCalls[0].args[5] != int64(2300) {
		t.Fatalf("unexpected created token=%+v exec=%+v", token, runner.execCalls[0])
	}
	if !strings.Contains(runner.execCalls[0].sql, "SELECT COUNT(*)") || runner.execCalls[0].args[6] != 42 || runner.execCalls[0].args[7] != int64(1700) || runner.execCalls[0].args[8] != 5 {
		t.Fatalf("expected guarded active-token insert, got %+v", runner.execCalls[0])
	}

	tokens, err := repository.ListMCPTokens(context.Background(), 42)
	if err != nil {
		t.Fatalf("ListMCPTokens returned error: %v", err)
	}
	if len(tokens) != 1 || tokens[0].ID != 7 || tokens[0].Scopes[1] != domainmcp.ScopeMessages || tokens[0].ExpiresAt != 2300 || tokens[0].RevokedAt != 0 {
		t.Fatalf("unexpected tokens: %+v", tokens)
	}
	if !strings.Contains(runner.calls[0].sql, "revoked_at = 0") || !strings.Contains(runner.calls[0].sql, "expires_at") {
		t.Fatalf("expected list query to hide revoked tokens, got %s", runner.calls[0].sql)
	}

	revoked, err := repository.RevokeMCPToken(context.Background(), 42, 7, 1900)
	if err != nil {
		t.Fatalf("RevokeMCPToken returned error: %v", err)
	}
	if !revoked || !strings.Contains(runner.execCalls[1].sql, "UPDATE `uni1_mcp_tokens` SET revoked_at") {
		t.Fatalf("unexpected revoke result=%v exec=%+v", revoked, runner.execCalls[1])
	}

	revoked, err = repository.RevokeMCPTokenByHash(context.Background(), " hash ", 1950)
	if err != nil {
		t.Fatalf("RevokeMCPTokenByHash returned error: %v", err)
	}
	if !revoked || !strings.Contains(runner.execCalls[2].sql, "WHERE token_hash = ?") || runner.execCalls[2].args[1] != "hash" {
		t.Fatalf("unexpected hash revoke result=%v exec=%+v", revoked, runner.execCalls[2])
	}
}

func TestMCPTokenRepositoryCreatesAndConsumesOAuthCodes(t *testing.T) {
	runner := &fakeMCPTokenRunner{
		fakeQueryer: fakeQueryer{results: []fakeQueryResult{{
			rows: fakeRowsFromValues([]any{11, 42, "desktop", "http://127.0.0.1:9000/callback", "https://game.example/mcp", "hash", strings.Repeat("a", 43), "S256", "openid,mcp:read", int64(1700), int64(2300), int64(1800)}),
		}}},
		result: fakeSQLResult(11),
	}
	repository := NewMCPTokenRepositoryWithRunner(runner, runner, "uni1_")

	code, err := repository.CreateMCPOAuthCode(context.Background(), domainmcp.OAuthAuthorizationCode{
		PlayerID:            42,
		ClientID:            " desktop ",
		RedirectURI:         " http://127.0.0.1:9000/callback ",
		Resource:            " https://game.example/mcp/ ",
		CodeHash:            "hash",
		CodeChallenge:       strings.Repeat("a", 43),
		CodeChallengeMethod: "S256",
		Scopes:              []string{"openid", domainmcp.ScopeRead, domainmcp.ScopeRead},
		CreatedAt:           1700,
		ExpiresAt:           2300,
	})
	if err != nil {
		t.Fatalf("CreateMCPOAuthCode returned error: %v", err)
	}
	if code.ID != 11 || code.ClientID != "desktop" || code.Resource != "https://game.example/mcp" || strings.Join(code.Scopes, ",") != "openid,mcp:read" {
		t.Fatalf("unexpected oauth code result: %+v", code)
	}
	if !strings.Contains(runner.execCalls[0].sql, "INSERT INTO `uni1_mcp_oauth_codes`") || runner.execCalls[0].args[3] != "https://game.example/mcp" || runner.execCalls[0].args[4] != "hash" {
		t.Fatalf("unexpected oauth code insert: %+v", runner.execCalls[0])
	}

	consumed, err := repository.ConsumeMCPOAuthCode(context.Background(), "hash", 1800)
	if err != nil {
		t.Fatalf("ConsumeMCPOAuthCode returned error: %v", err)
	}
	if consumed.ID != 11 || consumed.PlayerID != 42 || consumed.Resource != "https://game.example/mcp" || consumed.ConsumedAt != 1800 || consumed.Scopes[1] != domainmcp.ScopeRead {
		t.Fatalf("unexpected consumed code: %+v", consumed)
	}
	if !strings.Contains(runner.execCalls[1].sql, "consumed_at = ?") || runner.execCalls[1].args[2] != int64(1800) {
		t.Fatalf("unexpected oauth code consume update: %+v", runner.execCalls[1])
	}
}

func TestMCPTokenRepositoryVerifiesTokenByHash(t *testing.T) {
	oldNow := mcpNowUnix
	mcpNowUnix = func() int64 { return 2000 }
	defer func() { mcpNowUnix = oldNow }()

	runner := &fakeMCPTokenRunner{
		fakeQueryer: fakeQueryer{results: []fakeQueryResult{{
			rows: fakeRowsFromValues([]any{42, "mcp:read,mcp:fleet,mcp:operator", int64(0), int64(2300), 1}),
		}}},
		result: fakeSQLResult(1),
	}
	repository := NewMCPTokenRepositoryWithRunner(runner, runner, "uni1_")

	access, err := repository.VerifyMCPToken(context.Background(), "secret")
	if err != nil {
		t.Fatalf("VerifyMCPToken returned error: %v", err)
	}
	if !access.Authenticated || access.PlayerID != 42 || !access.HasScope(domainmcp.ScopeFleet) || access.UserType != 1 || access.Role != "operator" {
		t.Fatalf("unexpected access: %+v", access)
	}
	if runner.calls[0].args[0] != hashMCPToken("secret") {
		t.Fatalf("expected hashed lookup arg, got %+v", runner.calls[0].args)
	}
	if runner.execCalls[0].args[0] != int64(2000) {
		t.Fatalf("expected last_used_at touch, got %+v", runner.execCalls[0])
	}
	if runner.execCalls[0].args[2] != int64(2000) {
		t.Fatalf("expected expiry guard on last_used_at touch, got %+v", runner.execCalls[0])
	}
}

func TestMCPTokenRepositoryReadsCurrentUserType(t *testing.T) {
	repository := NewMCPTokenRepositoryWithRunner(&fakeMCPTokenRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{2})}}}}, nil, "uni1_")
	level, err := repository.GetMCPUserType(context.Background(), 42)
	if err != nil || level != 2 {
		t.Fatalf("GetMCPUserType level=%d err=%v", level, err)
	}
	if _, err := (MCPTokenRepository{}).GetMCPUserType(context.Background(), 42); err == nil {
		t.Fatal("expected missing queryer error")
	}
	repository = NewMCPTokenRepositoryWithRunner(&fakeMCPTokenRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues()}}}}, nil, "uni1_")
	if _, err := repository.GetMCPUserType(context.Background(), 42); !errors.Is(err, domainmcp.ErrUnauthorized) {
		t.Fatalf("expected missing user to be unauthorized, got %v", err)
	}
	wantErr := errors.New("user type failed")
	for _, test := range []struct {
		name       string
		repository MCPTokenRepository
	}{
		{name: "prefix", repository: NewMCPTokenRepositoryWithRunner(&fakeMCPTokenRunner{}, nil, "bad-prefix")},
		{name: "query", repository: NewMCPTokenRepositoryWithRunner(&fakeMCPTokenRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{err: wantErr}}}}, nil, "uni1_")},
		{name: "scan", repository: NewMCPTokenRepositoryWithRunner(&fakeMCPTokenRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{"bad"})}}}}, nil, "uni1_")},
		{name: "rows", repository: NewMCPTokenRepositoryWithRunner(&fakeMCPTokenRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValuesWithErr(wantErr, []any{1})}}}}, nil, "uni1_")},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := test.repository.GetMCPUserType(context.Background(), 42); err == nil {
				t.Fatal("expected user type error")
			}
		})
	}
}

func TestMCPTokenRepositoryRejectsMissingRevokedOrExpiredTokens(t *testing.T) {
	oldNow := mcpNowUnix
	mcpNowUnix = func() int64 { return 2000 }
	defer func() { mcpNowUnix = oldNow }()

	repository := NewMCPTokenRepositoryWithRunner(&fakeMCPTokenRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues()}}}}, nil, "uni1_")
	if _, err := repository.VerifyMCPToken(context.Background(), "missing"); !errors.Is(err, domainmcp.ErrUnauthorized) {
		t.Fatalf("expected missing token unauthorized, got %v", err)
	}

	repository = NewMCPTokenRepositoryWithRunner(&fakeMCPTokenRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{42, "mcp:read", int64(1), int64(2300), 0})}}}}, nil, "uni1_")
	if _, err := repository.VerifyMCPToken(context.Background(), "revoked"); !errors.Is(err, domainmcp.ErrUnauthorized) {
		t.Fatalf("expected revoked token unauthorized, got %v", err)
	}

	repository = NewMCPTokenRepositoryWithRunner(&fakeMCPTokenRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{42, "mcp:read", int64(0), int64(1999), 0})}}}}, &fakeMCPTokenRunner{}, "uni1_")
	if _, err := repository.VerifyMCPToken(context.Background(), "expired"); !errors.Is(err, domainmcp.ErrUnauthorized) {
		t.Fatalf("expected expired token unauthorized, got %v", err)
	}
}

func TestMCPTokenRepositoryCoversErrorBranches(t *testing.T) {
	if repository := NewMCPTokenRepository(nil, "uni1_"); repository.prefix != "uni1_" {
		t.Fatalf("unexpected constructor result: %+v", repository)
	}

	repository := NewMCPTokenRepositoryWithRunner(nil, nil, "uni1_")
	if err := repository.EnsureMCPTokenSchema(context.Background()); err == nil {
		t.Fatalf("expected nil execer schema error")
	}
	if _, err := repository.ListMCPTokens(context.Background(), 42); err == nil {
		t.Fatalf("expected nil queryer list error")
	}
	if _, err := repository.CreateMCPToken(context.Background(), domainmcp.Token{}, "hash", 5, 1); err == nil {
		t.Fatalf("expected nil execer create error")
	}
	if _, err := repository.RevokeMCPToken(context.Background(), 42, 7, 1); err == nil {
		t.Fatalf("expected nil execer revoke error")
	}
	if _, err := repository.RevokeMCPTokenByHash(context.Background(), "hash", 1); err == nil {
		t.Fatalf("expected nil execer hash revoke error")
	}
	repository = NewMCPTokenRepositoryWithRunner(nil, &fakeMCPTokenRunner{}, "uni1_;DROP")
	if _, err := repository.RevokeMCPTokenByHash(context.Background(), "hash", 1); err == nil {
		t.Fatalf("expected unsafe prefix hash revoke error")
	}
	if _, err := repository.VerifyMCPToken(context.Background(), "secret"); err == nil {
		t.Fatalf("expected nil queryer verify error")
	}
	if err := repository.EnsureMCPOAuthCodeSchema(context.Background()); err == nil {
		t.Fatalf("expected nil execer oauth schema error")
	}
	if _, err := repository.CreateMCPOAuthCode(context.Background(), domainmcp.OAuthAuthorizationCode{}); err == nil {
		t.Fatalf("expected nil execer oauth create error")
	}
	if _, err := repository.ConsumeMCPOAuthCode(context.Background(), "hash", 1); err == nil {
		t.Fatalf("expected nil oauth consume error")
	}

	wantErr := errors.New("db down")
	repository = NewMCPTokenRepositoryWithRunner(&fakeMCPTokenRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{err: wantErr}}}}, &fakeMCPTokenRunner{}, "uni1_")
	if _, err := repository.ListMCPTokens(context.Background(), 42); !errors.Is(err, wantErr) {
		t.Fatalf("expected list query error, got %v", err)
	}
	repository = NewMCPTokenRepositoryWithRunner(&fakeMCPTokenRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{"bad", "Desktop", "mcp:read", int64(1), int64(0), int64(0), int64(0)})}}}}, &fakeMCPTokenRunner{}, "uni1_")
	if _, err := repository.ListMCPTokens(context.Background(), 42); err == nil {
		t.Fatalf("expected list scan error")
	}
	repository = NewMCPTokenRepositoryWithRunner(&fakeMCPTokenRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValuesWithErr(wantErr)}}}}, &fakeMCPTokenRunner{}, "uni1_")
	if _, err := repository.ListMCPTokens(context.Background(), 42); !errors.Is(err, wantErr) {
		t.Fatalf("expected list rows error, got %v", err)
	}

	repository = NewMCPTokenRepositoryWithRunner(&fakeMCPTokenRunner{}, &fakeMCPTokenRunner{err: wantErr}, "uni1_")
	if _, err := repository.CreateMCPToken(context.Background(), domainmcp.Token{}, "hash", 5, 1); !errors.Is(err, wantErr) {
		t.Fatalf("expected create exec error, got %v", err)
	}
	repository = NewMCPTokenRepositoryWithRunner(&fakeMCPTokenRunner{}, &fakeMCPTokenRunner{result: fakeFleetSQLErrorResult{rows: 1, idErr: wantErr}}, "uni1_")
	if _, err := repository.CreateMCPToken(context.Background(), domainmcp.Token{}, "hash", 5, 1); !errors.Is(err, wantErr) {
		t.Fatalf("expected last insert id error, got %v", err)
	}
	repository = NewMCPTokenRepositoryWithRunner(&fakeMCPTokenRunner{}, &fakeMCPTokenRunner{result: fakeFleetSQLErrorResult{rowsErr: wantErr}}, "uni1_")
	if _, err := repository.CreateMCPToken(context.Background(), domainmcp.Token{}, "hash", 5, 1); !errors.Is(err, wantErr) {
		t.Fatalf("expected create rows affected error, got %v", err)
	}
	repository = NewMCPTokenRepositoryWithRunner(&fakeMCPTokenRunner{}, &fakeMCPTokenRunner{result: fakeFleetSQLErrorResult{rows: 0}}, "uni1_")
	if _, err := repository.CreateMCPToken(context.Background(), domainmcp.Token{}, "hash", 5, 1); !errors.Is(err, domainmcp.ErrTokenLimitReached) {
		t.Fatalf("expected active token limit error, got %v", err)
	}
	if _, err := repository.CreateMCPToken(context.Background(), domainmcp.Token{}, "hash", 0, 1); err == nil {
		t.Fatalf("expected invalid active token limit error")
	}

	repository = NewMCPTokenRepositoryWithRunner(&fakeMCPTokenRunner{}, &fakeMCPTokenRunner{err: wantErr}, "uni1_")
	if _, err := repository.RevokeMCPToken(context.Background(), 42, 7, 1); !errors.Is(err, wantErr) {
		t.Fatalf("expected revoke exec error, got %v", err)
	}
	repository = NewMCPTokenRepositoryWithRunner(&fakeMCPTokenRunner{}, &fakeMCPTokenRunner{result: fakeFleetSQLErrorResult{rowsErr: wantErr}}, "uni1_")
	if _, err := repository.RevokeMCPToken(context.Background(), 42, 7, 1); !errors.Is(err, wantErr) {
		t.Fatalf("expected rows affected error, got %v", err)
	}
	repository = NewMCPTokenRepositoryWithRunner(&fakeMCPTokenRunner{}, &fakeMCPTokenRunner{result: fakeFleetSQLErrorResult{rows: 0}}, "uni1_")
	revoked, err := repository.RevokeMCPToken(context.Background(), 42, 7, 1)
	if err != nil || revoked {
		t.Fatalf("expected no-op revoke, got revoked=%v err=%v", revoked, err)
	}

	repository = NewMCPTokenRepositoryWithRunner(&fakeMCPTokenRunner{}, &fakeMCPTokenRunner{err: wantErr}, "uni1_")
	if _, err := repository.RevokeMCPTokenByHash(context.Background(), "hash", 1); !errors.Is(err, wantErr) {
		t.Fatalf("expected hash revoke exec error, got %v", err)
	}
	repository = NewMCPTokenRepositoryWithRunner(&fakeMCPTokenRunner{}, &fakeMCPTokenRunner{result: fakeFleetSQLErrorResult{rowsErr: wantErr}}, "uni1_")
	if _, err := repository.RevokeMCPTokenByHash(context.Background(), "hash", 1); !errors.Is(err, wantErr) {
		t.Fatalf("expected hash revoke rows affected error, got %v", err)
	}
	repository = NewMCPTokenRepositoryWithRunner(&fakeMCPTokenRunner{}, &fakeMCPTokenRunner{result: fakeFleetSQLErrorResult{rows: 0}}, "uni1_")
	revoked, err = repository.RevokeMCPTokenByHash(context.Background(), "hash", 1)
	if err != nil || revoked {
		t.Fatalf("expected no-op hash revoke, got revoked=%v err=%v", revoked, err)
	}

	repository = NewMCPTokenRepositoryWithRunner(&fakeMCPTokenRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{err: wantErr}}}}, nil, "uni1_")
	if _, err := repository.VerifyMCPToken(context.Background(), "secret"); !errors.Is(err, wantErr) {
		t.Fatalf("expected verify query error, got %v", err)
	}
	repository = NewMCPTokenRepositoryWithRunner(&fakeMCPTokenRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{"bad", "mcp:read", int64(0), int64(0), 0})}}}}, nil, "uni1_")
	if _, err := repository.VerifyMCPToken(context.Background(), "secret"); err == nil {
		t.Fatalf("expected verify scan error")
	}
	repository = NewMCPTokenRepositoryWithRunner(&fakeMCPTokenRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{42, "", int64(0), int64(0), 0})}}}}, nil, "uni1_")
	if _, err := repository.VerifyMCPToken(context.Background(), "secret"); !errors.Is(err, domainmcp.ErrUnauthorized) {
		t.Fatalf("expected empty scope unauthorized, got %v", err)
	}
	repository = NewMCPTokenRepositoryWithRunner(&fakeMCPTokenRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValuesWithErr(wantErr, []any{42, "mcp:read", int64(0), int64(0), 0})}}}}, nil, "uni1_")
	if _, err := repository.VerifyMCPToken(context.Background(), "secret"); !errors.Is(err, wantErr) {
		t.Fatalf("expected verify rows error, got %v", err)
	}
	repository = NewMCPTokenRepositoryWithRunner(&fakeMCPTokenRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{42, "mcp:read", int64(0), int64(0), 0})}}}}, nil, "uni1_")
	access, err := repository.VerifyMCPToken(context.Background(), "secret")
	if err != nil || !access.Authenticated {
		t.Fatalf("expected verify success without touch execer, got access=%+v err=%v", access, err)
	}

	repository = NewMCPTokenRepositoryWithRunner(&fakeMCPTokenRunner{}, &fakeMCPTokenRunner{err: wantErr}, "uni1_")
	if _, err := repository.CreateMCPOAuthCode(context.Background(), domainmcp.OAuthAuthorizationCode{}); !errors.Is(err, wantErr) {
		t.Fatalf("expected oauth create exec error, got %v", err)
	}
	repository = NewMCPTokenRepositoryWithRunner(&fakeMCPTokenRunner{}, &fakeMCPTokenRunner{result: fakeFleetSQLErrorResult{idErr: wantErr}}, "uni1_")
	if _, err := repository.CreateMCPOAuthCode(context.Background(), domainmcp.OAuthAuthorizationCode{}); !errors.Is(err, wantErr) {
		t.Fatalf("expected oauth last insert id error, got %v", err)
	}
	repository = NewMCPTokenRepositoryWithRunner(&fakeMCPTokenRunner{}, &fakeMCPTokenRunner{err: wantErr}, "uni1_")
	if _, err := repository.ConsumeMCPOAuthCode(context.Background(), "hash", 1); !errors.Is(err, wantErr) {
		t.Fatalf("expected oauth consume exec error, got %v", err)
	}
	repository = NewMCPTokenRepositoryWithRunner(&fakeMCPTokenRunner{}, &fakeMCPTokenRunner{result: fakeFleetSQLErrorResult{rowsErr: wantErr}}, "uni1_")
	if _, err := repository.ConsumeMCPOAuthCode(context.Background(), "hash", 1); !errors.Is(err, wantErr) {
		t.Fatalf("expected oauth consume rows affected error, got %v", err)
	}
	repository = NewMCPTokenRepositoryWithRunner(&fakeMCPTokenRunner{}, &fakeMCPTokenRunner{result: fakeSQLResult(0)}, "uni1_")
	if _, err := repository.ConsumeMCPOAuthCode(context.Background(), "hash", 1); !errors.Is(err, domainmcp.ErrUnauthorized) {
		t.Fatalf("expected oauth consume missing code unauthorized, got %v", err)
	}
	repository = NewMCPTokenRepositoryWithRunner(&fakeMCPTokenRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{err: wantErr}}}}, &fakeMCPTokenRunner{result: fakeSQLResult(1)}, "uni1_")
	if _, err := repository.ConsumeMCPOAuthCode(context.Background(), "hash", 1); !errors.Is(err, wantErr) {
		t.Fatalf("expected oauth consume query error, got %v", err)
	}
	repository = NewMCPTokenRepositoryWithRunner(&fakeMCPTokenRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues()}}}}, &fakeMCPTokenRunner{result: fakeSQLResult(1)}, "uni1_")
	if _, err := repository.ConsumeMCPOAuthCode(context.Background(), "hash", 1); !errors.Is(err, domainmcp.ErrUnauthorized) {
		t.Fatalf("expected oauth consume empty rows unauthorized, got %v", err)
	}
	repository = NewMCPTokenRepositoryWithRunner(&fakeMCPTokenRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{"bad", 42, "client", "redirect", "resource", "hash", "challenge", "S256", "mcp:read", int64(1), int64(2), int64(3)})}}}}, &fakeMCPTokenRunner{result: fakeSQLResult(1)}, "uni1_")
	if _, err := repository.ConsumeMCPOAuthCode(context.Background(), "hash", 1); err == nil {
		t.Fatalf("expected oauth consume scan error")
	}
	repository = NewMCPTokenRepositoryWithRunner(&fakeMCPTokenRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValuesWithErr(wantErr, []any{11, 42, "client", "redirect", "resource", "hash", "challenge", "S256", "mcp:read", int64(1), int64(2), int64(3)})}}}}, &fakeMCPTokenRunner{result: fakeSQLResult(1)}, "uni1_")
	if _, err := repository.ConsumeMCPOAuthCode(context.Background(), "hash", 1); !errors.Is(err, wantErr) {
		t.Fatalf("expected oauth consume rows error, got %v", err)
	}
}

type fakeMCPTokenRunner struct {
	fakeQueryer
	execCalls []fakeMCPTokenExecCall
	result    sql.Result
	err       error
}

type sequencedMCPExecer struct {
	calls  []string
	failAt int
	err    error
}

func (f *sequencedMCPExecer) ExecContext(_ context.Context, query string, _ ...any) (sql.Result, error) {
	f.calls = append(f.calls, query)
	if len(f.calls) == f.failAt {
		return nil, f.err
	}
	return fakeSQLResult(1), nil
}

type fakeMCPTokenExecCall struct {
	sql  string
	args []any
}

func (f *fakeMCPTokenRunner) ExecContext(_ context.Context, sql string, args ...any) (sql.Result, error) {
	f.execCalls = append(f.execCalls, fakeMCPTokenExecCall{sql: sql, args: args})
	if f.result == nil {
		f.result = fakeSQLResult(1)
	}
	return f.result, f.err
}
