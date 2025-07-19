package database

import (
	"context"
	"embed"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations
var migrationsFS embed.FS

// Migrator handles database migrations
type Migrator struct {
	pool *pgxpool.Pool
}

// NewMigrator creates a new migrator
func NewMigrator(pool *pgxpool.Pool) *Migrator {
	return &Migrator{pool: pool}
}

// Up runs all pending migrations
func (m *Migrator) Up(ctx context.Context) error {
	migrator, err := m.createMigrator()
	if err != nil {
		return fmt.Errorf("failed to create migrator: %w", err)
	}
	defer func() { _, _ = migrator.Close() }()

	if err := migrator.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	return nil
}

// Down rolls back all migrations
func (m *Migrator) Down(ctx context.Context) error {
	migrator, err := m.createMigrator()
	if err != nil {
		return fmt.Errorf("failed to create migrator: %w", err)
	}
	defer func() { _, _ = migrator.Close() }()

	if err := migrator.Down(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("failed to rollback migrations: %w", err)
	}

	return nil
}

// Version returns the current migration version
func (m *Migrator) Version(ctx context.Context) (uint, bool, error) {
	migrator, err := m.createMigrator()
	if err != nil {
		return 0, false, fmt.Errorf("failed to create migrator: %w", err)
	}
	defer func() { _, _ = migrator.Close() }()

	version, dirty, err := migrator.Version()
	if err != nil {
		return 0, false, fmt.Errorf("failed to get migration version: %w", err)
	}

	return version, dirty, nil
}

// createMigrator creates a new migrate instance
func (m *Migrator) createMigrator() (*migrate.Migrate, error) {
	// Create source driver from embedded filesystem
	source, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		return nil, fmt.Errorf("failed to create source driver: %w", err)
	}

	// Get database config to construct DSN
	config := DefaultConfig()
	dsn := fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=%s",
		config.Username,
		config.Password,
		config.Host,
		config.Port,
		config.Database,
		config.SSLMode,
	)

	// Create migrator with DSN
	migrator, err := migrate.NewWithSourceInstance("iofs", source, dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to create migrator: %w", err)
	}

	return migrator, nil
}