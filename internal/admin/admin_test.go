package admin_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/flashyspeed/flashyspeed/internal/admin"
	"github.com/flashyspeed/flashyspeed/internal/auth"
	"github.com/flashyspeed/flashyspeed/internal/db"
)

// setup opens an in-memory SQLite database seeded with one admin user and
// one regular user, and returns the DB plus both user IDs.
func setup(t *testing.T) (database *db.DB, adminID, userID int64) {
	t.Helper()
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { database.Close() })

	hash, _ := auth.HashPassword("secret")

	res, err := database.Exec(
		`INSERT INTO users(username, email, password_hash, role) VALUES('admin','admin@test.com',?,'admin')`,
		hash,
	)
	if err != nil {
		t.Fatalf("seed admin: %v", err)
	}
	adminID, _ = res.LastInsertId()

	res, err = database.Exec(
		`INSERT INTO users(username, email, password_hash, role) VALUES('alice','alice@test.com',?,'user')`,
		hash,
	)
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}
	userID, _ = res.LastInsertId()

	return database, adminID, userID
}

// newRouter wires all admin routes the same way main.go does, with injected claims.
func newRouter(database *db.DB, claimsUserID int64, claimsRole string) http.Handler {
	h := admin.NewHandler(database)
	r := chi.NewRouter()

	// Inject claims directly (mirrors auth.Middleware behaviour in tests).
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := context.WithValue(r.Context(), auth.ClaimsContextKey, &auth.Claims{
				UserID: claimsUserID,
				Role:   claimsRole,
			})
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	})

	r.Get("/api/admin/users", h.ListUsers)
	r.Post("/api/admin/users", h.CreateUser)
	r.Patch("/api/admin/users/{id}", h.UpdateUser)
	r.Delete("/api/admin/users/{id}", h.DeleteUser)
	r.Get("/api/admin/storage", h.StorageDashboard)
	r.Get("/api/admin/tailscale/status", h.TailscaleStatus)
	return r
}

// idPath returns the route path suffix for a given user ID.
func idPath(base string, id int64) string {
	return fmt.Sprintf("%s/%d", base, id)
}

// ---- ListUsers ----

func TestListUsers(t *testing.T) {
	database, adminID, _ := setup(t)
	router := newRouter(database, adminID, "admin")

	req := httptest.NewRequest("GET", "/api/admin/users", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var users []map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&users); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(users) != 2 {
		t.Fatalf("expected 2 users (admin + alice), got %d", len(users))
	}
}

