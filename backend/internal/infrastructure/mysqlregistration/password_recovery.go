package mysqlregistration

import (
	"context"
	"database/sql"
	"fmt"

	domain "github.com/hjyoon/ogame-opensource/backend/internal/domain/publicsite"
)

type PasswordRecoveryRepository struct {
	queryer Queryer
	execer  Execer
	prefix  string
	secret  string
}

func NewPasswordRecoveryRepository(db *sql.DB, prefix string, secret string) PasswordRecoveryRepository {
	return PasswordRecoveryRepository{
		queryer: SQLQueryer{DB: db},
		execer:  SQLExecer{DB: db},
		prefix:  prefix,
		secret:  secret,
	}
}

func NewPasswordRecoveryRepositoryWithRunner(queryer Queryer, execer Execer, prefix string, secret string) PasswordRecoveryRepository {
	return PasswordRecoveryRepository{queryer: queryer, execer: execer, prefix: prefix, secret: secret}
}

func (r PasswordRecoveryRepository) RecoverPassword(ctx context.Context, email string, password string) (domain.PasswordRecoveryAccount, error) {
	usersTable, err := tableName(r.prefix, "users")
	if err != nil {
		return domain.PasswordRecoveryAccount{}, err
	}
	rows, err := r.queryer.QueryContext(
		ctx,
		fmt.Sprintf("SELECT player_id, COALESCE(oname, ''), COALESCE(pemail, '') FROM %s WHERE email = ? OR pemail = ? LIMIT 1", usersTable),
		email,
		email,
	)
	if err != nil {
		return domain.PasswordRecoveryAccount{}, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return domain.PasswordRecoveryAccount{}, err
		}
		return domain.PasswordRecoveryAccount{}, nil
	}

	account := domain.PasswordRecoveryAccount{Found: true}
	if err := rows.Scan(&account.PlayerID, &account.Character, &account.PermanentEmail); err != nil {
		return domain.PasswordRecoveryAccount{}, err
	}
	if err := rows.Err(); err != nil {
		return domain.PasswordRecoveryAccount{}, err
	}

	_, err = r.execer.ExecContext(
		ctx,
		fmt.Sprintf("UPDATE %s SET session = '', password = ? WHERE player_id = ?", usersTable),
		hashLegacyPassword(password, r.secret),
		account.PlayerID,
	)
	if err != nil {
		return domain.PasswordRecoveryAccount{}, err
	}
	return account, nil
}
