package store

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

const schemaMigrationsDDL = `
CREATE TABLE IF NOT EXISTS schema_migrations (
	version     VARCHAR(255) PRIMARY KEY,
	applied_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
`

// RunMigrations applies every *.sql file in dir, in filename order, that isn't
// already recorded in schema_migrations. Each file runs inside its own transaction.
func RunMigrations(ctx context.Context, pool *pgxpool.Pool, dir string) error {
	if _, err := pool.Exec(ctx, schemaMigrationsDDL); err != nil {
		return fmt.Errorf("create schema_migrations table: %w", err)
	}

	files, err := migrationFiles(dir)
	if err != nil {
		return fmt.Errorf("list migrations: %w", err)
	}

	applied, err := appliedVersions(ctx, pool)
	if err != nil {
		return fmt.Errorf("load applied migrations: %w", err)
	}

	for _, file := range files {
		version := filepath.Base(file)
		if applied[version] {
			continue
		}

		content, err := os.ReadFile(file)
		if err != nil {
			return fmt.Errorf("read %s: %w", version, err)
		}

		if err := applyMigration(ctx, pool, version, string(content)); err != nil {
			return fmt.Errorf("apply %s: %w", version, err)
		}
	}

	return nil
}

func migrationFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var files []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		files = append(files, filepath.Join(dir, e.Name()))
	}
	sort.Strings(files)
	return files, nil
}

func appliedVersions(ctx context.Context, pool *pgxpool.Pool) (map[string]bool, error) {
	rows, err := pool.Query(ctx, "SELECT version FROM schema_migrations")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	applied := make(map[string]bool)
	for rows.Next() {
		var version string
		if err := rows.Scan(&version); err != nil {
			return nil, err
		}
		applied[version] = true
	}
	return applied, rows.Err()
}

func applyMigration(ctx context.Context, pool *pgxpool.Pool, version, content string) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	for _, stmt := range splitStatements(content) {
		if _, err := tx.Exec(ctx, stmt); err != nil {
			return fmt.Errorf("exec statement: %w", err)
		}
	}

	if _, err := tx.Exec(ctx, "INSERT INTO schema_migrations (version) VALUES ($1)", version); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// splitStatements naively strips "--" line comments and splits a migration
// file on ";" statement terminators. Migration SQL in this repo is plain DDL
// with no dollar-quoted bodies or semicolons inside string literals, so this
// is sufficient.
func splitStatements(content string) []string {
	var withoutComments strings.Builder
	for _, line := range strings.Split(content, "\n") {
		if idx := strings.Index(line, "--"); idx >= 0 {
			line = line[:idx]
		}
		withoutComments.WriteString(line)
		withoutComments.WriteByte('\n')
	}

	var stmts []string
	for _, raw := range strings.Split(withoutComments.String(), ";") {
		stmt := strings.TrimSpace(raw)
		if stmt == "" {
			continue
		}
		stmts = append(stmts, stmt)
	}
	return stmts
}
