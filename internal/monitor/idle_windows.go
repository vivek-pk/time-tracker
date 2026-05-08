package monitor

import (
	"syscall"
	"unsafe"
)

var (
	user32              = syscall.NewLazyDLL("user32.dll")
	kernel32            = syscall.NewLazyDLL("kernel32.dll")
	procGetLastInputInfo = user32.NewProc("GetLastInputInfo")
	procGetTickCount     = kernel32.NewProc("GetTickCount")
)

// lastInputInfo mirrors the Win32 LASTINPUTINFO struct.
type lastInputInfo struct {
	cbSize uint32
	dwTime uint32
}

// idleSeconds returns the number of seconds since the last keyboard/mouse
// input on Windows, using GetLastInputInfo and GetTickCount.
// Returns -1 if the system call fails.
func idleSeconds() float64 {
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
