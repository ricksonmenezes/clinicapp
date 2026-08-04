package main

import (
	"context"
	"log"
	"net/http"

	"clinicapp/backend/internal/config"
	"clinicapp/backend/internal/mailer"
	"clinicapp/backend/internal/server"
	"clinicapp/backend/internal/sms"
	"clinicapp/backend/internal/store"
)

// migrationsDir, webTemplatesDir, and webStaticDir are relative to the
// process working directory, which is always backend/ per CLAUDE.md's
// documented run commands (local `cd backend && go run ./cmd/server`, and
// deploy/clinicapp.service's WorkingDirectory).
const migrationsDir = "../migrations"
const webTemplatesDir = "../web/templates"
const webStaticDir = "../web/static"

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

	if err := server.Bootstrap(ctx, pool, cfg); err != nil {
		log.Fatalf("bootstrap admin: %v", err)
	}

	m := mailer.NewResendMailer(cfg.ResendAPIKey, cfg.MailFrom)
	s := sms.NewPhilSMSSender(cfg.SMSAPIKey, cfg.SMSSenderID)
	router, err := server.NewRouter(pool, cfg, m, s, webTemplatesDir, webStaticDir)
	if err != nil {
		log.Fatalf("build router: %v", err)
	}

	log.Printf("clinicapp server listening on :%s", cfg.Port)
	if err := http.ListenAndServe(":"+cfg.Port, router); err != nil {
		log.Fatal(err)
	}
}
