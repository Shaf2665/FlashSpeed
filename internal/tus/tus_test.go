package tus_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/flashyspeed/flashyspeed/internal/auth"
	"github.com/flashyspeed/flashyspeed/internal/db"
	"github.com/flashyspeed/flashyspeed/internal/tus"
)

func setup(t *testing.T) (*db.DB, int64, int64, *tus.Handler) {
	t.Helper()
	tmp := t.TempDir()
	database, _ := db.Open(filepath.Join(tmp, "test.db"))
	t.Cleanup(func() { database.Close() })

	driveRoot := t.TempDir()
	res, _ := database.Exec(`INSERT INTO drives(name, mount_path) VALUES('t', ?)`, driveRoot)
	driveID, _ := res.LastInsertId()

	res, _ = database.Exec(`INSERT INTO users(username,email,password_hash,role) VALUES('u','u@b.com','x','user')`)
	userID, _ := res.LastInsertId()

	h := tus.NewHandler(database, tmp)
	return database, driveID, userID, h
}

func ctxWithClaims(ctx context.Context, c *auth.Claims) context.Context {
	return context.WithValue(ctx, auth.ClaimsContextKey, c)
}

func encodeBase64(s string) string {
	return base64.StdEncoding.EncodeToString([]byte(s))
}

func TestTUSCreateAndUpload(t *testing.T) {
	_, driveID, userID, h := setup(t)

	r := chi.NewRouter()
	r.Post("/api/tus/", h.Create)
	r.Patch("/api/tus/{id}", h.Upload)
	r.Head("/api/tus/{id}", h.Head)

	content := []byte("hello world")

	// POST — create upload
	req := httptest.NewRequest(http.MethodPost, "/api/tus/", nil)
	req.Header.Set("Upload-Length", fmt.Sprintf("%d", len(content)))
	req.Header.Set("Upload-Metadata", fmt.Sprintf("filename %s,drive_id %s",
		encodeBase64("test.txt"), encodeBase64(fmt.Sprintf("%d", driveID))))
	claims := &auth.Claims{UserID: userID}
	req = req.WithContext(ctxWithClaims(req.Context(), claims))

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("create: expected 201, got %d: %s", w.Code, w.Body.String())
	}
	location := w.Header().Get("Location")
	if location == "" {
		t.Fatal("expected Location header")
	}

	// PATCH — upload bytes
	req2 := httptest.NewRequest(http.MethodPatch, location, bytes.NewReader(content))
	req2.Header.Set("Content-Type", "application/offset+octet-stream")
	req2.Header.Set("Upload-Offset", "0")
	req2.Header.Set("Tus-Resumable", "1.0.0")
	req2 = req2.WithContext(ctxWithClaims(req2.Context(), claims))

	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)

	if w2.Code != http.StatusNoContent {
		t.Fatalf("upload: expected 204, got %d: %s", w2.Code, w2.Body.String())
	}
}
