package storage

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

type Store struct {
	db   *sql.DB
	path string
}

func Open(ctx context.Context, dataDir string) (*Store, error) {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, err
	}

	dbPath := filepath.Join(dataDir, "farm_go.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	if _, err := db.ExecContext(ctx, `PRAGMA busy_timeout=5000`); err != nil {
		_ = db.Close()
		return nil, err
	}

	store := &Store{db: db, path: dbPath}
	if err := store.Migrate(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}

	return store, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) Path() string {
	return s.path
}