func TestListUsers_Forbidden(t *testing.T) {
	database, _, userID := setup(t)
	router := newRouter(database, userID, "user")

	req := httptest.NewRequest("GET", "/api/admin/users", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
}

// ---- CreateUser ----

func TestCreateUser(t *testing.T) {
	database, adminID, _ := setup(t)
	router := newRouter(database, adminID, "admin")

	body, _ := json.Marshal(map[string]interface{}{
		"username":    "bob",
		"email":       "bob@test.com",
		"password":    "hunter2",
		"role":        "user",
		"quota_bytes": 0,
	})
	req := httptest.NewRequest("POST", "/api/admin/users", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var count int
	database.QueryRow(`SELECT COUNT(*) FROM users WHERE username='bob'`).Scan(&count)
	if count != 1 {
		t.Fatal("user bob was not inserted into the database")
	}
}

func TestCreateUser_MissingFields(t *testing.T) {
	database, adminID, _ := setup(t)
	router := newRouter(database, adminID, "admin")

	body, _ := json.Marshal(map[string]interface{}{"username": "noemail"})
	req := httptest.NewRequest("POST", "/api/admin/users", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestCreateUser_DuplicateUsername(t *testing.T) {
	database, adminID, _ := setup(t)
	router := newRouter(database, adminID, "admin")

	// "admin" username already exists in seed data
	body, _ := json.Marshal(map[string]interface{}{
		"username": "admin",
		"email":    "admin2@test.com",
		"password": "pass",
		"role":     "user",
	})
	req := httptest.NewRequest("POST", "/api/admin/users", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", w.Code)
	}
}

// ---- UpdateUser ----

func TestUpdateUser_QuotaAndRole(t *testing.T) {
	database, adminID, userID := setup(t)
	router := newRouter(database, adminID, "admin")

	newRole := "admin"
	newQuota := int64(5_000_000_000)
	body, _ := json.Marshal(map[string]interface{}{
		"role":        newRole,
		"quota_bytes": newQuota,
	})

	req := httptest.NewRequest("PATCH", idPath("/api/admin/users", userID), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var role string
	var quota int64
	database.QueryRow(`SELECT role, quota_bytes FROM users WHERE id=?`, userID).Scan(&role, &quota)
	if role != newRole {
		t.Errorf("role: want %q, got %q", newRole, role)
	}
	if quota != newQuota {
		t.Errorf("quota_bytes: want %d, got %d", newQuota, quota)
	}
}

func TestUpdateUser_InvalidRole(t *testing.T) {
	database, adminID, userID := setup(t)
	router := newRouter(database, adminID, "admin")

	body, _ := json.Marshal(map[string]interface{}{"role": "superuser"})
	req := httptest.NewRequest("PATCH", idPath("/api/admin/users", userID), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

// ---- DeleteUser ----

func TestDeleteUser(t *testing.T) {
	database, adminID, userID := setup(t)
	router := newRouter(database, adminID, "admin")

	req := httptest.NewRequest("DELETE", idPath("/api/admin/users", userID), nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", w.Code, w.Body.String())
	}

	var count int
	database.QueryRow(`SELECT COUNT(*) FROM users WHERE id=?`, userID).Scan(&count)
	if count != 0 {
		t.Fatal("user was not deleted from the database")
	}
}

func TestDeleteUser_Self(t *testing.T) {
	database, adminID, _ := setup(t)
	router := newRouter(database, adminID, "admin")

	req := httptest.NewRequest("DELETE", idPath("/api/admin/users", adminID), nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 (cannot delete yourself), got %d", w.Code)
	}
}

func TestDeleteUser_NotFound(t *testing.T) {
	database, adminID, _ := setup(t)
	router := newRouter(database, adminID, "admin")

	req := httptest.NewRequest("DELETE", "/api/admin/users/99999", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

// ---- StorageDashboard ----

func TestStorageDashboard(t *testing.T) {
	database, adminID, userID := setup(t)

	// Seed a drive and a file so there is something to count.
	res, err := database.Exec(`INSERT INTO drives(name, mount_path) VALUES('testdrive','/tmp/admtest')`)
	if err != nil {
		t.Fatalf("seed drive: %v", err)
	}
	driveID, _ := res.LastInsertId()

	if _, err := database.Exec(
		`INSERT INTO files(user_id, drive_id, name, rel_path, size_bytes, is_dir) VALUES(?,?,'f.bin','f.bin',1024,0)`,
		userID, driveID,
	); err != nil {
		t.Fatalf("seed file: %v", err)
	}

	router := newRouter(database, adminID, "admin")
	req := httptest.NewRequest("GET", "/api/admin/storage", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var report struct {
		Drives []struct {
			DriveID    int64 `json:"drive_id"`
			TotalFiles int64 `json:"total_files"`
			TotalBytes int64 `json:"total_bytes"`
		} `json:"drives"`
		Users []struct {
			UserID    int64 `json:"user_id"`
			UsedBytes int64 `json:"used_bytes"`
		} `json:"users"`
	}
	if err := json.NewDecoder(w.Body).Decode(&report); err != nil {
		t.Fatalf("decode: %v", err)
	}

	var driveFound bool
	for _, d := range report.Drives {
		if d.DriveID == driveID {
			driveFound = true
			if d.TotalFiles != 1 {
				t.Errorf("total_files: want 1, got %d", d.TotalFiles)
			}
			if d.TotalBytes != 1024 {
				t.Errorf("total_bytes: want 1024, got %d", d.TotalBytes)
			}
		}
	}
	if !driveFound {
		t.Fatal("seeded drive not found in storage report")
	}

	for _, u := range report.Users {
		if u.UserID == userID && u.UsedBytes != 1024 {
			t.Errorf("alice used_bytes: want 1024, got %d", u.UsedBytes)
		}
	}
}

// ---- TailscaleStatus ----

func TestTailscaleStatus_ReturnsOKWithoutTailscale(t *testing.T) {
	database, adminID, _ := setup(t)
	router := newRouter(database, adminID, "admin")

	req := httptest.NewRequest("GET", "/api/admin/tailscale/status", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Tailscale is never installed in CI — but the handler must return 200,
	// because TailscaleStatusCheck returns {Running:false} (no error) when
	// the binary is not found.
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var status map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&status); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if _, ok := status["running"]; !ok {
		t.Error("response JSON missing 'running' field")
	}
}
