package files_test

import (
	"database/sql"
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

func TestRestore(t *testing.T) {
	database, driveID, userID := setup(t)
	svc := files.NewService(database)

	id, _ := svc.Mkdir(userID, driveID, 0, "restore-me")
	if err := svc.Delete(userID, id); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if err := svc.Restore(userID, id); err != nil {
		t.Fatalf("restore: %v", err)
	}

	var deletedAt sql.NullString
	if err := database.QueryRow(`SELECT deleted_at FROM files WHERE id=?`, id).Scan(&deletedAt); err != nil {
		t.Fatalf("scan deleted_at: %v", err)
	}
	if deletedAt.Valid {
		t.Error("deleted_at should be NULL after restore")
	}

	entries, _ := svc.List(userID, driveID, 0)
	found := false
	for _, e := range entries {
		if e.ID == id {
			found = true
			break
		}
	}
	if !found {
		t.Error("restored folder should appear in listing")
	}
}

func TestPermanentDelete(t *testing.T) {
	database, driveID, userID := setup(t)
	svc := files.NewService(database)

	var mountPath string
	database.QueryRow(`SELECT mount_path FROM drives WHERE id=?`, driveID).Scan(&mountPath)

	id, _ := svc.Mkdir(userID, driveID, 0, "gone-forever")
	if err := svc.Delete(userID, id); err != nil {
		t.Fatalf("delete: %v", err)
	}
	dirPath := filepath.Join(mountPath, "gone-forever")
	if _, err := os.Stat(dirPath); err != nil {
		t.Fatalf("trashed folder should remain on disk: %v", err)
	}

	if err := svc.PermanentDelete(userID, id); err != nil {
		t.Fatalf("permanentDelete: %v", err)
	}

	if _, err := os.Stat(dirPath); !os.IsNotExist(err) {
		t.Fatalf("directory should be removed from disk, stat err=%v", err)
	}

	var cnt int
	database.QueryRow(`SELECT COUNT(*) FROM files WHERE id=?`, id).Scan(&cnt)
	if cnt != 0 {
		t.Error("database row should be removed")
	}
}

func TestEmptyTrash(t *testing.T) {
	database, driveID, userID := setup(t)
	svc := files.NewService(database)

	var mountPath string
	database.QueryRow(`SELECT mount_path FROM drives WHERE id=?`, driveID).Scan(&mountPath)

	// Create two directories and soft-delete both.
	id1, _ := svc.Mkdir(userID, driveID, 0, "trash-a")
	id2, _ := svc.Mkdir(userID, driveID, 0, "trash-b")
	svc.Delete(userID, id1)
	svc.Delete(userID, id2)

	trash, _ := svc.Trash(userID)
	if len(trash) != 2 {
		t.Fatalf("expected 2 items in trash before empty, got %d", len(trash))
	}

	if err := svc.EmptyTrash(userID); err != nil {
		t.Fatalf("emptyTrash: %v", err)
	}

	trash, _ = svc.Trash(userID)
	if len(trash) != 0 {
		t.Errorf("expected empty trash, got %d items", len(trash))
	}

	// Verify DB rows are gone.
	var cnt int
	database.QueryRow(`SELECT COUNT(*) FROM files WHERE user_id=?`, userID).Scan(&cnt)
	if cnt != 0 {
		t.Errorf("expected 0 DB rows after empty trash, got %d", cnt)
	}

	// Verify directories are removed from disk.
	for _, name := range []string{"trash-a", "trash-b"} {
		p := filepath.Join(mountPath, name)
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Errorf("expected %s to be removed from disk", name)
		}
	}
}
