# ⚡ FlashySpeed

**A fast, self-hosted file server.** Single Go binary, zero runtime dependencies, SQLite metadata, resumable uploads, and a built-in web UI.

> Think of it as a lean Nextcloud: drag-and-drop uploads, public share links, inline media preview, multi-user with quotas, and Tailscale remote access — all in one executable under 20 MB.

---

## Features

| Category | What's included |
|----------|----------------|
| **Files** | Browse, upload (resumable TUS), download, rename, create folders, soft-delete, trash & restore |
| **Search** | Instant filename search across all your files |
| **Sharing** | Password-protected public links, optional expiry and download limits |
| **Media** | Inline image/video/audio preview; HTTP Range streaming for large video files |
| **Bulk actions** | Multi-select files → batch delete or ZIP download |
| **Admin panel** | User management, per-user storage quotas, per-drive usage dashboard |
| **Tailscale** | One-click install and connect from the admin UI — access your server from anywhere |
| **TLS** | Self-signed (default), Let's Encrypt (auto), or bring-your-own certificate |
| **Auth** | JWT sessions, bcrypt passwords (cost 12), role-based access (admin / user) |

---

## Quick Start (60 seconds)

### Option A — Download a pre-built binary

```bash
# 1. Download latest release for your architecture
curl -L https://github.com/Shaf2665/FlashSpeed/releases/latest/download/flashyspeed_linux_amd64.tar.gz \
  | tar xz

# 2. Generate a JWT secret
export FS_JWT_SECRET=$(openssl rand -hex 32)

# 3. Run
./flashyspeed

# 4. Open https://localhost:8080 in your browser
#    Accept the self-signed certificate warning on first visit
#    Log in with:  admin / admin
#    ⚠ Change the admin password immediately under ⚙ Admin → Users
```

### Option B — Build from source

**Requirements:** Go 1.22+, Node.js 20+, npm

```bash
git clone https://github.com/Shaf2665/FlashSpeed.git
cd FlashSpeed
make build          # builds Svelte frontend then compiles Go binary
export FS_JWT_SECRET=$(openssl rand -hex 32)
./flashyspeed
```

---

## Install as a systemd Service (Linux)

This is the recommended setup for running FlashySpeed persistently on a server.

### 1. Create a dedicated user

```bash
sudo useradd --system --shell /bin/false --home /var/lib/flashyspeed flashyspeed
sudo mkdir -p /var/lib/flashyspeed /etc/flashyspeed
```

### 2. Copy the binary and config

```bash
sudo cp flashyspeed /usr/local/bin/
sudo chmod 755 /usr/local/bin/flashyspeed

sudo cp flashyspeed.example.yaml /etc/flashyspeed/config.yaml
sudo chown -R flashyspeed:flashyspeed /var/lib/flashyspeed /etc/flashyspeed
```

### 3. Install the systemd unit

```bash
sudo cp flashyspeed.service /etc/systemd/system/
```

Open the service file and set your JWT secret:

```bash
sudo nano /etc/systemd/system/flashyspeed.service
```

Find the line:
```
Environment=FS_JWT_SECRET=CHANGE_ME_TO_A_RANDOM_64_CHAR_STRING
```
Replace the placeholder with a real secret (minimum 32 characters):
```bash
# Generate one:
openssl rand -hex 32
```

### 4. Enable and start

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now flashyspeed

# Check it's running
sudo systemctl status flashyspeed

# Follow logs
sudo journalctl -u flashyspeed -f
```

FlashySpeed is now available at `https://your-server-ip:8080`.

---

## Configuration

All settings live in a YAML file (default: no file — built-in defaults apply).

Pass the config path as the first argument:
```bash
./flashyspeed /etc/flashyspeed/config.yaml
```

**`flashyspeed.example.yaml`** — annotated reference:

```yaml
server:
  port: 8080                        # listen port
  data_dir: /var/lib/flashyspeed    # DB, TUS temp files, TLS certs

tls:
  mode: self-signed     # self-signed | auto | manual  (see TLS section)
  domain: ""            # required for mode: auto
  email: ""             # required for mode: auto
  cert_file: ""         # required for mode: manual
  key_file: ""          # required for mode: manual

storage:
  auto_detect_drives: true   # scan /proc/mounts for writable drives
  manual_paths:              # add specific directories as drives
    - /mnt/external1
    - /home/user/files

admin:
  create_default_admin: true  # creates admin/admin on first run if no admin exists
```

### Environment Variables

Environment variables override the config file — useful for secrets and Docker deployments:

| Variable | Required | Default | Purpose |
|----------|----------|---------|---------|
| `FS_JWT_SECRET` | **Yes** — min 32 chars | — | Signs JWT session tokens. Treat like a password. |
| `FS_PORT` | No | `8080` | Override the listen port |
| `FS_DATA_DIR` | No | `/var/lib/flashyspeed` | Override the data directory |

---

## TLS Configuration

### `self-signed` (default)
FlashySpeed generates an ECDSA P-256 certificate on first run and caches it in `{data_dir}/tls/`. Your browser will show a "not secure" warning — click **Advanced → Proceed** to accept it. Suitable for local network or home use.

### `auto` — Let's Encrypt
Automatically provisions a trusted certificate via ACME (no setup needed beyond a domain name).

