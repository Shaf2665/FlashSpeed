package db_test

import (
	"path/filepath"
	"testing"

	"github.com/flashyspeed/flashyspeed/internal/db"
)

func TestOpenAndMigrate(t *testing.T) {
	tmp := t.TempDir()
	database, err := db.Open(filepath.Join(tmp, "test.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer database.Close()

	// verify users table exists
	var count int
	err = database.QueryRow("SELECT COUNT(*) FROM users").Scan(&count)
	if err != nil {
		t.Fatalf("users table missing: %v", err)
	}
}

func TestWALMode(t *testing.T) {
	tmp := t.TempDir()
	database, err := db.Open(filepath.Join(tmp, "wal.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer database.Close()

	var mode string
	database.QueryRow("PRAGMA journal_mode").Scan(&mode)
	if mode != "wal" {
		t.Errorf("expected WAL mode, got %s", mode)
	}
}
