package config

import (
	"log"
	"os"
	"strings"
)

// machineID returns the content of /etc/machine-id as the stable machine
// identifier on Linux. This file is populated by systemd at first boot and
// persists across reboots — it changes only on a full OS reinstall or manual
// deletion.
//
// Falls back to /var/lib/dbus/machine-id (older distros) then to the
// sanitised hostname.
func machineID() string {
	for _, path := range []string{"/etc/machine-id", "/var/lib/dbus/machine-id"} {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		id := strings.TrimSpace(string(data))
		if id != "" {
			return id
		}
	}
	log.Println("config: /etc/machine-id unavailable, falling back to hostname")
	h, _ := os.Hostname()
	return sanitiseHostname(h)
}
