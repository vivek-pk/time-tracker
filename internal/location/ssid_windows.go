//go:build windows

package location

import (
	"fmt"
	"os/exec"
	"regexp"
	"strings"
)

var ssidRe = regexp.MustCompile(`(?i)^\s*SSID\s*:\s*(.+)$`)

// connectedSSID returns the SSID of the currently connected WiFi network on Windows.
// Parses the output of "netsh wlan show interfaces".
func connectedSSID() (string, error) {
	out, err := exec.Command("netsh", "wlan", "show", "interfaces").CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("netsh wlan: %w", err)
	}

	for _, line := range strings.Split(string(out), "\n") {
		// Match "    SSID                   : MyNetwork" but not "    BSSID"
		line = strings.TrimRight(line, "\r")
		if strings.Contains(strings.ToUpper(line), "BSSID") {
			continue
		}
		if m := ssidRe.FindStringSubmatch(line); len(m) >= 2 {
			return strings.TrimSpace(m[1]), nil
		}
	}

	return "", nil // Not connected to WiFi
}
