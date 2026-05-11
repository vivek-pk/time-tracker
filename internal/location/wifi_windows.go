package location

import (
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

// scanWiFi scans nearby WiFi networks on Windows using netsh.
func scanWiFi() ([]wifiAP, error) {
	out, err := exec.Command("netsh", "wlan", "show", "networks", "mode=bssid").CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("netsh wlan: %w (%s)", err, string(out))
	}

	return parseNetshOutput(string(out))
}

var bssidRe = regexp.MustCompile(`(?i)BSSID\s*\d*\s*:\s*([0-9a-fA-F:]+)`)
var signalRe = regexp.MustCompile(`(?i)Signal\s*:\s*(\d+)%`)
var channelRe = regexp.MustCompile(`(?i)Channel\s*:\s*(\d+)`)

func parseNetshOutput(output string) ([]wifiAP, error) {
	var aps []wifiAP

	// Split by BSSID entries
	bssidMatches := bssidRe.FindAllStringSubmatchIndex(output, -1)
	for i, match := range bssidMatches {
		if len(match) < 4 {
			continue
		}

		bssid := output[match[2]:match[3]]

		// Get the text block from this BSSID to the next one
		start := match[0]
		end := len(output)
		if i+1 < len(bssidMatches) {
			end = bssidMatches[i+1][0]
		}
		block := output[start:end]

		ap := wifiAP{MACAddress: strings.TrimSpace(bssid)}

		// Extract signal strength (percentage -> dBm approximation)
		if m := signalRe.FindStringSubmatch(block); len(m) >= 2 {
			pct, _ := strconv.Atoi(m[1])
			ap.SignalStrength = (pct / 2) - 100
		}

		// Extract channel
		if m := channelRe.FindStringSubmatch(block); len(m) >= 2 {
			ap.Channel, _ = strconv.Atoi(m[1])
		}

		if ap.MACAddress != "" {
			aps = append(aps, ap)
		}
	}

	if len(aps) == 0 {
		return nil, fmt.Errorf("no WiFi networks found")
	}

	return aps, nil
}
