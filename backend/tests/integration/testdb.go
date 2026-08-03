package integration

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"clinicapp/backend/internal/store"
)

// migrationsDir is relative to this package's directory
// (backend/tests/integration/), which is what `go test` sets as the working
// directory for this package's test binary.
const migrationsDir = "../../../migrations"

const defaultTestDBDSN = "postgres://localhost:5432/clinicapp_test?sslmode=disable"

var appTables = []string{
	"user_providers",
	"password_reset_tokens",
	"refresh_tokens",
	"email_verification_tokens",
	"patients",
	"consultants",
	"attendants",
	"users",
}

// SetupTestDB connects to the test database, applies migrations, wipes any
// data left over from a prior run, and registers cleanup to close the pool.
func SetupTestDB(t *testing.T) *pgxpool.Pool {
	t.Helper()

	dsn := os.Getenv("TEST_DB_DSN")
	if dsn == "" {
		dsn = defaultTestDBDSN
	}

	ctx := context.Background()

	pool, err := store.NewPool(ctx, dsn)
	if err != nil {
		t.Fatalf("connect test db (%s): %v", dsn, err)
	}
	t.Cleanup(pool.Close)

	if err := store.RunMigrations(ctx, pool, migrationsDir); err != nil {
		t.Fatalf("run migrations: %v", err)
	}

	if err := truncateAll(ctx, pool); err != nil {
		t.Fatalf("truncate test db: %v", err)
	}

	return pool
}

func truncateAll(ctx context.Context, pool *pgxpool.Pool) error {
	for _, table := range appTables {
		if _, err := pool.Exec(ctx, "TRUNCATE TABLE "+table+" CASCADE"); err != nil {
			return err
		}
	}
	return nil
}
