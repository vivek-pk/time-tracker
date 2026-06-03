//go:build !darwin

package monitor

// startSleepWatcher is a no-op on non-macOS platforms.
// Linux and Windows don't exhibit the Power Nap brief-wake issue, so
// the existing poll-gap heuristic is sufficient.
func startSleepWatcher() {}

// isSystemSleeping always returns false on non-macOS platforms.
// The monitor falls back to the poll-gap heuristic for sleep detection.
func isSystemSleeping() bool { return false }
