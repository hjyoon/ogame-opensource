package main

import (
	"io"
	"log/slog"
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
