//go:build darwin

package location

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework CoreWLAN -framework Foundation

#import <CoreWLAN/CoreWLAN.h>
#import <Foundation/Foundation.h>

// getConnectedSSID returns the SSID of the currently connected WiFi network.
// The caller must free() the returned string. Returns NULL if not connected.
static const char* getConnectedSSID() {
    @autoreleasepool {
        CWWiFiClient *client = [CWWiFiClient sharedWiFiClient];
        if (!client) return NULL;

        CWInterface *iface = [client interface];
        if (!iface) return NULL;

        NSString *ssid = iface.ssid;
        if (!ssid || ssid.length == 0) return NULL;

        return strdup(ssid.UTF8String);
    }
}
*/
import "C"

import "unsafe"

// connectedSSID returns the SSID of the currently connected WiFi network on macOS
// using the CoreWLAN framework.
func connectedSSID() (string, error) {
	cstr := C.getConnectedSSID()
	if cstr == nil {
		return "", nil // Not connected to WiFi
	}
	defer C.free(unsafe.Pointer(cstr))
	return C.GoString(cstr), nil
}
