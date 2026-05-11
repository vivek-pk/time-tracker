package location

import (
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

// scanWiFi scans nearby WiFi networks on Linux using nmcli or iwlist.
func scanWiFi() ([]wifiAP, error) {
	// Try nmcli first (NetworkManager, most common on desktop Linux)
	aps, err := scanWithNmcli()
	if err == nil && len(aps) > 0 {
		return aps, nil
	}

	// Fallback to iwlist (requires root)
	aps, err = scanWithIwlist()
	if err == nil && len(aps) > 0 {
		return aps, nil
	}

	return nil, fmt.Errorf("no WiFi scanner available (install NetworkManager or wireless-tools)")
}

func scanWithNmcli() ([]wifiAP, error) {
	if _, err := exec.LookPath("nmcli"); err != nil {
		return nil, fmt.Errorf("nmcli not found")
	}

	// Force a rescan then list
	exec.Command("nmcli", "dev", "wifi", "rescan").Run()

	out, err := exec.Command("nmcli", "-t", "-f", "BSSID,SIGNAL,CHAN", "dev", "wifi", "list").Output()
	if err != nil {
		return nil, fmt.Errorf("nmcli: %w", err)
	}

	var aps []wifiAP
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// nmcli -t uses ':' as delimiter but BSSIDs also contain ':'
		// Format: AA\:BB\:CC\:DD\:EE\:FF:SIGNAL:CHAN
		// nmcli escapes colons in BSSID with backslash
		parts := strings.Split(line, ":")
		if len(parts) < 3 {
			continue
		}

		// Reconstruct BSSID - nmcli escapes : in MAC with backslash
		// Find the signal and channel (last two fields)
		// Work backwards since BSSID has variable escaping
		chanStr := parts[len(parts)-1]
		sigStr := parts[len(parts)-2]
		bssidParts := parts[:len(parts)-2]
		bssid := strings.Join(bssidParts, ":")
		bssid = strings.ReplaceAll(bssid, "\\", "")
		bssid = strings.TrimSpace(bssid)

		if bssid == "" || len(bssid) < 17 {
			continue
		}

		signal, _ := strconv.Atoi(strings.TrimSpace(sigStr))
		channel, _ := strconv.Atoi(strings.TrimSpace(chanStr))

		// nmcli reports signal as 0-100%, Google API wants dBm (typically -30 to -90)
		// Approximate conversion: dBm = (percentage / 2) - 100
		signalDBm := (signal / 2) - 100

		aps = append(aps, wifiAP{
			MACAddress:     bssid,
			SignalStrength: signalDBm,
			Channel:        channel,
		})
	}

	return aps, nil
}

var iwlistBSSIDRe = regexp.MustCompile(`Address:\s*([0-9A-Fa-f:]+)`)
var iwlistSignalRe = regexp.MustCompile(`Signal level[=:]\s*(-?\d+)`)
var iwlistChannelRe = regexp.MustCompile(`Channel[=:]\s*(\d+)`)

func scanWithIwlist() ([]wifiAP, error) {
	if _, err := exec.LookPath("iwlist"); err != nil {
		return nil, fmt.Errorf("iwlist not found")
	}

	// Find wireless interface
	iface, err := findWirelessInterface()
	if err != nil {
		return nil, err
	}

	out, err := exec.Command("iwlist", iface, "scan").CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("iwlist scan: %w", err)
	}

	var aps []wifiAP
	cells := strings.Split(string(out), "Cell ")
	for _, cell := range cells[1:] { // skip first empty split
		bssidMatch := iwlistBSSIDRe.FindStringSubmatch(cell)
		if len(bssidMatch) < 2 {
			continue
		}

		ap := wifiAP{MACAddress: bssidMatch[1]}

		if m := iwlistSignalRe.FindStringSubmatch(cell); len(m) >= 2 {
			ap.SignalStrength, _ = strconv.Atoi(m[1])
		}
		if m := iwlistChannelRe.FindStringSubmatch(cell); len(m) >= 2 {
			ap.Channel, _ = strconv.Atoi(m[1])
		}

		aps = append(aps, ap)
	}

	return aps, nil
}

func findWirelessInterface() (string, error) {
	out, err := exec.Command("iw", "dev").Output()
	if err != nil {
		// Fallback: try common names
		for _, name := range []string{"wlan0", "wlp2s0", "wlp3s0"} {
			if _, err := exec.Command("ip", "link", "show", name).Output(); err == nil {
				return name, nil
			}
		}
		return "", fmt.Errorf("no wireless interface found")
	}

	// Parse "iw dev" output for Interface line
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Interface ") {
			return strings.TrimPrefix(line, "Interface "), nil
		}
	}

	return "", fmt.Errorf("no wireless interface found in iw dev output")
}
