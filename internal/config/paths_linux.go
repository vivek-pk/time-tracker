package config

// Default paths for Linux — identical to macOS.
func init() {
	if DefaultDBPath == "" {
		DefaultDBPath = "/var/lib/time-tracker/tracker.db"
	}
	if DefaultLogPath == "" {
		DefaultLogPath = "/var/log/time-tracker"
	}
}
