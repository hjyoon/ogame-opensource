package mysqlregistration

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net"
	"regexp"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
	domain "github.com/hjyoon/ogame-opensource/backend/internal/domain/publicsite"
)

type UniverseDBConfig struct {
	Host     string
	User     string
	Password string
	Name     string
}

type Queryer interface {
	QueryContext(ctx context.Context, query string, args ...any) (Rows, error)
}

type Rows interface {
	Close() error
	Err() error
	Next() bool
	Scan(dest ...any) error
}

type SQLQueryer struct {
	DB *sql.DB
}

func (q SQLQueryer) QueryContext(ctx context.Context, query string, args ...any) (Rows, error) {
	return q.DB.QueryContext(ctx, query, args...)
}

type AvailabilityChecker struct {
	queryer Queryer
	prefix  string
}

func NewAvailabilityChecker(db *sql.DB, prefix string) AvailabilityChecker {
	return AvailabilityChecker{queryer: SQLQueryer{DB: db}, prefix: prefix}
}

func NewAvailabilityCheckerWithQueryer(queryer Queryer, prefix string) AvailabilityChecker {
	return AvailabilityChecker{queryer: queryer, prefix: prefix}
}

func Open(config UniverseDBConfig) (*sql.DB, error) {
	return sql.Open("mysql", DSN(config))
}

func DSN(config UniverseDBConfig) string {
	cfg := mysql.NewConfig()
	cfg.User = config.User
	cfg.Passwd = config.Password
	cfg.Net = "tcp"
	cfg.Addr = hostPort(config.Host)
	cfg.DBName = config.Name
	cfg.Timeout = 2 * time.Second
	cfg.ReadTimeout = 2 * time.Second
	cfg.WriteTimeout = 2 * time.Second
	cfg.Params = map[string]string{
		"charset":   "utf8mb4",
		"collation": "utf8mb4_general_ci",
	}
	return cfg.FormatDSN()
}

func (c AvailabilityChecker) CheckRegistrationAvailability(ctx context.Context, draft domain.RegistrationDraft) (domain.RegistrationAvailability, error) {
	usersTable, err := tableName(c.prefix, "users")
	if err != nil {
		return domain.RegistrationAvailability{}, err
	}
	uniTable, err := tableName(c.prefix, "uni")
	if err != nil {
		return domain.RegistrationAvailability{}, err
	}

	characterExists, err := c.exists(ctx, fmt.Sprintf("SELECT 1 FROM %s WHERE name = ? LIMIT 1", usersTable), strings.ToLower(strings.TrimSpace(draft.Character)))
	if err != nil {
		return domain.RegistrationAvailability{}, err
	}
	email := strings.ToLower(strings.TrimSpace(draft.Email))
	emailExists, err := c.exists(ctx, fmt.Sprintf("SELECT 1 FROM %s WHERE email = ? OR pemail = ? LIMIT 1", usersTable), email, email)
	if err != nil {
		return domain.RegistrationAvailability{}, err
	}
	userCount, err := c.singleInt(ctx, fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE admin = 0", usersTable))
	if err != nil {
		return domain.RegistrationAvailability{}, err
	}
	maxUsers, err := c.singleInt(ctx, fmt.Sprintf("SELECT maxusers FROM %s LIMIT 1", uniTable))
	if err != nil {
		return domain.RegistrationAvailability{}, err
	}

	return domain.RegistrationAvailability{
		CharacterExists: characterExists,
		EmailExists:     emailExists,
		UserCount:       userCount,
		MaxUsers:        maxUsers,
	}, nil
}

func (c AvailabilityChecker) exists(ctx context.Context, query string, args ...any) (bool, error) {
	rows, err := c.queryer.QueryContext(ctx, query, args...)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	exists := rows.Next()
	if err := rows.Err(); err != nil {
		return false, err
	}
	return exists, nil
}

func (c AvailabilityChecker) singleInt(ctx context.Context, query string, args ...any) (int, error) {
	rows, err := c.queryer.QueryContext(ctx, query, args...)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	if !rows.Next() {
		return 0, nil
	}
	var value int
	if err := rows.Scan(&value); err != nil {
		return 0, err
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}
	return value, nil
}

func hostPort(host string) string {
	host = strings.TrimSpace(host)
	if host == "" {
		return "mysql:3306"
	}
	if _, _, err := net.SplitHostPort(host); err == nil {
		return host
	}
	return net.JoinHostPort(host, "3306")
}

var identifierPattern = regexp.MustCompile(`^[A-Za-z0-9_]+$`)

func tableName(prefix string, name string) (string, error) {
	identifier := prefix + name
	if !identifierPattern.MatchString(identifier) {
		return "", errors.New("invalid database table prefix")
	}
	return "`" + identifier + "`", nil
}
