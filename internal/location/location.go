package location

import (
	"encoding/json"
	"os"
	"time"
)

// SharedFilePath is where the location helper writes and the system daemon reads.
// /tmp avoids permission conflicts between the user-session helper and the root daemon.
const SharedFilePath = "/tmp/time-tracker-location.json"

// Info holds GPS coordinates captured by the location helper.
type Info struct {
	Latitude  float64   `json:"lat"`
	Longitude float64   `json:"lon"`
	Accuracy  float64   `json:"accuracy_m"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Empty returns true when no valid fix has been captured yet.
func (i Info) Empty() bool {
	return i.Latitude == 0 && i.Longitude == 0
}

// ReadFromFile reads the last fix written by the location helper.
// Returns a zero-value Info (no error) when the file does not exist yet.
func ReadFromFile(path string) (Info, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return Info{}, nil
	}
	if err != nil {
		return Info{}, err
	}
	var info Info
	if err := json.Unmarshal(data, &info); err != nil {
		return Info{}, err
	}
	return info, nil
}

// WriteToFile writes location info to path with restricted permissions.
func WriteToFile(path string, info Info) error {
	data, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

// MaxLocationAge is the maximum age of a location fix before it's considered stale.
const MaxLocationAge = 10 * time.Minute

// ReadValidatedFromFile reads the last fix and validates it is recent enough to trust.
// Returns a zero-value Info (no error) if the file is missing or the fix is stale.
func ReadValidatedFromFile(path string) (Info, error) {
	info, err := ReadFromFile(path)
	if err != nil {
		return Info{}, err
	}
	if info.Empty() {
		return Info{}, nil
	}
	if time.Since(info.UpdatedAt) > MaxLocationAge {
		return Info{}, nil
	}
	return info, nil
}
