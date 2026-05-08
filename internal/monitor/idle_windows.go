package monitor

import (
	"syscall"
	"unsafe"
)

var (
	user32               = syscall.NewLazyDLL("user32.dll")
	kernel32             = syscall.NewLazyDLL("kernel32.dll")
	procGetLastInputInfo = user32.NewProc("GetLastInputInfo")
	procGetTickCount     = kernel32.NewProc("GetTickCount")
	procProcessIdToSessionId = kernel32.NewProc("ProcessIdToSessionId")
	procGetCurrentProcessId  = kernel32.NewProc("GetCurrentProcessId")
)

// lastInputInfo mirrors the Win32 LASTINPUTINFO struct.
type lastInputInfo struct {
	cbSize uint32
	dwTime uint32
}

// idleSeconds returns the number of seconds since the last keyboard/mouse
// input on Windows.
//
// Windows Services run in Session 0, which has no user desktop. In Session 0,
// GetLastInputInfo may succeed but return stale data (the session never
// receives real input), leading to permanently high idle times.
//
// We detect Session 0 and return 0 (assume active) to avoid false idle.
func idleSeconds() float64 {
	// Check if we are running in Session 0 (service isolation session).
	// If so, skip idle detection entirely -- there is no user input here.
	if isSession0() {
		return 0
	}

	var lii lastInputInfo
	lii.cbSize = uint32(unsafe.Sizeof(lii))

	r1, _, _ := procGetLastInputInfo.Call(uintptr(unsafe.Pointer(&lii)))
	if r1 == 0 {
		// GetLastInputInfo failed -- assume active to be safe.
		return 0
	}

	r2, _, _ := procGetTickCount.Call()
	tickCount := uint32(r2)

	// Both values are in milliseconds; handle wrap-around gracefully
	// (GetTickCount wraps every ~49.7 days).
	idleMs := tickCount - lii.dwTime
	return float64(idleMs) / 1000.0
}

// isSession0 returns true if the current process is running in Session 0
// (the isolated service session with no user desktop).
func isSession0() bool {
	pid, _, _ := procGetCurrentProcessId.Call()
	var sessionId uint32
	r, _, _ := procProcessIdToSessionId.Call(pid, uintptr(unsafe.Pointer(&sessionId)))
	if r == 0 {
		// API call failed -- assume we're NOT in Session 0 (try normal idle detection).
		return false
	}
	return sessionId == 0
}
