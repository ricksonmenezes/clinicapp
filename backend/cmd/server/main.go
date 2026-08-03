package main

import (
	"context"
	"log"
	"net/http"

	"clinicapp/backend/internal/config"
	"clinicapp/backend/internal/mailer"
	"clinicapp/backend/internal/server"
	"clinicapp/backend/internal/store"
)

// migrationsDir is relative to the process working directory, which is
// always backend/ per CLAUDE.md's documented run commands (local `cd backend
// && go run ./cmd/server`, and deploy/clinicapp.service's WorkingDirectory).
const migrationsDir = "../migrations"

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	ctx := context.Background()

	pool, err := store.NewPool(ctx, cfg.DBDSN)
	if err != nil {
		log.Fatalf("connect db: %v", err)
	}
	defer pool.Close()

	if err := store.RunMigrations(ctx, pool, migrationsDir); err != nil {
		log.Fatalf("run migrations: %v", err)
	}

	m := mailer.NewSMTPMailer(cfg.SMTPHost, cfg.SMTPPort, cfg.SMTPUser, cfg.SMTPPass, cfg.SMTPFrom)
	router := server.NewRouter(pool, cfg, m)

	log.Printf("clinicapp server listening on :%s", cfg.Port)
	if err := http.ListenAndServe(":"+cfg.Port, router); err != nil {
		log.Fatal(err)
	}
}
