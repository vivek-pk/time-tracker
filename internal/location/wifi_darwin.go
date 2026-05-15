package location

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework CoreWLAN -framework Foundation

#import <CoreWLAN/CoreWLAN.h>
#import <Foundation/Foundation.h>

// WiFiNetwork holds the data we need from a CWNetwork.
typedef struct {
    char bssid[18];   // "AA:BB:CC:DD:EE:FF\0"
    int  rssi;
    int  channel;
} WiFiNetwork;

// scanWiFiNetworks scans for nearby WiFi networks using CoreWLAN.
// Returns the number of networks found; results are written to `out`
// (caller allocates space for at most `maxOut` entries).
// Returns -1 on error.
static int scanWiFiNetworks(WiFiNetwork *out, int maxOut) {
    @autoreleasepool {
        CWWiFiClient *client = [CWWiFiClient sharedWiFiClient];
        if (!client) return -1;

        CWInterface *iface = [client interface];
        if (!iface) return -1;

        NSError *err = nil;
        NSSet<CWNetwork *> *networks = [iface scanForNetworksWithName:nil error:&err];
        if (err || !networks) return -1;

        int count = 0;
        for (CWNetwork *net in networks) {
            if (count >= maxOut) break;

            NSString *bssid = net.bssid;
            if (!bssid || bssid.length < 17) continue;

            strncpy(out[count].bssid, bssid.UTF8String, sizeof(out[count].bssid) - 1);
            out[count].bssid[sizeof(out[count].bssid) - 1] = '\0';
            out[count].rssi = (int)net.rssiValue;

            // CWChannel is available on macOS 10.7+
            CWChannel *ch = net.wlanChannel;
            out[count].channel = ch ? (int)ch.channelNumber : 0;

            count++;
        }
        return count;
    }
}
*/
import "C"

import (
	"fmt"
	"log"
)

// scanWiFi scans nearby WiFi networks on macOS using the CoreWLAN framework.
//
// This replaces the old approach using the `airport` CLI utility, which was
// removed by Apple in macOS 14.4 (Sonoma). CoreWLAN is the official API for
// WiFi scanning and works on macOS 10.6+.
//
// Note: On macOS 12+ this requires Location Services permission because Apple
// considers WiFi BSSID data as location information. The location-helper app
// bundle already requests this permission.
func scanWiFi() ([]wifiAP, error) {
	const maxNetworks = 256
	var buf [maxNetworks]C.WiFiNetwork

	n := C.scanWiFiNetworks(&buf[0], C.int(maxNetworks))
	if n < 0 {
		return nil, fmt.Errorf("CoreWLAN scan failed (Location Services may not be authorized)")
	}
	if n == 0 {
		return nil, fmt.Errorf("no WiFi networks found")
	}

	aps := make([]wifiAP, 0, int(n))
	for i := 0; i < int(n); i++ {
		bssid := C.GoString(&buf[i].bssid[0])
		if bssid == "" {
			continue
		}
		aps = append(aps, wifiAP{
			MACAddress:     bssid,
			SignalStrength: int(buf[i].rssi),
			Channel:        int(buf[i].channel),
		})
	}

	log.Printf("location: CoreWLAN scan found %d WiFi networks", len(aps))
	return aps, nil
}
