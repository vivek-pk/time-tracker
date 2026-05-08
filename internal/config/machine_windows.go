package config

import (
	"log"
	"os"
	"strings"

	"golang.org/x/sys/windows/registry"
)

// machineID returns the Windows MachineGuid from the registry as the stable
// machine identifier. This GUID is generated at install time and persists
// across reboots. It changes only on a full Windows reinstall.
//
// Falls back to the sanitised hostname if the registry read fails (e.g. in
// a container or heavily locked-down environment).
func machineID() string {
	k, err := registry.OpenKey(
		registry.LOCAL_MACHINE,
		`SOFTWARE\Microsoft\Cryptography`,
		registry.READ|registry.WOW64_64KEY,
	)
	if err != nil {
		log.Printf("config: registry MachineGuid unavailable: %v (falling back to hostname)", err)
		h, _ := os.Hostname()
		return sanitiseHostname(h)
	}
	defer k.Close()

	guid, _, err := k.GetStringValue("MachineGuid")
	if err != nil || strings.TrimSpace(guid) == "" {
		log.Printf("config: MachineGuid read failed: %v (falling back to hostname)", err)
		h, _ := os.Hostname()
		return sanitiseHostname(h)
	}
	return guid
}
