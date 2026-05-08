// Command location-helper-linux fetches GPS coordinates on Linux.
//
// Strategy (in priority order):
//  1. GeoClue2 D-Bus service — available on most desktop Linux distributions
//     (GNOME, KDE, etc). Uses WiFi/cell triangulation for decent accuracy.
//  2. IP geolocation fallback — uses free APIs for city-level accuracy.
//
// The output is written to the shared location JSON file in the same format
// as the macOS CoreLocation helper, so the main daemon reads it the same way.
//
// Install: copy to /usr/local/bin/time-tracker-location
// Usage:   time-tracker-location [output-path]
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/vivek/time-tracker/internal/location"
)

func main() {
	outPath := location.SharedFilePath
	if len(os.Args) > 1 {
		outPath = os.Args[1]
	}

	// Try GeoClue2 first
	info, err := fetchGeoClue2()
	if err != nil {
		fmt.Fprintf(os.Stderr, "location-helper: GeoClue2 failed: %v\n", err)
		fmt.Fprintln(os.Stderr, "location-helper: falling back to IP geolocation")

		// Fallback to IP geolocation
		info, err = location.FetchIPGeolocation()
		if err != nil {
			fmt.Fprintf(os.Stderr, "location-helper: IP geolocation also failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Fprintln(os.Stderr, "location-helper: using IP geolocation (city-level accuracy)")
	}

	if err := location.WriteToFile(outPath, info); err != nil {
		fmt.Fprintf(os.Stderr, "location-helper: write %s: %v\n", outPath, err)
		os.Exit(1)
	}

	out, _ := json.Marshal(info)
	fmt.Printf("location-helper: ok %s\n", out)
}

// fetchGeoClue2 uses the GeoClue2 D-Bus service via the `gdbus` CLI tool.
// This avoids needing a Go D-Bus library dependency.
//
// GeoClue2 is standard on GNOME and KDE desktops and provides WiFi/cell-based
// location with reasonable accuracy (~100m-1km in urban areas).
func fetchGeoClue2() (location.Info, error) {
	// Check if gdbus is available
	if _, err := exec.LookPath("gdbus"); err != nil {
		return location.Info{}, fmt.Errorf("gdbus not found: GeoClue2 requires gdbus CLI tool")
	}

	// Step 1: Create a GeoClue2 client
	out, err := exec.Command("gdbus", "call", "--session",
		"--dest", "org.freedesktop.GeoClue2",
		"--object-path", "/org/freedesktop/GeoClue2/Manager",
		"--method", "org.freedesktop.GeoClue2.Manager.GetClient",
	).CombinedOutput()
	if err != nil {
		return location.Info{}, fmt.Errorf("create client: %v (%s)", err, strings.TrimSpace(string(out)))
	}

	// Parse client path from output like: ('/org/freedesktop/GeoClue2/Client/1',)
	clientPath := extractObjectPath(string(out))
	if clientPath == "" {
		return location.Info{}, fmt.Errorf("could not parse client path from: %s", string(out))
	}

	// Step 2: Set DesktopId (required by GeoClue2)
	exec.Command("gdbus", "call", "--session",
		"--dest", "org.freedesktop.GeoClue2",
		"--object-path", clientPath,
		"--method", "org.freedesktop.DBus.Properties.Set",
		"org.freedesktop.GeoClue2.Client", "DesktopId",
		"<string 'time-tracker'>",
	).Run()

	// Step 3: Set requested accuracy level (city-level is fine for attendance)
	exec.Command("gdbus", "call", "--session",
		"--dest", "org.freedesktop.GeoClue2",
		"--object-path", clientPath,
		"--method", "org.freedesktop.DBus.Properties.Set",
		"org.freedesktop.GeoClue2.Client", "RequestedAccuracyLevel",
		"<uint32 4>", // GCLUE_ACCURACY_LEVEL_CITY=4
	).Run()

	// Step 4: Start the client (triggers location acquisition)
	out, err = exec.Command("gdbus", "call", "--session",
		"--dest", "org.freedesktop.GeoClue2",
		"--object-path", clientPath,
		"--method", "org.freedesktop.GeoClue2.Client.Start",
	).CombinedOutput()
	if err != nil {
		return location.Info{}, fmt.Errorf("start client: %v (%s)", err, strings.TrimSpace(string(out)))
	}

	// Step 5: Poll for the Location property (wait up to 15 seconds)
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		time.Sleep(1 * time.Second)

		out, err = exec.Command("gdbus", "call", "--session",
			"--dest", "org.freedesktop.GeoClue2",
			"--object-path", clientPath,
			"--method", "org.freedesktop.DBus.Properties.Get",
			"org.freedesktop.GeoClue2.Client", "Location",
		).CombinedOutput()
		if err != nil {
			continue
		}

		locPath := extractObjectPath(string(out))
		if locPath == "" || locPath == "/" {
			continue // no fix yet
		}

		// Step 6: Read latitude and longitude from the location object
		info, err := readGeoClueLocation(locPath)

		// Step 7: Stop the client (cleanup)
		exec.Command("gdbus", "call", "--session",
			"--dest", "org.freedesktop.GeoClue2",
			"--object-path", clientPath,
			"--method", "org.freedesktop.GeoClue2.Client.Stop",
		).Run()

		return info, err
	}

	// Cleanup on timeout
	exec.Command("gdbus", "call", "--session",
		"--dest", "org.freedesktop.GeoClue2",
		"--object-path", clientPath,
		"--method", "org.freedesktop.GeoClue2.Client.Stop",
	).Run()

	return location.Info{}, fmt.Errorf("GeoClue2 timeout: no location fix within 15 seconds")
}

// readGeoClueLocation reads lat/lon/accuracy from a GeoClue2 Location object.
func readGeoClueLocation(locPath string) (location.Info, error) {
	out, err := exec.Command("gdbus", "call", "--session",
		"--dest", "org.freedesktop.GeoClue2",
		"--object-path", locPath,
		"--method", "org.freedesktop.DBus.Properties.GetAll",
		"org.freedesktop.GeoClue2.Location",
	).CombinedOutput()
	if err != nil {
		return location.Info{}, fmt.Errorf("read location properties: %v", err)
	}

	props := string(out)

	lat := extractDouble(props, "Latitude")
	lon := extractDouble(props, "Longitude")
	acc := extractDouble(props, "Accuracy")

	if lat == 0 && lon == 0 {
		return location.Info{}, fmt.Errorf("no valid coordinates in GeoClue2 response")
	}

	return location.Info{
		Latitude:  lat,
		Longitude: lon,
		Accuracy:  acc,
		UpdatedAt: time.Now().UTC(),
	}, nil
}

// extractObjectPath extracts a D-Bus object path from gdbus output.
// Input looks like: ('/org/freedesktop/GeoClue2/Client/1',) or
// (<'/org/freedesktop/GeoClue2/Location/1'>,)
var objectPathRe = regexp.MustCompile(`'(/[^']+)'`)

func extractObjectPath(s string) string {
	m := objectPathRe.FindStringSubmatch(s)
	if len(m) < 2 {
		return ""
	}
	return m[1]
}

// extractDouble extracts a double value for a named property from gdbus GetAll output.
// The output looks like: {'Latitude': <12.345>, 'Longitude': <67.890>, 'Accuracy': <100.0>, ...}
func extractDouble(props, key string) float64 {
	// Match patterns like: 'Key': <1.234> or 'Key': <double 1.234>
	re := regexp.MustCompile(fmt.Sprintf(`'%s':\s*<(?:double\s+)?([0-9eE.+-]+)>`, key))
	m := re.FindStringSubmatch(props)
	if len(m) < 2 {
		return 0
	}
	v, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		return 0
	}
	return v
}
