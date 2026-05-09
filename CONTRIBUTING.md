# Contributing to FlashySpeed

Thank you for your interest in contributing to FlashySpeed!

## Building from Source

### Prerequisites

- Go 1.21+
- Node.js 18+ and npm

### Build Steps

```bash
# 1. Install frontend dependencies
cd web && npm install && cd ..

# 2. Build frontend (compiled output gets embedded into the Go binary)
cd web && npm run build && cd ..

# 3. Build Go binary
go build -o flashyspeed ./cmd/flashyspeed

# Or use the Makefile shorthand:
make build
```

### Running Tests

```bash
go test ./...
```

### Running Locally

```bash
export FS_JWT_SECRET="your-secret-at-least-32-bytes-long"
./flashyspeed
# Open https://localhost:8080 (accept the self-signed certificate warning)
# Default login: admin / admin
```

## Project Structure

```
flashyspeed/
├── cmd/flashyspeed/        # Server entry point
├── internal/
│   ├── admin/              # Admin API (user CRUD, storage dashboard, Tailscale)
│   ├── auth/               # JWT + bcrypt authentication
│   ├── config/             # YAML config + env var overrides
│   ├── db/                 # SQLite (WAL mode) + migrations
│   ├── drives/             # Drive scanner (/proc/mounts + manual paths)
│   ├── files/              # File CRUD (list, mkdir, download, trash, search)
│   ├── media/              # HTTP Range streaming (video/audio/image)
│   ├── shares/             # Public link + user-to-user sharing
│   ├── tlsmgr/             # TLS: self-signed, manual cert, Let's Encrypt
│   └── tus/                # TUS 1.0.0 resumable upload handler
├── web/                    # Svelte frontend (compiled into binary via go:embed)
├── embed.go                # go:embed directive
├── Makefile
└── flashyspeed.example.yaml
```

## Design Principles

- **Single binary** — no runtime dependencies, everything embedded
- **Ownership gates** — every data operation verifies `user_id` in SQL
- **Soft delete** — files go to trash before permanent deletion
- **Service/handler split** — each package has `service.go` (logic) and `handler.go` (HTTP)

## Submitting Changes

1. Fork the repository
2. Create a feature branch: `git checkout -b feat/my-feature`
3. Write tests for new functionality
4. Ensure `go test ./...` and `go build ./...` pass
5. Open a pull request with a clear description of the change
