package location

import (
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

// scanWiFi scans nearby WiFi networks on macOS using the airport utility.
func scanWiFi() ([]wifiAP, error) {
	airportPath := "/System/Library/PrivateFrameworks/Apple80211.framework/Versions/Current/Resources/airport"

	out, err := exec.Command(airportPath, "-s").CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("airport scan: %w", err)
	}

	return parseAirportOutput(string(out))
}

// Airport -s output format (tab/space separated):
//                             SSID BSSID             RSSI CHANNEL HT CC SECURITY
//                         MyWiFi aa:bb:cc:dd:ee:ff  -45  6       Y  -- WPA2
var airportLineRe = regexp.MustCompile(`\s+([0-9a-fA-F:]{17})\s+(-?\d+)\s+(\d+)`)

func parseAirportOutput(output string) ([]wifiAP, error) {
	var aps []wifiAP

	for _, line := range strings.Split(output, "\n") {
		m := airportLineRe.FindStringSubmatch(line)
		if len(m) < 4 {
			continue
		}

		signal, _ := strconv.Atoi(m[2])
		channel, _ := strconv.Atoi(m[3])

		aps = append(aps, wifiAP{
			MACAddress:     m[1],
			SignalStrength: signal, // airport reports dBm directly
			Channel:        channel,
		})
	}

	if len(aps) == 0 {
		return nil, fmt.Errorf("no WiFi networks found")
	}

	return aps, nil
}
