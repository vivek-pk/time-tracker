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
// a fresh GPS fix, then reads the result. Falls back to cached/stale data if
// the refresh fails.
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
