package location

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

// wifiAP represents a WiFi access point for geolocation APIs.
type wifiAP struct {
	MACAddress         string `json:"macAddress"`
	SignalStrength     int    `json:"signalStrength,omitempty"`
	Channel            int    `json:"channel,omitempty"`
	SignalToNoiseRatio int    `json:"signalToNoiseRatio,omitempty"`
}

// --- Unwired Labs (FREE: 100 requests/day, no credit card) ---
// Sign up at https://unwiredlabs.com/ for a free API token.

type unwiredRequest struct {
	Token           string        `json:"token"`
	WiFi            []unwiredWiFi `json:"wifi,omitempty"`
	Fallbacks       interface{}   `json:"fallbacks,omitempty"`
	Address         int           `json:"address,omitempty"`
}

type unwiredWiFi struct {
	BSSID   string `json:"bssid"`
	Signal  int    `json:"signal,omitempty"`
	Channel int    `json:"channel,omitempty"`
}

type unwiredResponse struct {
	Status  string  `json:"status"`
	Message string  `json:"message,omitempty"`
	Lat     float64 `json:"lat"`
	Lon     float64 `json:"lon"`
	Accuracy float64 `json:"accuracy"`
}

// FetchUnwiredLabsGeolocation scans nearby WiFi networks and queries the
// Unwired Labs API for accurate coordinates (~20-200m accuracy).
//
// FREE tier: 100 requests/day (no credit card required).
// Sign up: https://unwiredlabs.com/
func FetchUnwiredLabsGeolocation(token string) (Info, error) {
	if token == "" {
		return Info{}, fmt.Errorf("unwired labs API token not configured")
	}

	accessPoints, err := scanWiFi()
	if err != nil {
		log.Printf("location: WiFi scan failed: %v", err)
	}

	// Convert to Unwired Labs format
	var wifiList []unwiredWiFi
	for _, ap := range accessPoints {
		wifiList = append(wifiList, unwiredWiFi{
			BSSID:   ap.MACAddress,
			Signal:  ap.SignalStrength,
			Channel: ap.Channel,
		})
	}

	reqBody := unwiredRequest{
		Token: token,
		WiFi:  wifiList,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return Info{}, fmt.Errorf("marshal: %w", err)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post("https://us1.unwiredlabs.com/v2/process", "application/json", bytes.NewReader(body))
	if err != nil {
		return Info{}, fmt.Errorf("unwired labs request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if err != nil {
		return Info{}, fmt.Errorf("read response: %w", err)
	}

	var uwResp unwiredResponse
	if err := json.Unmarshal(respBody, &uwResp); err != nil {
		return Info{}, fmt.Errorf("parse: %w", err)
	}

	if uwResp.Status != "ok" {
		return Info{}, fmt.Errorf("unwired labs error: %s", uwResp.Message)
	}

	if uwResp.Lat == 0 && uwResp.Lon == 0 {
		return Info{}, fmt.Errorf("no coordinates returned")
	}

	apCount := len(wifiList)
	log.Printf("location: Unwired Labs: lat=%.5f lon=%.5f accuracy=%.0fm (%d WiFi APs)",
		uwResp.Lat, uwResp.Lon, uwResp.Accuracy, apCount)

	return Info{
		Latitude:  uwResp.Lat,
		Longitude: uwResp.Lon,
		Accuracy:  uwResp.Accuracy,
		UpdatedAt: time.Now().UTC(),
	}, nil
}

// --- Google Geolocation API (paid, $200/month free credit) ---

type googleGeoRequest struct {
	ConsiderIP       bool     `json:"considerIp"`
	WiFiAccessPoints []wifiAP `json:"wifiAccessPoints,omitempty"`
}

type googleGeoResponse struct {
	Location struct {
		Lat float64 `json:"lat"`
		Lng float64 `json:"lng"`
	} `json:"location"`
	Accuracy float64 `json:"accuracy"`
	Error    *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// FetchGoogleGeolocation scans WiFi and queries Google's Geolocation API.
func FetchGoogleGeolocation(apiKey string) (Info, error) {
	if apiKey == "" {
		return Info{}, fmt.Errorf("google geolocation API key not configured")
	}

	accessPoints, err := scanWiFi()
	if err != nil {
		log.Printf("location: WiFi scan failed: %v", err)
	}

	reqBody := googleGeoRequest{
		ConsiderIP:       true,
		WiFiAccessPoints: accessPoints,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return Info{}, fmt.Errorf("marshal: %w", err)
	}

	url := fmt.Sprintf("https://www.googleapis.com/geolocation/v1/geolocate?key=%s", apiKey)
	client := &http.Client{Timeout: 10 * time.Second}

	resp, err := client.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return Info{}, fmt.Errorf("google geolocation request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if err != nil {
		return Info{}, fmt.Errorf("read response: %w", err)
	}

	var geoResp googleGeoResponse
	if err := json.Unmarshal(respBody, &geoResp); err != nil {
		return Info{}, fmt.Errorf("parse: %w", err)
	}

	if geoResp.Error != nil {
		return Info{}, fmt.Errorf("google API error %d: %s", geoResp.Error.Code, geoResp.Error.Message)
	}

	if geoResp.Location.Lat == 0 && geoResp.Location.Lng == 0 {
		return Info{}, fmt.Errorf("no coordinates returned")
	}

	apCount := len(accessPoints)
	log.Printf("location: Google Geolocation: lat=%.5f lon=%.5f accuracy=%.0fm (%d WiFi APs)",
		geoResp.Location.Lat, geoResp.Location.Lng, geoResp.Accuracy, apCount)

	return Info{
		Latitude:  geoResp.Location.Lat,
		Longitude: geoResp.Location.Lng,
		Accuracy:  geoResp.Accuracy,
		UpdatedAt: time.Now().UTC(),
	}, nil
}

// --- Convenience wrappers ---

// FetchAndWriteWiFiGeolocation tries Unwired Labs (free), then Google, and
// writes the result. Skips if a recent accurate fix already exists.
func FetchAndWriteWiFiGeolocation(path string, unwiredToken string, googleKey string) error {
	// Skip if we already have a recent, accurate fix
	existing, stale, err := ReadValidatedFromFile(path)
	if err == nil && !existing.Empty() && !stale && existing.Accuracy < 1000 {
		return nil
	}

	// Try Unwired Labs first (free)
	if unwiredToken != "" {
		info, err := FetchUnwiredLabsGeolocation(unwiredToken)
		if err == nil {
			return WriteToFile(path, info)
		}
		log.Printf("location: Unwired Labs failed: %v", err)
	}

	// Fallback to Google
	if googleKey != "" {
		info, err := FetchGoogleGeolocation(googleKey)
		if err == nil {
			return WriteToFile(path, info)
		}
		log.Printf("location: Google Geolocation failed: %v", err)
	}

	return fmt.Errorf("no WiFi geolocation API configured or all failed")
}
