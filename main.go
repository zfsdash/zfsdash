package main

import (
	"context"
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

func main() {
	configPath := flag.String("config", "config.yaml", "path to config file")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	database, err := db.Open(cfg.DB.Path)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer database.Close()

	st := store.New()
	alertMgr := alerts.New(cfg.Alerts)
	collectors := make(web.CollectorMap)

	for _, hostCfg := range cfg.Hosts {
		col, err := buildCollector(hostCfg)
		if err != nil {
			log.Printf("[main] host %s: build collector: %v \u2014 skipping", hostCfg.Name, err)
			continue
		}
		collectors[hostCfg.Name] = col
		st.SetError(hostCfg.Name, "initializing...")
	}

	for name, col := range collectors {
		go collectLoop(name, col, st, database, alertMgr)
	}

	apiHandler := web.New(st, database, collectors)
	mux := http.NewServeMux()
	mux.Handle("/api/", apiHandler)

	staticFS, err := fs.Sub(web.Static, "static")
	if err == nil {
		mux.Handle("/", http.FileServer(http.FS(staticFS)))
	} else {
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprintf(w, `<!DOCTYPE html><html><head><title>ZFSdash</title></head>`+
				`<body style="background:#0f172a;color:#e2e8f0;font-family:monospace;padding:2rem">`+
				`<h1>ZFS<span style="color:#60a5fa">dash</span></h1>`+
				`<p>API is running. Build the frontend: <code>cd web && npm run build</code></p>`+
				`<p><a href="/api/health" style="color:#60a5fa">/api/health</a> \u00b7 `+
				`<a href="/api/hosts" style="color:#60a5fa">/api/hosts</a></p>`+
				`</body></html>`)
		})
	}

	srv := &http.Server{
		Addr:         cfg.Server.Addr,
		Handler:      mux,
		ReadTimeout:  time.Duration(cfg.Server.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(cfg.Server.WriteTimeout) * time.Second,
	}

	go func() {
		log.Printf("[main] ZFSdash listening on %s", cfg.Server.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("[main] shutting down...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
	for _, col := range collectors {
		_ = col.Close()
	}
	log.Println("[main] goodbye")
}

func buildCollector(h config.HostConfig) (zfs.Collector, error) {
	switch h.Mode {
	case "local", "":
		return zfs.NewLocalCollector(), nil
	case "ssh":
		pem := h.SSH.PrivateKeyPEM
		if pem == "" && h.SSH.PrivateKeyPath != "" {
			data, err := os.ReadFile(h.SSH.PrivateKeyPath)
			if err != nil {
				return nil, fmt.Errorf("read key %s: %w", h.SSH.PrivateKeyPath, err)
			}
			pem = string(data)
		}
		return zfs.NewSSHCollector(zfs.SSHConfig{
			Host:          h.SSH.Host,
			Port:          h.SSH.Port,
			User:          h.SSH.User,
			Password:      h.SSH.Password,
			PrivateKeyPEM: pem,
		})
	case "truenas":
		return zfs.NewTrueNASCollector(zfs.TrueNASConfig{
			URL:      h.TrueNAS.URL,
			APIKey:   h.TrueNAS.APIKey,
			Username: h.TrueNAS.Username,
			Password: h.TrueNAS.Password,
			Insecure: h.TrueNAS.Insecure,
		}), nil
	default:
		return nil, fmt.Errorf("unknown mode %q", h.Mode)
	}
}

func collectLoop(host string, col zfs.Collector, st *store.Store, database *db.DB, alertMgr *alerts.Manager) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	collect(host, col, st, database, alertMgr)
	for range ticker.C {
		collect(host, col, st, database, alertMgr)
	}
}

func collect(host string, col zfs.Collector, st *store.Store, database *db.DB, alertMgr *alerts.Manager) {
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()

	pools, err := col.GetPools(ctx)
	if err != nil {
		log.Printf("[collector][%s] GetPools: %v", host, err)
		st.SetError(host, err.Error())
	} else {
		st.SetPools(host, pools)
		for _, p := range pools {
			_ = database.RecordPoolHistory(host, p.Name, p.Capacity, p.Allocated, p.Size, p.State)
		}
		alertMgr.CheckPools(host, pools)
	}

	ds, err := col.GetDatasets(ctx, "")
	if err != nil {
		log.Printf("[collector][%s] GetDatasets: %v", host, err)
	} else {
		st.SetDatasets(host, ds)
	}

	snaps, err := col.GetSnapshots(ctx, "")
	if err != nil {
		log.Printf("[collector][%s] GetSnapshots: %v", host, err)
	} else {
		st.SetSnapshots(host, snaps)
	}

	smart, err := col.GetSMARTData(ctx)
	if err != nil {
		log.Printf("[collector][%s] GetSMARTData: %v", host, err)
	} else if smart != nil {
		st.SetSMARTData(host, smart)
		alertMgr.CheckSMART(host, smart)
	}
}
