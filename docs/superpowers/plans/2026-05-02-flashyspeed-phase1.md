# FlashySpeed Phase 1 — Foundation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a working single-binary Go file server with JWT auth, file browsing/upload/download, TUS resumable uploads, SQLite metadata, and a Svelte dark-theme web UI accessible on the local network.

**Architecture:** Single Go binary embedding a compiled Svelte frontend via `go:embed`. SQLite (WAL mode, pure-Go driver) stores all metadata; actual file bytes live on disk at real paths. `chi` handles HTTP routing with JWT middleware on protected routes. TUS protocol handles chunked resumable uploads.

**Tech Stack:** Go 1.22+, `go-chi/chi v5`, `modernc.org/sqlite`, `golang-jwt/jwt v5`, `golang.org/x/crypto` (bcrypt), `gopkg.in/yaml.v3`, `google/uuid`, Svelte 4 + Vite 5, `svelte-routing`

**Development note:** Code is developed on Windows but targets Linux. Drive scanner uses `/proc/mounts` — those tests are skipped on non-Linux platforms using `//go:build linux` build tags.

---

## File Map

```
flashyspeed/
├── cmd/flashyspeed/
│   └── main.go                      # server bootstrap, graceful shutdown
├── internal/
│   ├── config/
│   │   ├── config.go                # Config struct, YAML load, env overrides
│   │   └── config_test.go
│   ├── db/
│   │   ├── db.go                    # open SQLite, WAL pragma, connection pool
│   │   ├── migrations.go            # embedded SQL migrations, version tracking
│   │   └── db_test.go
│   ├── auth/
│   │   ├── auth.go                  # bcrypt hash/verify, JWT sign/verify
│   │   ├── middleware.go            # chi middleware: extract + validate JWT
│   │   ├── handler.go               # POST /api/auth/login, logout, GET /api/auth/me
│   │   └── auth_test.go
│   ├── drives/
│   │   ├── scanner.go               # /proc/mounts parser, manual paths, DB sync
│   │   ├── scanner_linux_test.go    # linux-only tests (build tag)
│   │   └── handler.go               # GET /api/drives, POST /api/drives, DELETE, POST /scan
│   ├── files/
│   │   ├── service.go               # list dir, mkdir, delete→trash, rename, quota check
│   │   ├── handler.go               # HTTP handlers wired to service
│   │   └── files_test.go
│   ├── tus/
│   │   ├── handler.go               # TUS 1.0.0 protocol: POST/PATCH/HEAD, finalize
│   │   └── tus_test.go
│   └── tlsmgr/
│       └── manager.go               # self-signed cert generation (Phase 1 only)
├── web/
│   ├── src/
│   │   ├── main.js                  # Svelte app mount
│   │   ├── App.svelte               # top-level router
│   │   ├── lib/
│   │   │   ├── api.js               # fetch wrapper, auth token injection
│   │   │   └── stores.js            # auth store, current-path store
│   │   └── routes/
│   │       ├── Login.svelte         # login form
│   │       └── Files.svelte         # file browser: list, upload, download, mkdir, delete
│   ├── package.json
│   ├── vite.config.js
│   └── index.html
├── embed.go                         # //go:embed web/dist
├── go.mod
├── go.sum
├── Makefile
├── flashyspeed.example.yaml
├── flashyspeed.service              # systemd unit
└── README.md
```

---

## Task 1: Project Scaffold

**Files:**
- Create: `go.mod`
- Create: `Makefile`
- Create: `flashyspeed.example.yaml`
- Create: `embed.go`
- Create: `cmd/flashyspeed/main.go` (stub)
- Create: `web/package.json`
- Create: `web/vite.config.js`
- Create: `web/index.html`
- Create: `web/src/main.js` (stub)
- Create: `web/src/App.svelte` (stub)
- Create: `.gitignore`

- [ ] **Step 1: Initialize Go module**

```bash
cd /path/to/flashyspeed   # your project root, e.g. ~/flashyspeed
go mod init github.com/flashyspeed/flashyspeed
```

Expected: `go.mod` created with `module github.com/flashyspeed/flashyspeed` and `go 1.22`

- [ ] **Step 2: Add Go dependencies**

```bash
go get github.com/go-chi/chi/v5@v5.1.0
go get github.com/golang-jwt/jwt/v5@v5.2.1
go get github.com/google/uuid@v1.6.0
go get golang.org/x/crypto@v0.22.0
go get gopkg.in/yaml.v3@v3.0.1
go get modernc.org/sqlite@v1.29.8
go mod tidy
```

Expected: `go.sum` populated, no errors.

- [ ] **Step 3: Create directory structure**

```bash
mkdir -p cmd/flashyspeed \
  internal/config \
  internal/db \
  internal/auth \
  internal/drives \
  internal/files \
  internal/tus \
  internal/tlsmgr \
  web/src/lib \
  web/src/routes \
  docs/superpowers/plans
```

- [ ] **Step 4: Write stub `cmd/flashyspeed/main.go`**

```go
package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Fprintln(os.Stdout, "FlashySpeed starting...")
	os.Exit(0)
}
```

- [ ] **Step 5: Write `embed.go`**

```go
package main

import "embed"

//go:embed web/dist
var webDist embed.FS
```

Note: `web/dist` does not exist yet — the build will fail until the frontend is compiled (Task 12). For now this file is created but `go build` won't succeed until Task 13.

- [ ] **Step 6: Write `flashyspeed.example.yaml`**

```yaml
server:
  port: 8080
  data_dir: /var/lib/flashyspeed

tls:
  mode: self-signed   # self-signed | auto | manual
  domain: ""          # required for mode: auto (Let's Encrypt)
  email: ""           # required for mode: auto
  cert_file: ""       # required for mode: manual
  key_file: ""        # required for mode: manual

storage:
  auto_detect_drives: true
  manual_paths: []

admin:
  create_default_admin: true
```

- [ ] **Step 7: Write `Makefile`**

```makefile
.PHONY: build build-frontend build-backend test clean

build: build-frontend build-backend

build-frontend:
	cd web && npm install && npm run build

build-backend:
	go build -o flashyspeed ./cmd/flashyspeed

test:
	go test ./...

clean:
	rm -f flashyspeed
	rm -rf web/dist
```

- [ ] **Step 8: Write `web/package.json`**

```json
{
  "name": "flashyspeed-web",
  "version": "1.0.0",
  "private": true,
  "scripts": {
    "build": "vite build",
    "dev": "vite"
  },
  "dependencies": {
    "svelte-routing": "^2.13.0"
  },
  "devDependencies": {
    "@sveltejs/vite-plugin-svelte": "^3.1.0",
    "svelte": "^4.2.18",
    "vite": "^5.2.11"
  }
}
```

- [ ] **Step 9: Write `web/vite.config.js`**

```js
import { defineConfig } from 'vite'
import { svelte } from '@sveltejs/vite-plugin-svelte'

export default defineConfig({
  plugins: [svelte()],
  build: {
    outDir: 'dist',
    emptyOutDir: true,
  },
  server: {
    proxy: {
      '/api': 'http://localhost:8080',
    },
  },
})
```

- [ ] **Step 10: Write `web/index.html`**

```html
<!DOCTYPE html>
<html lang="en">
  <head>
    <meta charset="UTF-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1.0" />
    <title>FlashySpeed</title>
  </head>
  <body>
    <div id="app"></div>
    <script type="module" src="/src/main.js"></script>
  </body>
</html>
```

- [ ] **Step 11: Write `web/src/main.js` stub**

```js
import App from './App.svelte'

const app = new App({ target: document.getElementById('app') })

export default app
```

- [ ] **Step 12: Write `web/src/App.svelte` stub**

```svelte
<script>
  import { Router, Route } from 'svelte-routing'
</script>

<Router>
  <Route path="/" component={() => import('./routes/Files.svelte')} />
</Router>
```

- [ ] **Step 13: Write `.gitignore`**

```
flashyspeed
web/dist
web/node_modules
*.db
*.db-shm
*.db-wal
*.pem
*.key
.superpowers/
```

- [ ] **Step 14: Commit**

```bash
git init
git add .
git commit -m "chore: project scaffold — Go module, Svelte setup, Makefile"
```

---

## Task 2: Config Package

**Files:**
- Create: `internal/config/config.go`
- Create: `internal/config/config_test.go`

- [ ] **Step 1: Write failing test**

