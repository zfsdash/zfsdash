package main

import (
	"context"
	"embed"
	"flag"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/zfsdash/zfsdash/internal/auth"
	"github.com/zfsdash/zfsdash/internal/config"
	"github.com/zfsdash/zfsdash/internal/db"
	"github.com/zfsdash/zfsdash/internal/web"
)

//go:embed internal/web/static/*
var staticFiles embed.FS

var version = "dev"

func main() {
	if err := run(); err != nil {
		slog.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run() error {
	var (
		addr     = flag.String("addr", ":8080", "listen address")
		dataDir  = flag.String("data", "/var/lib/zfsdash", "data directory for SQLite DB")
		printVer = flag.Bool("version", false, "print version and exit")
	)
	flag.Parse()

	if *printVer {
		fmt.Printf("zfsdash %s\n", version)
		return nil
	}

	// Home dir fallback for non-root users
	if os.Getuid() != 0 {
		home, _ := os.UserHomeDir()
		if home != "" {
			*dataDir = home + "/.zfsdash"
		}
	}

	if err := os.MkdirAll(*dataDir, 0750); err != nil {
		return fmt.Errorf("create data dir: %w", err)
	}

	// Initialize SQLite database
	store, err := db.New(*dataDir + "/zfsdash.db")
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	defer store.Close()

	if err := store.Migrate(); err != nil {
		return fmt.Errorf("migrate db: %w", err)
	}

	// Auth service
	authSvc := auth.New(store)

	// Load config
	cfg := config.New(*dataDir)

	// Build static file server from embedded FS
	staticFS, err := fs.Sub(staticFiles, "internal/web/static")
	if err != nil {
		return fmt.Errorf("static fs: %w", err)
	}

	// Build router
	r := chi.NewRouter()
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Compress(5))

	// Register all routes
	web.RegisterRoutes(r, store, authSvc, cfg, staticFS)

	srv := &http.Server{
		Addr:         *addr,
		Handler:      r,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		slog.Info("ZFSdash starting", "addr", *addr, "data", *dataDir, "version", version)
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)

	select {
	case err := <-errCh:
		return err
	case sig := <-quit:
		slog.Info("shutting down", "signal", sig)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return srv.Shutdown(ctx)
}
