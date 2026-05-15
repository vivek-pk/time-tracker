//go:build !darwin

package monitor

import (
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/vivek/time-tracker/internal/location"
	"github.com/vivek/time-tracker/internal/storage"
)

// refreshAndReadLocation on non-macOS platforms:
//  1. Tries to trigger a platform-native location helper if available
//  2. Tries WiFi geolocation: Unwired Labs (free) then Google (paid)
//  3. Falls back to IP-based geolocation (city-level accuracy)
func (m *Monitor) refreshAndReadLocation() storage.LocationInfo {
	// First, try to trigger the platform-native location helper binary.
	for _, helperPath := range locationHelperCandidates() {
		if _, err := exec.LookPath(helperPath); err != nil && !filepath.IsAbs(helperPath) {
			continue
		}
		if filepath.IsAbs(helperPath) {
			if _, err := os.Stat(helperPath); err != nil {
				continue
			}
		}
		log.Printf("monitor: triggering platform location helper (%s)", helperPath)
		if out, err := exec.Command(helperPath).CombinedOutput(); err != nil {
			log.Printf("monitor: location helper failed: %v (%s)", err, string(out))
		}
		loc := m.readLocation()
		if loc.Latitude != 0 || loc.Longitude != 0 {
			return loc
		}
		break
	}

	// Second, try WiFi geolocation (Unwired Labs free, then Google)
	if m.cfg.UnwiredLabsToken != "" || m.cfg.GoogleGeoAPIKey != "" {
		log.Println("monitor: trying WiFi geolocation")
		if err := location.FetchAndWriteWiFiGeolocation(m.locPath, m.cfg.UnwiredLabsToken, m.cfg.GoogleGeoAPIKey); err != nil {
			log.Printf("monitor: WiFi geolocation failed: %v", err)
		} else {
			loc := m.readLocation()
			if loc.Latitude != 0 || loc.Longitude != 0 {
				return loc
			}
		}
	}

	// Fallback: IP-based geolocation (works everywhere, ~city accuracy)
	log.Println("monitor: using IP geolocation fallback")
	if err := location.FetchAndWriteIPGeolocation(m.locPath); err != nil {
		log.Printf("monitor: IP geolocation failed: %v", err)
	}

	return m.readLocation()
}

func locationHelperCandidates() []string {
	if runtime.GOOS == "windows" {
		base := os.Getenv("ProgramData")
		if base == "" {
			base = `C:\ProgramData`
		}
		return []string{
			filepath.Join(base, "time-tracker", "time-tracker-location.exe"),
			"time-tracker-location.exe",
		}
	}
	return []string{
		"/usr/local/bin/time-tracker-location",
		"time-tracker-location",
	}
}
