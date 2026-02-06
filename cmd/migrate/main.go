package main

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"DORAPollCredit/internal/config"
	"DORAPollCredit/internal/db"
)

func main() {
	cfg, err := config.Load("")
	if err != nil {
		log.Fatalf("config load failed: %v", err)
	}

	ctx := context.Background()
	pool, err := db.Connect(ctx, cfg.DB.DSN)
	if err != nil {
		log.Fatalf("db connect failed: %v", err)
	}
	defer pool.Close()

	if err := ensureSchemaTable(ctx, pool); err != nil {
		log.Fatalf("ensure schema table failed: %v", err)
	}

	files, err := listSQLFiles("migrations")
	if err != nil {
		log.Fatalf("list migrations failed: %v", err)
	}

	for _, file := range files {
		applied, err := isApplied(ctx, pool, file)
		if err != nil {
			log.Fatalf("check migration failed (%s): %v", file, err)
		}
		if applied {
			continue
		}

		if err := applyMigration(ctx, pool, file); err != nil {
			log.Fatalf("apply migration failed (%s): %v", file, err)
		}
		if err := markApplied(ctx, pool, file); err != nil {
			log.Fatalf("mark migration failed (%s): %v", file, err)
		}
		log.Printf("applied %s", file)
	}
}

func ensureSchemaTable(ctx context.Context, pool *db.Pool) error {
	_, err := pool.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (filename TEXT PRIMARY KEY, applied_at TIMESTAMPTZ NOT NULL DEFAULT now())`)
	return err
}

func listSQLFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var files []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasSuffix(name, ".sql") {
			files = append(files, filepath.Join(dir, name))
		}
	}
	sort.Strings(files)
	return files, nil
}

func isApplied(ctx context.Context, pool *db.Pool, file string) (bool, error) {
	var exists bool
	row := pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE filename=$1)`, file)
	if err := row.Scan(&exists); err != nil {
		return false, err
	}
	return exists, nil
}

func applyMigration(ctx context.Context, pool *db.Pool, file string) error {
	data, err := os.ReadFile(file)
	if err != nil {
		return err
	}
	if strings.TrimSpace(string(data)) == "" {
		return nil
	}
	_, err = pool.Exec(ctx, string(data))
	return err
}

func markApplied(ctx context.Context, pool *db.Pool, file string) error {
	_, err := pool.Exec(ctx, `INSERT INTO schema_migrations (filename) VALUES ($1)`, file)
	return err
}
