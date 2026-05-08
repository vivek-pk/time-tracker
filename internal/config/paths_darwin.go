package config

// Default paths for macOS.
// These are compile-time fallbacks; config.json and env vars override them.
func init() {
	if DefaultDBPath == "" {
		DefaultDBPath = "/var/lib/time-tracker/tracker.db"
	}
	if DefaultLogPath == "" {
		DefaultLogPath = "/var/log/time-tracker"
	}
}

// defaultEnvFilePath returns the OS-specific default location for the .env file.
func defaultEnvFilePath() string {
	return "/etc/time-tracker/.env"
}
