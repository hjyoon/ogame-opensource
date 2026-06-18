package mysqlregistration

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	domain "github.com/hjyoon/ogame-opensource/backend/internal/domain/publicsite"
)

type AccountActivator struct {
	txer   registrationTxRunner
	prefix string
}

func NewAccountActivator(db *sql.DB, prefix string) AccountActivator {
	return AccountActivator{txer: SQLTxRunner{DB: db}, prefix: prefix}
}

func NewAccountActivatorWithRunner(txer registrationTxRunner, prefix string) AccountActivator {
	return AccountActivator{txer: txer, prefix: prefix}
}

func (a AccountActivator) ActivateRegistrationAccount(ctx context.Context, activationCode string) (domain.ActivatedAccount, error) {
	if a.txer == nil {
		return domain.ActivatedAccount{}, errors.New("registration activation dependency unavailable")
	}
	usersTable, err := tableName(a.prefix, "users")
	if err != nil {
		return domain.ActivatedAccount{}, err
	}

	var account domain.ActivatedAccount
	code := strings.TrimSpace(activationCode)
	err = a.txer.WithTx(ctx, func(tx registrationTx) error {
		rows, err := tx.QueryContext(ctx, fmt.Sprintf("SELECT player_id, email, validated FROM %s WHERE validatemd = ? LIMIT 1", usersTable), code)
		if err != nil {
			return err
		}

		if !rows.Next() {
			if err := rows.Err(); err != nil {
				_ = rows.Close()
				return err
			}
			return rows.Close()
		}
		var playerID int
		var email string
		var validated int
		if err := rows.Scan(&playerID, &email, &validated); err != nil {
			_ = rows.Close()
			return err
		}
		if err := rows.Err(); err != nil {
			_ = rows.Close()
			return err
		}
		if err := rows.Close(); err != nil {
			return err
		}
		if validated == 0 {
			if _, err := tx.ExecContext(ctx, fmt.Sprintf("UPDATE %s SET pemail = ? WHERE player_id = ?", usersTable), email, playerID); err != nil {
				return err
			}
		}
		if _, err := tx.ExecContext(ctx, fmt.Sprintf("UPDATE %s SET validatemd = '', validated = 1 WHERE player_id = ?", usersTable), playerID); err != nil {
			return err
		}
		account = domain.ActivatedAccount{Found: true, PlayerID: playerID}
		return nil
	})
	if err != nil {
		return domain.ActivatedAccount{}, err
	}
	return account, nil
}
