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
	"github.com/flashyspeed/flashyspeed/internal/media"
	"github.com/flashyspeed/flashyspeed/internal/shares"
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
		if err := scanner.Sync(drives.ScanSystem()); err != nil {
			log.Printf("drive sync failed: %v", err)
		}
	} else {
		if err := scanner.Sync(nil); err != nil {
			log.Printf("drive sync failed: %v", err)
		}
	}

	// handlers
	authHandler := auth.NewHandler(database, jwtSecret)
	driveHandler := drives.NewHandler(database, scanner)
	fileSvc := files.NewService(database)
	fileHandler := files.NewHandler(database, fileSvc)
	tusHandler := tus.NewHandler(database, cfg.Server.DataDir+"/tus-tmp")
	shareHandler := shares.NewHandler(database, jwtSecret)
	mediaHandler := media.NewHandler(database, fileSvc)

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
		r.Get("/api/files/{id}/stream", mediaHandler.Stream)
		r.Head("/api/files/{id}/stream", mediaHandler.Stream)

		r.Post("/api/tus/", tusHandler.Create)
		r.Head("/api/tus/{id}", tusHandler.Head)
		r.Patch("/api/tus/{id}", tusHandler.Upload)

		r.Get("/api/drives", driveHandler.List)
		r.Post("/api/drives/scan", driveHandler.TriggerScan)

		r.Get("/api/shares", shareHandler.List)
		r.Post("/api/shares", shareHandler.Create)
		r.Delete("/api/shares/{id}", shareHandler.Delete)
	})

	// Public share endpoints — no auth required, must be before SPA catch-all.
	r.Get("/api/s/{token}", shareHandler.Resolve)
	r.Get("/api/s/{token}/download", shareHandler.Download)

	// serve embedded Svelte SPA (wired in Task 13)
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
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("shutdown: %v", err)
	}
	log.Println("FlashySpeed stopped.")
}

func seedAdmin(database *db.DB) {
	var count int
	if err := database.QueryRow(`SELECT COUNT(*) FROM users WHERE role='admin'`).Scan(&count); err != nil {
		log.Printf("seedAdmin: count query failed: %v", err)
		return
	}
	if count > 0 {
		return
	}
	hash, err := auth.HashPassword("admin")
	if err != nil {
		log.Printf("seedAdmin: hash failed: %v", err)
		return
	}
	if _, err := database.Exec(
		`INSERT INTO users(username,email,password_hash,role) VALUES('admin','admin@localhost',?,'admin')`,
		hash,
	); err != nil {
		log.Printf("seedAdmin: insert failed: %v", err)
		return
	}
	log.Println("Created default admin user: admin / admin — change the password immediately!")
}

