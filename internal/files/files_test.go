package files_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/flashyspeed/flashyspeed/internal/db"
	"github.com/flashyspeed/flashyspeed/internal/files"
)

func setup(t *testing.T) (*db.DB, int64, int64) {
	t.Helper()
	database, _ := db.Open(filepath.Join(t.TempDir(), "test.db"))
	t.Cleanup(func() { database.Close() })

	// seed a drive pointing at a temp dir
	driveRoot := t.TempDir()
	res, _ := database.Exec(
		`INSERT INTO drives(name, mount_path) VALUES('test', ?)`, driveRoot,
	)
	driveID, _ := res.LastInsertId()

	// seed a user
	res, _ = database.Exec(
		`INSERT INTO users(username, email, password_hash, role) VALUES('alice','a@b.com','x','user')`,
	)
	userID, _ := res.LastInsertId()

	return database, driveID, userID
}

func TestMkdir(t *testing.T) {
	database, driveID, userID := setup(t)
	svc := files.NewService(database)

	id, err := svc.Mkdir(userID, driveID, 0, "documents")
	if err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if id == 0 {
		t.Error("expected non-zero file ID")
	}

	// dir should exist on disk
	var mountPath string
	database.QueryRow(`SELECT mount_path FROM drives WHERE id=?`, driveID).Scan(&mountPath)
	stat, err := os.Stat(filepath.Join(mountPath, "documents"))
	if err != nil || !stat.IsDir() {
		t.Error("directory should exist on disk")
	}
}

func TestList(t *testing.T) {
	database, driveID, userID := setup(t)
	svc := files.NewService(database)

	svc.Mkdir(userID, driveID, 0, "docs")
	svc.Mkdir(userID, driveID, 0, "photos")

	entries, err := svc.List(userID, driveID, 0)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(entries) != 2 {
		t.Errorf("expected 2 entries, got %d", len(entries))
	}
}

func TestSoftDelete(t *testing.T) {
	database, driveID, userID := setup(t)
	svc := files.NewService(database)

	id, _ := svc.Mkdir(userID, driveID, 0, "todelete")
	if err := svc.Delete(userID, id); err != nil {
		t.Fatalf("delete: %v", err)
	}

	// should not appear in normal listing
	entries, _ := svc.List(userID, driveID, 0)
	for _, e := range entries {
		if e.ID == id {
			t.Error("deleted item should not appear in listing")
		}
	}

	// but should appear in trash
	trash, _ := svc.Trash(userID)
	found := false
	for _, e := range trash {
		if e.ID == id {
			found = true
		}
	}
	if !found {
		t.Error("deleted item should appear in trash")
	}
}
