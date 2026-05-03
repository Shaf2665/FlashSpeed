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

func TestTUSMissingDriveID(t *testing.T) {
	_, _, userID, h := setup(t)

	r := chi.NewRouter()
	r.Post("/api/tus/", h.Create)

	req := httptest.NewRequest(http.MethodPost, "/api/tus/", nil)
	req.Header.Set("Upload-Length", "100")
	req.Header.Set("Upload-Metadata", fmt.Sprintf("filename %s", encodeBase64("test.txt")))
	// no drive_id in metadata

	claims := &auth.Claims{UserID: userID}
	req = req.WithContext(ctxWithClaims(req.Context(), claims))

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestTUSOversizedPatchDoesNotCorrupt(t *testing.T) {
	_, driveID, userID, h := setup(t)

	r := chi.NewRouter()
	r.Post("/api/tus/", h.Create)
	r.Patch("/api/tus/{id}", h.Upload)
	r.Head("/api/tus/{id}", h.Head)

	claims := &auth.Claims{UserID: userID}

	// declare 5 bytes but try to upload 10
	req := httptest.NewRequest(http.MethodPost, "/api/tus/", nil)
	req.Header.Set("Upload-Length", "5")
	req.Header.Set("Upload-Metadata", fmt.Sprintf("filename %s,drive_id %s",
		encodeBase64("test.txt"), encodeBase64(fmt.Sprintf("%d", driveID))))
	req = req.WithContext(ctxWithClaims(req.Context(), claims))

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("create: expected 201, got %d", w.Code)
	}
	location := w.Header().Get("Location")

	// send 10 bytes when only 5 declared
	oversized := bytes.Repeat([]byte("x"), 10)
	req2 := httptest.NewRequest(http.MethodPatch, location, bytes.NewReader(oversized))
	req2.Header.Set("Content-Type", "application/offset+octet-stream")
	req2.Header.Set("Upload-Offset", "0")
	req2 = req2.WithContext(ctxWithClaims(req2.Context(), claims))

	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)

	// Should succeed since LimitReader caps at 5 bytes and upload completes
	// (5 bytes written == uploadLength=5, so finalize triggers)
	// OR returns 413 — both acceptable behaviors. What matters: no corruption.
	// Check that HEAD returns consistent state afterward if 413 was returned.
	if w2.Code == http.StatusRequestEntityTooLarge {
		// 413 returned: temp file should have only 0 bytes written (LimitReader bounded it)
		// HEAD should still show offset=0
		req3 := httptest.NewRequest(http.MethodHead, location, nil)
		req3 = req3.WithContext(ctxWithClaims(req3.Context(), claims))
		w3 := httptest.NewRecorder()
		r.ServeHTTP(w3, req3)
		if w3.Code == http.StatusOK {
			offsetStr := w3.Header().Get("Upload-Offset")
			if offsetStr != "0" {
				t.Errorf("expected offset=0 after rejected oversized upload, got %s", offsetStr)
			}
		}
	} else if w2.Code != http.StatusNoContent {
		t.Errorf("expected 204 or 413, got %d: %s", w2.Code, w2.Body.String())
	}
}

func TestTUSOffsetMismatch(t *testing.T) {
	_, driveID, userID, h := setup(t)

	r := chi.NewRouter()
	r.Post("/api/tus/", h.Create)
	r.Patch("/api/tus/{id}", h.Upload)

	claims := &auth.Claims{UserID: userID}

	req := httptest.NewRequest(http.MethodPost, "/api/tus/", nil)
	req.Header.Set("Upload-Length", "100")
	req.Header.Set("Upload-Metadata", fmt.Sprintf("filename %s,drive_id %s",
		encodeBase64("test.txt"), encodeBase64(fmt.Sprintf("%d", driveID))))
	req = req.WithContext(ctxWithClaims(req.Context(), claims))

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	location := w.Header().Get("Location")

	// send with wrong offset (should be 0, sending 50)
	req2 := httptest.NewRequest(http.MethodPatch, location, bytes.NewReader([]byte("hello")))
	req2.Header.Set("Content-Type", "application/offset+octet-stream")
	req2.Header.Set("Upload-Offset", "50")
	req2 = req2.WithContext(ctxWithClaims(req2.Context(), claims))

	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)

	if w2.Code != http.StatusConflict {
		t.Errorf("expected 409 conflict on offset mismatch, got %d", w2.Code)
	}
}

func TestTUSOwnershipEnforced(t *testing.T) {
	database, driveID, userID, h := setup(t)

	// insert a second user
	res, _ := database.Exec(`INSERT INTO users(username,email,password_hash,role) VALUES('v','v@b.com','x','user')`)
	otherUserID, _ := res.LastInsertId()

	r := chi.NewRouter()
	r.Post("/api/tus/", h.Create)
	r.Patch("/api/tus/{id}", h.Upload)
	r.Head("/api/tus/{id}", h.Head)

	// userID creates an upload
	createReq := httptest.NewRequest(http.MethodPost, "/api/tus/", nil)
	createReq.Header.Set("Upload-Length", "100")
	createReq.Header.Set("Upload-Metadata", fmt.Sprintf("filename %s,drive_id %s",
		encodeBase64("secret.txt"), encodeBase64(fmt.Sprintf("%d", driveID))))
	createReq = createReq.WithContext(ctxWithClaims(createReq.Context(), &auth.Claims{UserID: userID}))

	cw := httptest.NewRecorder()
	r.ServeHTTP(cw, createReq)
	if cw.Code != http.StatusCreated {
		t.Fatalf("create: expected 201, got %d", cw.Code)
	}
	location := cw.Header().Get("Location")

	// otherUserID tries PATCH — should get 403
	patchReq := httptest.NewRequest(http.MethodPatch, location, bytes.NewReader([]byte("hello")))
	patchReq.Header.Set("Content-Type", "application/offset+octet-stream")
	patchReq.Header.Set("Upload-Offset", "0")
	patchReq = patchReq.WithContext(ctxWithClaims(patchReq.Context(), &auth.Claims{UserID: otherUserID}))

	pw := httptest.NewRecorder()
	r.ServeHTTP(pw, patchReq)
	if pw.Code != http.StatusForbidden {
		t.Errorf("PATCH by other user: expected 403, got %d", pw.Code)
	}

	// otherUserID tries HEAD — should get 403
	headReq := httptest.NewRequest(http.MethodHead, location, nil)
	headReq = headReq.WithContext(ctxWithClaims(headReq.Context(), &auth.Claims{UserID: otherUserID}))

	hw := httptest.NewRecorder()
	r.ServeHTTP(hw, headReq)
	if hw.Code != http.StatusForbidden {
		t.Errorf("HEAD by other user: expected 403, got %d", hw.Code)
	}
}
