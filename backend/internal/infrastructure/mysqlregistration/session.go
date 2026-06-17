package mysqlregistration

import (
	"context"
	"database/sql"
	"fmt"

	domain "github.com/hjyoon/ogame-opensource/backend/internal/domain/publicsite"
)

type Execer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

type SQLExecer struct {
	DB *sql.DB
}

func (e SQLExecer) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return e.DB.ExecContext(ctx, query, args...)
}

type SessionStore struct {
	execer  Execer
	queryer Queryer
	prefix  string
}

func NewSessionStore(db *sql.DB, prefix string) SessionStore {
	return SessionStore{execer: SQLExecer{DB: db}, queryer: SQLQueryer{DB: db}, prefix: prefix}
}

func NewSessionStoreWithExecer(execer Execer, prefix string) SessionStore {
	return SessionStore{execer: execer, prefix: prefix}
}

func NewSessionStoreWithQueryer(queryer Queryer, prefix string) SessionStore {
	return SessionStore{queryer: queryer, prefix: prefix}
}

func (s SessionStore) SaveLoginSession(ctx context.Context, session domain.LoginSession, remoteAddr string) error {
	usersTable, err := tableName(s.prefix, "users")
	if err != nil {
		return err
	}
	_, err = s.execer.ExecContext(
		ctx,
		fmt.Sprintf("UPDATE %s SET lastlogin = ?, session = ?, private_session = ?, ip_addr = ? WHERE player_id = ?", usersTable),
		session.LastLogin,
		session.PublicID,
		session.PrivateID,
		remoteAddr,
		session.PlayerID,
	)
	return err
}

func (s SessionStore) FindGameSession(ctx context.Context, publicSession string) (domain.GameSession, error) {
	usersTable, err := tableName(s.prefix, "users")
	if err != nil {
		return domain.GameSession{}, err
	}
	rows, err := s.queryer.QueryContext(
		ctx,
		fmt.Sprintf("SELECT player_id, name, session, private_session, ip_addr, deact_ip, banned, hplanetid FROM %s WHERE session = ? LIMIT 1", usersTable),
		publicSession,
	)
	if err != nil {
		return domain.GameSession{}, err
	}
	defer rows.Close()

	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return domain.GameSession{}, err
		}
		return domain.GameSession{Found: false}, nil
	}

	var playerID int
	var commander string
	var sessionID string
	var privateID string
	var ipAddress string
	var deactIP int
	var banned int
	var homePlanetID int
	if err := rows.Scan(&playerID, &commander, &sessionID, &privateID, &ipAddress, &deactIP, &banned, &homePlanetID); err != nil {
		return domain.GameSession{}, err
	}
	if err := rows.Err(); err != nil {
		return domain.GameSession{}, err
	}

	return domain.GameSession{
		Found:          true,
		PlayerID:       playerID,
		Commander:      commander,
		PublicID:       sessionID,
		PrivateID:      privateID,
		IPAddress:      ipAddress,
		DisableIPCheck: deactIP != 0,
		Banned:         banned != 0,
		HomePlanetID:   homePlanetID,
	}, nil
}