**Requirements:**
- A public domain name pointed at your server's IP
- Port **443** open in your firewall (used for ALPN-01 challenges)

```yaml
tls:
  mode: auto
  domain: files.yourdomain.com
  email: you@yourdomain.com
```

Certificates are cached in `{data_dir}/tls/` and renew automatically.

### `manual` — Bring your own certificate
```yaml
tls:
  mode: manual
  cert_file: /etc/ssl/certs/mysite.crt
  key_file:  /etc/ssl/private/mysite.key
```

---

## Storage / Drives

FlashySpeed stores files directly on your filesystem — no proprietary format, no database blobs. Files are always accessible even without FlashySpeed running.

**Auto-detection** (`auto_detect_drives: true`) scans `/proc/mounts` and registers writable partitions as drives. Works on most Linux systems out of the box.

**Manual paths** let you add any directory as a drive:
```yaml
storage:
  auto_detect_drives: false
  manual_paths:
    - /mnt/usb
    - /home/alice/Documents
```

Admins can re-scan drives from the UI: **⚙ Admin → (drives are rescanned on startup)** or call `POST /api/drives/scan`.

---

## User Guide

### File Browser

- **Upload** — click ⬆ Upload or drag-and-drop (uses TUS for resumable uploads — safe to refresh mid-upload)
- **New Folder** — click 📁 New Folder, type a name, press Enter
- **Download** — click ⬇ on any file row
- **Delete** — click 🗑 to move to trash; restore anytime from **🗑 Trash**
- **Rename** — click ✏ on any row, type a new name, press Enter or click ✓
- **Search** — type in the search bar at the top, press Enter or click **Search**

### Bulk Actions

1. Tick the checkbox on one or more files (or the header checkbox to select all)
2. A bar appears at the top: **🗑 Delete Selected** or **⬇ ZIP Download**

### Sharing a File

1. Click **🔗 Share** on any file
2. Click **Create Share Link** — a public URL is generated
3. Copy the link and send it to anyone — no login required to download

### Media Preview

Click **▶ Preview** on any image, video, or audio file. Large videos stream via HTTP Range requests so they start playing immediately without buffering the whole file.

### Trash

Deleted files move to trash, not permanent storage. Open **🗑 Trash** in the nav to:
- **Restore** individual files
- **Delete Forever** individual files
- **Empty Trash** to wipe everything permanently

---

## Admin Panel

Click **⚙ Admin** in the nav bar (admin users only).

### Users

- **Create** — set username, email, password, role (`user` or `admin`), and optional quota
- **Edit** — change role, quota, or password
- **Delete** — removes the account (files on disk are kept)

**Quota:** Set `quota_bytes` to a positive integer (e.g. `10737418240` = 10 GB). Leave at `0` for unlimited. When a user hits their quota the next upload will be rejected with an error.

### Storage Dashboard

Shows per-drive file count and total bytes, plus per-user usage bars so you can see who is consuming the most space at a glance.

### Tailscale

Lets you access FlashySpeed from any of your Tailscale devices — phone, laptop, remote server — without opening a port to the internet.

1. Click **⬇ Install Tailscale** (runs the official install script — Linux only)
2. Get an auth key from [tailscale.com/admin/settings/keys](https://login.tailscale.com/admin/settings/keys)
3. Paste the key → click **Connect**
4. Your server's Tailscale IP appears in the status line. Use it in place of `localhost`.

---

## Build from Source (detailed)

```bash
# Clone
git clone https://github.com/Shaf2665/FlashSpeed.git
cd FlashSpeed

# Install frontend dependencies
cd web && npm install && cd ..

# Build frontend (compiled into Go binary via go:embed)
cd web && npm run build && cd ..

# Build Go binary
go build -o flashyspeed ./cmd/flashyspeed

# Run tests
go test ./...
```

The resulting `flashyspeed` binary is fully self-contained — the web UI is embedded inside it.

---

## Creating a Release

```bash
# Install goreleaser
go install github.com/goreleaser/goreleaser/v2@latest

# Tag
git tag v1.0.0
git push origin v1.0.0

# Build and publish (GITHUB_TOKEN must have 'contents: write' permission)
export GITHUB_TOKEN=ghp_your_token_here
goreleaser release --clean
```

This produces `linux/amd64` and `linux/arm64` tarballs uploaded automatically to GitHub Releases.

---

## Troubleshooting

| Symptom | Fix |
|---------|-----|
| Browser shows "connection refused" | Check `systemctl status flashyspeed` and that the port is open in your firewall |
| Self-signed cert warning in browser | Click **Advanced → Proceed to site** — this is expected for `self-signed` mode |
| "FS_JWT_SECRET env var must be at least 32 bytes" | Set a longer secret: `export FS_JWT_SECRET=$(openssl rand -hex 32)` |
| Upload fails with "quota exceeded" | Admin panel → Users → increase or remove that user's quota |
| Let's Encrypt cert not issuing | Ensure port 443 is publicly reachable and the domain's DNS points to the server |
| Files missing after restore | Check that the drive is still mounted; run a drive re-scan from the admin panel |
| Can't log in after changing JWT secret | All existing sessions are invalidated — log in fresh with username/password |

---

## License

MIT — see [LICENSE](LICENSE).

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for how to build from source and submit changes.
