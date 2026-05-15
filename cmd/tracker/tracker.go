package main

import (
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/vivek/time-tracker/internal/config"
	"github.com/vivek/time-tracker/internal/location"
	"github.com/vivek/time-tracker/internal/monitor"
	"github.com/vivek/time-tracker/internal/storage"
	"github.com/vivek/time-tracker/internal/syncer"
)

// runTracker contains the core application logic, shared between all platforms.
func runTracker(stopCh chan struct{}) {
	cfg, err := config.Load("")
	if err != nil {
		log.Fatalf("time-tracker: config error: %v", err)
	}

	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	log.SetPrefix("[time-tracker] ")

	_ = os.MkdirAll(cfg.LogPath, 0o750)

	// Open a log file so output is captured even when stderr is unavailable
	// (e.g. Windows scheduled tasks). Writes go to both stderr and the file.
	logFile, logErr := os.OpenFile(
		filepath.Join(cfg.LogPath, "output.log"),
		os.O_CREATE|os.O_WRONLY|os.O_APPEND,
		0o644,
	)
	if logErr == nil {
		log.SetOutput(io.MultiWriter(os.Stderr, logFile))
		defer logFile.Close()
	}

	log.Printf("starting machine=%s db=%s", cfg.MachineID, cfg.DBPath)

	_ = os.MkdirAll(filepath.Dir(cfg.DBPath), 0o750)

	db, err := storage.Open(cfg.DBPath)
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	log.Printf("database open -- %d existing session(s)", db.SessionCount())

	if n, err := db.CloseHangingSessions(time.Now()); err != nil {
		log.Printf("warning: could not close hanging sessions: %v", err)
	} else if n > 0 {
		log.Printf("closed %d hanging session(s) from previous run", n)
	}

	// Read the last GPS fix written by the location helper.
	// Empty if the helper hasn't run yet -- sessions will have no coordinates.
	loc, locErr := location.ReadFromFile(location.SharedFilePath)
	if locErr != nil {
		log.Printf("location: read failed: %v (continuing without location)", locErr)
	} else if loc.Empty() {
		log.Println("location: no fix yet -- ensure location helper is installed and authorised")
	} else {
		log.Printf("location: lat=%.5f lon=%.5f accuracy=%.0fm (fixed %s ago)",
			loc.Latitude, loc.Longitude, loc.Accuracy,
			time.Since(loc.UpdatedAt).Round(time.Second))
	}

	mon := monitor.New(cfg, db, location.SharedFilePath)
	syn := syncer.New(cfg, db, mon)

	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); mon.Run(stopCh) }()
	go func() { defer wg.Done(); syn.Run(stopCh, mon.WakeEvents) }()

	wg.Wait()
	log.Println("shutdown complete")
}
