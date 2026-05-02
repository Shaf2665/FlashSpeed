package auth_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/flashyspeed/flashyspeed/internal/auth"
	"github.com/flashyspeed/flashyspeed/internal/db"
)

func testDB(t *testing.T) *db.DB {
	t.Helper()
	d, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	return d
}

func TestHashAndVerify(t *testing.T) {
	hash, err := auth.HashPassword("hunter2")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	if !auth.CheckPassword("hunter2", hash) {
		t.Error("correct password should verify")
	}
	if auth.CheckPassword("wrongpass", hash) {
		t.Error("wrong password should not verify")
	}
}

func TestJWTRoundtrip(t *testing.T) {
	secret := []byte("test-secret-32-bytes-long-padded!")
	token, err := auth.SignToken(42, "admin", secret, 1*time.Hour)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}

	claims, err := auth.VerifyToken(token, secret)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if claims.UserID != 42 {
		t.Errorf("expected userID 42, got %d", claims.UserID)
	}
	if claims.Role != "admin" {
		t.Errorf("expected role admin, got %s", claims.Role)
	}
}

func TestExpiredToken(t *testing.T) {
	secret := []byte("test-secret-32-bytes-long-padded!")
	token, _ := auth.SignToken(1, "user", secret, -1*time.Second)
	_, err := auth.VerifyToken(token, secret)
	if err == nil {
		t.Error("expired token should fail verification")
	}
}

func TestLoginHandler(t *testing.T) {
	database := testDB(t)
	h := auth.NewHandler(database, []byte("secret"))

	// seed a user
	hash, _ := auth.HashPassword("pass123")
	database.Exec(`INSERT INTO users(username,email,password_hash,role) VALUES(?,?,?,?)`,
		"alice", "alice@example.com", hash, "user")

	body := `{"username":"alice","password":"pass123"}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Login(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]string
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["token"] == "" {
		t.Error("expected token in response")
	}
}
