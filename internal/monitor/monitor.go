package monitor

import (
	"log"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/vivek/time-tracker/internal/config"
	"github.com/vivek/time-tracker/internal/location"
	"github.com/vivek/time-tracker/internal/storage"
)

// WakeEvent is sent when the monitor detects a sleep/wake gap.
type WakeEvent struct {
	WokeAt   time.Time
	AsleepAt time.Time
}

// Monitor polls the macOS HID idle counter and records activity sessions.
type Monitor struct {
	cfg              *config.Config
	db               *storage.DB
	locPath          string
	WakeEvents       chan WakeEvent
	mu               sync.Mutex
	currentSessionID int64
	currentState     storage.State
	lastPollAt       time.Time
}

// New creates a Monitor (does not start it).
func New(cfg *config.Config, db *storage.DB, locPath string) *Monitor {
	return &Monitor{
		cfg:        cfg,
		db:         db,
		locPath:    locPath,
		WakeEvents: make(chan WakeEvent, 8),
	}
}

// Run starts the polling loop. Call in its own goroutine.
func (m *Monitor) Run(stopCh <-chan struct{}) {
	log.Printf("monitor: starting poll=%s idle_threshold=%s",
		m.cfg.PollInterval(), m.cfg.IdleThreshold())

	// One-time HID probe check — immediately reveals if idle detection works.
	testIdle := idleSeconds()
	if testIdle < 0 {
		log.Println("monitor: WARNING — HID idle probe returned -1; idle detection will NOT work in this context")
		log.Println("monitor: this happens when running as a root daemon without a display session")
	} else {
		log.Printf("monitor: HID probe OK (current idle: %.1fs)", testIdle)
	}

	ticker := time.NewTicker(m.cfg.PollInterval())
	defer ticker.Stop()

	m.lastPollAt = time.Now().Round(0)

	for {
		select {
		case <-stopCh:
			log.Println("monitor: shutting down")
			m.mu.Lock()
			m.closeCurrentSession(time.Now())
			m.mu.Unlock()
			return
		case now := <-ticker.C:
			m.poll(now)
		}
	}
}

func (m *Monitor) poll(_ time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// IMPORTANT: Use wall clock, not monotonic clock, for gap detection.
	// Go's time.Sub() defaults to the monotonic clock, which does NOT
	// advance during macOS system sleep. Round(0) strips the monotonic
	// reading so Sub() falls back to wall-clock comparison.
	now := time.Now().Round(0)
	gap := now.Sub(m.lastPollAt)
	sleepDetected := gap > 2*m.cfg.PollInterval()
	m.lastPollAt = now

	if sleepDetected {
		log.Printf("monitor: sleep gap detected %.0fs", gap.Seconds())
		asleepAt := now.Add(-gap)
		if m.currentSessionID != 0 {
			m.closeCurrentSession(asleepAt)
		}
		offlineID, err := m.db.StartSession(m.cfg.MachineID, storage.StateOffline, asleepAt, storage.LocationInfo{})
		if err != nil {
			log.Printf("monitor: start offline session: %v", err)
		} else if closeErr := m.db.CloseSession(offlineID, now); closeErr != nil {
			log.Printf("monitor: close offline session: %v", closeErr)
		}
		select {
		case m.WakeEvents <- WakeEvent{WokeAt: now, AsleepAt: asleepAt}:
		default:
		}
		m.currentSessionID = 0
		m.currentState = ""
	}

	newState := m.classifyState()
	if m.currentSessionID == 0 || newState != m.currentState {
		if m.currentSessionID != 0 {
			m.closeCurrentSession(now)
		}
		loc := m.refreshAndReadLocation()
		id, err := m.db.StartSession(m.cfg.MachineID, newState, now, loc)
		if err != nil {
			log.Printf("monitor: start session state=%s: %v", newState, err)
			return
		}
		m.currentSessionID = id
		m.currentState = newState
		log.Printf("monitor: new session id=%d state=%s", id, newState)
	}
}

