package database

import (
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/jackc/pgx/v5/stdlib"
)

// Open establishes a pooled connection to PostgreSQL and verifies connectivity.
func Open(databaseURL string, logger *slog.Logger) (*sql.DB, error) {
	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to open database handle: %w", err)
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(15 * time.Minute)
	db.SetConnMaxIdleTime(5 * time.Minute)

	// Verify connection with timeout
	for attempts := 1; attempts <= 10; attempts++ {
		err = db.Ping()
		if err == nil {
			logger.Info("Successfully connected to PostgreSQL")
			break
		}
		logger.Warn("Waiting for PostgreSQL connection...", "attempt", attempts, "error", err)
		time.Sleep(1 * time.Second)
	}
	if err != nil {
		return nil, fmt.Errorf("could not connect to PostgreSQL after retries: %w", err)
	}

	// Apply pending schema migrations
	if err := runMigrations(databaseURL, logger); err != nil {
		return nil, fmt.Errorf("failed to apply database migrations: %w", err)
	}

	return db, nil
}

func runMigrations(databaseURL string, logger *slog.Logger) error {
	dir := findMigrationsDir()
	absPath, err := filepath.Abs(dir)
	if err != nil {
		absPath = dir
	}

	migrationSource := fmt.Sprintf("file://%s", filepath.ToSlash(absPath))
	logger.Info("Applying PostgreSQL migrations", "source", migrationSource)

	m, err := migrate.New(migrationSource, databaseURL)
	if err != nil {
		return fmt.Errorf("failed to initialize migrator with source %s: %w", migrationSource, err)
	}
	defer m.Close()

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("migration run failure: %w", err)
	}

	logger.Info("Database migrations up to date")
	return nil
}

func findMigrationsDir() string {
	candidates := []string{
		os.Getenv("MIGRATIONS_PATH"),
		"migrations",
		"./migrations",
		"../migrations",
		"../../migrations",
		"/migrations",
	}
	for _, c := range candidates {
		if c != "" {
			if stat, err := os.Stat(c); err == nil && stat.IsDir() {
				return c
			}
		}
	}
	return "migrations"
}
