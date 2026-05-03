package media_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/flashyspeed/flashyspeed/internal/auth"
	"github.com/flashyspeed/flashyspeed/internal/db"
	"github.com/flashyspeed/flashyspeed/internal/files"
	"github.com/flashyspeed/flashyspeed/internal/media"
)

// setup creates a test DB, a drive, and a user. Returns database, driveID, userID, driveRoot.
func setup(t *testing.T) (*db.DB, int64, int64, string) {
	t.Helper()
	database, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })

	driveRoot := t.TempDir()
	res, err := database.Exec(`INSERT INTO drives(name, mount_path) VALUES('test', ?)`, driveRoot)
	if err != nil {
		t.Fatal(err)
	}
	driveID, _ := res.LastInsertId()

	res, err = database.Exec(`INSERT INTO users(username, email, password_hash, role) VALUES('alice','a@b.com','x','user')`)
	if err != nil {
		t.Fatal(err)
	}
	userID, _ := res.LastInsertId()

	return database, driveID, userID, driveRoot
}

// insertFile inserts a file record into the DB and writes fileBytes to disk.
// Returns the file ID.
func insertFile(t *testing.T, database *db.DB, driveID, userID int64, driveRoot, name, mimeType string, isDir bool, fileBytes []byte) int64 {
	t.Helper()
	relPath := name
	isDirInt := 0
	if isDir {
		isDirInt = 1
		// Create directory on disk
		if err := os.MkdirAll(filepath.Join(driveRoot, relPath), 0755); err != nil {
			t.Fatal(err)
		}
	} else {
		// Write file on disk
		if err := os.WriteFile(filepath.Join(driveRoot, relPath), fileBytes, 0644); err != nil {
			t.Fatal(err)
		}
	}

	var mimeArg interface{}
	if mimeType != "" {
		mimeArg = mimeType
	} else {
		mimeArg = nil
	}

	res, err := database.Exec(`
		INSERT INTO files(user_id, drive_id, name, rel_path, size_bytes, mime_type, is_dir)
		VALUES(?,?,?,?,?,?,?)
	`, userID, driveID, name, relPath, len(fileBytes), mimeArg, isDirInt)
	if err != nil {
		t.Fatal(err)
	}
	id, _ := res.LastInsertId()
	return id
}

func ctxWithClaims(ctx context.Context, c *auth.Claims) context.Context {
	return context.WithValue(ctx, auth.ClaimsContextKey, c)
}

func newRouter(h *media.Handler) *chi.Mux {
	r := chi.NewRouter()
	r.Get("/api/files/{id}/stream", h.Stream)
	return r
}

