package mysqlcatalog

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
	domain "github.com/hjyoon/ogame-opensource/backend/internal/domain/publicsite"
)

type MasterDBConfig struct {
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

type MasterUniverseCatalog struct {
	queryer Queryer
}

func NewMasterUniverseCatalog(db *sql.DB) MasterUniverseCatalog {
	return MasterUniverseCatalog{queryer: SQLQueryer{DB: db}}
}

func NewMasterUniverseCatalogWithQueryer(queryer Queryer) MasterUniverseCatalog {
	return MasterUniverseCatalog{queryer: queryer}
}

func Open(config MasterDBConfig) (*sql.DB, error) {
	return sql.Open("mysql", DSN(config))
}

func DSN(config MasterDBConfig) string {
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

func (c MasterUniverseCatalog) ListUniverses(ctx context.Context) ([]domain.Universe, error) {
	rows, err := c.queryer.QueryContext(ctx, "SELECT num, uniurl FROM unis ORDER BY num ASC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	universes := make([]domain.Universe, 0)
	for rows.Next() {
		var number int
		var rawURL string
		if err := rows.Scan(&number, &rawURL); err != nil {
			return nil, err
		}
		baseURL := normalizeLegacyURL(rawURL)
		if number <= 0 || baseURL == "" {
			continue
		}
		universes = append(universes, domain.Universe{
			Number:     number,
			Name:       fmt.Sprintf("Universe %d", number),
			BaseURL:    baseURL,
			Language:   "en",
			Speed:      1,
			FleetSpeed: 1,
			Status:     domain.UniverseOnline,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return universes, nil
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

func normalizeLegacyURL(rawURL string) string {
	value := strings.TrimSpace(rawURL)
	if value == "" {
		return value
	}
	if strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://") {
		return value
	}
	return "http://" + value
}
