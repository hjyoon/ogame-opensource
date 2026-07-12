package mysqlhealth

import (
	"context"
	"database/sql"
)

type Probe struct {
	db *sql.DB
}

func New(db *sql.DB) Probe {
	return Probe{db: db}
}

func (p Probe) Ready(ctx context.Context) bool {
	return p.db != nil && p.db.PingContext(ctx) == nil
}
