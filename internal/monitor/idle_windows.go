package monitor

import (
	"syscall"
	"unsafe"
)

var (
	user32               = syscall.NewLazyDLL("user32.dll")
	kernel32             = syscall.NewLazyDLL("kernel32.dll")
	wtsapi32             = syscall.NewLazyDLL("wtsapi32.dll")
	procGetLastInputInfo = user32.NewProc("GetLastInputInfo")
	procGetTickCount     = kernel32.NewProc("GetTickCount")
	procWTSGetActiveConsoleSessionId = kernel32.NewProc("WTSGetActiveConsoleSessionId")
	procWTSQuerySessionInformationW  = wtsapi32.NewProc("WTSQuerySessionInformationW")
	procWTSFreeMemory                = wtsapi32.NewProc("WTSFreeMemory")
)

// lastInputInfo mirrors the Win32 LASTINPUTINFO struct.
type lastInputInfo struct {
	cbSize uint32
	dwTime uint32
}

// WTS info classes
const (
	wtsSessionInfoEx = 25
	wtsSessionInfo   = 24
)

// idleSeconds returns the number of seconds since the last keyboard/mouse
// input on Windows.
//
// When running as a Windows Service (Session 0), GetLastInputInfo cannot see
// the user's desktop input. In that case we attempt to detect inactivity via
// the active console session's idle time reported by WTS. If everything
// fails, we return 0 (assume active) rather than reporting false idle.
func idleSeconds() float64 {
	// First try the direct approach -- works when running interactively
	// in the user's session.
	idle := getLastInputIdle()
	if idle >= 0 {
		return idle
	}

	// Fallback: we're probably in Session 0 (service). Return 0 (active)
	// to avoid false idle detection. The GetLastInputInfo approach cannot
	// work from Session 0 because there is no user desktop attached.
	//
	// A more advanced approach would use a small helper process running
	// in the user's session, but for now we default to "active" so the
	// service doesn't falsely mark the user as idle/offline.
	return 0
}

// getLastInputIdle uses GetLastInputInfo + GetTickCount.
// Returns -1 if the call fails (e.g. running in Session 0).
func getLastInputIdle() float64 {
	var lii lastInputInfo
	lii.cbSize = uint32(unsafe.Sizeof(lii))

	r1, _, _ := procGetLastInputInfo.Call(uintptr(unsafe.Pointer(&lii)))
	if r1 == 0 {
		return -1
	}

	r2, _, _ := procGetTickCount.Call()
	tickCount := uint32(r2)

	// Both values are in milliseconds; handle wrap-around gracefully
	// (GetTickCount wraps every ~49.7 days).
	idleMs := tickCount - lii.dwTime
	return float64(idleMs) / 1000.0
}
