package location

import (
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// NetworkInfo holds the public IP and connected WiFi SSID captured at session start.
type NetworkInfo struct {
	PublicIP string
	SSID     string
}

// FetchNetworkInfo returns the current public IPv4 address and connected WiFi SSID.
// Both fields are best-effort; failures result in empty strings.
func FetchNetworkInfo() NetworkInfo {
	return NetworkInfo{
		PublicIP: FetchPublicIPv4(),
		SSID:    GetConnectedSSID(),
	}
}

// --- Public IP (IPv4 only, cached) ---

var (
	ipCacheMu    sync.Mutex
	cachedIP     string
	cachedIPTime time.Time
	ipCacheTTL   = 5 * time.Minute
)

// FetchPublicIPv4 returns the machine's public IPv4 address.
// Results are cached for 5 minutes to avoid excessive API calls.
// Returns "" on any failure.
func FetchPublicIPv4() string {
	ipCacheMu.Lock()
	defer ipCacheMu.Unlock()

	if cachedIP != "" && time.Since(cachedIPTime) < ipCacheTTL {
		return cachedIP
	}

	ip := fetchIPv4()
	if ip != "" {
		cachedIP = ip
		cachedIPTime = time.Now()
	}
	return ip
}

func fetchIPv4() string {
	// ipify.org returns IPv4 only (api4 subdomain guarantees it)
	if ip := httpGetTrimmed("https://api4.ipify.org", 5*time.Second); isIPv4(ip) {
		return ip
	}
	log.Println("location: ipify.org failed, trying icanhazip.com")

	// Fallback: ipv4.icanhazip.com (Cloudflare-operated, IPv4-only endpoint)
	if ip := httpGetTrimmed("https://ipv4.icanhazip.com", 5*time.Second); isIPv4(ip) {
		return ip
	}
	log.Println("location: all public IP providers failed")
	return ""
}

func httpGetTrimmed(url string, timeout time.Duration) string {
	client := &http.Client{Timeout: timeout}
	resp, err := client.Get(url)
	if err != nil {
		log.Printf("location: GET %s: %v", url, err)
		return ""
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 256))
	if err != nil {
		return ""
	}
	if resp.StatusCode != 200 {
		return ""
	}
	return strings.TrimSpace(string(body))
}

// isIPv4 checks that the string is a valid IPv4 address (not IPv6).
func isIPv4(s string) bool {
	ip := net.ParseIP(s)
	if ip == nil {
		return false
	}
	return ip.To4() != nil && !strings.Contains(s, ":")
}

// GetConnectedSSID returns the SSID of the currently connected WiFi network.
// Returns "" if not connected to WiFi or on error.
// Delegates to the platform-specific connectedSSID() function.
func GetConnectedSSID() string {
	ssid, err := connectedSSID()
	if err != nil {
		log.Printf("location: SSID detection failed: %v", err)
		return ""
	}
	return ssid
}
