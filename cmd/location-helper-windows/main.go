// Command location-helper-windows fetches GPS coordinates on Windows.
//
// Strategy (in priority order):
//  1. Windows Location API via PowerShell — uses System.Device.Location which
//     leverages WiFi triangulation, cell towers, and GPS (if available).
//     Requires "Location Services" to be enabled in Windows Settings.
//  2. IP geolocation fallback — uses free APIs for city-level accuracy.
//
// The output is written to the shared location JSON file in the same format
// as the macOS CoreLocation helper, so the main daemon reads it the same way.
//
// Install: copy to %ProgramData%\time-tracker\ or any PATH directory
// Usage:   time-tracker-location.exe [output-path]
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
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

	// Try Windows Location API first
	info, err := fetchWindowsLocation()
	if err != nil {
		fmt.Fprintf(os.Stderr, "location-helper: Windows Location API failed: %v\n", err)
		fmt.Fprintln(os.Stderr, "location-helper: falling back to IP geolocation")
		fmt.Fprintln(os.Stderr, "  Tip: Enable Location Services in Settings > Privacy & Security > Location")

		// Fallback to IP geolocation
		info, err = location.FetchIPGeolocation()
		if err != nil {
			fmt.Fprintf(os.Stderr, "location-helper: IP geolocation also failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Fprintln(os.Stderr, "location-helper: using IP geolocation (city-level accuracy)")
	}

	info.SSID = location.GetConnectedSSID()

	if err := location.WriteToFile(outPath, info); err != nil {
		fmt.Fprintf(os.Stderr, "location-helper: write %s: %v\n", outPath, err)
		os.Exit(1)
	}

	out, _ := json.Marshal(info)
	fmt.Printf("location-helper: ok %s\n", out)
}

// fetchWindowsLocation uses PowerShell to call the .NET
// System.Device.Location.GeoCoordinateWatcher API.
//
// This API leverages Windows Location Services which can use:
//   - WiFi triangulation
//   - Cell tower data
//   - GPS hardware (if present)
//   - IP-based location as last resort
//
// Requires "Location Services" to be enabled in Windows Settings.
func fetchWindowsLocation() (location.Info, error) {
	// PowerShell script that fetches location and outputs "LAT,LON,ACC"
	script := `
Add-Type -AssemblyName System.Device
$w = New-Object System.Device.Location.GeoCoordinateWatcher([System.Device.Location.GeoPositionAccuracy]::Default)
$w.Start()
$timeout = 15
$elapsed = 0
while (($w.Status -ne 'Ready') -and ($w.Permission -ne 'Denied') -and ($elapsed -lt $timeout)) {
    Start-Sleep -Milliseconds 500
    $elapsed += 0.5
}
if ($w.Permission -eq 'Denied') {
    Write-Error 'DENIED'
    exit 2
}
if ($w.Status -ne 'Ready') {
    Write-Error 'TIMEOUT'
    exit 1
}
$pos = $w.Position.Location
if ($pos.IsUnknown) {
    Write-Error 'UNKNOWN'
    exit 1
}
Write-Output ("{0},{1},{2}" -f $pos.Latitude, $pos.Longitude, $pos.HorizontalAccuracy)
$w.Stop()
`

	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", script)
	out, err := cmd.CombinedOutput()
	outStr := strings.TrimSpace(string(out))

	if err != nil {
		if strings.Contains(outStr, "DENIED") {
			return location.Info{}, fmt.Errorf("location permission denied — enable in Settings > Privacy & Security > Location")
		}
		if strings.Contains(outStr, "TIMEOUT") {
			return location.Info{}, fmt.Errorf("location timeout (15s) — Location Services may be disabled")
		}
		return location.Info{}, fmt.Errorf("powershell: %v (%s)", err, outStr)
	}

	// Parse "LAT,LON,ACC" output
	parts := strings.Split(outStr, ",")
	if len(parts) < 2 {
		return location.Info{}, fmt.Errorf("unexpected output: %s", outStr)
	}

	lat, err := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
	if err != nil {
		return location.Info{}, fmt.Errorf("parse latitude: %w", err)
	}
	lon, err := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
	if err != nil {
		return location.Info{}, fmt.Errorf("parse longitude: %w", err)
	}
	var acc float64
	if len(parts) >= 3 {
		acc, _ = strconv.ParseFloat(strings.TrimSpace(parts[2]), 64)
	}
	if acc <= 0 || acc > 100000 {
		acc = 1000 // default to 1km accuracy for WiFi-based fixes
	}

	if lat == 0 && lon == 0 {
		return location.Info{}, fmt.Errorf("no valid coordinates returned")
	}

	return location.Info{
		Latitude:  lat,
		Longitude: lon,
		Accuracy:  acc,
		UpdatedAt: time.Now().UTC(),
	}, nil
}
