//go:build darwin || linux

package location

// sharedFilePath returns a path visible to both the privileged tracker and
// the user/session location helper on Unix-like systems.
func sharedFilePath() string {
	return "/tmp/time-tracker-location.json"
}
