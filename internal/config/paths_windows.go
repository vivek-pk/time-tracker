package config

import "os"

// Default paths for Windows.
// Uses %ProgramData%\time-tracker which is writable by services and admins.
func init() {
	base := os.Getenv("ProgramData")
	if base == "" {
		base = `C:\ProgramData`
	}
	if DefaultDBPath == "" {
		DefaultDBPath = base + `\time-tracker\tracker.db`
	}
	if DefaultLogPath == "" {
		DefaultLogPath = base + `\time-tracker\logs`
	}
}
