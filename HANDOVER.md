# FlashySpeed — Project Handover

*Generated: 2026-05-03 — covers all work through commit `aeb8788`*

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

## Phase 2 — 🔄 In Progress

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

### ⚠️ P2-4: Frontend Phase 2 — CODE WRITTEN, NOT COMMITTED

**Status:** The P2-4 implementer wrote all code but hit a rate limit before committing. The changes exist as **uncommitted modifications** on disk.

**Modified files (not yet committed):**
- `web/src/routes/Files.svelte` — ~140 lines added
- `web/src/lib/api.js` — 3 methods added

**What was written:**

`web/src/lib/api.js` additions:
```js
createShare: (fileId) => request('POST', '/shares', { file_id: fileId }),
listShares:  ()       => request('GET', '/shares'),
deleteShare: (id)     => request('DELETE', `/shares/${id}`),
```

`web/src/routes/Files.svelte` additions:
- **Share dialog state:** `shareEntry`, `shareUrl`, `shareError`, `shareLoading`, `shareCopied`
- **Share functions:** `openShareDialog(entry)`, `closeShareDialog()`, `handleShareBackdropClick()`, `createShare()`, `copyShareUrl()`
- **Preview state:** `previewEntry`, `previewBlobUrl`
- **Preview functions:** `openPreview(entry)`, `closePreview()`, `handlePreviewBackdropClick()`, `isPreviewable(entry)`, `revokePreviewBlob()`
- **`onDestroy`** hook to revoke blob URLs on component unmount
- **Share button** (🔗) on each non-directory file row
- **Preview button** (▶) on previewable files (image/*, video/*, audio/*)
- **Trash nav button** (🗑 Trash) in nav bar → navigates to `/trash`
- **Share modal:** creates share, shows URL, copy-to-clipboard
- **Preview modal:** `<img>` for images, `<video controls>` for video, `<audio controls>` for audio — all via Blob URLs fetched with auth header
- CSS for both modals (dark theme, fixed overlay, centered card)

**Next action required:** Run `cd web && npm run build`, then run spec + quality review on the uncommitted changes and commit.

**Review checklist for P2-4:**
- [ ] `npm run build` passes clean
- [ ] Share dialog: clicking 🔗 calls `api.createShare(id)`, shows URL, copy works
- [ ] Preview: `isPreviewable` checks mime starts with `image/`, `video/`, `audio/`
- [ ] Blob URLs fetched with `Authorization: Bearer` header
- [ ] `URL.revokeObjectURL` called on modal close AND `onDestroy`
- [ ] Trash button navigates to `/trash`
- [ ] No regressions to upload, folder creation, download, delete

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

## Phase 3 — Full Plan

### P3-1: Tailscale Integration (`internal/admin/tailscale.go`)

Detect/install/configure Tailscale from the admin panel:
- `Status() (TailscaleStatus, error)` — check if `tailscaled` is running, get Tailscale IP
- `Install() error` — run `curl -fsSL https://tailscale.com/install.sh | sh`
- `Up(authKey string) error` — run `tailscale up --authkey=<key>`

Routes (admin-only, check `claims.Role == "admin"`):
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

Quota enforcement: in `files/service.go` before writing a file, check `SUM(size_bytes) + new_size <= quota_bytes` (0 = unlimited).

### P3-3: Storage Dashboard

`GET /api/admin/storage` — per-drive usage stats (total files, total bytes, per-user breakdown).
Frontend: bar charts showing drive usage and per-user quota utilization.

### P3-4: Bulk Operations

Multi-select files in `Files.svelte` (checkbox column), bulk delete, zip download.
New backend: `POST /api/files/zip` — streams a ZIP archive of selected file IDs.

### P3-5: Search

`GET /api/files/search?q=<term>` — filename `LIKE` search within user's files.
Frontend: search bar in nav, results list.

### P3-6: Frontend Phase 3

- `web/src/routes/Admin.svelte` — user CRUD table, quota progress bars, drive usage
- Tailscale wizard: status card, Install button, auth key input, Up button
- Bulk selection UI in `Files.svelte` — checkbox per row, bulk action toolbar
- Search bar in nav + results component

### P3-7: GitHub Open Source Setup

- `LICENSE` (MIT)
- `CONTRIBUTING.md`
- `.github/workflows/ci.yml` — `go test ./...` + `go build ./...` on every PR
- `goreleaser.yml` — pre-built binaries for `linux/amd64` and `linux/arm64`

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
