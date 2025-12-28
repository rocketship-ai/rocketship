package persistence

import (
	"context"
	"embed"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

type Store struct {
	db       *sqlx.DB
	tokenKey []byte
}

func NewStore(ctx context.Context, dsn string, tokenKey []byte) (*Store, error) {
	db, err := sqlx.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	store := &Store{
		db:       db,
		tokenKey: append([]byte(nil), tokenKey...),
	}

	if err := store.migrate(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}

	return store, nil
}

func (s *Store) Close() error {
	if s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *Store) migrate(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
        version TEXT PRIMARY KEY,
        applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
    )`); err != nil {
		return fmt.Errorf("failed to create schema_migrations table: %w", err)
	}

	entries, err := migrationFiles.ReadDir("migrations")
	if err != nil {
		return fmt.Errorf("failed to read embedded migrations: %w", err)
	}

	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		names = append(names, entry.Name())
	}
	sort.Strings(names)

	for _, name := range names {
		applied, err := s.migrationApplied(ctx, name)
		if err != nil {
			return err
		}
		if applied {
			continue
		}

		contents, err := migrationFiles.ReadFile("migrations/" + name)
		if err != nil {
			return fmt.Errorf("failed to read migration %s: %w", name, err)
		}

		stmt := strings.TrimSpace(string(contents))
		if stmt == "" {
			if err := s.markMigration(ctx, name); err != nil {
				return err
			}
			continue
		}

		tx, err := s.db.BeginTxx(ctx, nil)
		if err != nil {
			return fmt.Errorf("failed to begin migration transaction: %w", err)
		}

		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("migration %s failed: %w", name, err)
		}

		if _, err := tx.ExecContext(ctx, `INSERT INTO schema_migrations (version, applied_at) VALUES ($1, NOW())`, name); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("failed to record migration %s: %w", name, err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("failed to commit migration %s: %w", name, err)
		}
	}

	return nil
}

func (s *Store) migrationApplied(ctx context.Context, version string) (bool, error) {
	var count int
	if err := s.db.GetContext(ctx, &count, `SELECT COUNT(1) FROM schema_migrations WHERE version = $1`, version); err != nil {
		return false, fmt.Errorf("failed to check migration %s: %w", version, err)
	}
	return count > 0, nil
}

func (s *Store) markMigration(ctx context.Context, version string) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO schema_migrations (version, applied_at) VALUES ($1, NOW())`, version)
	if err != nil {
		return fmt.Errorf("failed to mark migration %s: %w", version, err)
	}
	return nil
}