// TestStreamOwnershipEnforced verifies that user 2 cannot stream user 1's file.
func TestStreamOwnershipEnforced(t *testing.T) {
	database, driveID, userID, driveRoot := setup(t)
	fileSvc := files.NewService(database)
	h := media.NewHandler(database, fileSvc)

	// Insert a file for user 1
	content := bytes.Repeat([]byte("a"), 64)
	fileID := insertFile(t, database, driveID, userID, driveRoot, "secret.txt", "text/plain", false, content)

	// Insert user 2
	res, _ := database.Exec(`INSERT INTO users(username,email,password_hash,role) VALUES('bob','b@b.com','x','user')`)
	user2ID, _ := res.LastInsertId()

	r := newRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/files/"+itoa(fileID)+"/stream", nil)
	req = req.WithContext(ctxWithClaims(req.Context(), &auth.Claims{UserID: user2ID}))

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

// TestStreamRange uploads a 1024-byte file and requests bytes 0-511 via Range header.
func TestStreamRange(t *testing.T) {
	database, driveID, userID, driveRoot := setup(t)
	fileSvc := files.NewService(database)
	h := media.NewHandler(database, fileSvc)

	content := bytes.Repeat([]byte{0x00}, 1024)
	fileID := insertFile(t, database, driveID, userID, driveRoot, "zeros.bin", "application/octet-stream", false, content)

	r := newRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/files/"+itoa(fileID)+"/stream", nil)
	req.Header.Set("Range", "bytes=0-511")
	req = req.WithContext(ctxWithClaims(req.Context(), &auth.Claims{UserID: userID}))

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusPartialContent {
		t.Errorf("expected 206 Partial Content, got %d: %s", w.Code, w.Body.String())
	}
	if w.Body.Len() != 512 {
		t.Errorf("expected 512 bytes in body, got %d", w.Body.Len())
	}
	contentRange := w.Header().Get("Content-Range")
	if contentRange != "bytes 0-511/1024" {
		t.Errorf("expected Content-Range bytes 0-511/1024, got %q", contentRange)
	}
}

// TestStreamMIMEDetection verifies that MIME type is sniffed when DB record has none.
func TestStreamMIMEDetection(t *testing.T) {
	database, driveID, userID, driveRoot := setup(t)
	fileSvc := files.NewService(database)
	h := media.NewHandler(database, fileSvc)

	// PNG magic header + padding
	pngMagic := []byte("\x89PNG\r\n\x1a\n")
	content := make([]byte, 512)
	copy(content, pngMagic)

	// Insert with empty mime_type (pass "" so we insert NULL)
	fileID := insertFile(t, database, driveID, userID, driveRoot, "image.png", "", false, content)

	r := newRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/files/"+itoa(fileID)+"/stream", nil)
	req = req.WithContext(ctxWithClaims(req.Context(), &auth.Claims{UserID: userID}))

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	ct := w.Header().Get("Content-Type")
	if ct != "image/png" {
		t.Errorf("expected Content-Type image/png, got %q", ct)
	}
	if w.Body.Len() != 512 {
		t.Errorf("expected 512-byte body after MIME sniff seek-back, got %d", w.Body.Len())
	}
}

// TestStreamDirectory verifies that streaming a directory returns 400.
func TestStreamDirectory(t *testing.T) {
	database, driveID, userID, driveRoot := setup(t)
	fileSvc := files.NewService(database)
	h := media.NewHandler(database, fileSvc)

	dirID := insertFile(t, database, driveID, userID, driveRoot, "mydir", "", true, nil)

	r := newRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/files/"+itoa(dirID)+"/stream", nil)
	req = req.WithContext(ctxWithClaims(req.Context(), &auth.Claims{UserID: userID}))

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

// TestStreamRangeOpenEnded requests bytes=512- on a 1024-byte file and expects 206 with 512 bytes.
func TestStreamRangeOpenEnded(t *testing.T) {
	database, driveID, userID, driveRoot := setup(t)
	fileSvc := files.NewService(database)
	h := media.NewHandler(database, fileSvc)

	content := bytes.Repeat([]byte{0x01}, 1024)
	fileID := insertFile(t, database, driveID, userID, driveRoot, "ones.bin", "application/octet-stream", false, content)

	r := newRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/files/"+itoa(fileID)+"/stream", nil)
	req.Header.Set("Range", "bytes=512-")
	req = req.WithContext(ctxWithClaims(req.Context(), &auth.Claims{UserID: userID}))

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusPartialContent {
		t.Errorf("expected 206 Partial Content, got %d: %s", w.Code, w.Body.String())
	}
	if w.Body.Len() != 512 {
		t.Errorf("expected 512 bytes in body, got %d", w.Body.Len())
	}
	contentRange := w.Header().Get("Content-Range")
	if contentRange != "bytes 512-1023/1024" {
		t.Errorf("expected Content-Range bytes 512-1023/1024, got %q", contentRange)
	}
}

// TestStreamRangeUnsatisfiable requests an out-of-range interval on a 1024-byte file and expects 416.
func TestStreamRangeUnsatisfiable(t *testing.T) {
	database, driveID, userID, driveRoot := setup(t)
	fileSvc := files.NewService(database)
	h := media.NewHandler(database, fileSvc)

	content := bytes.Repeat([]byte{0x02}, 1024)
	fileID := insertFile(t, database, driveID, userID, driveRoot, "twos.bin", "application/octet-stream", false, content)

	r := newRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/files/"+itoa(fileID)+"/stream", nil)
	req.Header.Set("Range", "bytes=2000-3000")
	req = req.WithContext(ctxWithClaims(req.Context(), &auth.Claims{UserID: userID}))

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusRequestedRangeNotSatisfiable {
		t.Errorf("expected 416 Range Not Satisfiable, got %d: %s", w.Code, w.Body.String())
	}
}

// TestStreamBadID verifies that a non-numeric file ID returns 400.
func TestStreamBadID(t *testing.T) {
	database, driveID, userID, driveRoot := setup(t)
	fileSvc := files.NewService(database)
	h := media.NewHandler(database, fileSvc)

	// Insert a file just to ensure the handler is wired correctly.
	insertFile(t, database, driveID, userID, driveRoot, "file.txt", "text/plain", false, []byte("hello"))

	r := newRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/api/files/abc/stream", nil)
	req = req.WithContext(ctxWithClaims(req.Context(), &auth.Claims{UserID: userID}))

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

// itoa converts an int64 to a string (avoids importing strconv in test helpers).
func itoa(n int64) string {
	return strconv.FormatInt(n, 10)
}
