package monitor

import (
	"os/exec"
	"strconv"
	"strings"
)

// idleSeconds returns the number of seconds since the last keyboard/mouse
// input on Linux.
//
// Strategy (in priority order):
//  1. xprintidle — lightweight X11 idle-time utility (returns milliseconds)
//  2. Fallback to -1 (treat as active)
//
// On headless / server Linux boxes (no X11, no Wayland) idleSeconds returns -1,
// and the monitor defaults to "active" — same behaviour as the macOS root daemon
// case where IOHIDSystem is unreachable.
//
// NOTE: For Wayland desktops, xprintidle does not work.
// A future enhancement could use the idle-inhibit or ext-idle-notify
// Wayland protocols, but those require per-compositor support.
func idleSeconds() float64 {
	// Try xprintidle first (most common on X11 desktops)
	out, err := exec.Command("xprintidle").Output()
	if err == nil {
		ms, parseErr := strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
		if parseErr == nil {
			return ms / 1000.0
		}
	}

	// No idle detection available — return -1 so the monitor defaults to active.
	return -1
}
