//go:build darwin

package monitor

import (
	"log"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/vivek/time-tracker/internal/location"
	"github.com/vivek/time-tracker/internal/storage"
)

// refreshAndReadLocation triggers the macOS location helper LaunchAgent to get
// a fresh GPS fix, then reads the result. Falls back to WiFi geolocation and
// then IP geolocation if the LaunchAgent approach fails — matching the same
// fallback chain used on Linux and Windows.
func (m *Monitor) refreshAndReadLocation() storage.LocationInfo {
	// ── Step 1: Try CoreLocation via LaunchAgent ──
	loc := m.tryCoreLaunchAgent()
	if loc.Latitude != 0 || loc.Longitude != 0 {
		return loc
	}

	// ── Step 2: Try WiFi geolocation (Unwired Labs free, then Google) ──
	if m.cfg.UnwiredLabsToken != "" || m.cfg.GoogleGeoAPIKey != "" {
		log.Println("monitor: CoreLocation unavailable, trying WiFi geolocation")
		if err := location.FetchAndWriteWiFiGeolocation(m.locPath, m.cfg.UnwiredLabsToken, m.cfg.GoogleGeoAPIKey); err != nil {
			log.Printf("monitor: WiFi geolocation failed: %v", err)
		} else {
			loc := m.readLocation()
			if loc.Latitude != 0 || loc.Longitude != 0 {
				return loc
			}
		}
	}

	// ── Step 3: Fallback to IP geolocation (works everywhere, ~city accuracy) ──
	log.Println("monitor: using IP geolocation fallback")
	if err := location.FetchAndWriteIPGeolocation(m.locPath); err != nil {
		log.Printf("monitor: IP geolocation failed: %v", err)
	}

	return m.readLocation()
}

// tryCoreLaunchAgent kicks the macOS location helper LaunchAgent and waits for
// a fresh GPS fix. Returns a zero-value LocationInfo if the helper is not
// installed, not authorized, or times out.
func (m *Monitor) tryCoreLaunchAgent() storage.LocationInfo {
	// Get the console user's UID for launchctl
	uidBytes, err := exec.Command("stat", "-f", "%u", "/dev/console").Output()
	if err != nil {
		log.Printf("monitor: cannot determine console user: %v (trying fallbacks)", err)
		return storage.LocationInfo{}
	}
	uid := strings.TrimSpace(string(uidBytes))
	if uid == "" || uid == "0" {
		log.Println("monitor: no console user (headless?), trying fallbacks")
		return storage.LocationInfo{}
	}

	// Record the file's current mod time so we can detect when it's updated
	var oldModTime time.Time
	if fi, err := os.Stat(m.locPath); err == nil {
		oldModTime = fi.ModTime()
	}

	// Trigger the LaunchAgent in the user's GUI session (has CoreLocation access)
	label := "gui/" + uid + "/com.timetracker.locationhelper"
	log.Printf("monitor: triggering location helper (label=%s)", label)
	out, err := exec.Command("launchctl", "kickstart", "-k", label).CombinedOutput()
	if err != nil {
		log.Printf("monitor: launchctl kickstart failed: %v, output: %s (trying fallbacks)", err, string(out))
		return m.readLocation() // Return whatever is cached
	}
	log.Printf("monitor: launchctl kickstart success, waiting for location update...")

	// Wait up to 35 seconds for the file to be updated (helper timeout is 30s)
	deadline := time.Now().Add(35 * time.Second)
	updated := false
	for time.Now().Before(deadline) {
		time.Sleep(1 * time.Second)
		if fi, err := os.Stat(m.locPath); err == nil {
			if fi.ModTime().After(oldModTime) {
				updated = true
				log.Printf("monitor: location file updated after %.0fs", time.Since(oldModTime.Add(-35*time.Second)).Seconds())
				break // fresh fix written!
			}
		}
	}
	if !updated {
		log.Printf("monitor: location file not updated after 35s timeout")
	}

	return m.readLocation()
}
