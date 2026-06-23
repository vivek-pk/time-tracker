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

// Monitor polls the OS idle counter and records activity sessions.
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
	log.Printf("monitor: starting poll=%s idle_threshold=%s version=%s",
		m.cfg.PollInterval(), m.cfg.IdleThreshold(), m.cfg.VersionID)

	// Start the platform-specific sleep/wake watcher.
	// On macOS this uses IOKit power notifications to set an atomic flag
	// that prevents the monitor from processing during brief maintenance
	// wakes (Power Nap). On other platforms this is a no-op.
	startSleepWatcher()

	// One-time HID probe check
	testIdle := idleSeconds()
	if testIdle < 0 {
		log.Println("monitor: idle detection unavailable (no display session) - will classify as idle")
	} else {
		log.Printf("monitor: idle detection OK (current idle: %.1fs)", testIdle)
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

	// If the IOKit sleep watcher reports the system is sleeping, skip all
	// processing. This prevents spurious "active" sessions during brief
	// macOS maintenance wakes (Power Nap, push notifications, etc.).
	//
	// IMPORTANT: Do NOT update m.lastPollAt here. Maintenance wakes must
	// not advance the timestamp, otherwise the full sleep gap gets eaten
	// incrementally and the offline session only covers the last
	// maintenance-wake-to-real-wake interval.
	if isSystemSleeping() {
		if sleepDetected && m.currentSessionID != 0 {
			// Close the current session at the estimated sleep time.
			asleepAt := now.Add(-gap)
			log.Printf("monitor: system sleeping (IOKit), closing session at sleep time")
			m.closeCurrentSession(asleepAt)
		}
		log.Printf("monitor: system sleeping (IOKit), skipping poll")
		return
	}

	// Only advance lastPollAt for real wakes (not maintenance wakes).
	m.lastPollAt = now

	if sleepDetected {
		log.Printf("monitor: sleep gap detected %.0fs", gap.Seconds())
		asleepAt := now.Add(-gap)
		if m.currentSessionID != 0 {
			m.closeCurrentSession(asleepAt)
		}
		offlineID, err := m.db.StartSession(m.cfg.MachineID, storage.StateOffline, asleepAt, storage.LocationInfo{}, m.cfg.VersionID)
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
	
	// Get current idle time for logging
	idleTime := idleSeconds()
	
	// Log every poll to show monitor is alive (log level: debug)
	if m.currentSessionID != 0 && newState == m.currentState {
		// Heartbeat: update last_heartbeat so that if the process is killed
		// (power loss, OOM, SIGKILL), crash recovery can close this session
		// at the last known alive time instead of hours/days later.
		if err := m.db.TouchSession(m.currentSessionID, now); err != nil {
			log.Printf("monitor: heartbeat session id=%d: %v", m.currentSessionID, err)
		}
		log.Printf("monitor: poll idle=%.1fs state=%s session=%d (no change)",
			idleTime, m.currentState, m.currentSessionID)
	}
	
	if m.currentSessionID == 0 || newState != m.currentState {
		if m.currentSessionID != 0 {
			log.Printf("monitor: state changed %s->%s, closing session id=%d", m.currentState, newState, m.currentSessionID)
			m.closeCurrentSession(now)
		}
		loc := enrichWithNetworkInfo(m.refreshAndReadLocation())
		// Update lastPollAt AFTER the location refresh completes.
		// If the machine sleeps during the ~35s location wait, the next poll
		// would see a huge gap from the pre-refresh lastPollAt and falsely
		// fire sleep detection again, creating cascading 0-second sessions.
		// Anchoring here means the next gap is measured from when we actually
		// finished creating this session, not before the refresh started.
		m.lastPollAt = time.Now().Round(0)
		id, err := m.db.StartSession(m.cfg.MachineID, newState, now, loc, m.cfg.VersionID)
		if err != nil {
			log.Printf("monitor: start session state=%s: %v", newState, err)
			return
		}
		m.currentSessionID = id
		m.currentState = newState
		log.Printf("monitor: new session id=%d state=%s idle=%.1fs", id, newState, idleTime)
	}
}

func (m *Monitor) classifyState() storage.State {
	idle := idleSeconds()
	if idle < 0 {
		// No HID session available. This typically happens during macOS
		// DarkWake (Power Nap) when the display is off and no user is
		// interacting. Classify as idle to avoid false "active" sessions.
		return storage.StateIdle
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

// readLocation returns GPS coordinates from the location helper's output file.
// Uses stale coordinates (with a warning) rather than returning 0,0.
func (m *Monitor) readLocation() storage.LocationInfo {
	info, stale, err := location.ReadValidatedFromFile(m.locPath)
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

// enrichWithNetworkInfo fetches the current public IPv4 and WiFi SSID
// and merges them into the given LocationInfo.
func enrichWithNetworkInfo(loc storage.LocationInfo) storage.LocationInfo {
	ni := location.FetchNetworkInfo()
	loc.PublicIP = ni.PublicIP
	loc.SSID = ni.SSID
	if ni.PublicIP != "" || ni.SSID != "" {
		log.Printf("monitor: network info: ip=%s ssid=%q", ni.PublicIP, ni.SSID)
	}
	return loc
}
