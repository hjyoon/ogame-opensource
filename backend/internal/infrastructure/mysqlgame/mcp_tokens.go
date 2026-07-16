package mysqlgame

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
	domainmcp "github.com/hjyoon/ogame-opensource/backend/internal/domain/mcp"
)

type MCPTokenRepository struct {
	queryer Queryer
	execer  Execer
	prefix  string
}

func NewMCPTokenRepository(db *sql.DB, prefix string) MCPTokenRepository {
	runner := SQLQueryer{DB: db}
	return NewMCPTokenRepositoryWithRunner(runner, runner, prefix)
}

func NewMCPTokenRepositoryWithRunner(queryer Queryer, execer Execer, prefix string) MCPTokenRepository {
	return MCPTokenRepository{queryer: queryer, execer: execer, prefix: prefix}
}

func (r MCPTokenRepository) EnsureMCPTokenSchema(ctx context.Context) error {
	if r.execer == nil {
		return errors.New("mcp token repository execer unavailable")
	}
	table, err := tableName(r.prefix, "mcp_tokens")
	if err != nil {
		return err
	}
	physicalTable := r.prefix + "mcp_tokens"
	_, err = r.execer.ExecContext(ctx, "CREATE TABLE IF NOT EXISTS "+table+" ("+
		"id INT NOT NULL AUTO_INCREMENT,"+
		"player_id INT NOT NULL,"+
		"name VARCHAR(64) NOT NULL,"+
		"token_hash CHAR(64) NOT NULL,"+
		"scopes TEXT NOT NULL,"+
		"created_at INT NOT NULL,"+
		"expires_at INT NOT NULL DEFAULT 0,"+
		"last_used_at INT NOT NULL DEFAULT 0,"+
		"revoked_at INT NOT NULL DEFAULT 0,"+
		"PRIMARY KEY (id),"+
		"UNIQUE KEY uniq_token_hash (token_hash),"+
		"KEY idx_player_id (player_id),"+
		"KEY idx_expires_at (expires_at),"+
		"KEY idx_revoked_at (revoked_at)"+
		") CHARACTER SET utf8 COLLATE utf8_general_ci")
	if err != nil {
		return err
	}
	return r.ensureMCPTokenExpiresColumn(ctx, table, physicalTable)
}

func (r MCPTokenRepository) EnsureMCPOAuthCodeSchema(ctx context.Context) error {
	if r.execer == nil {
		return errors.New("mcp oauth code repository execer unavailable")
	}
	if r.queryer == nil {
		return errors.New("mcp oauth code repository queryer unavailable")
	}
	table, err := tableName(r.prefix, "mcp_oauth_codes")
	if err != nil {
		return err
	}
	physicalTable := r.prefix + "mcp_oauth_codes"
	_, err = r.execer.ExecContext(ctx, "CREATE TABLE IF NOT EXISTS "+table+" ("+
		"id INT NOT NULL AUTO_INCREMENT,"+
		"player_id INT NOT NULL,"+
		"client_id VARCHAR(128) NOT NULL,"+
		"redirect_uri TEXT NOT NULL,"+
		"resource TEXT NOT NULL,"+
		"code_hash CHAR(64) NOT NULL,"+
		"code_challenge VARCHAR(128) NOT NULL,"+
		"code_challenge_method VARCHAR(16) NOT NULL,"+
		"scopes TEXT NOT NULL,"+
		"created_at INT NOT NULL,"+
		"expires_at INT NOT NULL,"+
		"consumed_at INT NOT NULL DEFAULT 0,"+
		"PRIMARY KEY (id),"+
		"UNIQUE KEY uniq_code_hash (code_hash),"+
		"KEY idx_player_id (player_id),"+
		"KEY idx_expires_at (expires_at),"+
		"KEY idx_consumed_at (consumed_at)"+
		") CHARACTER SET utf8 COLLATE utf8_general_ci")
	if err != nil {
		return err
	}
	return r.ensureOAuthCodeResourceColumn(ctx, table, physicalTable)
}

