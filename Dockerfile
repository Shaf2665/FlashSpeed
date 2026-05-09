# ─────────────────────────────────────────────
# Stage 1: Build the Svelte frontend
# ─────────────────────────────────────────────
FROM node:20-alpine AS frontend

WORKDIR /build/web

# Copy only dependency manifests first — better layer caching
COPY web/package.json web/package-lock.json ./

RUN npm ci

# Copy the rest of the frontend source and build
COPY web/ ./

RUN npm run build

# ─────────────────────────────────────────────
# Stage 2: Build the Go binary
# ─────────────────────────────────────────────
FROM golang:1.25-alpine AS builder

WORKDIR /build

# Copy Go dependency manifests first — better layer caching
COPY go.mod go.sum ./

RUN go mod download

# Copy source (web/dist excluded by .dockerignore)
COPY . .

# Copy the built frontend from stage 1 into the embed path
COPY --from=frontend /build/web/dist ./web/dist

# CGO_ENABLED=0 is safe — modernc.org/sqlite is pure Go
# -ldflags="-s -w" strips debug symbols for a smaller binary
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /flashyspeed ./cmd/flashyspeed

# ─────────────────────────────────────────────
# Stage 3: Minimal runtime image
# ─────────────────────────────────────────────
FROM alpine:3.19

# ca-certificates — needed for HTTPS outbound (e.g. Let's Encrypt ACME renewal)
# tzdata         — needed for correct time zone handling in JWT / share expiry
RUN apk add --no-cache ca-certificates tzdata

# Copy the compiled binary
COPY --from=builder /flashyspeed /usr/local/bin/flashyspeed

# Copy the Docker-specific config file into the image
COPY config.docker.yaml /etc/flashyspeed/config.yaml

# /data  — database, TLS certs, tus upload temp files (use a named volume)
# /files — user file storage (use a bind mount so files are visible on the host)
VOLUME ["/data"]

EXPOSE 8080

# argv[1] must be the config path — without it auto_detect_drives defaults to
# true, /proc/mounts is scanned, and no drives are registered in the container.
CMD ["/usr/local/bin/flashyspeed", "/etc/flashyspeed/config.yaml"]
