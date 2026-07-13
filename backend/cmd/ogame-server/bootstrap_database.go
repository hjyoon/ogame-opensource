package main

import (
	"database/sql"
	"log/slog"
	"time"

	appsystem "github.com/hjyoon/ogame-opensource/backend/internal/application/system"
	"github.com/hjyoon/ogame-opensource/backend/internal/config"
	"github.com/hjyoon/ogame-opensource/backend/internal/infrastructure/mysqlcatalog"
	"github.com/hjyoon/ogame-opensource/backend/internal/infrastructure/mysqlhealth"
	"github.com/hjyoon/ogame-opensource/backend/internal/infrastructure/mysqlregistration"
)

type databasePools struct {
	master   *sql.DB
	universe *sql.DB
}

func openDatabasePools(cfg config.Config, logger *slog.Logger) databasePools {
	var pools databasePools
	if cfg.MasterDBEnabled {
		db, err := mysqlcatalog.Open(mysqlcatalog.MasterDBConfig{Host: cfg.MasterDBHost, User: cfg.MasterDBUser, Password: cfg.MasterDBPassword, Name: cfg.MasterDBName})
		if err != nil {
			logger.Warn("master DB pool unavailable", "error", err)
		} else {
			configureDatabasePool(db, cfg)
			pools.master = db
		}
	}
	if cfg.UniDBEnabled {
		db, err := mysqlregistration.Open(mysqlregistration.UniverseDBConfig{Host: cfg.UniDBHost, User: cfg.UniDBUser, Password: cfg.UniDBPassword, Name: cfg.UniDBName})
		if err != nil {
			logger.Warn("universe DB pool unavailable", "error", err)
		} else {
			configureDatabasePool(db, cfg)
			pools.universe = db
		}
	}
	return pools
}

func configureDatabasePool(db *sql.DB, cfg config.Config) {
	if cfg.DBMaxOpenConns > 0 {
		db.SetMaxOpenConns(cfg.DBMaxOpenConns)
	}
	if cfg.DBMaxIdleConns >= 0 {
		db.SetMaxIdleConns(cfg.DBMaxIdleConns)
	}
	if cfg.DBConnMaxLifetimeSec > 0 {
		db.SetConnMaxLifetime(time.Duration(cfg.DBConnMaxLifetimeSec) * time.Second)
	}
}

func (p databasePools) readinessProbes(prefix string) (appsystem.ReadinessProbe, appsystem.ReadinessProbe, appsystem.ReadinessProbe) {
	var master, universe, mods appsystem.ReadinessProbe
	if p.master != nil {
		master = mysqlhealth.New(p.master)
	}
	if p.universe != nil {
		universe = mysqlhealth.New(p.universe)
		mods = mysqlhealth.NewModRuntimeProbe(p.universe, prefix)
	}
	return master, universe, mods
}

func (p databasePools) Close(logger *slog.Logger) {
	if p.master != nil {
		if err := p.master.Close(); err != nil {
			logger.Warn("master DB pool close failed", "error", err)
		}
	}
	if p.universe != nil {
		if err := p.universe.Close(); err != nil {
			logger.Warn("universe DB pool close failed", "error", err)
		}
	}
}
