# FlashySpeed — Project Handover

*Updated: 2026-05-09 — covers all work through commit `5e63788` (Phase 3 complete)*

---

## Phase 1 — ✅ 100% Complete

All 14 Phase 1 tasks implemented, reviewed (spec + quality), and committed.

### Delivered Features

| # | Feature | Key Files |
|---|---------|-----------|
| 1 | Project scaffold, Go module, Makefile | `go.mod`, `Makefile`, directory layout |
| 2 | SQLite (WAL mode, migrations v2) | `internal/db/db.go`, `internal/db/migrations.go` |
| 3 | YAML config + env var overrides | `internal/config/config.go` |
| 4 | JWT auth (HS256, bcrypt cost=12) | `internal/auth/auth.go`, `handler.go`, `middleware.go` |
| 5 | Drive scanner (`/proc/mounts` + manual) | `internal/drives/scanner.go`, `handler.go` |
| 6 | File API (list, mkdir, download, soft-delete, rename) | `internal/files/service.go`, `handler.go` |
| 7 | TUS resumable upload handler | `internal/tus/handler.go` |
| 8 | Self-signed TLS (ECDSA P-256, cached) | `internal/tlsmgr/manager.go` |
| 9 | Svelte SPA (login, file browser, upload, dark theme) | `web/src/routes/Login.svelte`, `Files.svelte` |
| 10 | systemd service + README | `flashyspeed.service`, `README.md` |
| 11 | Server bootstrap (chi router, graceful shutdown) | `cmd/flashyspeed/main.go`, `frontend.go` |
| 12 | Security hardening (path traversal, LIKE injection, atomic rename) | `internal/files/service.go`, `internal/tus/handler.go` |
| 13 | Embedded frontend via `go:embed` | `embed.go` |
| 14 | Admin-only drive scan, unlimited DB connections | `internal/drives/handler.go`, `internal/db/db.go` |

**Binary delivery:** `go build ./...` → single `flashyspeed` binary, zero runtime deps.

---

## Phase 2 — ✅ 100% Complete

### ✅ P2-1: Shares API — COMPLETE (commit `c7a267b` → `58c8684`)

**Files:** `internal/shares/service.go`, `internal/shares/handler.go`, `internal/shares/shares_test.go`

Tokenized public-link + user-to-user file sharing with:
- UUID share tokens, bcrypt password protection, expiry, max-download limiting
- Atomic `download_count` increment (conditional UPDATE prevents TOCTOU race)
- File ownership verified before share creation
- `target_user_id` enforcement: restricted shares require matching Bearer token
- Password via `X-Share-Password` header (not query string)
- Sentinel errors: `ErrWrongPassword`, `ErrShareExpired`, `ErrNotAuthorized`, `ErrNotOwned`
- Index on `shares.owner_id` (migration v2)
- 8 tests covering all error paths including handler-level HTTP status codes

**Routes added to `main.go`:**
```
GET/POST  /api/shares              (auth required)
DELETE    /api/shares/{id}         (auth required, owner only)
GET       /api/s/{token}           (public)
GET       /api/s/{token}/download  (public, streams file)
```

---

### ✅ P2-2: Public Share Page — COMPLETE (commits `1ac8305`, `bb2511f`, `d30892e`)

**Files:** `web/src/routes/Share.svelte` (new), `web/src/App.svelte` (modified)

Public Svelte page at `/s/:token`:
- Handles 200/401/410/404 backend responses
- Password gate with `X-Share-Password` header; `submitting` state prevents form flash on retry
- Download button → `/api/s/{token}/download` (no auth required)
- Directory files: no download button shown
- `size_bytes` exposed in Resolve JSON response
- MIME-type icons (🎬 video, 🎵 audio, 🖼 image, etc.)
- Dark theme consistent with `Login.svelte`

---

### ✅ P2-3: Media Streaming — COMPLETE (commits `774c98b`, `aeb8788`)

**Files:** `internal/media/handler.go` (new), `internal/media/handler_test.go` (new)

HTTP Range-request streaming via `GET /api/files/{id}/stream`:
- `http.ServeContent` handles 206, Range, ETag, Last-Modified, 304 automatically
- MIME detection: DB-stored value first; fallback `http.DetectContentType` on 512-byte sniff + seek-back
- Ownership verification before any disk access
- Directory streaming rejected (400)
- HEAD method also registered (required by browser `<video>` elements)
- 7 tests: ownership, Range prefix/open-ended/unsatisfiable (416), MIME sniff, directory, bad ID

