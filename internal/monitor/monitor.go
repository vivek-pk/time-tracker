package monitor

import (
	"log"
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

	ticker := time.NewTicker(m.cfg.PollInterval())
	defer ticker.Stop()

	m.lastPollAt = time.Now()

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

func (m *Monitor) poll(now time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()

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
		loc := m.readLocation()
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
	if idle < 0 {
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

// readLocation returns fresh GPS coordinates from the location helper's output file.
// Returns empty coordinates if the file is missing, unreadable, or stale.
func (m *Monitor) readLocation() storage.LocationInfo {
	info, err := location.ReadValidatedFromFile(m.locPath)
	if err != nil || info.Empty() {
		return storage.LocationInfo{}
	}
	return storage.LocationInfo{
		Latitude:  info.Latitude,
		Longitude: info.Longitude,
	}
}
