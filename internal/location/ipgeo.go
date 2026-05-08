package location

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

// ipAPIResponse mirrors the JSON returned by ip-api.com/json/.
type ipAPIResponse struct {
	Status  string  `json:"status"`
	Lat     float64 `json:"lat"`
	Lon     float64 `json:"lon"`
	City    string  `json:"city"`
	Country string  `json:"country"`
	Query   string  `json:"query"`
}

// FetchIPGeolocation queries a free IP geolocation API to determine
// approximate location based on the machine's public IP address.
//
// Accuracy is roughly city-level (~1-5 km), which is sufficient for
// office-based attendance tracking.
//
// This function tries multiple free APIs in order:
//  1. ip-api.com (free, no key, HTTP only)
//  2. ipapi.co (free tier, HTTPS)
//
// Returns a zero-value Info on any failure (callers should handle gracefully).
func FetchIPGeolocation() (Info, error) {
	client := &http.Client{Timeout: 10 * time.Second}

	// Try ip-api.com first (free, no API key, rate limit: 45/min)
	info, err := tryIPAPI(client)
	if err == nil {
		return info, nil
	}
	log.Printf("location: ip-api.com failed: %v, trying fallback", err)

	// Fallback: ipapi.co (free tier, HTTPS, rate limit: 1000/day)
	info, err = tryIPAPICo(client)
	if err == nil {
		return info, nil
	}
	log.Printf("location: ipapi.co failed: %v", err)

	return Info{}, fmt.Errorf("all IP geolocation providers failed")
}

func tryIPAPI(client *http.Client) (Info, error) {
	resp, err := client.Get("http://ip-api.com/json/?fields=status,lat,lon,city,country,query")
	if err != nil {
		return Info{}, fmt.Errorf("request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if err != nil {
		return Info{}, fmt.Errorf("read body: %w", err)
	}

	if resp.StatusCode != 200 {
		return Info{}, fmt.Errorf("status %d: %s", resp.StatusCode, string(body))
	}

	var data ipAPIResponse
	if err := json.Unmarshal(body, &data); err != nil {
		return Info{}, fmt.Errorf("parse: %w", err)
	}
	if data.Status != "success" {
		return Info{}, fmt.Errorf("api error: status=%s", data.Status)
	}
	if data.Lat == 0 && data.Lon == 0 {
		return Info{}, fmt.Errorf("no coordinates returned")
	}

	return Info{
		Latitude:  data.Lat,
		Longitude: data.Lon,
		Accuracy:  5000, // IP geolocation is ~city level (~5km)
		UpdatedAt: time.Now().UTC(),
	}, nil
}

// ipapiCoResponse mirrors the JSON returned by ipapi.co/json/.
type ipapiCoResponse struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	City      string  `json:"city"`
	Country   string  `json:"country_name"`
	IP        string  `json:"ip"`
	Error     bool    `json:"error"`
	Reason    string  `json:"reason"`
}

func tryIPAPICo(client *http.Client) (Info, error) {
	req, err := http.NewRequest("GET", "https://ipapi.co/json/", nil)
	if err != nil {
		return Info{}, fmt.Errorf("build request: %w", err)
	}
	// ipapi.co requires a user-agent or it returns 403
	req.Header.Set("User-Agent", "time-tracker/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return Info{}, fmt.Errorf("request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if err != nil {
		return Info{}, fmt.Errorf("read body: %w", err)
	}

	if resp.StatusCode != 200 {
		return Info{}, fmt.Errorf("status %d: %s", resp.StatusCode, string(body))
	}

	var data ipapiCoResponse
	if err := json.Unmarshal(body, &data); err != nil {
		return Info{}, fmt.Errorf("parse: %w", err)
	}
	if data.Error {
		return Info{}, fmt.Errorf("api error: %s", data.Reason)
	}
	if data.Latitude == 0 && data.Longitude == 0 {
		return Info{}, fmt.Errorf("no coordinates returned")
	}

	return Info{
		Latitude:  data.Latitude,
		Longitude: data.Longitude,
		Accuracy:  5000, // IP geolocation is ~city level (~5km)
		UpdatedAt: time.Now().UTC(),
	}, nil
}

// FetchAndWriteIPGeolocation fetches IP-based location and writes it to the
// shared location file. This is a convenience wrapper for use as a fallback
// when no platform-native location helper is available.
//
// It skips the fetch if a recent fix (< MaxLocationAge) already exists on disk.
func FetchAndWriteIPGeolocation(path string) error {
	// Don't fetch if we already have a recent fix
	existing, stale, err := ReadValidatedFromFile(path)
	if err == nil && !existing.Empty() && !stale {
		return nil // fresh fix already available
	}

	info, err := FetchIPGeolocation()
	if err != nil {
		return err
	}

	return WriteToFile(path, info)
}