**Routes added to `main.go`:**
```
GET  /api/files/{id}/stream   (auth required)
HEAD /api/files/{id}/stream   (auth required)
```

---

### ✅ P2-4: Frontend Phase 2 — COMPLETE (commit `1996bb2`)

**Files:** `web/src/routes/Files.svelte`, `web/src/lib/api.js`

- Share dialog: 🔗 Share button per file → creates public link → copy-to-clipboard
- Media preview modal: ▶ Preview on image/video/audio → fetches with auth header → Blob URL → `<img>` / `<video>` / `<audio>`
- `URL.revokeObjectURL` called on close and `onDestroy`
- Trash nav button in nav bar → `/trash`
- `api.js` additions: `createShare`, `listShares`, `deleteShare`

---

### ✅ P2-5: Trash API + UI — COMPLETE (CTO-verified 2026-05-04)

**Files:** `internal/files/service.go`, `internal/files/handler.go`, `internal/files/files_test.go`, `cmd/flashyspeed/main.go`, `web/src/routes/Trash.svelte`, `web/src/lib/api.js`, `web/src/App.svelte`, `web/src/routes/Files.svelte` (Trash nav)

Trash list, restore, permanent delete, and empty-trash (authenticated):
- `TrashList` / `Restore` / `PermanentDelete` / `EmptyTrash` handlers; service includes `EmptyTrash` iterating trashed rows
- Routes (auth group): `GET /api/trash`, `DELETE /api/trash`, `POST /api/trash/{id}/restore`, `DELETE /api/trash/{id}`
- `Trash.svelte`: rows with restore + delete forever + empty trash; dark theme; wired in `App.svelte` as `/trash`
- API client: `listTrash`, `restoreFile`, `permanentDelete`, `emptyTrash`
- Tests: `TestRestore`, `TestPermanentDelete`, `TestEmptyTrash`, plus trash listing after soft-delete (see `internal/files/files_test.go`)

**Verification:** `go test ./...` and `cd web && npm run build` pass on reviewed workspace.

---

### ✅ P2-6: Let's Encrypt TLS — IMPLEMENTED (CTO, 2026-05-04; commit pending)

**Goal:** When `tls.mode = auto` in config, use `golang.org/x/crypto/acme/autocert`; `manual` loads PEM paths; default remains self-signed.

**Delivered:** `internal/tlsmgr/manager.go` — `AutoCert`, `Manual` (with required-field validation); `cmd/flashyspeed/main.go` — mode switch using `filepath.Join` for `{data_dir}/tls`; `README.md` TLS modes table; `flashyspeed.example.yaml` comments. `go test ./...` clean.

**Operational:** ACME needs public DNS for `tls.domain` and reachability on **:443** for the hostname. Staging smoke tracked on **NEH-27** (Developer).

**Config:** `internal/config/config.go` TLS fields unchanged.

---

## Phase 3 — ✅ 100% Complete

All 7 Phase 3 tasks implemented and pushed to `main` on GitHub (`https://github.com/Shaf2665/FlashSpeed`).

### ✅ P3-1: Tailscale Integration — COMPLETE (commit `0d91274`)

**Files:** `internal/admin/tailscale.go` (new), `internal/admin/handler.go` (new)

Admin-only endpoints to manage Tailscale from the web UI:
- `TailscaleStatusCheck()` — runs `tailscale status --json`; returns `{Running: false}` gracefully if not installed
- `TailscaleInstall()` — runs `curl -fsSL https://tailscale.com/install.sh | sh`
- `TailscaleUp(authKey)` — runs `tailscale up --authkey=<key>`

Routes (admin-only):
```
GET  /api/admin/tailscale/status
POST /api/admin/tailscale/install
POST /api/admin/tailscale/up
```

### P3-2: Admin Panel — User Management (`internal/admin/`)

CRUD for users, quota management.

Routes (admin-only):
```
GET    /api/admin/users
POST   /api/admin/users
PATCH  /api/admin/users/{id}
DELETE /api/admin/users/{id}
```

Quota enforcement added to `internal/tus/handler.go` `finalize()`: checks `SUM(size_bytes) + upload_size <= quota_bytes` (0 = unlimited) before moving file into place.

