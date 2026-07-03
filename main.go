package main

import (
	"context"
	"embed"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/zfsdash/zfsdash/internal/alerts"
	"github.com/zfsdash/zfsdash/internal/config"
	"github.com/zfsdash/zfsdash/internal/db"
	"github.com/zfsdash/zfsdash/internal/store"
	"github.com/zfsdash/zfsdash/internal/web"
	"github.com/zfsdash/zfsdash/internal/zfs"
)

//go:embed internal/web/static
var staticFiles embed.FS

func main() {
	configPath := flag.String("config", "", "Path to config file (optional)")
	flag.Parse()

	var cfg *config.Config
	var err error
	if *configPath != "" {
		cfg, err = config.Load(*configPath)
		if err != nil {
			log.Fatalf("Failed to load config: %v", err)
		}
	} else {
		cfg = config.DefaultConfig()
	}

	// Open database
	var database *db.DB
	if err := os.MkdirAll("/var/lib/zfsdash", 0755); err == nil {
		database, err = db.Open(cfg.DB.Path)
		if err != nil {
			log.Printf("Warning: could not open database: %v (history disabled)", err)
			database = nil
		}
	} else {
		log.Printf("Warning: could not create data dir: %v (history disabled)", err)
	}

	// Build collectors
	collectors := make(map[string]zfs.Collector)
	for _, hcfg := range cfg.Hosts {
		col, err := newCollector(hcfg)
		if err != nil {
			log.Printf("Warning: failed to create collector for host %s: %v", hcfg.Hostname, err)
			continue
		}
		collectors[hcfg.Hostname] = col
		log.Printf("Registered host: %s (mode: %s)", hcfg.Hostname, hcfg.Mode)
	}

	// Store and alert engine
	s := store.New()
	ae := alerts.New(&cfg.Alerts)

	// Pre-populate store with known hosts
	for name := range collectors {
		s.SetHostData(name, &store.HostData{
			Datasets:  make(map[string][]*zfs.Dataset),
			Snapshots: make(map[string][]*zfs.Snapshot),
		})
	}

	// HTTP handler
	h := web.New(s, database, ae, collectors, cfg.Server.APIKey)
	r := h.Router()

	// Serve embedded static files at root
	staticFS, err := fs.Sub(staticFiles, "internal/web/static")
	if err != nil {
		log.Fatalf("Failed to create static FS: %v", err)
	}
	fileServer := http.FileServer(http.FS(staticFS))

	mux := http.NewServeMux()
	mux.Handle("/api/", r)
	mux.Handle("/", fileServer)

	srv := &http.Server{
		Addr:         cfg.Server.Listen,
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Background collection loop
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		// Run immediately on start
		collectAll(collectors, s, database, ae)
		for range ticker.C {
			collectAll(collectors, s, database, ae)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	log.Printf("ZFSdash listening on %s", cfg.Server.Listen)
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	<-quit
	log.Println("Shutting down...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("Shutdown error: %v", err)
	}
	for _, col := range collectors {
		col.Close()
	}
	if database != nil {
		database.Close()
	}
	log.Println("ZFSdash stopped.")
}

func newCollector(cfg zfs.CollectorConfig) (zfs.Collector, error) {
	switch cfg.Mode {
	case "local", "":
		return zfs.NewLocalCollector(cfg.Timeout), nil
	case "ssh":
		return zfs.NewSSHCollector(cfg)
	case "truenas":
		return zfs.NewTrueNASCollector(cfg), nil
	default:
		return nil, fmt.Errorf("unknown mode: %s", cfg.Mode)
	}
}

func collectAll(collectors map[string]zfs.Collector, s *store.Store, database *db.DB, ae *alerts.Engine) {
	for name, col := range collectors {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		pools, err := col.CollectPools(ctx)
		cancel()
		if err != nil {
			log.Printf("[collect] %s: %v", name, err)
			s.SetHostError(name, err.Error())
			continue
		}
		hd := &store.HostData{
			Pools:     pools,
			Datasets:  make(map[string][]*zfs.Dataset),
			Snapshots: make(map[string][]*zfs.Snapshot),
		}
		s.SetHostData(name, hd)
		if database != nil {
			for _, p := range pools {
				if err := database.RecordPoolSnapshot(name, p); err != nil {
					log.Printf("[db] %v", err)
				}
			}
		}
		if ae != nil {
			ae.CheckPools(name, pools)
		}
	}
}
