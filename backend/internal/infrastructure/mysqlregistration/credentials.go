package mysqlregistration

import (
	"context"
	"crypto/md5"
	"database/sql"
	"fmt"
	"strings"

	domain "github.com/hjyoon/ogame-opensource/backend/internal/domain/publicsite"
)

type CredentialChecker struct {
	queryer Queryer
	prefix  string
	secret  string
}

func NewCredentialChecker(db *sql.DB, prefix string, secret string) CredentialChecker {
	return CredentialChecker{queryer: SQLQueryer{DB: db}, prefix: prefix, secret: secret}
}

func NewCredentialCheckerWithQueryer(queryer Queryer, prefix string, secret string) CredentialChecker {
	return CredentialChecker{queryer: queryer, prefix: prefix, secret: secret}
}

func (c CredentialChecker) CheckLoginCredentials(ctx context.Context, draft domain.LoginDraft) (domain.LoginCredentials, error) {
	usersTable, err := tableName(c.prefix, "users")
	if err != nil {
		return domain.LoginCredentials{}, err
	}

	rows, err := c.queryer.QueryContext(
		ctx,
		fmt.Sprintf("SELECT player_id, banned, banned_until FROM %s WHERE name = ? AND password = ? LIMIT 1", usersTable),
		strings.ToLower(strings.TrimSpace(draft.Login)),
		hashLegacyPassword(draft.Password, c.secret),
	)
	if err != nil {
		return domain.LoginCredentials{}, err
	}
	defer rows.Close()

	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return domain.LoginCredentials{}, err
		}
		return domain.LoginCredentials{Authenticated: false}, nil
	}

	var playerID int
	var banned int
	var bannedUntil int
	if err := rows.Scan(&playerID, &banned, &bannedUntil); err != nil {
		return domain.LoginCredentials{}, err
	}
	if err := rows.Err(); err != nil {
		return domain.LoginCredentials{}, err
	}

	return domain.LoginCredentials{
		Authenticated: true,
		PlayerID:      playerID,
		Banned:        banned != 0,
		BannedUntil:   bannedUntil,
	}, nil
}

func hashLegacyPassword(password string, secret string) string {
	sum := md5.Sum([]byte(password + secret))
	return fmt.Sprintf("%x", sum)
}
