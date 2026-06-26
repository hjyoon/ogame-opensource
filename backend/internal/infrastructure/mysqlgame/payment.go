package mysqlgame

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	appgame "github.com/hjyoon/ogame-opensource/backend/internal/application/game"
	domaingame "github.com/hjyoon/ogame-opensource/backend/internal/domain/game"
)

type PaymentRepository struct {
	queryer       Queryer
	execer        Execer
	masterQueryer Queryer
	masterExecer  Execer
	prefix        string
	uniNumber     int
}

func NewPaymentRepository(db *sql.DB, masterDB *sql.DB, prefix string, uniNumber int) PaymentRepository {
	runner := SQLQueryer{DB: db}
	master := SQLQueryer{DB: masterDB}
	return NewPaymentRepositoryWithRunners(runner, runner, master, master, prefix, uniNumber)
}

func NewPaymentRepositoryWithRunners(queryer Queryer, execer Execer, masterQueryer Queryer, masterExecer Execer, prefix string, uniNumber int) PaymentRepository {
	if uniNumber <= 0 {
		uniNumber = 1
	}
	return PaymentRepository{
		queryer:       queryer,
		execer:        execer,
		masterQueryer: masterQueryer,
		masterExecer:  masterExecer,
		prefix:        prefix,
		uniNumber:     uniNumber,
	}
}

func (r PaymentRepository) CheckCoupon(ctx context.Context, query appgame.PaymentMutationQuery) (domaingame.PaymentCoupon, bool, error) {
	return r.loadActivePaymentCoupon(ctx, normalizeCouponCode(query.CouponCode))
}

func (r PaymentRepository) ActivateCoupon(ctx context.Context, query appgame.PaymentMutationQuery) (domaingame.PaymentCoupon, bool, error) {
	if r.execer == nil || r.masterExecer == nil {
		return domaingame.PaymentCoupon{}, false, errors.New("payment updater unavailable")
	}
	code := normalizeCouponCode(query.CouponCode)
	coupon, found, err := r.loadActivePaymentCoupon(ctx, code)
	if err != nil || !found {
		return domaingame.PaymentCoupon{}, false, err
	}
	user, found, err := r.loadPaymentUser(ctx, query.PlayerID)
	if err != nil || !found {
		return domaingame.PaymentCoupon{}, false, err
	}
	result, err := r.masterExecer.ExecContext(
		ctx,
		"UPDATE coupons SET used = 1, user_uni = ?, user_id = ?, user_name = ? WHERE id = ? AND used = 0",
		r.uniNumber,
		user.ID,
		user.Name,
		coupon.ID,
	)
	if err != nil {
		return domaingame.PaymentCoupon{}, false, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return domaingame.PaymentCoupon{}, false, err
	}
	if affected == 0 {
		return domaingame.PaymentCoupon{}, false, nil
	}
	usersTable, err := tableName(r.prefix, "users")
	if err != nil {
		return domaingame.PaymentCoupon{}, false, err
	}
	if _, err := r.execer.ExecContext(ctx, fmt.Sprintf("UPDATE %s SET dm = dm + ? WHERE player_id = ? LIMIT 1", usersTable), coupon.Amount, user.ID); err != nil {
		return domaingame.PaymentCoupon{}, false, err
	}
	coupon.Used = true
	coupon.UserUniverse = r.uniNumber
	coupon.UserID = user.ID
	coupon.UserName = user.Name
	return coupon, true, nil
}

func (r PaymentRepository) loadActivePaymentCoupon(ctx context.Context, code string) (domaingame.PaymentCoupon, bool, error) {
	if r.masterQueryer == nil {
		return domaingame.PaymentCoupon{}, false, errors.New("payment master DB unavailable")
	}
	if code == "" {
		return domaingame.PaymentCoupon{}, false, nil
	}
	rows, err := r.masterQueryer.QueryContext(
		ctx,
		"SELECT id, COALESCE(code, ''), COALESCE(amount, 0), COALESCE(used, 0), COALESCE(user_uni, 0), COALESCE(user_id, 0), COALESCE(user_name, '') FROM coupons WHERE used = 0 AND code = ? LIMIT 1",
		code,
	)
	if err != nil {
		return domaingame.PaymentCoupon{}, false, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return domaingame.PaymentCoupon{}, false, err
		}
		return domaingame.PaymentCoupon{}, false, nil
	}
	var coupon domaingame.PaymentCoupon
	var used int
	if err := rows.Scan(&coupon.ID, &coupon.Code, &coupon.Amount, &used, &coupon.UserUniverse, &coupon.UserID, &coupon.UserName); err != nil {
		return domaingame.PaymentCoupon{}, false, err
	}
	coupon.Used = used != 0
	return coupon, true, rows.Err()
}

type paymentUser struct {
	ID   int
	Name string
}

func (r PaymentRepository) loadPaymentUser(ctx context.Context, playerID int) (paymentUser, bool, error) {
	usersTable, err := tableName(r.prefix, "users")
	if err != nil {
		return paymentUser{}, false, err
	}
	rows, err := r.queryer.QueryContext(ctx, fmt.Sprintf("SELECT player_id, COALESCE(oname, '') FROM %s WHERE player_id = ? LIMIT 1", usersTable), playerID)
	if err != nil {
		return paymentUser{}, false, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return paymentUser{}, false, err
		}
		return paymentUser{}, false, nil
	}
	var user paymentUser
	if err := rows.Scan(&user.ID, &user.Name); err != nil {
		return paymentUser{}, false, err
	}
	return user, true, rows.Err()
}

func normalizeCouponCode(code string) string {
	return strings.ToUpper(strings.TrimSpace(code))
}
