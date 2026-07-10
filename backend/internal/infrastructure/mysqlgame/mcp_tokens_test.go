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
	runner := &fakeMCPTokenRunner{}
	repository := NewMCPTokenRepositoryWithRunner(runner, runner, "uni1_")

	if err := repository.EnsureMCPTokenSchema(context.Background()); err != nil {
		t.Fatalf("EnsureMCPTokenSchema returned error: %v", err)
	}
	if !strings.Contains(runner.execCalls[0].sql, "CREATE TABLE IF NOT EXISTS `uni1_mcp_tokens`") {
		t.Fatalf("unexpected schema SQL: %s", runner.execCalls[0].sql)
	}

	repository = NewMCPTokenRepositoryWithRunner(runner, runner, "uni1_;DROP")
	if err := repository.EnsureMCPTokenSchema(context.Background()); err == nil {
		t.Fatalf("expected unsafe prefix to be rejected")
	}
}

func TestMCPTokenRepositoryCreatesListsAndRevokesTokens(t *testing.T) {
	runner := &fakeMCPTokenRunner{
		fakeQueryer: fakeQueryer{results: []fakeQueryResult{{
			rows: fakeRowsFromValues([]any{7, "Desktop", "mcp:read,mcp:messages", int64(1700), int64(0), int64(0)}),
		}}},
		result: fakeSQLResult(7),
	}
	repository := NewMCPTokenRepositoryWithRunner(runner, runner, "uni1_")

	token, err := repository.CreateMCPToken(context.Background(), domainmcp.Token{
		PlayerID:  42,
		Name:      "Desktop",
		Scopes:    []string{domainmcp.ScopeRead, domainmcp.ScopeMessages, domainmcp.ScopeRead},
		CreatedAt: 1700,
	}, "hash")
	if err != nil {
		t.Fatalf("CreateMCPToken returned error: %v", err)
	}
	if token.ID != 7 || !strings.Contains(runner.execCalls[0].sql, "INSERT INTO `uni1_mcp_tokens`") || runner.execCalls[0].args[3] != "mcp:read,mcp:messages" {
		t.Fatalf("unexpected created token=%+v exec=%+v", token, runner.execCalls[0])
	}

	tokens, err := repository.ListMCPTokens(context.Background(), 42)
	if err != nil {
		t.Fatalf("ListMCPTokens returned error: %v", err)
	}
	if len(tokens) != 1 || tokens[0].ID != 7 || tokens[0].Scopes[1] != domainmcp.ScopeMessages || tokens[0].RevokedAt != 0 {
		t.Fatalf("unexpected tokens: %+v", tokens)
	}
	if !strings.Contains(runner.calls[0].sql, "revoked_at = 0") {
		t.Fatalf("expected list query to hide revoked tokens, got %s", runner.calls[0].sql)
	}

	revoked, err := repository.RevokeMCPToken(context.Background(), 42, 7, 1900)
	if err != nil {
		t.Fatalf("RevokeMCPToken returned error: %v", err)
	}
	if !revoked || !strings.Contains(runner.execCalls[1].sql, "UPDATE `uni1_mcp_tokens` SET revoked_at") {
		t.Fatalf("unexpected revoke result=%v exec=%+v", revoked, runner.execCalls[1])
	}
}

func TestMCPTokenRepositoryVerifiesTokenByHash(t *testing.T) {
	oldNow := mcpNowUnix
	mcpNowUnix = func() int64 { return 2000 }
	defer func() { mcpNowUnix = oldNow }()

	runner := &fakeMCPTokenRunner{
		fakeQueryer: fakeQueryer{results: []fakeQueryResult{{
			rows: fakeRowsFromValues([]any{42, "mcp:read,mcp:fleet", int64(0)}),
		}}},
		result: fakeSQLResult(1),
	}
	repository := NewMCPTokenRepositoryWithRunner(runner, runner, "uni1_")

	access, err := repository.VerifyMCPToken(context.Background(), "secret")
	if err != nil {
		t.Fatalf("VerifyMCPToken returned error: %v", err)
	}
	if !access.Authenticated || access.PlayerID != 42 || !access.HasScope(domainmcp.ScopeFleet) {
		t.Fatalf("unexpected access: %+v", access)
	}
	if runner.calls[0].args[0] != hashMCPToken("secret") {
		t.Fatalf("expected hashed lookup arg, got %+v", runner.calls[0].args)
	}
	if runner.execCalls[0].args[0] != int64(2000) {
		t.Fatalf("expected last_used_at touch, got %+v", runner.execCalls[0])
	}
}

