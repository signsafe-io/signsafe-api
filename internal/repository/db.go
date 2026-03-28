package repository

import (
	"fmt"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

// NewDB opens a sqlx connection to PostgreSQL.
func NewDB(databaseURL string) (*sqlx.DB, error) {
	db, err := sqlx.Open("postgres", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("repository.NewDB: open: %w", err)
	}
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("repository.NewDB: ping: %w", err)
	}
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	// Recycle connections after 30 minutes to prevent stale connections
	// that have exceeded the PostgreSQL server's idle timeout (default: 600s).
	db.SetConnMaxLifetime(30 * time.Minute)
	// Close idle connections that have been idle for more than 5 minutes.
	db.SetConnMaxIdleTime(5 * time.Minute)
	return db, nil
}

// RunMigrations executes pending migrations from the given directory.
func RunMigrations(databaseURL, migrationsPath string) error {
	m, err := migrate.New("file://"+migrationsPath, databaseURL)
	if err != nil {
		return fmt.Errorf("repository.RunMigrations: new: %w", err)
	}
	defer m.Close()

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("repository.RunMigrations: up: %w", err)
	}
	return nil
}