Routes added:
```
GET    /api/admin/users
POST   /api/admin/users
PATCH  /api/admin/users/{id}
DELETE /api/admin/users/{id}
```

### ✅ P3-3: Storage Dashboard — COMPLETE (commit `7be95a2`)

**File:** `internal/admin/handler.go` — `StorageDashboard` handler

`GET /api/admin/storage` returns:
```json
{
  "drives": [{"drive_id":1,"drive_name":"...","total_files":42,"total_bytes":102400}],
  "users":  [{"user_id":1,"username":"alice","quota_bytes":0,"used_bytes":51200}]
}
```
Frontend (`Admin.svelte`): drive table + per-user quota progress bars with colour coding (blue → amber at 75% → red at 90%).

### ✅ P3-4: Bulk Operations — COMPLETE (commit `7be95a2`)

**File:** `internal/files/handler.go` — `BulkDelete`, `ZipDownload`

- `DELETE /api/files` body `{"ids":[1,2,3]}` — soft-deletes selected files, returns 207 with `failed_ids` if any fail
- `POST /api/files/zip` body `{"ids":[1,2,3]}` — streams a ZIP archive; skips directories and non-owned files
- Frontend: checkbox column in file table; bulk action bar appears when ≥1 file is selected

### ✅ P3-5: Search — COMPLETE (commit `7be95a2`)

**Files:** `internal/files/service.go` — `Search()`, `internal/files/handler.go` — `Search` handler

`GET /api/files/search?q=<term>` — case-insensitive LIKE search across all user's live files (LIMIT 200). LIKE wildcards escaped. Frontend: search bar at top of Files page; shows inline result count; **Clear** button returns to normal directory listing.

### ✅ P3-6: Frontend Phase 3 — COMPLETE (commit `1af1a19`)

**New file:** `web/src/routes/Admin.svelte`

- **Tailscale wizard** — status dot (green/grey), Install button, auth key input + Connect
- **Storage dashboard** — drive table + animated per-user quota bars
- **User CRUD** — create user form; per-row Edit modal (role, quota, password); delete with confirmation
- **Files.svelte additions** — search bar (Enter or button), checkbox column for bulk selection, bulk action bar (Delete / ZIP), inline rename (✏ button + input), breadcrumb path bar
- **App.svelte** — `/admin` route added; `Admin` component imported

### ✅ P3-7: GitHub OSS Setup — COMPLETE (commit `5e63788`)

- `LICENSE` — MIT
- `CONTRIBUTING.md` — build-from-source guide, package tour, PR process
- `.github/workflows/ci.yml` — Node + Go matrix; runs `npm ci`, `npm run build`, `go test ./...`, `go build`, `go vet`
- `.goreleaser.yml` — `linux/amd64` + `linux/arm64` tarballs, sha256 checksums, draft GitHub Release

---

## Technical Context

### Stack

| Layer | Technology | Notes |
|-------|-----------|-------|
| Language | Go 1.25.5 | Single binary, no CGO |
| Router | `go-chi/chi v5` | Lightweight, idiomatic |
| Database | SQLite (WAL mode) | `modernc.org/sqlite` — pure Go, no CGO |
| Auth | JWT HS256 + bcrypt | `golang-jwt/jwt/v5`, `golang.org/x/crypto` |
| Upload | TUS 1.0.0 | Resumable, chunked |
| Streaming | `http.ServeContent` | Range, ETag, 206 handled automatically |
| TLS | Self-signed ECDSA P-256 | Cached in `{DataDir}/tls/` |
| Frontend | Svelte 4 + Vite 5 | `go:embed web/dist` at compile time |
| Routing (SPA) | `svelte-routing` 2.x | `let:params` slot pattern for dynamic segments |
| State | Svelte stores | `token`, `currentDriveId`, `currentParentId` |
| UUID | `github.com/google/uuid` | Used for share tokens and TUS upload IDs |

### Design Patterns Used

**1. Package layout:** Each feature lives in `internal/<name>/` with two files:
- `service.go` — business logic, zero HTTP knowledge
- `handler.go` — HTTP adapter, zero business logic

**2. Ownership gates:** Every data operation verifies `user_id = claims.UserID` in SQL before acting:
```sql
WHERE id=? AND user_id=? AND deleted_at IS NULL
```

