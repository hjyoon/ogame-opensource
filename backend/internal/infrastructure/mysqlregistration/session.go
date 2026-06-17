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
	execer Execer
	prefix string
}

func NewSessionStore(db *sql.DB, prefix string) SessionStore {
	return SessionStore{execer: SQLExecer{DB: db}, prefix: prefix}
}

func NewSessionStoreWithExecer(execer Execer, prefix string) SessionStore {
	return SessionStore{execer: execer, prefix: prefix}
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
