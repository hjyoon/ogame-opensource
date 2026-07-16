package main

import (
	"context"
	"io"
	"log/slog"
	"path/filepath"
	"testing"

	"github.com/hjyoon/ogame-opensource/backend/internal/config"
	"github.com/hjyoon/ogame-opensource/backend/internal/infrastructure/mysqlregistration"
)

func TestConfigureAndCloseSharedDatabasePool(t *testing.T) {
	db, err := mysqlregistration.Open(mysqlregistration.UniverseDBConfig{Host: "127.0.0.1:1", User: "test", Name: "test"})
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{DBMaxOpenConns: 7, DBMaxIdleConns: 2, DBConnMaxLifetimeSec: 60}
	configureDatabasePool(db, cfg)
	if db.Stats().MaxOpenConnections != 7 {
		t.Fatalf("unexpected max open connections: %+v", db.Stats())
	}
	pools := databasePools{universe: db}
	master, universe, mods := pools.readinessProbes("uni1_")
	if master != nil || universe == nil || mods == nil {
		t.Fatalf("unexpected probes: master=%v universe=%v mods=%v", master, universe, mods)
	}
	pools.Close(slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err := db.Ping(); err == nil {
		t.Fatal("closed shared pool must reject ping")
	}
}

func TestOpenSQLiteDatabasePoolsBootstrapsBothDatabases(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Config{
		DBDriver:            "sqlite",
		MasterDBEnabled:     true,
		UniDBEnabled:        true,
		SQLiteMasterPath:    filepath.Join(dir, "master.sqlite"),
		SQLiteUniversePath:  filepath.Join(dir, "universe.sqlite"),
		SQLiteAutoMigrate:   true,
		SQLiteAdminEmail:    "admin@example.local",
		SQLiteAdminPassword: "admin",
		UniDBPrefix:         "uni1_",
		UniDBSecret:         "secret",
		UniNumber:           1,
		PublicBaseURL:       "http://localhost:8080",
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	pools := openDatabasePools(cfg, logger)
	defer pools.Close(logger)
	if pools.driver != "sqlite" || pools.master == nil || pools.universe == nil {
		t.Fatalf("unexpected SQLite pools: %+v", pools)
	}
	if err := pools.master.PingContext(context.Background()); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := pools.universe.QueryRowContext(context.Background(), "SELECT COUNT(*) FROM `uni1_users`").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("expected seeded SQLite users, got %d", count)
	}
	master, universe, mods := pools.readinessProbes("uni1_")
	if master == nil || universe == nil || mods != nil {
		t.Fatalf("unexpected SQLite probes: master=%v universe=%v mods=%v", master, universe, mods)
	}
}