**3. Path containment check (path traversal defense):**
```go
mountClean := filepath.Clean(mountPath) + string(os.PathSeparator)
if !strings.HasPrefix(filepath.Clean(absPath)+string(os.PathSeparator), mountClean) {
    return fmt.Errorf("name escapes drive root")
}
```
Applied in: `files.Mkdir`, `files.Rename`, `tus.finalize`.

**4. Soft delete:** `deleted_at DATETIME` — NULL = live, set = trashed. Files are never hard-deleted until the user empties trash.

**5. Atomic DB operations:** Shared-resource updates use a conditional UPDATE + `RowsAffected` check instead of read-check-write to prevent TOCTOU races:
```go
res, _ := tx.Exec(`UPDATE shares SET download_count = download_count + 1
    WHERE id = ? AND (max_downloads IS NULL OR download_count < max_downloads)`, token)
if n, _ := res.RowsAffected(); n == 0 { return ErrShareExpired }
```

**6. LIKE wildcard escaping (SQLite):**
```go
escaped := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(prefix)
// query uses: LIKE ? ESCAPE '\'
```
Used in `files.Rename` when cascading `rel_path` updates to descendants.

**7. Step-based migrations:**
```go
if version < 1 { /* apply full schema */ ; version = 1 }
if version < 2 { /* CREATE INDEX ... */ ; version = 2 }
db.Exec(`INSERT OR REPLACE INTO schema_version(version) VALUES(?)`, version)
```
Current version: **2**. Increment and add a step for every schema change.

**8. Blob URL pattern (frontend auth for binary resources):**
```js
const res = await fetch(`/api/files/${id}/stream`, {
  headers: { 'Authorization': `Bearer ${$token}` }
})
const blob = await res.blob()
mediaSrc = URL.createObjectURL(blob)
// ALWAYS call on close: URL.revokeObjectURL(mediaSrc)
```
Used because `<video src="...">` can't send custom headers — instead fetch with auth, create an object URL, pass that to the element. Revoke on close and `onDestroy`.

**9. `embed.go` in root package:** Root package is `package flashyspeed` (not `package main`) so `go:embed web/dist` works — embed paths cannot escape the source file's directory. `cmd/flashyspeed/main.go` imports the root package to get `WebDist embed.FS`.

**10. Content-Disposition filename safety:**
```go
import "mime"
disposition := mime.FormatMediaType("attachment", map[string]string{"filename": name})
w.Header().Set("Content-Disposition", disposition)
```
Used everywhere a file download is served to handle quotes and special chars in filenames.

### Critical File Map

| Concern | File(s) |
|---------|---------|
| Server entry, all routes | `cmd/flashyspeed/main.go` |
| DB schema + migrations | `internal/db/migrations.go` |
| Auth (JWT + bcrypt) | `internal/auth/auth.go`, `middleware.go` |
| File CRUD + path logic | `internal/files/service.go`, `handler.go` |
| TUS upload | `internal/tus/handler.go` |
| Shares | `internal/shares/service.go`, `handler.go` |
| Media streaming | `internal/media/handler.go` |
| Drive scanner | `internal/drives/scanner.go` |
| TLS (self-signed) | `internal/tlsmgr/manager.go` |
| Config loading | `internal/config/config.go` |
| Frontend SPA router | `web/src/App.svelte` |
| File browser (main UI) | `web/src/routes/Files.svelte` |
| Public share page | `web/src/routes/Share.svelte` |
| API client | `web/src/lib/api.js` |
| Svelte state stores | `web/src/lib/stores.js` |
| Binary embedding | `embed.go` |

### Build & Run

```bash
# Build (requires Node ≥18 + Go ≥1.21)
cd web && npm install && npm run build && cd ..
go build -o flashyspeed ./cmd/flashyspeed

# Run
export FS_JWT_SECRET="your-32-byte-minimum-secret-here"
./flashyspeed
# Server: https://localhost:8080 (self-signed cert — accept browser warning)
# Default credentials: admin / admin  ← change immediately

# Tests
go test ./...
```

### Environment Variables

| Variable | Default | Purpose |
|----------|---------|---------|
| `FS_JWT_SECRET` | *(required, min 32 bytes)* | JWT HMAC signing secret |
| `FS_PORT` | `8080` | Listen port |
| `FS_DATA_DIR` | `/var/lib/flashyspeed` | DB file, TUS temp dir, TLS certs |

---

*End of handover document.*