func (m *Monitor) classifyState() storage.State {
	idle := idleSeconds()
	// TODO: remove this debug log once idle detection is confirmed working
	log.Printf("monitor: [debug] idle=%.1fs threshold=%s idle<0=%v", idle, m.cfg.IdleThreshold(), idle < 0)
	if idle < 0 {
		// IOHIDSystem probe failed — common when running as a root daemon
		// without a display session. Log once per transition only.
		if m.currentState != storage.StateActive {
			log.Println("monitor: HID idle probe returned -1 (no display session?), defaulting to active")
		}
		return storage.StateActive
	}
	if time.Duration(float64(time.Second)*idle) >= m.cfg.IdleThreshold() {
		return storage.StateIdle
	}
	return storage.StateActive
}

func (m *Monitor) closeCurrentSession(at time.Time) {
	if m.currentSessionID == 0 {
		return
	}
	if err := m.db.CloseSession(m.currentSessionID, at); err != nil {
		log.Printf("monitor: close session id=%d: %v", m.currentSessionID, err)
	}
	m.currentSessionID = 0
	m.currentState = ""
}

// FlushCurrentSession closes any open session at the given time so it can be
// synced. The monitor will open a fresh session on its next poll tick.
// Safe to call from any goroutine.
func (m *Monitor) FlushCurrentSession(at time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.currentSessionID == 0 {
		return
	}
	log.Printf("monitor: flushing session id=%d for sync", m.currentSessionID)
	if err := m.db.CloseSession(m.currentSessionID, at); err != nil {
		log.Printf("monitor: flush session id=%d: %v", m.currentSessionID, err)
	}
	m.currentSessionID = 0
	m.currentState = ""
}

// refreshAndReadLocation triggers the location helper to get a fresh GPS fix,
// then reads the result. Falls back to cached/stale data if the refresh fails.
func (m *Monitor) refreshAndReadLocation() storage.LocationInfo {
	// Get the console user's UID for launchctl
	uidBytes, err := exec.Command("stat", "-f", "%u", "/dev/console").Output()
	if err != nil {
		log.Printf("monitor: cannot determine console user: %v (using cached location)", err)
		return m.readLocation()
	}
	uid := strings.TrimSpace(string(uidBytes))
	if uid == "" || uid == "0" {
		return m.readLocation()
	}

	// Record the file's current mod time so we can detect when it's updated
	var oldModTime time.Time
	if fi, err := os.Stat(location.SharedFilePath); err == nil {
		oldModTime = fi.ModTime()
	}

	// Trigger the LaunchAgent in the user's GUI session (has CoreLocation access)
	label := "gui/" + uid + "/com.timetracker.locationhelper"
	exec.Command("launchctl", "kickstart", label).Run()

	// Wait up to 35 seconds for the file to be updated (helper timeout is 30s)
	deadline := time.Now().Add(35 * time.Second)
	for time.Now().Before(deadline) {
		time.Sleep(1 * time.Second)
		if fi, err := os.Stat(location.SharedFilePath); err == nil {
			if fi.ModTime().After(oldModTime) {
				break // fresh fix written!
			}
		}
	}

	return m.readLocation()
}

// readLocation returns GPS coordinates from the location helper's output file.
// Uses stale coordinates (with a warning) rather than returning 0,0.
func (m *Monitor) readLocation() storage.LocationInfo {
	info, stale, err := location.ReadValidatedFromFile(location.SharedFilePath)
	if err != nil {
		log.Printf("monitor: location read error: %v", err)
		return storage.LocationInfo{}
	}
	if info.Empty() {
		log.Println("monitor: no location available")
		return storage.LocationInfo{}
	}
	age := time.Since(info.UpdatedAt).Round(time.Second)
	if stale {
		log.Printf("monitor: location: lat=%.5f lon=%.5f (stale, age: %s)", info.Latitude, info.Longitude, age)
	} else {
		log.Printf("monitor: location: lat=%.5f lon=%.5f (fresh, age: %s)", info.Latitude, info.Longitude, age)
	}
	return storage.LocationInfo{
		Latitude:  info.Latitude,
		Longitude: info.Longitude,
	}
}
