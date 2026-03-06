package config

import (
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// These vars are injected at build time via -ldflags -X.
// They act as compiled-in defaults; env vars / .env file still override them.
var (
	DefaultSyncAPIURL = ""
	DefaultSyncAPIKey = ""
	DefaultDBPath     = "/var/lib/time-tracker/tracker.db"
	DefaultLogPath    = "/var/log/time-tracker"
)

// Config holds all runtime configuration loaded from environment variables.
type Config struct {
	MachineID            string
	SyncAPIURL           string
	SyncAPIKey           string
	MorningSyncHour      int
	EveningSyncHour      int
	EveningSyncMinute    int
	IdleThresholdMinutes int
	PollIntervalSeconds  int
	DBPath               string
	LogPath              string
	RetentionDays        int
	SyncTimeoutSeconds   int
}

// Load reads config from an optional .env file then environment variables.
func Load(envFilePath string) (*Config, error) {
	if envFilePath != "" {
		if err := godotenv.Load(envFilePath); err != nil {
			log.Printf("config: .env not found at %s, using env vars only", envFilePath)
		}
	}
	cfg := &Config{
		MachineID:            machineID(),
		SyncAPIURL:           envOrDef("SYNC_API_URL", DefaultSyncAPIURL),
		SyncAPIKey:           envOrDef("SYNC_API_KEY", DefaultSyncAPIKey),
		MorningSyncHour:      intEnv("MORNING_SYNC_HOUR", 6),
		EveningSyncHour:      intEnv("EVENING_SYNC_HOUR", 18),
		EveningSyncMinute:    intEnv("EVENING_SYNC_MINUTE", 30),
		IdleThresholdMinutes: intEnv("IDLE_THRESHOLD_MINUTES", 5),
		PollIntervalSeconds:  intEnv("POLL_INTERVAL_SECONDS", 30),
		DBPath:               envOrDef("DB_PATH", DefaultDBPath),
		LogPath:              envOrDef("LOG_PATH", DefaultLogPath),
		RetentionDays:        intEnv("RETENTION_DAYS", 3),
		SyncTimeoutSeconds:   intEnv("SYNC_TIMEOUT_SECONDS", 30),
	}
	return cfg, cfg.validate()
}

func (c *Config) IdleThreshold() time.Duration {
	return time.Duration(c.IdleThresholdMinutes) * time.Minute
}
func (c *Config) PollInterval() time.Duration {
	return time.Duration(c.PollIntervalSeconds) * time.Second
}
func (c *Config) SyncTimeout() time.Duration {
	return time.Duration(c.SyncTimeoutSeconds) * time.Second
}

func (c *Config) validate() error {
	if c.SyncAPIURL == "" {
		return fmt.Errorf("SYNC_API_URL must be set")
	}
	if !strings.HasPrefix(c.SyncAPIURL, "http://") && !strings.HasPrefix(c.SyncAPIURL, "https://") {
		return fmt.Errorf("SYNC_API_URL must start with http:// or https://")
	}
	if c.SyncAPIKey == "" {
		log.Println("config: WARNING — SYNC_API_KEY is empty; API requests will be unauthenticated")
	}
	if c.IdleThresholdMinutes < 1 {
		return fmt.Errorf("IDLE_THRESHOLD_MINUTES must be >= 1")
	}
	if c.PollIntervalSeconds < 5 {
		return fmt.Errorf("POLL_INTERVAL_SECONDS must be >= 5")
	}
	return nil
}

func envOrDef(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func intEnv(key string, def int) int {
	s := os.Getenv(key)
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		log.Printf("config: invalid int for %s, using %d", key, def)
		return def
	}
	return n
}

func sanitiseHostname(h string) string {
	if i := strings.IndexByte(h, '.'); i != -1 {
		h = h[:i]
	}
	var sb strings.Builder
	for _, r := range h {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' {
			sb.WriteRune(r)
		} else {
			sb.WriteRune('-')
		}
	}
	if ip := primaryIP(); ip != "" {
		sb.WriteByte('-')
		sb.WriteString(ip)
	}
	return sb.String()
}

func primaryIP() string {
	ifaces, _ := net.Interfaces()
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, _ := iface.Addrs()
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip == nil || ip.IsLoopback() {
				continue
			}
			if ip4 := ip.To4(); ip4 != nil {
				return fmt.Sprintf("%d-%d", ip4[2], ip4[3])
			}
		}
	}
	return ""
}
