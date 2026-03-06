package main

import (
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/vivek/time-tracker/internal/config"
	"github.com/vivek/time-tracker/internal/location"
	"github.com/vivek/time-tracker/internal/monitor"
	"github.com/vivek/time-tracker/internal/storage"
	"github.com/vivek/time-tracker/internal/syncer"
)

func main() {
	envFile := envOrDefault("ENV_FILE", "/etc/time-tracker/.env")

	cfg, err := config.Load(envFile)
	if err != nil {
		log.Fatalf("time-tracker: config error: %v", err)
	}

	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	log.SetPrefix("[time-tracker] ")

	_ = os.MkdirAll(cfg.LogPath, 0o750)

	log.Printf("starting machine=%s db=%s", cfg.MachineID, cfg.DBPath)

	_ = os.MkdirAll(filepath.Dir(cfg.DBPath), 0o750)

	db, err := storage.Open(cfg.DBPath)
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	log.Printf("database open — %d existing session(s)", db.SessionCount())

	if n, err := db.CloseHangingSessions(time.Now()); err != nil {
		log.Printf("warning: could not close hanging sessions: %v", err)
	} else if n > 0 {
		log.Printf("closed %d hanging session(s) from previous run", n)
	}

	// Read the last GPS fix written by the location helper LaunchAgent.
	// Empty if the helper hasn't run yet — sessions will have no coordinates.
	loc, locErr := location.ReadFromFile(location.SharedFilePath)
	if locErr != nil {
		log.Printf("location: read failed: %v (continuing without location)", locErr)
	} else if loc.Empty() {
		log.Println("location: no fix yet — ensure location helper is installed and authorised")
	} else {
		log.Printf("location: lat=%.5f lon=%.5f accuracy=%.0fm (fixed %s ago)",
			loc.Latitude, loc.Longitude, loc.Accuracy,
			time.Since(loc.UpdatedAt).Round(time.Second))
	}

	mon := monitor.New(cfg, db, location.SharedFilePath)
	syn := syncer.New(cfg, db, mon)

	stopCh := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); mon.Run(stopCh) }()
	go func() { defer wg.Done(); syn.Run(stopCh, mon.WakeEvents) }()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	sig := <-sigCh
	log.Printf("received signal %s shutting down gracefully", sig)
	close(stopCh)
	wg.Wait()
	log.Println("shutdown complete")
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
