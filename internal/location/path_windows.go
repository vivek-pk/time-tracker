//go:build windows

package location

import (
	"os"
	"path/filepath"
)

// sharedFilePath stores the helper output in ProgramData so the scheduled
// task and helper agree without relying on a Unix-style /tmp path.
func sharedFilePath() string {
	base := os.Getenv("ProgramData")
	if base == "" {
		base = `C:\ProgramData`
	}
	return filepath.Join(base, "time-tracker", "time-tracker-location.json")
}