func TestMCPTokenRepositoryRejectsMissingOrRevokedTokens(t *testing.T) {
	repository := NewMCPTokenRepositoryWithRunner(&fakeMCPTokenRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues()}}}}, nil, "uni1_")
	if _, err := repository.VerifyMCPToken(context.Background(), "missing"); !errors.Is(err, domainmcp.ErrUnauthorized) {
		t.Fatalf("expected missing token unauthorized, got %v", err)
	}

	repository = NewMCPTokenRepositoryWithRunner(&fakeMCPTokenRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{42, "mcp:read", int64(1)})}}}}, nil, "uni1_")
	if _, err := repository.VerifyMCPToken(context.Background(), "revoked"); !errors.Is(err, domainmcp.ErrUnauthorized) {
		t.Fatalf("expected revoked token unauthorized, got %v", err)
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
	if _, err := repository.CreateMCPToken(context.Background(), domainmcp.Token{}, "hash"); err == nil {
		t.Fatalf("expected nil execer create error")
	}
	if _, err := repository.RevokeMCPToken(context.Background(), 42, 7, 1); err == nil {
		t.Fatalf("expected nil execer revoke error")
	}
	if _, err := repository.VerifyMCPToken(context.Background(), "secret"); err == nil {
		t.Fatalf("expected nil queryer verify error")
	}

	wantErr := errors.New("db down")
	repository = NewMCPTokenRepositoryWithRunner(&fakeMCPTokenRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{err: wantErr}}}}, &fakeMCPTokenRunner{}, "uni1_")
	if _, err := repository.ListMCPTokens(context.Background(), 42); !errors.Is(err, wantErr) {
		t.Fatalf("expected list query error, got %v", err)
	}
	repository = NewMCPTokenRepositoryWithRunner(&fakeMCPTokenRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{"bad", "Desktop", "mcp:read", int64(1), int64(0), int64(0)})}}}}, &fakeMCPTokenRunner{}, "uni1_")
	if _, err := repository.ListMCPTokens(context.Background(), 42); err == nil {
		t.Fatalf("expected list scan error")
	}
	repository = NewMCPTokenRepositoryWithRunner(&fakeMCPTokenRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValuesWithErr(wantErr)}}}}, &fakeMCPTokenRunner{}, "uni1_")
	if _, err := repository.ListMCPTokens(context.Background(), 42); !errors.Is(err, wantErr) {
		t.Fatalf("expected list rows error, got %v", err)
	}

	repository = NewMCPTokenRepositoryWithRunner(&fakeMCPTokenRunner{}, &fakeMCPTokenRunner{err: wantErr}, "uni1_")
	if _, err := repository.CreateMCPToken(context.Background(), domainmcp.Token{}, "hash"); !errors.Is(err, wantErr) {
		t.Fatalf("expected create exec error, got %v", err)
	}
	repository = NewMCPTokenRepositoryWithRunner(&fakeMCPTokenRunner{}, &fakeMCPTokenRunner{result: fakeFleetSQLErrorResult{idErr: wantErr}}, "uni1_")
	if _, err := repository.CreateMCPToken(context.Background(), domainmcp.Token{}, "hash"); !errors.Is(err, wantErr) {
		t.Fatalf("expected last insert id error, got %v", err)
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

	repository = NewMCPTokenRepositoryWithRunner(&fakeMCPTokenRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{err: wantErr}}}}, nil, "uni1_")
	if _, err := repository.VerifyMCPToken(context.Background(), "secret"); !errors.Is(err, wantErr) {
		t.Fatalf("expected verify query error, got %v", err)
	}
	repository = NewMCPTokenRepositoryWithRunner(&fakeMCPTokenRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{"bad", "mcp:read", int64(0)})}}}}, nil, "uni1_")
	if _, err := repository.VerifyMCPToken(context.Background(), "secret"); err == nil {
		t.Fatalf("expected verify scan error")
	}
	repository = NewMCPTokenRepositoryWithRunner(&fakeMCPTokenRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{42, "", int64(0)})}}}}, nil, "uni1_")
	if _, err := repository.VerifyMCPToken(context.Background(), "secret"); !errors.Is(err, domainmcp.ErrUnauthorized) {
		t.Fatalf("expected empty scope unauthorized, got %v", err)
	}
	repository = NewMCPTokenRepositoryWithRunner(&fakeMCPTokenRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValuesWithErr(wantErr, []any{42, "mcp:read", int64(0)})}}}}, nil, "uni1_")
	if _, err := repository.VerifyMCPToken(context.Background(), "secret"); !errors.Is(err, wantErr) {
		t.Fatalf("expected verify rows error, got %v", err)
	}
	repository = NewMCPTokenRepositoryWithRunner(&fakeMCPTokenRunner{fakeQueryer: fakeQueryer{results: []fakeQueryResult{{rows: fakeRowsFromValues([]any{42, "mcp:read", int64(0)})}}}}, nil, "uni1_")
	access, err := repository.VerifyMCPToken(context.Background(), "secret")
	if err != nil || !access.Authenticated {
		t.Fatalf("expected verify success without touch execer, got access=%+v err=%v", access, err)
	}
}

type fakeMCPTokenRunner struct {
	fakeQueryer
	execCalls []fakeMCPTokenExecCall
	result    sql.Result
	err       error
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
