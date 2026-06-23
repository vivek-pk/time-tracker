//go:build linux

package location

import (
	"fmt"
	"os/exec"
	"strings"
)

// connectedSSID returns the SSID of the currently connected WiFi network on Linux.
// Tries nmcli first, then iwgetid as a fallback.
func connectedSSID() (string, error) {
	// Try nmcli (NetworkManager)
	if ssid, err := ssidFromNmcli(); err == nil && ssid != "" {
		return ssid, nil
	}

	// Fallback: iwgetid
	if ssid, err := ssidFromIwgetid(); err == nil && ssid != "" {
		return ssid, nil
	}

	return "", fmt.Errorf("no WiFi SSID detection method available")
}

func ssidFromNmcli() (string, error) {
	if _, err := exec.LookPath("nmcli"); err != nil {
		return "", err
	}

	out, err := exec.Command("nmcli", "-t", "-f", "active,ssid", "dev", "wifi").Output()
	if err != nil {
		return "", err
	}

	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "yes:") {
			return strings.TrimPrefix(line, "yes:"), nil
		}
	}
	return "", nil
}

func ssidFromIwgetid() (string, error) {
	if _, err := exec.LookPath("iwgetid"); err != nil {
		return "", err
	}

	out, err := exec.Command("iwgetid", "-r").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}
