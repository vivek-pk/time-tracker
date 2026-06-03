package config

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"strings"
	"time"
)

// These vars are injected at build time via -ldflags -X.
// They act as the lowest-priority fallback; embedded config.json overrides them.
var (
	DefaultSyncAPIURL = ""
	DefaultSyncAPIKey = ""
	DefaultDBPath     = "" // Set per-platform in paths_*.go init()
	DefaultLogPath    = "" // Set per-platform in paths_*.go init()
	Version           = "1.5.5" // Set at build time: -X 'config.Version=1.5.5'
)

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
	RealtimeSync         *bool  `json:"realtime_sync,omitempty"`
	GoogleGeoAPIKey      string `json:"google_geolocation_api_key,omitempty"`
	UnwiredLabsToken     string `json:"unwired_labs_api_token,omitempty"`
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
	RealtimeSync         bool
	GoogleGeoAPIKey      string
	UnwiredLabsToken     string
	VersionID            string // Release version of the binary (e.g. "v1.5.4")
}

// Load reads config with the following priority (highest wins):
//
//  1. Embedded config.json (baked into the binary at build time)
//  2. Compiled-in defaults (set via -ldflags or hardcoded)
func Load(envFilePath string) (*Config, error) {
	_ = envFilePath // Kept for older diagnostic helpers; runtime .env loading is intentionally disabled.

	// Step 1: Parse embedded config.json.
	var jc jsonConfig
	if err := json.Unmarshal(embeddedConfigJSON, &jc); err != nil {
		log.Printf("config: embedded config.json parse error: %v (using defaults)", err)
	}

	// Step 2: Build config — config.json > compiled defaults.
	cfg := &Config{
		MachineID:            machineID(),
		SyncAPIURL:           strConfig(jc.SyncAPIURL, DefaultSyncAPIURL),
		SyncAPIKey:           strConfig(jc.SyncAPIKey, DefaultSyncAPIKey),
		MorningSyncHour:      intConfig(jc.MorningSyncHour, 6),
		EveningSyncHour:      intConfig(jc.EveningSyncHour, 18),
		EveningSyncMinute:    intConfig(jc.EveningSyncMinute, 30),
		IdleThresholdMinutes: intConfig(jc.IdleThresholdMinutes, 5),
		PollIntervalSeconds:  intConfig(jc.PollIntervalSeconds, 30),
		DBPath:               strConfig(jc.DBPath, DefaultDBPath),
		LogPath:              strConfig(jc.LogPath, DefaultLogPath),
		RetentionDays:        intConfig(jc.RetentionDays, 3),
		SyncTimeoutSeconds:   intConfig(jc.SyncTimeoutSeconds, 30),
		RealtimeSync:         boolConfig(jc.RealtimeSync, false),
		GoogleGeoAPIKey:      strConfig(jc.GoogleGeoAPIKey, ""),
		UnwiredLabsToken:     strConfig(jc.UnwiredLabsToken, ""),
		VersionID:            Version,
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
		return fmt.Errorf("sync_api_url must be set in config.json")
	}
	if !strings.HasPrefix(c.SyncAPIURL, "http://") && !strings.HasPrefix(c.SyncAPIURL, "https://") {
		return fmt.Errorf("sync_api_url must start with http:// or https://")
	}
	if c.SyncAPIKey == "" {
		log.Println("config: WARNING — sync_api_key is empty; API requests will be unauthenticated")
	}
	if c.IdleThresholdMinutes < 1 {
		return fmt.Errorf("idle_threshold_minutes must be >= 1")
	}
	if c.PollIntervalSeconds < 5 {
		return fmt.Errorf("poll_interval_seconds must be >= 5")
	}
	return nil
}

// strConfig returns: config.json value > compiled default.
func strConfig(jsonVal, def string) string {
	if jsonVal != "" {
		return jsonVal
	}
	return def
}

// intConfig returns: config.json value > compiled default.
func intConfig(jsonVal *int, def int) int {
	if jsonVal != nil {
		return *jsonVal
	}
	return def
}

// boolConfig returns: config.json value > compiled default.
func boolConfig(jsonVal *bool, def bool) bool {
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
