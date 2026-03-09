package config

import (
	_ "embed"
	"encoding/json"
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
// They act as the lowest-priority fallback; config.json and env vars override them.
var (
	DefaultSyncAPIURL = ""
	DefaultSyncAPIKey = ""
	DefaultDBPath     = "/var/lib/time-tracker/tracker.db"
	DefaultLogPath    = "/var/log/time-tracker"
)

// embeddedConfigJSON is the config.json file baked into the binary at build time.
// Edit internal/config/config.json with your API URL and key before building.
//
//go:embed config.json
var embeddedConfigJSON []byte

// jsonConfig mirrors config.json structure. Pointer types for ints let us
// distinguish "not set" from "set to 0".
type jsonConfig struct {
	SyncAPIURL           string `json:"sync_api_url,omitempty"`
	SyncAPIKey           string `json:"sync_api_key,omitempty"`
	MorningSyncHour      *int   `json:"morning_sync_hour,omitempty"`
	EveningSyncHour      *int   `json:"evening_sync_hour,omitempty"`
	EveningSyncMinute    *int   `json:"evening_sync_minute,omitempty"`
	IdleThresholdMinutes *int   `json:"idle_threshold_minutes,omitempty"`
	PollIntervalSeconds  *int   `json:"poll_interval_seconds,omitempty"`
	DBPath               string `json:"db_path,omitempty"`
	LogPath              string `json:"log_path,omitempty"`
	RetentionDays        *int   `json:"retention_days,omitempty"`
	SyncTimeoutSeconds   *int   `json:"sync_timeout_seconds,omitempty"`
}

// Config holds all runtime configuration.
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

// Load reads config with the following priority (highest wins):
//
//	1. Environment variables (including those set by .env file)
//	2. Embedded config.json (baked into the binary at build time)
//	3. Compiled-in defaults (set via -ldflags or hardcoded)
func Load(envFilePath string) (*Config, error) {
	// Step 1: Parse embedded config.json
	var jc jsonConfig
	if err := json.Unmarshal(embeddedConfigJSON, &jc); err != nil {
		log.Printf("config: embedded config.json parse error: %v (using defaults)", err)
	}

	// Step 2: Load optional .env file (sets env vars that take priority)
	if envFilePath != "" {
		if err := godotenv.Load(envFilePath); err != nil {
			log.Printf("config: .env not found at %s, using embedded config", envFilePath)
		}
	}

	// Step 3: Build config — env vars > config.json > compiled defaults
	cfg := &Config{
		MachineID:            machineID(),
		SyncAPIURL:           strPriority("SYNC_API_URL", jc.SyncAPIURL, DefaultSyncAPIURL),
		SyncAPIKey:           strPriority("SYNC_API_KEY", jc.SyncAPIKey, DefaultSyncAPIKey),
		MorningSyncHour:      intPriority("MORNING_SYNC_HOUR", jc.MorningSyncHour, 6),
		EveningSyncHour:      intPriority("EVENING_SYNC_HOUR", jc.EveningSyncHour, 18),
		EveningSyncMinute:    intPriority("EVENING_SYNC_MINUTE", jc.EveningSyncMinute, 30),
		IdleThresholdMinutes: intPriority("IDLE_THRESHOLD_MINUTES", jc.IdleThresholdMinutes, 5),
		PollIntervalSeconds:  intPriority("POLL_INTERVAL_SECONDS", jc.PollIntervalSeconds, 30),
		DBPath:               strPriority("DB_PATH", jc.DBPath, DefaultDBPath),
		LogPath:              strPriority("LOG_PATH", jc.LogPath, DefaultLogPath),
		RetentionDays:        intPriority("RETENTION_DAYS", jc.RetentionDays, 3),
		SyncTimeoutSeconds:   intPriority("SYNC_TIMEOUT_SECONDS", jc.SyncTimeoutSeconds, 30),
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

// strPriority returns: env var > jsonVal > compiled default.
func strPriority(envKey, jsonVal, def string) string {
	if v := os.Getenv(envKey); v != "" {
		return v
	}
	if jsonVal != "" {
		return jsonVal
	}
	return def
}

// intPriority returns: env var > jsonVal > compiled default.
func intPriority(envKey string, jsonVal *int, def int) int {
	if s := os.Getenv(envKey); s != "" {
		n, err := strconv.Atoi(strings.TrimSpace(s))
		if err != nil {
			log.Printf("config: invalid int for %s, using default", envKey)
		} else {
			return n
		}
	}
	if jsonVal != nil {
		return *jsonVal
	}
	return def
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
