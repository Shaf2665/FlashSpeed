package shares_test

import (
	"testing"
	"time"

	"github.com/flashyspeed/flashyspeed/internal/db"
	"github.com/flashyspeed/flashyspeed/internal/shares"
)

// setup opens an in-memory SQLite database, seeds a drive + user + file,
// and returns the DB, userID and fileID.
func setup(t *testing.T) (*db.DB, int64, int64) {
	t.Helper()
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { database.Close() })

	// seed a drive
	res, err := database.Exec(`INSERT INTO drives(name, mount_path) VALUES('test', '/tmp/shares_test_drive')`)
	if err != nil {
		t.Fatalf("seed drive: %v", err)
	}
	driveID, _ := res.LastInsertId()

	// seed a user
	res, err = database.Exec(`INSERT INTO users(username, email, password_hash, role) VALUES('alice','alice@example.com','x','user')`)
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}
	userID, _ := res.LastInsertId()

	// seed a file
	res, err = database.Exec(`INSERT INTO files(user_id, drive_id, name, rel_path, mime_type) VALUES(?,?,'test.mp4','test.mp4','video/mp4')`, userID, driveID)
	if err != nil {
		t.Fatalf("seed file: %v", err)
	}
	fileID, _ := res.LastInsertId()

	return database, userID, fileID
}

func TestCreateAndResolveShare(t *testing.T) {
	database, userID, fileID := setup(t)
	svc := shares.NewService(database)

	sh, err := svc.Create(userID, shares.CreateShareRequest{FileID: fileID})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if sh.ID == "" {
		t.Fatal("expected non-empty share token")
	}

	resolved, file, err := svc.Resolve(sh.ID, "")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if resolved.DownloadCount != 1 {
		t.Errorf("expected download_count=1, got %d", resolved.DownloadCount)
	}
	if file.ID != fileID {
		t.Errorf("expected file ID %d, got %d", fileID, file.ID)
	}
	if file.MimeType != "video/mp4" {
		t.Errorf("expected mime_type video/mp4, got %q", file.MimeType)
	}
}

func TestPasswordProtectedShare(t *testing.T) {
	database, userID, fileID := setup(t)
	svc := shares.NewService(database)

	sh, err := svc.Create(userID, shares.CreateShareRequest{
		FileID:   fileID,
		Password: "secret123",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Wrong password must return ErrWrongPassword.
	_, _, err = svc.Resolve(sh.ID, "wrongpass")
	if err == nil {
		t.Fatal("expected ErrWrongPassword, got nil")
	}
	if err != shares.ErrWrongPassword {
		t.Fatalf("expected ErrWrongPassword, got %v", err)
	}

	// Correct password must succeed.
	resolved, _, err := svc.Resolve(sh.ID, "secret123")
	if err != nil {
		t.Fatalf("Resolve with correct password: %v", err)
	}
	if resolved.DownloadCount != 1 {
		t.Errorf("expected download_count=1, got %d", resolved.DownloadCount)
	}
}

func TestExpiredShare(t *testing.T) {
	database, userID, fileID := setup(t)
	svc := shares.NewService(database)

	past := time.Now().Add(-time.Hour)
	sh, err := svc.Create(userID, shares.CreateShareRequest{
		FileID:    fileID,
		ExpiresAt: &past,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	_, _, err = svc.Resolve(sh.ID, "")
	if err != shares.ErrShareExpired {
		t.Fatalf("expected ErrShareExpired, got %v", err)
	}
}

func TestMaxDownloadsExhausted(t *testing.T) {
	database, userID, fileID := setup(t)
	svc := shares.NewService(database)

	maxDL := 1
	sh, err := svc.Create(userID, shares.CreateShareRequest{
		FileID:       fileID,
		MaxDownloads: &maxDL,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// First resolve: should succeed.
	_, _, err = svc.Resolve(sh.ID, "")
	if err != nil {
		t.Fatalf("first Resolve: %v", err)
	}

	// Second resolve: should be exhausted.
	_, _, err = svc.Resolve(sh.ID, "")
	if err != shares.ErrShareExpired {
		t.Fatalf("expected ErrShareExpired on second resolve, got %v", err)
	}
}

func TestDeleteShare(t *testing.T) {
	database, userID, fileID := setup(t)
	svc := shares.NewService(database)

	sh, err := svc.Create(userID, shares.CreateShareRequest{FileID: fileID})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := svc.Delete(userID, sh.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	_, _, err = svc.Resolve(sh.ID, "")
	if err == nil {
		t.Fatal("expected error resolving deleted share, got nil")
	}
}
