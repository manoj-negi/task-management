package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func Connect(databaseURL string) (*sql.DB, error) {
	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("open PostgreSQL connection: %w", err)
	}

	db.SetMaxOpenConns(20)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)
	db.SetConnMaxIdleTime(5 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("connect to PostgreSQL: %w", err)
	}

	return db, nil
}

// EnsureTaskSoftDeleteSchema keeps the application compatible with databases
// that have not yet run migration 000004. The change is additive and the
// checked-in migration remains the source of truth for migration tracking.
func EnsureTaskSoftDeleteSchema(db *sql.DB) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin task schema compatibility update: %w", err)
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`ALTER TABLE tasks ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ`); err != nil {
		return fmt.Errorf("ensure task deleted_at column: %w", err)
	}
	if _, err := tx.Exec(`CREATE INDEX IF NOT EXISTS idx_tasks_active ON tasks(id) WHERE deleted_at IS NULL`); err != nil {
		return fmt.Errorf("ensure active task index: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit task schema compatibility update: %w", err)
	}
	return nil
}