func (r MCPTokenRepository) ListMCPTokens(ctx context.Context, playerID int) ([]domainmcp.Token, error) {
	if r.queryer == nil {
		return nil, errors.New("mcp token repository queryer unavailable")
	}
	table, err := tableName(r.prefix, "mcp_tokens")
	if err != nil {
		return nil, err
	}
	now := mcpNowUnix()
	rows, err := r.queryer.QueryContext(ctx, "SELECT id, name, scopes, created_at, expires_at, last_used_at, revoked_at FROM "+table+" WHERE player_id = ? AND revoked_at = 0 AND (expires_at = 0 OR expires_at >= ?) ORDER BY id DESC", playerID, now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tokens := make([]domainmcp.Token, 0)
	for rows.Next() {
		var token domainmcp.Token
		var scopes string
		if err := rows.Scan(&token.ID, &token.Name, &scopes, &token.CreatedAt, &token.ExpiresAt, &token.LastUsedAt, &token.RevokedAt); err != nil {
			return nil, err
		}
		token.Scopes = parseMCPScopes(scopes)
		tokens = append(tokens, token)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return tokens, nil
}

func (r MCPTokenRepository) CreateMCPToken(ctx context.Context, token domainmcp.Token, tokenHash string) (domainmcp.Token, error) {
	if r.execer == nil {
		return domainmcp.Token{}, errors.New("mcp token repository execer unavailable")
	}
	table, err := tableName(r.prefix, "mcp_tokens")
	if err != nil {
		return domainmcp.Token{}, err
	}
	result, err := r.execer.ExecContext(ctx, "INSERT INTO "+table+" (player_id, name, token_hash, scopes, created_at, expires_at, last_used_at, revoked_at) VALUES (?, ?, ?, ?, ?, ?, 0, 0)",
		token.PlayerID,
		token.Name,
		strings.TrimSpace(tokenHash),
		joinMCPScopes(token.Scopes),
		token.CreatedAt,
		token.ExpiresAt,
	)
	if err != nil {
		return domainmcp.Token{}, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return domainmcp.Token{}, err
	}
	token.ID = int(id)
	token.LastUsedAt = 0
	token.RevokedAt = 0
	return token, nil
}

func (r MCPTokenRepository) RevokeMCPToken(ctx context.Context, playerID int, tokenID int, revokedAt int64) (bool, error) {
	if r.execer == nil {
		return false, errors.New("mcp token repository execer unavailable")
	}
	table, err := tableName(r.prefix, "mcp_tokens")
	if err != nil {
		return false, err
	}
	result, err := r.execer.ExecContext(ctx, "UPDATE "+table+" SET revoked_at = ? WHERE id = ? AND player_id = ? AND revoked_at = 0", revokedAt, tokenID, playerID)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return affected > 0, nil
}

func (r MCPTokenRepository) RevokeMCPTokenByHash(ctx context.Context, tokenHash string, revokedAt int64) (bool, error) {
	if r.execer == nil {
		return false, errors.New("mcp token repository execer unavailable")
	}
	table, err := tableName(r.prefix, "mcp_tokens")
	if err != nil {
		return false, err
	}
	result, err := r.execer.ExecContext(ctx, "UPDATE "+table+" SET revoked_at = ? WHERE token_hash = ? AND revoked_at = 0", revokedAt, strings.TrimSpace(tokenHash))
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return affected > 0, nil
}

func (r MCPTokenRepository) CreateMCPOAuthCode(ctx context.Context, code domainmcp.OAuthAuthorizationCode) (domainmcp.OAuthAuthorizationCode, error) {
	if r.execer == nil {
		return domainmcp.OAuthAuthorizationCode{}, errors.New("mcp oauth code repository execer unavailable")
	}
	table, err := tableName(r.prefix, "mcp_oauth_codes")
	if err != nil {
		return domainmcp.OAuthAuthorizationCode{}, err
	}
	result, err := r.execer.ExecContext(ctx, "INSERT INTO "+table+" (player_id, client_id, redirect_uri, resource, code_hash, code_challenge, code_challenge_method, scopes, created_at, expires_at, consumed_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 0)",
		code.PlayerID,
		strings.TrimSpace(code.ClientID),
		strings.TrimSpace(code.RedirectURI),
		strings.TrimRight(strings.TrimSpace(code.Resource), "/"),
		strings.TrimSpace(code.CodeHash),
		strings.TrimSpace(code.CodeChallenge),
		strings.TrimSpace(code.CodeChallengeMethod),
		joinMCPScopes(code.Scopes),
		code.CreatedAt,
		code.ExpiresAt,
	)
	if err != nil {
		return domainmcp.OAuthAuthorizationCode{}, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return domainmcp.OAuthAuthorizationCode{}, err
	}
	code.ID = int(id)
	code.ClientID = strings.TrimSpace(code.ClientID)
	code.RedirectURI = strings.TrimSpace(code.RedirectURI)
	code.Resource = strings.TrimRight(strings.TrimSpace(code.Resource), "/")
	code.CodeHash = strings.TrimSpace(code.CodeHash)
	code.CodeChallenge = strings.TrimSpace(code.CodeChallenge)
	code.CodeChallengeMethod = strings.TrimSpace(code.CodeChallengeMethod)
	code.Scopes = parseMCPScopes(joinMCPScopes(code.Scopes))
	code.ConsumedAt = 0
	return code, nil
}

func (r MCPTokenRepository) ConsumeMCPOAuthCode(ctx context.Context, codeHash string, now int64) (domainmcp.OAuthAuthorizationCode, error) {
	if r.queryer == nil || r.execer == nil {
		return domainmcp.OAuthAuthorizationCode{}, errors.New("mcp oauth code repository unavailable")
	}
	table, err := tableName(r.prefix, "mcp_oauth_codes")
	if err != nil {
		return domainmcp.OAuthAuthorizationCode{}, err
	}
	codeHash = strings.TrimSpace(codeHash)
	result, err := r.execer.ExecContext(ctx, "UPDATE "+table+" SET consumed_at = ? WHERE code_hash = ? AND consumed_at = 0 AND expires_at >= ?", now, codeHash, now)
	if err != nil {
		return domainmcp.OAuthAuthorizationCode{}, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return domainmcp.OAuthAuthorizationCode{}, err
	}
	if affected == 0 {
		return domainmcp.OAuthAuthorizationCode{}, domainmcp.ErrUnauthorized
	}
	rows, err := r.queryer.QueryContext(ctx, "SELECT id, player_id, client_id, redirect_uri, COALESCE(resource, ''), code_hash, code_challenge, code_challenge_method, scopes, created_at, expires_at, consumed_at FROM "+table+" WHERE code_hash = ? LIMIT 1", codeHash)
	if err != nil {
		return domainmcp.OAuthAuthorizationCode{}, err
	}
	defer rows.Close()
	if !rows.Next() {
		return domainmcp.OAuthAuthorizationCode{}, domainmcp.ErrUnauthorized
	}
	var code domainmcp.OAuthAuthorizationCode
	var scopes string
	if err := rows.Scan(&code.ID, &code.PlayerID, &code.ClientID, &code.RedirectURI, &code.Resource, &code.CodeHash, &code.CodeChallenge, &code.CodeChallengeMethod, &scopes, &code.CreatedAt, &code.ExpiresAt, &code.ConsumedAt); err != nil {
		return domainmcp.OAuthAuthorizationCode{}, err
	}
	if err := rows.Err(); err != nil {
		return domainmcp.OAuthAuthorizationCode{}, err
	}
	code.Scopes = parseMCPScopes(scopes)
	return code, nil
}

func (r MCPTokenRepository) ensureOAuthCodeResourceColumn(ctx context.Context, table string, physicalTable string) error {
	rows, err := r.queryer.QueryContext(ctx, "SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ? AND COLUMN_NAME = 'resource'", physicalTable)
	if err != nil {
		return err
	}
	defer rows.Close()
	if !rows.Next() {
		return errors.New("mcp oauth code resource column check returned no rows")
	}
	var count int
	if err := rows.Scan(&count); err != nil {
		return err
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	_, err = r.execer.ExecContext(ctx, "ALTER TABLE "+table+" ADD COLUMN resource TEXT NULL AFTER redirect_uri")
	return err
}

func (r MCPTokenRepository) ensureMCPTokenExpiresColumn(ctx context.Context, table string, physicalTable string) error {
	if r.queryer == nil {
		return nil
	}
	rows, err := r.queryer.QueryContext(ctx, "SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ? AND COLUMN_NAME = 'expires_at'", physicalTable)
	if err != nil {
		return err
	}
	defer rows.Close()
	if !rows.Next() {
		return errors.New("mcp token expires column check returned no rows")
	}
	var count int
	if err := rows.Scan(&count); err != nil {
		return err
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	_, err = r.execer.ExecContext(ctx, "ALTER TABLE "+table+" ADD COLUMN expires_at INT NOT NULL DEFAULT 0 AFTER created_at, ADD KEY idx_expires_at (expires_at)")
	return err
}

func (r MCPTokenRepository) VerifyMCPToken(ctx context.Context, secret string) (domainmcp.Access, error) {
	if r.queryer == nil {
		return domainmcp.Access{}, errors.New("mcp token repository queryer unavailable")
	}
	table, err := tableName(r.prefix, "mcp_tokens")
	if err != nil {
		return domainmcp.Access{}, err
	}
	usersTable, err := tableName(r.prefix, "users")
	if err != nil {
		return domainmcp.Access{}, err
	}
	now := mcpNowUnix()
	rows, err := r.queryer.QueryContext(ctx, "SELECT t.player_id, t.scopes, t.revoked_at, t.expires_at, COALESCE(u.admin, 0) FROM "+table+" t INNER JOIN "+usersTable+" u ON u.player_id = t.player_id WHERE t.token_hash = ? LIMIT 1", hashMCPToken(secret))
	if err != nil {
		return domainmcp.Access{}, err
	}
	defer rows.Close()
	if !rows.Next() {
		return domainmcp.Access{}, domainmcp.ErrUnauthorized
	}
	var playerID int
	var scopesRaw string
	var revokedAt int64
	var expiresAt int64
	var userType int
	if err := rows.Scan(&playerID, &scopesRaw, &revokedAt, &expiresAt, &userType); err != nil {
		return domainmcp.Access{}, err
	}
	if err := rows.Err(); err != nil {
		return domainmcp.Access{}, err
	}
	scopes := parseMCPScopes(scopesRaw)
	if playerID <= 0 || revokedAt > 0 || (expiresAt > 0 && expiresAt < now) || len(scopes) == 0 {
		return domainmcp.Access{}, domainmcp.ErrUnauthorized
	}
	if r.execer != nil {
		_, _ = r.execer.ExecContext(ctx, "UPDATE "+table+" SET last_used_at = ? WHERE token_hash = ? AND revoked_at = 0 AND (expires_at = 0 OR expires_at >= ?)", now, hashMCPToken(secret), now)
	}
	return domainmcp.Access{
		Authenticated: true,
		PlayerID:      playerID,
		UserType:      userType,
		Role:          domaingame.AdminRoleName(userType),
		Scopes:        scopes,
	}, nil
}

func (r MCPTokenRepository) GetMCPUserType(ctx context.Context, playerID int) (int, error) {
	if r.queryer == nil {
		return domaingame.AdminLevelPlayer, errors.New("mcp token repository queryer unavailable")
	}
	usersTable, err := tableName(r.prefix, "users")
	if err != nil {
		return domaingame.AdminLevelPlayer, err
	}
	rows, err := r.queryer.QueryContext(ctx, "SELECT COALESCE(admin, 0) FROM "+usersTable+" WHERE player_id = ? LIMIT 1", playerID)
	if err != nil {
		return domaingame.AdminLevelPlayer, err
	}
	defer rows.Close()
	if !rows.Next() {
		return domaingame.AdminLevelPlayer, domainmcp.ErrUnauthorized
	}
	var userType int
	if err := rows.Scan(&userType); err != nil {
		return domaingame.AdminLevelPlayer, err
	}
	if err := rows.Err(); err != nil {
		return domaingame.AdminLevelPlayer, err
	}
	return userType, nil
}

var mcpNowUnix = func() int64 {
	return time.Now().Unix()
}

func hashMCPToken(secret string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(secret)))
	return hex.EncodeToString(sum[:])
}

func joinMCPScopes(scopes []string) string {
	seen := map[string]bool{}
	clean := make([]string, 0, len(scopes))
	for _, raw := range scopes {
		scope := strings.TrimSpace(raw)
		if scope == "" || seen[scope] {
			continue
		}
		seen[scope] = true
		clean = append(clean, scope)
	}
	return strings.Join(clean, ",")
}

func parseMCPScopes(raw string) []string {
	joined := joinMCPScopes(strings.Split(raw, ","))
	if joined == "" {
		return nil
	}
	return strings.Split(joined, ",")
}
