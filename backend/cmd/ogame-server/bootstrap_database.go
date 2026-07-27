package main

import (
	"context"
	"database/sql"
	"log/slog"
	"strings"
	"time"

	appsystem "github.com/hjyoon/ogame-opensource/backend/internal/application/system"
	"github.com/hjyoon/ogame-opensource/backend/internal/config"
	"github.com/hjyoon/ogame-opensource/backend/internal/infrastructure/mysqlcatalog"
	"github.com/hjyoon/ogame-opensource/backend/internal/infrastructure/mysqlhealth"
	"github.com/hjyoon/ogame-opensource/backend/internal/infrastructure/mysqlregistration"
	"github.com/hjyoon/ogame-opensource/backend/internal/infrastructure/sqlitedb"
)

type databasePools struct {
	master   *sql.DB
	universe *sql.DB
	driver   string
}

func openDatabasePools(cfg config.Config, logger *slog.Logger) databasePools {
	driver := strings.ToLower(strings.TrimSpace(cfg.DBDriver))
	if driver == "" {
		driver = "mysql"
	}
	pools := databasePools{driver: driver}
	if driver == "sqlite" {
		return openSQLiteDatabasePools(cfg, logger, pools)
	}
	if driver != "mysql" {
		logger.Error("unsupported database driver", "driver", driver)
		return pools
	}
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

func openSQLiteDatabasePools(cfg config.Config, logger *slog.Logger, pools databasePools) databasePools {
	options := sqlitedb.BootstrapOptions{
		Prefix:        cfg.UniDBPrefix,
		Secret:        cfg.UniDBSecret,
		Universe:      cfg.UniNumber,
		PublicBaseURL: cfg.PublicBaseURL,
		AdminEmail:    cfg.SQLiteAdminEmail,
		AdminPassword: cfg.SQLiteAdminPassword,
		RapidFire:     &cfg.UniRapidFire,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if cfg.MasterDBEnabled {
		db, err := sqlitedb.Open(cfg.SQLiteMasterPath)
		if err == nil && cfg.SQLiteAutoMigrate {
			err = sqlitedb.BootstrapMaster(ctx, db, options)
		}
		if err != nil {
			logger.Warn("sqlite master DB unavailable", "path", cfg.SQLiteMasterPath, "error", err)
			if db != nil {
				_ = db.Close()
			}
		} else {
			pools.master = db
		}
	}
	if cfg.UniDBEnabled {
		db, err := sqlitedb.Open(cfg.SQLiteUniversePath)
		if err == nil && cfg.SQLiteAutoMigrate {
			err = sqlitedb.BootstrapUniverse(ctx, db, options)
		}
		if err != nil {
			logger.Warn("sqlite universe DB unavailable", "path", cfg.SQLiteUniversePath, "error", err)
			if db != nil {
				_ = db.Close()
			}
		} else {
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
		if p.driver != "sqlite" {
			mods = mysqlhealth.NewModRuntimeProbe(p.universe, prefix)
		}
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
