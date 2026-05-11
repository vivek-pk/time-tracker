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

// wifiAP represents a WiFi access point for the Google Geolocation API.
type wifiAP struct {
	MACAddress         string `json:"macAddress"`
	SignalStrength     int    `json:"signalStrength,omitempty"`
	Channel            int    `json:"channel,omitempty"`
	SignalToNoiseRatio int    `json:"signalToNoiseRatio,omitempty"`
}

// geoRequest is the request body for the Google Geolocation API.
type geoRequest struct {
	ConsiderIP      bool     `json:"considerIp"`
	WiFiAccessPoints []wifiAP `json:"wifiAccessPoints,omitempty"`
}

// geoResponse is the response from the Google Geolocation API.
type geoResponse struct {
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

// FetchWiFiGeolocation scans nearby WiFi networks and queries the Google
// Geolocation API to determine accurate coordinates (~20-100m accuracy).
// This is the same technology browsers use for navigator.geolocation.
//
// Requires a valid Google Geolocation API key.
// Enable at: https://console.cloud.google.com/apis/library/geolocation.googleapis.com
func FetchWiFiGeolocation(apiKey string) (Info, error) {
	if apiKey == "" {
		return Info{}, fmt.Errorf("google geolocation API key not configured")
	}

	// Scan nearby WiFi access points
	accessPoints, err := scanWiFi()
	if err != nil {
		log.Printf("location: WiFi scan failed: %v (sending request without WiFi data)", err)
	}

	reqBody := geoRequest{
		ConsiderIP:       true,
		WiFiAccessPoints: accessPoints,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return Info{}, fmt.Errorf("marshal request: %w", err)
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

	var geoResp geoResponse
	if err := json.Unmarshal(respBody, &geoResp); err != nil {
		return Info{}, fmt.Errorf("parse response: %w", err)
	}

	if geoResp.Error != nil {
		return Info{}, fmt.Errorf("google API error %d: %s", geoResp.Error.Code, geoResp.Error.Message)
	}

	if geoResp.Location.Lat == 0 && geoResp.Location.Lng == 0 {
		return Info{}, fmt.Errorf("no coordinates returned")
	}

	apCount := len(accessPoints)
	log.Printf("location: Google Geolocation API: lat=%.5f lon=%.5f accuracy=%.0fm (using %d WiFi APs)",
		geoResp.Location.Lat, geoResp.Location.Lng, geoResp.Accuracy, apCount)

	return Info{
		Latitude:  geoResp.Location.Lat,
		Longitude: geoResp.Location.Lng,
		Accuracy:  geoResp.Accuracy,
		UpdatedAt: time.Now().UTC(),
	}, nil
}

// FetchAndWriteWiFiGeolocation fetches WiFi-based location and writes it to
// the shared location file. Skips if a recent accurate fix already exists.
func FetchAndWriteWiFiGeolocation(path string, apiKey string) error {
	// Don't fetch if we already have a recent, accurate fix
	existing, stale, err := ReadValidatedFromFile(path)
	if err == nil && !existing.Empty() && !stale && existing.Accuracy < 1000 {
		return nil // already have a good fix
	}

	info, err := FetchWiFiGeolocation(apiKey)
	if err != nil {
		return err
	}

	return WriteToFile(path, info)
}
