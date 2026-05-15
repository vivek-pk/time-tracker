//go:build linux

package monitor

import (
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// idleSeconds returns the number of seconds since the last keyboard/mouse
// input on Linux.
//
// Strategy (in priority order):
//  1. xprintidle — precise X11 idle time when running in a desktop session.
//  2. systemd-logind IdleHint — works for many X11/Wayland desktop sessions.
//  3. Fallback to -1 (treat as active).
func idleSeconds() float64 {
	if idle, ok := idleFromXprintidle(); ok {
		return idle
	}
	if idle, ok := idleFromLoginctl(); ok {
		return idle
	}

	// No idle detection available — return -1 so the monitor defaults to active.
	return -1
}

func idleFromXprintidle() (float64, bool) {
	out, err := exec.Command("xprintidle").Output()
	if err != nil {
		return 0, false
	}
	ms, err := strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
	if err != nil {
		return 0, false
	}
	return ms / 1000.0, true
}

func idleFromLoginctl() (float64, bool) {
	if _, err := exec.LookPath("loginctl"); err != nil {
		return 0, false
	}
	out, err := exec.Command("loginctl", "list-sessions", "--no-legend").Output()
	if err != nil {
		return 0, false
	}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		props, ok := loginctlSessionProps(fields[0])
		if !ok || props["Active"] != "yes" {
			continue
		}
		switch props["IdleHint"] {
		case "no":
			return 0, true
		case "yes":
			idle, ok := idleSinceMonotonic(props["IdleSinceHintMonotonic"])
			if ok {
				return idle, true
			}
		}
	}
	return 0, false
}

func loginctlSessionProps(sessionID string) (map[string]string, bool) {
	out, err := exec.Command(
		"loginctl", "show-session", sessionID,
		"-p", "Active",
		"-p", "IdleHint",
		"-p", "IdleSinceHintMonotonic",
	).Output()
	if err != nil {
		return nil, false
	}
	props := make(map[string]string)
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		key, val, ok := strings.Cut(line, "=")
		if ok {
			props[key] = val
		}
	}
	return props, true
}

func idleSinceMonotonic(raw string) (float64, bool) {
	sinceUsec, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if err != nil || sinceUsec <= 0 {
		return 0, false
	}
	data, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return 0, false
	}
	fields := strings.Fields(string(data))
	if len(fields) == 0 {
		return 0, false
	}
	uptimeSeconds, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return 0, false
	}
	idle := uptimeSeconds - sinceUsec/1_000_000
	if idle < 0 {
		idle = 0
	}
	return idle, true
}
