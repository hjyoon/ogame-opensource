package main

import (
	"log"
	"net/http"
	"time"

	"github.com/hjyoon/ogame-opensource/backend/internal/config"
	"github.com/hjyoon/ogame-opensource/backend/internal/httpserver"
)

func main() {
	cfg := config.Load()
	server := &http.Server{
		Addr:              cfg.Addr,
		Handler:           httpserver.New(cfg),
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("starting ogame go server on %s", cfg.Addr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
