# ⚡ FlashySpeed

A fast, lean, self-hosted file server. Single binary, zero runtime dependencies.

## Quick Start

1. Download the latest binary from [Releases](https://github.com/flashyspeed/flashyspeed/releases)
2. Copy config: `cp flashyspeed.example.yaml /etc/flashyspeed/config.yaml`
3. Set secret: `export FS_JWT_SECRET=$(openssl rand -hex 32)`
4. Run: `./flashyspeed /etc/flashyspeed/config.yaml`
5. Open: `https://localhost:8080` (accept self-signed cert warning)
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
| `FS_PORT` | Override server port (default: 8080) |
| `FS_DATA_DIR` | Override data directory |

## TLS modes (`tls.mode` in config)

| Mode | Behavior |
|------|----------|
| `self-signed` (default) | Local ECDSA cert under `{data_dir}/tls/` — fine for LAN or first boot. |
| `auto` | Let's Encrypt via ACME (`golang.org/x/crypto/acme/autocert`). Requires **`tls.domain`** (public DNS pointing at this host) and **`tls.email`**. Cache lives under `{data_dir}/tls/`. The process must be reachable on **port 443** for the public hostname so HTTP-01 / TLS-ALPN-01 challenges succeed. No extra unit tests in-repo (needs real ACME). |
| `manual` | Load **`tls.cert_file`** and **`tls.key_file`** (PEM) with `tls.LoadX509KeyPair`. |
