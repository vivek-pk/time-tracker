//go:build !darwin

package monitor

import (
	"log"
	"os/exec"
	"runtime"

	"github.com/vivek/time-tracker/internal/location"
	"github.com/vivek/time-tracker/internal/storage"
)

// refreshAndReadLocation on non-macOS platforms:
//  1. Tries to trigger a platform-native location helper if available
//  2. Tries Google WiFi Geolocation API (if API key configured) for accurate location
//  3. Falls back to IP-based geolocation (city-level accuracy)
func (m *Monitor) refreshAndReadLocation() storage.LocationInfo {
	// First, try to trigger the platform-native location helper binary.
	// On Linux this uses GeoClue2 D-Bus; on Windows, the Windows Location API.
	helperName := "time-tracker-location"
	if runtime.GOOS == "windows" {
		helperName = "time-tracker-location.exe"
	}

	if _, err := exec.LookPath(helperName); err == nil {
		log.Println("monitor: triggering platform location helper")
		if out, err := exec.Command(helperName).CombinedOutput(); err != nil {
			log.Printf("monitor: location helper failed: %v (%s)", err, string(out))
		}
		loc := m.readLocation()
		if (loc.Latitude != 0 || loc.Longitude != 0) {
			return loc
		}
	}

	// Second, try Google WiFi Geolocation (accurate, ~20-100m)
	if m.cfg.GoogleGeoAPIKey != "" {
		log.Println("monitor: trying Google WiFi Geolocation")
		if err := location.FetchAndWriteWiFiGeolocation(location.SharedFilePath, m.cfg.GoogleGeoAPIKey); err != nil {
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
	if err := location.FetchAndWriteIPGeolocation(location.SharedFilePath); err != nil {
		log.Printf("monitor: IP geolocation failed: %v", err)
	}

	return m.readLocation()
}