```go
// internal/config/config_test.go
package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/flashyspeed/flashyspeed/internal/config"
)

func TestLoadDefaults(t *testing.T) {
	cfg, err := config.Load("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Server.Port != 8080 {
		t.Errorf("expected default port 8080, got %d", cfg.Server.Port)
	}
}

func TestLoadFromFile(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "cfg.yaml")
	os.WriteFile(path, []byte("server:\n  port: 9000\n"), 0644)

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Server.Port != 9000 {
		t.Errorf("expected port 9000, got %d", cfg.Server.Port)
	}
}

func TestEnvOverride(t *testing.T) {
	t.Setenv("FS_PORT", "7777")
	cfg, err := config.Load("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Server.Port != 7777 {
		t.Errorf("expected port 7777, got %d", cfg.Server.Port)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/config/...
```

Expected: `FAIL — cannot find package "github.com/flashyspeed/flashyspeed/internal/config"`

- [ ] **Step 3: Write `internal/config/config.go`**

```go
package config

import (
	"os"
	"strconv"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server  ServerConfig  `yaml:"server"`
	TLS     TLSConfig     `yaml:"tls"`
	Storage StorageConfig `yaml:"storage"`
	Admin   AdminConfig   `yaml:"admin"`
}

type ServerConfig struct {
	Port    int    `yaml:"port"`
	DataDir string `yaml:"data_dir"`
}

type TLSConfig struct {
	Mode     string `yaml:"mode"`
	Domain   string `yaml:"domain"`
	Email    string `yaml:"email"`
	CertFile string `yaml:"cert_file"`
	KeyFile  string `yaml:"key_file"`
}

type StorageConfig struct {
	AutoDetectDrives bool     `yaml:"auto_detect_drives"`
	ManualPaths      []string `yaml:"manual_paths"`
}

type AdminConfig struct {
	CreateDefaultAdmin bool `yaml:"create_default_admin"`
}

func Load(path string) (*Config, error) {
	cfg := &Config{}
	cfg.Server.Port = 8080
	cfg.Server.DataDir = "/var/lib/flashyspeed"
	cfg.TLS.Mode = "self-signed"
	cfg.Storage.AutoDetectDrives = true
	cfg.Admin.CreateDefaultAdmin = true

	if path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		if err := yaml.Unmarshal(data, cfg); err != nil {
			return nil, err
		}
	}

	if v := os.Getenv("FS_PORT"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			cfg.Server.Port = p
		}
	}
	if v := os.Getenv("FS_DATA_DIR"); v != "" {
		cfg.Server.DataDir = v
	}

	return cfg, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./internal/config/... -v
```

Expected: `PASS` for all 3 tests.

- [ ] **Step 5: Commit**

```bash
git add internal/config/
git commit -m "feat: config package — YAML load with env overrides"
```

---

## Task 3: Database Package

**Files:**
- Create: `internal/db/db.go`
- Create: `internal/db/migrations.go`
- Create: `internal/db/db_test.go`

- [ ] **Step 1: Write failing test**

```go
// internal/db/db_test.go
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
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/db/...
```

Expected: FAIL — package not found.

- [ ] **Step 3: Write `internal/db/db.go`**

```go
package db

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

type DB struct {
	*sql.DB
}

func Open(path string) (*DB, error) {
	sqldb, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	pragmas := []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA foreign_keys=ON",
		"PRAGMA busy_timeout=5000",
	}
	for _, p := range pragmas {
		if _, err := sqldb.Exec(p); err != nil {
			return nil, fmt.Errorf("pragma %q: %w", p, err)
		}
	}

	sqldb.SetMaxOpenConns(1) // SQLite WAL: one writer, many readers via pool

	db := &DB{sqldb}
	if err := db.migrate(); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return db, nil
}
```

- [ ] **Step 4: Write `internal/db/migrations.go`**

```go
package db

import "fmt"

const schema = `
CREATE TABLE IF NOT EXISTS schema_version (
  version INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS users (
  id            INTEGER PRIMARY KEY AUTOINCREMENT,
  username      TEXT    UNIQUE NOT NULL,
  email         TEXT    UNIQUE NOT NULL,
  password_hash TEXT    NOT NULL,
  role          TEXT    NOT NULL DEFAULT 'user',
  quota_bytes   INTEGER NOT NULL DEFAULT 0,
  created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS sessions (
  id          TEXT PRIMARY KEY,
  user_id     INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  token_hash  TEXT    NOT NULL,
  expires_at  DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS drives (
  id               INTEGER PRIMARY KEY AUTOINCREMENT,
  name             TEXT    NOT NULL,
  mount_path       TEXT    UNIQUE NOT NULL,
  is_auto_detected INTEGER NOT NULL DEFAULT 0,
  enabled          INTEGER NOT NULL DEFAULT 1
);

CREATE TABLE IF NOT EXISTS files (
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id     INTEGER NOT NULL REFERENCES users(id)  ON DELETE CASCADE,
  drive_id    INTEGER NOT NULL REFERENCES drives(id) ON DELETE CASCADE,
  name        TEXT    NOT NULL,
  rel_path    TEXT    NOT NULL,
  size_bytes  INTEGER NOT NULL DEFAULT 0,
  mime_type   TEXT,
  is_dir      INTEGER NOT NULL DEFAULT 0,
  parent_id   INTEGER REFERENCES files(id) ON DELETE SET NULL,
  deleted_at  DATETIME,
  created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS shares (
  id             TEXT    PRIMARY KEY,
  file_id        INTEGER NOT NULL REFERENCES files(id)  ON DELETE CASCADE,
  owner_id       INTEGER NOT NULL REFERENCES users(id)  ON DELETE CASCADE,
  target_user_id INTEGER REFERENCES users(id) ON DELETE CASCADE,
  password_hash  TEXT,
  expires_at     DATETIME,
  download_count INTEGER NOT NULL DEFAULT 0,
  max_downloads  INTEGER,
  created_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS tus_uploads (
  id            TEXT    PRIMARY KEY,
  user_id       INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  drive_id      INTEGER NOT NULL REFERENCES drives(id),
  dest_path     TEXT    NOT NULL,
  upload_length INTEGER NOT NULL,
  upload_offset INTEGER NOT NULL DEFAULT 0,
  temp_path     TEXT    NOT NULL,
  metadata      TEXT,
  created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
`

const currentVersion = 1

func (db *DB) migrate() error {
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_version (version INTEGER NOT NULL)`); err != nil {
		return err
	}

	var version int
	_ = db.QueryRow(`SELECT version FROM schema_version LIMIT 1`).Scan(&version)

	if version >= currentVersion {
		return nil
	}

	if _, err := db.Exec(schema); err != nil {
		return fmt.Errorf("apply schema: %w", err)
	}

	_, err := db.Exec(`INSERT OR REPLACE INTO schema_version(version) VALUES(?)`, currentVersion)
	return err
}
```

- [ ] **Step 5: Run tests to verify they pass**

```bash
go test ./internal/db/... -v
```

Expected: `PASS` for both tests.

- [ ] **Step 6: Commit**

```bash
git add internal/db/
git commit -m "feat: db package — SQLite WAL, schema migrations"
```

---

## Task 4: Auth Package — Core Logic

**Files:**
- Create: `internal/auth/auth.go`
- Create: `internal/auth/auth_test.go`

- [ ] **Step 1: Write failing test**

```go
// internal/auth/auth_test.go
package auth_test

import (
	"testing"
	"time"

	"github.com/flashyspeed/flashyspeed/internal/auth"
)

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
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/auth/...
```

Expected: FAIL — package not found.

- [ ] **Step 3: Write `internal/auth/auth.go`**

```go
package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

const bcryptCost = 12

type Claims struct {
	UserID int64  `json:"uid"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}

func HashPassword(password string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	return string(b), err
}

func CheckPassword(password, hash string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

func SignToken(userID int64, role string, secret []byte, ttl time.Duration) (string, error) {
	claims := Claims{
		UserID: userID,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(ttl)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(secret)
}

func VerifyToken(tokenStr string, secret []byte) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return secret, nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token")
	}
	return claims, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./internal/auth/... -v
```

Expected: `PASS` for all 3 tests.

- [ ] **Step 5: Commit**

```bash
git add internal/auth/auth.go internal/auth/auth_test.go
git commit -m "feat: auth core — bcrypt password hashing, JWT sign/verify"
```

---

## Task 5: Auth Middleware & HTTP Handlers

**Files:**
- Create: `internal/auth/middleware.go`
- Create: `internal/auth/handler.go`

- [ ] **Step 1: Write failing test**

```go
// internal/auth/auth_test.go — append to existing file

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
```

Add these imports at the top of `auth_test.go`:
```go
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
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/auth/...
```

Expected: FAIL — `auth.NewHandler` undefined.

- [ ] **Step 3: Write `internal/auth/middleware.go`**

```go
package auth

import (
	"context"
	"net/http"
	"strings"
)

type contextKey string

const claimsKey contextKey = "claims"

func Middleware(secret []byte) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Authorization")
			if !strings.HasPrefix(header, "Bearer ") {
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}
			claims, err := VerifyToken(strings.TrimPrefix(header, "Bearer "), secret)
			if err != nil {
				http.Error(w, `{"error":"invalid token"}`, http.StatusUnauthorized)
				return
			}
			ctx := context.WithValue(r.Context(), claimsKey, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func ClaimsFromCtx(r *http.Request) *Claims {
	v, _ := r.Context().Value(claimsKey).(*Claims)
	return v
}
```

- [ ] **Step 4: Write `internal/auth/handler.go`**

```go
package auth

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/flashyspeed/flashyspeed/internal/db"
)

const tokenTTL = 24 * time.Hour

type Handler struct {
	db     *db.DB
	secret []byte
}

func NewHandler(database *db.DB, secret []byte) *Handler {
	return &Handler{db: database, secret: secret}
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"bad request"}`, http.StatusBadRequest)
		return
	}

	var id int64
	var hash, role string
	err := h.db.QueryRow(
		`SELECT id, password_hash, role FROM users WHERE username=?`, req.Username,
	).Scan(&id, &hash, &role)
	if err != nil || !CheckPassword(req.Password, hash) {
		http.Error(w, `{"error":"invalid credentials"}`, http.StatusUnauthorized)
		return
	}

	token, err := SignToken(id, role, h.secret, tokenTTL)
	if err != nil {
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"token": token})
}

func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	claims := ClaimsFromCtx(r)
	if claims == nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	var username, email, role string
	var id int64
	err := h.db.QueryRow(
		`SELECT id, username, email, role FROM users WHERE id=?`, claims.UserID,
	).Scan(&id, &username, &email, &role)
	if err != nil {
		http.Error(w, `{"error":"user not found"}`, http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"id": id, "username": username, "email": email, "role": role,
	})
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	// JWT is stateless — client discards the token.
	// Phase 2 will add server-side session revocation.
	w.WriteHeader(http.StatusNoContent)
}
```

- [ ] **Step 5: Run tests to verify they pass**

```bash
go test ./internal/auth/... -v
```

Expected: `PASS` for all tests.

- [ ] **Step 6: Commit**

```bash
git add internal/auth/
git commit -m "feat: auth handlers — login, logout, me, JWT middleware"
```

---

## Task 6: Drive Scanner

**Files:**
- Create: `internal/drives/scanner.go`
- Create: `internal/drives/scanner_linux_test.go`
- Create: `internal/drives/handler.go`

- [ ] **Step 1: Write failing test**

```go
// internal/drives/scanner_linux_test.go
//go:build linux

