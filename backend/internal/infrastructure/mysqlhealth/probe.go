package mysqlhealth

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"strings"
)

var prefixPattern = regexp.MustCompile(`^[A-Za-z0-9_]+$`)

type Probe struct {
	db *sql.DB
}

type ModRuntimeProbe struct {
	db     *sql.DB
	prefix string
}

func NewModRuntimeProbe(db *sql.DB, prefix string) ModRuntimeProbe {
	return ModRuntimeProbe{db: db, prefix: prefix}
}

func (p ModRuntimeProbe) Ready(ctx context.Context) bool {
	if p.db == nil || !prefixPattern.MatchString(p.prefix) {
		return false
	}
	var modlist string
	err := p.db.QueryRowContext(ctx, fmt.Sprintf("SELECT COALESCE(modlist, '') FROM `%suni` LIMIT 1", p.prefix)).Scan(&modlist)
	return err == nil && strings.TrimSpace(modlist) == ""
}

func New(db *sql.DB) Probe {
	return Probe{db: db}
}

func (p Probe) Ready(ctx context.Context) bool {
	return p.db != nil && p.db.PingContext(ctx) == nil
}