package drives_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/flashyspeed/flashyspeed/internal/db"
	"github.com/flashyspeed/flashyspeed/internal/drives"
)

func TestParseMounts(t *testing.T) {
	// write a fake /proc/mounts
	tmp := t.TempDir()
	mounts := `sysfs /sys sysfs rw 0 0
proc /proc proc rw 0 0
/dev/sda1 / ext4 rw 0 1
/dev/sdb1 /mnt/external ext4 rw 0 0
tmpfs /tmp tmpfs rw 0 0
`
	path := filepath.Join(tmp, "mounts")
	os.WriteFile(path, []byte(mounts), 0644)

	results := drives.ParseMountsFile(path)

	// should include /mnt/external (real block device) but skip sysfs/proc/tmpfs
	found := false
	for _, d := range results {
		if d.MountPath == "/mnt/external" {
			found = true
		}
		if d.MountPath == "/tmp" {
			t.Error("/tmp (tmpfs) should be excluded")
		}
		if d.MountPath == "/sys" {
			t.Error("/sys should be excluded")
		}
	}
	if !found {
		t.Error("expected /mnt/external in results")
	}
}

func TestSyncDrives(t *testing.T) {
	database, _ := db.Open(filepath.Join(t.TempDir(), "test.db"))
	defer database.Close()

	scanner := drives.NewScanner(database)
	scanner.AddManual("/mnt/custom")

	if err := scanner.Sync(nil); err != nil {
		t.Fatalf("sync: %v", err)
	}

	var count int
	database.QueryRow(`SELECT COUNT(*) FROM drives WHERE mount_path=?`, "/mnt/custom").Scan(&count)
	if count != 1 {
		t.Error("manual drive should be in DB")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/drives/... -tags linux
```

Expected: FAIL — package not found.

- [ ] **Step 3: Write `internal/drives/scanner.go`**

```go
package drives

import (
	"bufio"
	"os"
	"strings"

	"github.com/flashyspeed/flashyspeed/internal/db"
)

var skipFSTypes = map[string]bool{
	"sysfs": true, "proc": true, "tmpfs": true, "devtmpfs": true,
	"devpts": true, "cgroup": true, "cgroup2": true, "pstore": true,
	"mqueue": true, "hugetlbfs": true, "debugfs": true, "securityfs": true,
	"fusectl": true, "bpf": true, "overlay": true,
}

type Drive struct {
	Name      string
	MountPath string
	IsAuto    bool
}

func ParseMountsFile(path string) []Drive {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	var results []Drive
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		parts := strings.Fields(scanner.Text())
		if len(parts) < 3 {
			continue
		}
		device, mountPath, fsType := parts[0], parts[1], parts[2]
		if skipFSTypes[fsType] {
			continue
		}
		if !strings.HasPrefix(device, "/dev/") {
			continue
		}
		results = append(results, Drive{
			Name:      mountPath,
			MountPath: mountPath,
			IsAuto:    true,
		})
	}
	return results
}

type Scanner struct {
	db           *db.DB
	manualPaths  []string
}

func NewScanner(database *db.DB) *Scanner {
	return &Scanner{db: database}
}

func (s *Scanner) AddManual(path string) {
	s.manualPaths = append(s.manualPaths, path)
}

// Sync upserts drives into the DB. autoDetected is the result of ParseMountsFile;
// pass nil to skip auto-detection (manual paths are always synced).
func (s *Scanner) Sync(autoDetected []Drive) error {
	drives := append([]Drive{}, autoDetected...)
	for _, p := range s.manualPaths {
		drives = append(drives, Drive{Name: p, MountPath: p, IsAuto: false})
	}

	for _, d := range drives {
		isAuto := 0
		if d.IsAuto {
			isAuto = 1
		}
		_, err := s.db.Exec(`
			INSERT INTO drives(name, mount_path, is_auto_detected)
			VALUES(?,?,?)
			ON CONFLICT(mount_path) DO UPDATE SET name=excluded.name
		`, d.Name, d.MountPath, isAuto)
		if err != nil {
			return err
		}
	}
	return nil
}

func ScanSystem() []Drive {
	return ParseMountsFile("/proc/mounts")
}
```

- [ ] **Step 4: Write `internal/drives/handler.go`**

```go
package drives

import (
	"encoding/json"
	"net/http"

	"github.com/flashyspeed/flashyspeed/internal/db"
)

type Handler struct {
	db      *db.DB
	scanner *Scanner
}

func NewHandler(database *db.DB, scanner *Scanner) *Handler {
	return &Handler{db: database, scanner: scanner}
}

type driveRow struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	MountPath string `json:"mount_path"`
	IsAuto    bool   `json:"is_auto_detected"`
	Enabled   bool   `json:"enabled"`
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.Query(`SELECT id, name, mount_path, is_auto_detected, enabled FROM drives`)
	if err != nil {
		http.Error(w, `{"error":"db error"}`, http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var result []driveRow
	for rows.Next() {
		var d driveRow
		var isAuto, enabled int
		rows.Scan(&d.ID, &d.Name, &d.MountPath, &isAuto, &enabled)
		d.IsAuto = isAuto == 1
		d.Enabled = enabled == 1
		result = append(result, d)
	}
	if result == nil {
		result = []driveRow{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func (h *Handler) TriggerScan(w http.ResponseWriter, r *http.Request) {
	detected := ScanSystem()
	if err := h.scanner.Sync(detected); err != nil {
		http.Error(w, `{"error":"scan failed"}`, http.StatusInternalServerError)
		return
	}
	h.List(w, r)
}
```

- [ ] **Step 5: Run tests**

```bash
go test ./internal/drives/...
```

Expected: PASS (tests run on Linux; on Windows they are skipped due to build tag).

- [ ] **Step 6: Commit**

```bash
git add internal/drives/
git commit -m "feat: drive scanner — /proc/mounts parser, manual paths, DB sync"
```

---

## Task 7: File Service

**Files:**
- Create: `internal/files/service.go`
- Create: `internal/files/files_test.go`

- [ ] **Step 1: Write failing tests**

```go
// internal/files/files_test.go
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
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/files/...
```

Expected: FAIL — package not found.

- [ ] **Step 3: Write `internal/files/service.go`**

```go
package files

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/flashyspeed/flashyspeed/internal/db"
)

type Entry struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	IsDir     bool      `json:"is_dir"`
	SizeBytes int64     `json:"size_bytes"`
	MimeType  string    `json:"mime_type"`
	ParentID  *int64    `json:"parent_id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Service struct {
	db *db.DB
}

func NewService(database *db.DB) *Service {
	return &Service{db: database}
}

func (s *Service) drivePath(driveID int64) (string, error) {
	var p string
	err := s.db.QueryRow(`SELECT mount_path FROM drives WHERE id=?`, driveID).Scan(&p)
	return p, err
}

func (s *Service) Mkdir(userID, driveID, parentID int64, name string) (int64, error) {
	mountPath, err := s.drivePath(driveID)
	if err != nil {
		return 0, fmt.Errorf("drive not found: %w", err)
	}

	relPath := name
	if parentID != 0 {
		var parentRel string
		s.db.QueryRow(`SELECT rel_path FROM files WHERE id=?`, parentID).Scan(&parentRel)
		relPath = filepath.Join(parentRel, name)
	}

	absPath := filepath.Join(mountPath, relPath)
	if err := os.MkdirAll(absPath, 0755); err != nil {
		return 0, fmt.Errorf("mkdir on disk: %w", err)
	}

	var pID interface{} = nil
	if parentID != 0 {
		pID = parentID
	}
	res, err := s.db.Exec(`
		INSERT INTO files(user_id, drive_id, name, rel_path, is_dir, parent_id)
		VALUES(?,?,?,?,1,?)
	`, userID, driveID, name, relPath, pID)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *Service) List(userID, driveID, parentID int64) ([]Entry, error) {
	var rows interface{ Scan(...interface{}) error }
	var query string
	var args []interface{}

	if parentID == 0 {
		query = `SELECT id,name,is_dir,size_bytes,COALESCE(mime_type,''),parent_id,created_at,updated_at
		         FROM files WHERE user_id=? AND drive_id=? AND parent_id IS NULL AND deleted_at IS NULL`
		args = []interface{}{userID, driveID}
	} else {
		query = `SELECT id,name,is_dir,size_bytes,COALESCE(mime_type,''),parent_id,created_at,updated_at
		         FROM files WHERE user_id=? AND drive_id=? AND parent_id=? AND deleted_at IS NULL`
		args = []interface{}{userID, driveID, parentID}
	}

	dbRows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer dbRows.Close()
	_ = rows

	var entries []Entry
	for dbRows.Next() {
		var e Entry
		var pID *int64
		dbRows.Scan(&e.ID, &e.Name, &e.IsDir, &e.SizeBytes, &e.MimeType, &pID, &e.CreatedAt, &e.UpdatedAt)
		e.ParentID = pID
		entries = append(entries, e)
	}
	return entries, nil
}

func (s *Service) Delete(userID, fileID int64) error {
	res, err := s.db.Exec(`
		UPDATE files SET deleted_at=CURRENT_TIMESTAMP
		WHERE id=? AND user_id=? AND deleted_at IS NULL
	`, fileID, userID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("file not found or already deleted")
	}
	return nil
}

func (s *Service) Trash(userID int64) ([]Entry, error) {
	rows, err := s.db.Query(`
		SELECT id,name,is_dir,size_bytes,COALESCE(mime_type,''),parent_id,created_at,updated_at
		FROM files WHERE user_id=? AND deleted_at IS NOT NULL
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []Entry
	for rows.Next() {
		var e Entry
		var pID *int64
		rows.Scan(&e.ID, &e.Name, &e.IsDir, &e.SizeBytes, &e.MimeType, &pID, &e.CreatedAt, &e.UpdatedAt)
		e.ParentID = pID
		entries = append(entries, e)
	}
	return entries, nil
}

func (s *Service) Rename(userID, fileID int64, newName string) error {
	_, err := s.db.Exec(`
		UPDATE files SET name=?, updated_at=CURRENT_TIMESTAMP
		WHERE id=? AND user_id=? AND deleted_at IS NULL
	`, newName, fileID, userID)
	return err
}

func (s *Service) AbsPath(fileID int64) (string, error) {
	var relPath string
	var mountPath string
	err := s.db.QueryRow(`
		SELECT f.rel_path, d.mount_path
		FROM files f JOIN drives d ON f.drive_id=d.id
		WHERE f.id=?
	`, fileID).Scan(&relPath, &mountPath)
	if err != nil {
		return "", err
	}
	return filepath.Join(mountPath, relPath), nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./internal/files/... -v
```

Expected: `PASS` for all 3 tests.

- [ ] **Step 5: Commit**

```bash
git add internal/files/
git commit -m "feat: file service — list, mkdir, soft-delete to trash, rename"
```

---

## Task 8: File HTTP Handlers

**Files:**
- Create: `internal/files/handler.go`

- [ ] **Step 1: Write failing test (append to `files_test.go`)**

```go
func TestFileHandlerList(t *testing.T) {
	database, driveID, userID := setup(t)
	svc := files.NewService(database)
	svc.Mkdir(userID, driveID, 0, "docs")

	// sign a token
	token, _ := auth.SignToken(userID, "user", []byte("secret"), time.Hour)

	h := files.NewHandler(database, svc)
	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/files?drive_id=%d", driveID), nil)
	req.Header.Set("Authorization", "Bearer "+token)

	// inject claims manually (middleware would do this in real request)
	claims := &auth.Claims{UserID: userID, Role: "user"}
	ctx := context.WithValue(req.Context(), auth.ClaimsContextKey, claims)
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	h.List(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}
```

Add `"context"`, `"fmt"`, and `"github.com/flashyspeed/flashyspeed/internal/auth"` to imports.

Also export the context key from auth package — add to `internal/auth/middleware.go`:
```go
// ClaimsContextKey is exported so tests can inject claims directly.
const ClaimsContextKey = claimsKey
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/files/...
```

Expected: FAIL — `files.NewHandler` undefined.

- [ ] **Step 3: Write `internal/files/handler.go`**

```go
package files

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/flashyspeed/flashyspeed/internal/auth"
	"github.com/flashyspeed/flashyspeed/internal/db"
)

type Handler struct {
	db  *db.DB
	svc *Service
}

func NewHandler(database *db.DB, svc *Service) *Handler {
	return &Handler{db: database, svc: svc}
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromCtx(r)
	driveID, _ := strconv.ParseInt(r.URL.Query().Get("drive_id"), 10, 64)
	parentID, _ := strconv.ParseInt(r.URL.Query().Get("parent_id"), 10, 64)

	entries, err := h.svc.List(claims.UserID, driveID, parentID)
	if err != nil {
		http.Error(w, `{"error":"list failed"}`, http.StatusInternalServerError)
		return
	}
	if entries == nil {
		entries = []Entry{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(entries)
}

func (h *Handler) Mkdir(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromCtx(r)
	var body struct {
		DriveID  int64  `json:"drive_id"`
		ParentID int64  `json:"parent_id"`
		Name     string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Name == "" {
		http.Error(w, `{"error":"bad request"}`, http.StatusBadRequest)
		return
	}

	id, err := h.svc.Mkdir(claims.UserID, body.DriveID, body.ParentID, body.Name)
	if err != nil {
		http.Error(w, `{"error":"mkdir failed"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]int64{"id": id})
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromCtx(r)
	fileID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, `{"error":"bad id"}`, http.StatusBadRequest)
		return
	}
	if err := h.svc.Delete(claims.UserID, fileID); err != nil {
		http.Error(w, `{"error":"delete failed"}`, http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) Rename(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromCtx(r)
	fileID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, `{"error":"bad id"}`, http.StatusBadRequest)
		return
	}
	var body struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Name == "" {
		http.Error(w, `{"error":"bad request"}`, http.StatusBadRequest)
		return
	}
	if err := h.svc.Rename(claims.UserID, fileID, body.Name); err != nil {
		http.Error(w, `{"error":"rename failed"}`, http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) Download(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromCtx(r)
	fileID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, `{"error":"bad id"}`, http.StatusBadRequest)
		return
	}

	// verify ownership
	var ownerID int64
	var name string
	if err := h.db.QueryRow(`SELECT user_id, name FROM files WHERE id=? AND deleted_at IS NULL`, fileID).
		Scan(&ownerID, &name); err != nil || ownerID != claims.UserID {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
		return
	}

	absPath, err := h.svc.AbsPath(fileID)
	if err != nil {
		http.Error(w, `{"error":"resolve path failed"}`, http.StatusInternalServerError)
		return
	}

	f, err := open(absPath)
	if err != nil {
		http.Error(w, `{"error":"file open failed"}`, http.StatusInternalServerError)
		return
	}
	defer f.Close()

	w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)
	http.ServeContent(w, r, name, fileModTime(f), f)
}
```

Add these helpers at the bottom of `handler.go`:
```go
import "os"

func open(path string) (*os.File, error) { return os.Open(path) }
func fileModTime(f *os.File) time.Time {
	info, _ := f.Stat()
	if info == nil {
		return time.Time{}
	}
	return info.ModTime()
}
```

Fix import: add `"time"` to the import block.

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./internal/files/... -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/files/handler.go
git commit -m "feat: file HTTP handlers — list, mkdir, delete, rename, download"
```

---

## Task 9: TUS Upload Handler

**Files:**
- Create: `internal/tus/handler.go`
- Create: `internal/tus/tus_test.go`

- [ ] **Step 1: Write failing test**

```go
// internal/tus/tus_test.go
package tus_test

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/flashyspeed/flashyspeed/internal/auth"
	"github.com/flashyspeed/flashyspeed/internal/db"
	"github.com/flashyspeed/flashyspeed/internal/tus"
	"time"
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

func TestTUSCreateAndUpload(t *testing.T) {
	_, driveID, userID, h := setup(t)

	// POST to create upload
	r := chi.NewRouter()
	r.Post("/api/tus/", h.Create)
	r.Patch("/api/tus/{id}", h.Upload)
	r.Head("/api/tus/{id}", h.Head)

	content := []byte("hello world")

	req := httptest.NewRequest(http.MethodPost, "/api/tus/", nil)
	req.Header.Set("Upload-Length", fmt.Sprintf("%d", len(content)))
	req.Header.Set("Upload-Metadata", fmt.Sprintf("filename %s,drive_id %d",
		encodeBase64("test.txt"), driveID))

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

	// PATCH to upload bytes
	req2 := httptest.NewRequest(http.MethodPatch, location, bytes.NewReader(content))
	req2.Header.Set("Content-Type", "application/offset+octet-stream")
	req2.Header.Set("Upload-Offset", "0")
	req2 = req2.WithContext(ctxWithClaims(req2.Context(), claims))

	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)

	if w2.Code != http.StatusNoContent {
		t.Fatalf("upload: expected 204, got %d: %s", w2.Code, w2.Body.String())
	}
}
```

Add helpers:
```go
import (
	"context"
	"encoding/base64"
)

func encodeBase64(s string) string { return base64.StdEncoding.EncodeToString([]byte(s)) }

func ctxWithClaims(ctx context.Context, c *auth.Claims) context.Context {
	return context.WithValue(ctx, auth.ClaimsContextKey, c)
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/tus/...
```

Expected: FAIL — package not found.

- [ ] **Step 3: Write `internal/tus/handler.go`**

```go
package tus

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/flashyspeed/flashyspeed/internal/auth"
	"github.com/flashyspeed/flashyspeed/internal/db"
)

const tusVersion = "1.0.0"

type Handler struct {
	db      *db.DB
	tempDir string // where in-progress upload chunks are written
}

func NewHandler(database *db.DB, tempDir string) *Handler {
	os.MkdirAll(tempDir, 0755)
	return &Handler{db: database, tempDir: tempDir}
}

// Create handles POST /api/tus/ — initiates a new upload
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromCtx(r)
	uploadLength, err := strconv.ParseInt(r.Header.Get("Upload-Length"), 10, 64)
	if err != nil || uploadLength < 0 {
		http.Error(w, "invalid Upload-Length", http.StatusBadRequest)
		return
	}

	meta := parseMetadata(r.Header.Get("Upload-Metadata"))
	driveID, _ := strconv.ParseInt(meta["drive_id"], 10, 64)
	filename := meta["filename"]
	if filename == "" {
		filename = "upload-" + time.Now().Format("20060102-150405")
	}

	id := uuid.New().String()
	tempPath := filepath.Join(h.tempDir, id+".tmp")

	// create empty temp file
	f, err := os.Create(tempPath)
	if err != nil {
		http.Error(w, "create temp file failed", http.StatusInternalServerError)
		return
	}
	f.Close()

	metaJSON, _ := json.Marshal(meta)
	_, err = h.db.Exec(`
		INSERT INTO tus_uploads(id, user_id, drive_id, dest_path, upload_length, temp_path, metadata)
		VALUES(?,?,?,?,?,?,?)
	`, id, claims.UserID, driveID, filename, uploadLength, tempPath, string(metaJSON))
	if err != nil {
		http.Error(w, "db insert failed", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Location", "/api/tus/"+id)
	w.Header().Set("Tus-Resumable", tusVersion)
	w.WriteHeader(http.StatusCreated)
}

// Head handles HEAD /api/tus/:id — returns current upload offset
func (h *Handler) Head(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var offset, length int64
	err := h.db.QueryRow(`SELECT upload_offset, upload_length FROM tus_uploads WHERE id=?`, id).
		Scan(&offset, &length)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Upload-Offset", strconv.FormatInt(offset, 10))
	w.Header().Set("Upload-Length", strconv.FormatInt(length, 10))
	w.Header().Set("Tus-Resumable", tusVersion)
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
}

// Upload handles PATCH /api/tus/:id — appends bytes to upload
func (h *Handler) Upload(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var uploadOffset, uploadLength int64
	var tempPath, destPath string
	var userID, driveID int64
	var metaJSON string

	err := h.db.QueryRow(`
		SELECT upload_offset, upload_length, temp_path, dest_path, user_id, drive_id, metadata
		FROM tus_uploads WHERE id=?
	`, id).Scan(&uploadOffset, &uploadLength, &tempPath, &destPath, &userID, &driveID, &metaJSON)
	if err != nil {
		http.Error(w, "upload not found", http.StatusNotFound)
		return
	}

	offset, err := strconv.ParseInt(r.Header.Get("Upload-Offset"), 10, 64)
	if err != nil || offset != uploadOffset {
		http.Error(w, "incorrect offset", http.StatusConflict)
		return
	}

	f, err := os.OpenFile(tempPath, os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		http.Error(w, "open temp file failed", http.StatusInternalServerError)
		return
	}
	written, err := io.Copy(f, r.Body)
	f.Close()
	if err != nil {
		http.Error(w, "write failed", http.StatusInternalServerError)
		return
	}

	newOffset := uploadOffset + written
	h.db.Exec(`UPDATE tus_uploads SET upload_offset=? WHERE id=?`, newOffset, id)

	if newOffset >= uploadLength {
		// upload complete — move to final destination
		if err := h.finalize(id, userID, driveID, tempPath, destPath, uploadLength); err != nil {
			http.Error(w, "finalize failed: "+err.Error(), http.StatusInternalServerError)
			return
		}
	}

	w.Header().Set("Upload-Offset", strconv.FormatInt(newOffset, 10))
	w.Header().Set("Tus-Resumable", tusVersion)
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) finalize(uploadID string, userID, driveID int64, tempPath, destPath string, size int64) error {
	var mountPath string
	if err := h.db.QueryRow(`SELECT mount_path FROM drives WHERE id=?`, driveID).Scan(&mountPath); err != nil {
		return fmt.Errorf("drive not found: %w", err)
	}

	finalPath := filepath.Join(mountPath, destPath)
	if err := os.MkdirAll(filepath.Dir(finalPath), 0755); err != nil {
		return err
	}
	if err := os.Rename(tempPath, finalPath); err != nil {
		return err
	}

	mime := detectMIME(finalPath)
	_, err := h.db.Exec(`
		INSERT INTO files(user_id, drive_id, name, rel_path, size_bytes, mime_type, is_dir)
		VALUES(?,?,?,?,?,?,0)
	`, userID, driveID, filepath.Base(destPath), destPath, size, mime)
	if err != nil {
		return err
	}

	h.db.Exec(`DELETE FROM tus_uploads WHERE id=?`, uploadID)
	return nil
}

func parseMetadata(header string) map[string]string {
	result := make(map[string]string)
	for _, pair := range strings.Split(header, ",") {
		pair = strings.TrimSpace(pair)
		parts := strings.SplitN(pair, " ", 2)
		if len(parts) == 2 {
			decoded, err := base64.StdEncoding.DecodeString(parts[1])
			if err == nil {
				result[parts[0]] = string(decoded)
			}
		} else if len(parts) == 1 && parts[0] != "" {
			result[parts[0]] = ""
		}
	}
	return result
}

func detectMIME(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return "application/octet-stream"
	}
	defer f.Close()
	buf := make([]byte, 512)
	n, _ := f.Read(buf)
	return http.DetectContentType(buf[:n])
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./internal/tus/... -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/tus/
git commit -m "feat: TUS upload handler — create, head, patch, finalize to file record"
```

---

## Task 10: TLS Manager (Self-Signed)

**Files:**
- Create: `internal/tlsmgr/manager.go`

- [ ] **Step 1: Write `internal/tlsmgr/manager.go`**

No test here — crypto/tls is well-tested by stdlib. We verify it works in the integration smoke test (Task 11).

```go
package tlsmgr

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"time"
)

// SelfSigned generates (or loads cached) a self-signed TLS cert for local use.
// certDir is where the cert/key PEM files are stored.
func SelfSigned(certDir string) (*tls.Config, error) {
	certPath := filepath.Join(certDir, "server.crt")
	keyPath := filepath.Join(certDir, "server.key")

	// reuse if exists and not expired
	if cert, err := tls.LoadX509KeyPair(certPath, keyPath); err == nil {
		return &tls.Config{Certificates: []tls.Certificate{cert}}, nil
	}

	if err := os.MkdirAll(certDir, 0700); err != nil {
		return nil, err
	}

	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}

	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{Organization: []string{"FlashySpeed"}},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
		DNSNames:     []string{"localhost"},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:     x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &priv.PublicKey, priv)
	if err != nil {
		return nil, err
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	keyDER, _ := x509.MarshalECPrivateKey(priv)
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})

	os.WriteFile(certPath, certPEM, 0644)
	os.WriteFile(keyPath, keyPEM, 0600)

	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return nil, err
	}
	return &tls.Config{Certificates: []tls.Certificate{cert}}, nil
}
```

- [ ] **Step 2: Commit**

```bash
git add internal/tlsmgr/
git commit -m "feat: TLS manager — self-signed cert generation and caching"
```

---

## Task 11: Server Bootstrap

**Files:**
- Modify: `cmd/flashyspeed/main.go`

- [ ] **Step 1: Write `cmd/flashyspeed/main.go`**

```go
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/flashyspeed/flashyspeed/internal/auth"
	"github.com/flashyspeed/flashyspeed/internal/config"
	"github.com/flashyspeed/flashyspeed/internal/db"
	"github.com/flashyspeed/flashyspeed/internal/drives"
	"github.com/flashyspeed/flashyspeed/internal/files"
	"github.com/flashyspeed/flashyspeed/internal/tlsmgr"
	"github.com/flashyspeed/flashyspeed/internal/tus"
)

func main() {
	cfgPath := ""
	if len(os.Args) > 1 {
		cfgPath = os.Args[1]
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	if err := os.MkdirAll(cfg.Server.DataDir, 0755); err != nil {
		log.Fatalf("create data dir: %v", err)
	}

	database, err := db.Open(cfg.Server.DataDir + "/flashyspeed.db")
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer database.Close()

	jwtSecret := []byte(os.Getenv("FS_JWT_SECRET"))
	if len(jwtSecret) < 32 {
		log.Fatal("FS_JWT_SECRET env var must be at least 32 bytes")
	}

	// seed default admin on first run
	if cfg.Admin.CreateDefaultAdmin {
		seedAdmin(database)
	}

	// drives
	scanner := drives.NewScanner(database)
	for _, p := range cfg.Storage.ManualPaths {
		scanner.AddManual(p)
	}
	if cfg.Storage.AutoDetectDrives {
		scanner.Sync(drives.ScanSystem())
	} else {
		scanner.Sync(nil)
	}

	// handlers
	authHandler := auth.NewHandler(database, jwtSecret)
	driveHandler := drives.NewHandler(database, scanner)
	fileSvc := files.NewService(database)
	fileHandler := files.NewHandler(database, fileSvc)
	tusHandler := tus.NewHandler(database, cfg.Server.DataDir+"/tus-tmp")

	authMW := auth.Middleware(jwtSecret)

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Post("/api/auth/login", authHandler.Login)
	r.Post("/api/auth/logout", authHandler.Logout)

	r.Group(func(r chi.Router) {
		r.Use(authMW)

		r.Get("/api/auth/me", authHandler.Me)

		r.Get("/api/files", fileHandler.List)
		r.Post("/api/files/mkdir", fileHandler.Mkdir)
		r.Delete("/api/files/{id}", fileHandler.Delete)
		r.Patch("/api/files/{id}", fileHandler.Rename)
		r.Get("/api/files/{id}/download", fileHandler.Download)

		r.Post("/api/tus/", tusHandler.Create)
		r.Head("/api/tus/{id}", tusHandler.Head)
		r.Patch("/api/tus/{id}", tusHandler.Upload)

		r.Get("/api/drives", driveHandler.List)
		r.Post("/api/drives/scan", driveHandler.TriggerScan)
	})

	// serve embedded Svelte SPA (built in Task 12/13)
	r.Get("/*", serveFrontend())

	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	srv := &http.Server{Addr: addr, Handler: r}

	tlsCfg, err := tlsmgr.SelfSigned(cfg.Server.DataDir + "/tls")
	if err != nil {
		log.Fatalf("TLS setup: %v", err)
	}
	srv.TLSConfig = tlsCfg

	go func() {
		log.Printf("FlashySpeed listening on https://localhost%s", addr)
		if err := srv.ListenAndServeTLS("", ""); err != nil && err != http.ErrServerClosed {
			log.Fatalf("serve: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	srv.Shutdown(ctx)
	log.Println("FlashySpeed stopped.")
}

func seedAdmin(database *db.DB) {
	var count int
	database.QueryRow(`SELECT COUNT(*) FROM users WHERE role='admin'`).Scan(&count)
	if count > 0 {
		return
	}
	hash, _ := auth.HashPassword("admin")
	database.Exec(
		`INSERT INTO users(username,email,password_hash,role) VALUES('admin','admin@localhost',?,'admin')`,
		hash,
	)
	log.Println("Created default admin user: admin / admin — change the password immediately!")
}
```

- [ ] **Step 2: Add `serveFrontend()` placeholder to `cmd/flashyspeed/main.go`**

This will serve the embedded frontend (wired in Task 13). Add before the closing brace:

```go
// In a separate file cmd/flashyspeed/frontend.go — created in Task 13.
// Placeholder stub so the server compiles now.
func serveFrontend() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte("<h1>FlashySpeed</h1><p>Frontend not built yet. Run <code>make build</code>.</p>"))
	}
}
```

- [ ] **Step 3: Verify compilation (no frontend yet)**

```bash
go build ./cmd/flashyspeed/
```

Expected: binary produced, no errors (web/dist doesn't need to exist yet because embed.go is not yet hooked into main).

Note: If `embed.go` causes a compilation error because `web/dist` doesn't exist, comment out the `//go:embed` line temporarily until Task 13.

- [ ] **Step 4: Commit**

```bash
git add cmd/flashyspeed/
git commit -m "feat: server bootstrap — chi routing, auth middleware, graceful shutdown"
```

---

## Task 12: Svelte Frontend

**Files:**
- Create: `web/src/lib/api.js`
- Create: `web/src/lib/stores.js`
- Create: `web/src/routes/Login.svelte`
- Create: `web/src/routes/Files.svelte`
- Modify: `web/src/App.svelte`

- [ ] **Step 1: Install frontend dependencies**

```bash
cd web && npm install
```

Expected: `node_modules/` created, no errors.

- [ ] **Step 2: Write `web/src/lib/stores.js`**

```js
import { writable, derived } from 'svelte/store'

export const token = writable(localStorage.getItem('fs_token') || null)
export const isLoggedIn = derived(token, $t => !!$t)

token.subscribe(val => {
  if (val) localStorage.setItem('fs_token', val)
  else localStorage.removeItem('fs_token')
})

export const currentDriveId = writable(null)
export const currentParentId = writable(0)
```

- [ ] **Step 3: Write `web/src/lib/api.js`**

```js
import { get } from 'svelte/store'
import { token } from './stores.js'

const base = '/api'

async function request(method, path, body) {
  const headers = { 'Content-Type': 'application/json' }
  const tok = get(token)
  if (tok) headers['Authorization'] = `Bearer ${tok}`

  const res = await fetch(base + path, {
    method,
    headers,
    body: body ? JSON.stringify(body) : undefined,
  })

  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: res.statusText }))
    throw new Error(err.error || res.statusText)
  }
  if (res.status === 204) return null
  return res.json()
}

export const api = {
  login: (username, password) => request('POST', '/auth/login', { username, password }),
  me: () => request('GET', '/auth/me'),
  logout: () => request('POST', '/auth/logout'),

  listFiles: (driveId, parentId) =>
    request('GET', `/files?drive_id=${driveId}&parent_id=${parentId || 0}`),
  mkdir: (driveId, parentId, name) =>
    request('POST', '/files/mkdir', { drive_id: driveId, parent_id: parentId, name }),
  deleteFile: (id) => request('DELETE', `/files/${id}`),
  renameFile: (id, name) => request('PATCH', `/files/${id}`, { name }),
  downloadUrl: (id) => `${base}/files/${id}/download`,

  listDrives: () => request('GET', '/drives'),
}
```

- [ ] **Step 4: Write `web/src/routes/Login.svelte`**

```svelte
<script>
  import { token } from '../lib/stores.js'
  import { api } from '../lib/api.js'
  import { navigate } from 'svelte-routing'

  let username = ''
  let password = ''
  let error = ''
  let loading = false

  async function handleSubmit() {
    error = ''
    loading = true
    try {
      const res = await api.login(username, password)
      token.set(res.token)
      navigate('/', { replace: true })
    } catch (e) {
      error = e.message
    } finally {
      loading = false
    }
  }
</script>

<style>
  :global(body) { margin: 0; background: #0f172a; color: #e2e8f0; font-family: monospace; }
  .login-wrap { display: flex; align-items: center; justify-content: center; min-height: 100vh; }
  .login-box { background: #1e293b; border: 1px solid #334155; border-radius: 8px; padding: 32px; width: 320px; }
  h1 { color: #38bdf8; margin: 0 0 24px; font-size: 20px; }
  label { display: block; margin-bottom: 4px; color: #94a3b8; font-size: 12px; }
  input { width: 100%; box-sizing: border-box; background: #0f172a; border: 1px solid #334155;
          color: #e2e8f0; padding: 8px; border-radius: 4px; font-family: monospace; margin-bottom: 16px; }
  button { width: 100%; background: #38bdf8; color: #0f172a; border: none; padding: 10px;
           border-radius: 4px; font-weight: bold; cursor: pointer; font-family: monospace; }
  button:disabled { opacity: 0.5; cursor: default; }
  .error { color: #f87171; font-size: 12px; margin-bottom: 12px; }
</style>

<div class="login-wrap">
  <div class="login-box">
    <h1>⚡ FlashySpeed</h1>
    {#if error}<div class="error">{error}</div>{/if}
    <form on:submit|preventDefault={handleSubmit}>
      <label>Username</label>
      <input bind:value={username} autocomplete="username" />
      <label>Password</label>
      <input type="password" bind:value={password} autocomplete="current-password" />
      <button disabled={loading}>{loading ? 'Signing in...' : 'Sign In'}</button>
    </form>
  </div>
</div>
```

- [ ] **Step 5: Write `web/src/routes/Files.svelte`**

```svelte
<script>
  import { onMount } from 'svelte'
  import { token, currentDriveId, currentParentId, isLoggedIn } from '../lib/stores.js'
  import { api } from '../lib/api.js'
  import { navigate } from 'svelte-routing'

  let drives = []
  let entries = []
  let loading = true
  let error = ''
  let newFolderName = ''
  let showNewFolder = false

  $: if ($currentDriveId) loadFiles()

  onMount(async () => {
    if (!$isLoggedIn) { navigate('/login', { replace: true }); return }
    try {
      drives = await api.listDrives()
      if (drives.length > 0 && !$currentDriveId) {
        currentDriveId.set(drives[0].id)
      }
    } catch (e) { error = e.message }
    loading = false
  })

  async function loadFiles() {
    if (!$currentDriveId) return
    try {
      entries = await api.listFiles($currentDriveId, $currentParentId)
    } catch (e) { error = e.message }
  }

  async function createFolder() {
    if (!newFolderName.trim()) return
    try {
      await api.mkdir($currentDriveId, $currentParentId, newFolderName.trim())
      newFolderName = ''
      showNewFolder = false
      await loadFiles()
    } catch (e) { error = e.message }
  }

  async function deleteEntry(id) {
    if (!confirm('Move to trash?')) return
    try {
      await api.deleteFile(id)
      await loadFiles()
    } catch (e) { error = e.message }
  }

  function handleUpload(e) {
    const file = e.target.files[0]
    if (!file) return
    uploadTUS(file)
  }

  async function uploadTUS(file) {
    // Minimal TUS client — single PATCH for Phase 1 simplicity
    const createRes = await fetch('/api/tus/', {
      method: 'POST',
      headers: {
        'Authorization': `Bearer ${$token}`,
        'Upload-Length': file.size,
        'Upload-Metadata': `filename ${btoa(file.name)},drive_id ${btoa(String($currentDriveId))}`,
        'Tus-Resumable': '1.0.0',
      }
    })
    if (!createRes.ok) { error = 'Upload create failed'; return }
    const location = createRes.headers.get('Location')

    const patchRes = await fetch(location, {
      method: 'PATCH',
      headers: {
        'Authorization': `Bearer ${$token}`,
        'Content-Type': 'application/offset+octet-stream',
        'Upload-Offset': '0',
        'Tus-Resumable': '1.0.0',
      },
      body: file,
    })
    if (!patchRes.ok) { error = 'Upload failed'; return }
    await loadFiles()
  }

  function logout() {
    token.set(null)
    navigate('/login', { replace: true })
  }
</script>

<style>
  * { box-sizing: border-box; }
  :global(body) { margin: 0; background: #0f172a; color: #e2e8f0; font-family: monospace; }
  nav { background: #1e293b; border-bottom: 1px solid #334155; padding: 12px 20px;
        display: flex; align-items: center; gap: 16px; }
  nav h1 { color: #38bdf8; margin: 0; font-size: 16px; flex: 1; }
  select { background: #0f172a; color: #e2e8f0; border: 1px solid #334155;
           padding: 4px 8px; border-radius: 4px; font-family: monospace; }
  .toolbar { padding: 10px 20px; display: flex; gap: 8px; border-bottom: 1px solid #1e293b; }
  button { background: #1e293b; color: #e2e8f0; border: 1px solid #334155; padding: 6px 12px;
           border-radius: 4px; cursor: pointer; font-family: monospace; font-size: 12px; }
  button:hover { background: #334155; }
  table { width: 100%; border-collapse: collapse; }
  th { text-align: left; padding: 8px 20px; color: #64748b; font-size: 11px;
       border-bottom: 1px solid #1e293b; }
  td { padding: 8px 20px; border-bottom: 1px solid #0f172a; font-size: 13px; }
  tr:hover td { background: #1e293b; }
  .icon { margin-right: 6px; }
  .error { color: #f87171; padding: 8px 20px; font-size: 12px; }
  .actions { display: flex; gap: 6px; }
  .new-folder { display: flex; gap: 8px; padding: 8px 20px; align-items: center; }
  .new-folder input { background: #0f172a; border: 1px solid #334155; color: #e2e8f0;
                      padding: 5px 8px; border-radius: 4px; font-family: monospace; }
</style>

<nav>
  <h1>⚡ FlashySpeed</h1>
  <select bind:value={$currentDriveId} on:change={() => currentParentId.set(0)}>
    {#each drives as d}
      <option value={d.id}>{d.name}</option>
    {/each}
  </select>
  <button on:click={logout}>Logout</button>
</nav>

{#if error}<div class="error">{error}</div>{/if}

<div class="toolbar">
  <label>
    <button>⬆ Upload</button>
    <input type="file" style="display:none" on:change={handleUpload} />
  </label>
  <button on:click={() => showNewFolder = !showNewFolder}>📁 New Folder</button>
</div>

{#if showNewFolder}
<div class="new-folder">
  <input bind:value={newFolderName} placeholder="Folder name" on:keydown={e => e.key==='Enter' && createFolder()} />
  <button on:click={createFolder}>Create</button>
  <button on:click={() => showNewFolder = false}>Cancel</button>
</div>
{/if}

{#if loading}
  <p style="padding:20px;color:#64748b">Loading...</p>
{:else}
  <table>
    <thead>
      <tr>
        <th>Name</th>
        <th>Size</th>
        <th>Modified</th>
        <th></th>
      </tr>
    </thead>
    <tbody>
      {#each entries as e}
        <tr>
          <td>
            {#if e.is_dir}
              <span class="icon">📁</span>
              <span style="cursor:pointer;color:#38bdf8"
                    on:click={() => { currentParentId.set(e.id); loadFiles() }}>
                {e.name}
              </span>
            {:else}
              <span class="icon">📄</span>{e.name}
            {/if}
          </td>
          <td style="color:#64748b">{e.is_dir ? '—' : formatBytes(e.size_bytes)}</td>
          <td style="color:#64748b">{formatDate(e.updated_at)}</td>
          <td>
            <div class="actions">
              {#if !e.is_dir}
                <a href={api.downloadUrl(e.id)} download={e.name}>
                  <button>⬇</button>
                </a>
              {/if}
              <button on:click={() => deleteEntry(e.id)}>🗑</button>
            </div>
          </td>
        </tr>
      {/each}
      {#if entries.length === 0}
        <tr><td colspan="4" style="color:#64748b;text-align:center;padding:40px">Empty folder</td></tr>
      {/if}
    </tbody>
  </table>
{/if}

<script context="module">
  function formatBytes(b) {
    if (!b) return '0 B'
    const units = ['B','KB','MB','GB','TB']
    let i = 0
    while (b >= 1024 && i < units.length - 1) { b /= 1024; i++ }
    return b.toFixed(1) + ' ' + units[i]
  }
  function formatDate(d) {
    if (!d) return ''
    return new Date(d).toLocaleDateString()
  }
</script>
```

- [ ] **Step 6: Update `web/src/App.svelte`**

```svelte
<script>
  import { Router, Route } from 'svelte-routing'
  import Login from './routes/Login.svelte'
  import Files from './routes/Files.svelte'
</script>

<Router>
  <Route path="/login" component={Login} />
  <Route path="/" component={Files} />
  <Route path="/*" component={Files} />
</Router>
```

- [ ] **Step 7: Build frontend**

```bash
cd web && npm run build
```

Expected: `web/dist/` directory created with `index.html`, `assets/*.js`, `assets/*.css`. No errors.

- [ ] **Step 8: Commit**

```bash
git add web/
git commit -m "feat: Svelte frontend — login, file browser, upload, dark theme"
```

---

## Task 13: go:embed Integration

**Files:**
- Modify: `embed.go`
- Create: `cmd/flashyspeed/frontend.go`
- Modify: `cmd/flashyspeed/main.go` (remove stub serveFrontend)

- [ ] **Step 1: Verify `embed.go` is correct**

The file already exists from Task 1. Verify its contents:

```go
package main

import "embed"

//go:embed web/dist
var webDist embed.FS
```

- [ ] **Step 2: Create `cmd/flashyspeed/frontend.go`**

```go
package main

import (
	"io/fs"
	"net/http"
)

func serveFrontend() http.HandlerFunc {
	dist, err := fs.Sub(webDist, "web/dist")
	if err != nil {
		panic("embed: " + err.Error())
	}
	fsHandler := http.FileServer(http.FS(dist))

	return func(w http.ResponseWriter, r *http.Request) {
		// Try to serve the static file; if not found, serve index.html (SPA routing)
		_, err := dist.Open(r.URL.Path[1:]) // strip leading /
		if err != nil {
			// serve index.html for all unknown paths (client-side routing)
			r2 := *r
			r2.URL.Path = "/"
			fsHandler.ServeHTTP(w, &r2)
			return
		}
		fsHandler.ServeHTTP(w, r)
	}
}
```

- [ ] **Step 3: Remove stub `serveFrontend` from `main.go`**

Delete the placeholder function added in Task 11 Step 2 from `cmd/flashyspeed/main.go`. The real one is now in `frontend.go`.

- [ ] **Step 4: Build the full binary**

```bash
make build
```

Expected: `web/dist/` compiled, then `flashyspeed` binary produced. No errors.

- [ ] **Step 5: Smoke test**

```bash
export FS_JWT_SECRET="this-is-a-test-secret-at-least-32-chars"
./flashyspeed
```

Open `https://localhost:8080` in browser (accept self-signed cert warning). You should see the FlashySpeed login page. Log in with `admin` / `admin`. File browser should load.

- [ ] **Step 6: Commit**

```bash
git add embed.go cmd/flashyspeed/frontend.go cmd/flashyspeed/main.go
git commit -m "feat: embed Svelte dist into Go binary via go:embed"
```

---

## Task 14: Systemd Service & README

**Files:**
- Create: `flashyspeed.service`
- Create: `README.md`

- [ ] **Step 1: Write `flashyspeed.service`**

```ini
[Unit]
Description=FlashySpeed file server
After=network.target

[Service]
Type=simple
User=flashyspeed
ExecStart=/usr/local/bin/flashyspeed /etc/flashyspeed/config.yaml
Restart=on-failure
RestartSec=5
Environment=FS_JWT_SECRET=CHANGE_ME_TO_A_RANDOM_64_CHAR_STRING
WorkingDirectory=/var/lib/flashyspeed

[Install]
WantedBy=multi-user.target
```

- [ ] **Step 2: Write `README.md`**

```markdown
# ⚡ FlashySpeed

A fast, lean, self-hosted file server. Single binary, zero dependencies.

## Quick Start

1. Download the latest binary from [Releases](https://github.com/flashyspeed/flashyspeed/releases)
2. Copy config: `cp flashyspeed.example.yaml /etc/flashyspeed/config.yaml`
3. Set secret: `export FS_JWT_SECRET=$(openssl rand -hex 32)`
4. Run: `./flashyspeed /etc/flashyspeed/config.yaml`
5. Open: `https://localhost:8080` (accept self-signed cert)
6. Login: `admin` / `admin` — **change this password immediately**

## Install as systemd service

```bash
sudo useradd -r -s /bin/false flashyspeed
sudo mkdir -p /var/lib/flashyspeed /etc/flashyspeed
sudo cp flashyspeed /usr/local/bin/
sudo cp flashyspeed.example.yaml /etc/flashyspeed/config.yaml
sudo cp flashyspeed.service /etc/systemd/system/
# Edit the JWT secret in the service file first:
sudo nano /etc/systemd/system/flashyspeed.service
sudo systemctl enable --now flashyspeed
```

## Build from Source

Requires: Go 1.22+, Node.js 20+

```bash
git clone https://github.com/flashyspeed/flashyspeed
cd flashyspeed
make build
```

## Configuration

See `flashyspeed.example.yaml` for all options.

| Env var | Purpose |
|---------|---------|
| `FS_JWT_SECRET` | **Required.** 32+ char random string for JWT signing |
| `FS_PORT` | Override server port (default 8080) |
| `FS_DATA_DIR` | Override data directory |
```

- [ ] **Step 3: Run all tests**

```bash
go test ./...
```

Expected: All PASS.

- [ ] **Step 4: Final build check**

```bash
make build
ls -lh flashyspeed
```

Expected: binary exists, size ~15–30 MB (includes embedded frontend).

- [ ] **Step 5: Final commit**

```bash
git add flashyspeed.service README.md
git commit -m "chore: systemd service file and README quickstart"
```

---

## Phase 1 Completion Checklist

- [ ] `make build` produces a single `flashyspeed` binary
- [ ] Binary starts and serves HTTPS on localhost:8080 with self-signed cert
- [ ] Login page appears and `admin`/`admin` works
- [ ] File browser lists drives, navigates folders
- [ ] Upload a file (any size) via the upload button
- [ ] Create a new folder
- [ ] Download a file
- [ ] Delete a file and confirm it appears in trash (`GET /api/trash`)
- [ ] Kill server mid-upload, restart, verify TUS resumes (test with a large file)
- [ ] `go test ./...` passes

**Next:** Phase 2 plan covers sharing, media streaming, trash UI, and Let's Encrypt TLS.
