package config

// Reads the hardware serial number from the IOPlatformExpertDevice IOKit
// registry entry.  This is the same value shown in "About This Mac → Serial
// Number" and is burned into the logic board by Apple — it cannot be changed
// by a normal (or even admin) user without Apple service tools.
//
// Falls back to the sanitised hostname only if IOKit fails (e.g. VM with no
// serial number set).

/*
#cgo LDFLAGS: -framework IOKit -framework CoreFoundation
#include <stdlib.h>
#include <CoreFoundation/CoreFoundation.h>
#include <IOKit/IOKitLib.h>

// Returns a malloc'd C string containing the serial number, or NULL on error.
// Caller must free() the result.
static char* readHWSerial() {
    io_service_t expert = IOServiceGetMatchingService(
        (mach_port_t)0,
        IOServiceMatching("IOPlatformExpertDevice")
    );
    if (!expert) return NULL;

    CFStringRef ref = (CFStringRef)IORegistryEntryCreateCFProperty(
        expert,
        CFSTR("IOPlatformSerialNumber"),
        kCFAllocatorDefault,
        0
    );
    IOObjectRelease(expert);
    if (!ref) return NULL;

    CFIndex maxLen = CFStringGetMaximumSizeForEncoding(
        CFStringGetLength(ref), kCFStringEncodingUTF8) + 1;
    char* buf = (char*)malloc(maxLen);
    if (!buf) { CFRelease(ref); return NULL; }

    if (!CFStringGetCString(ref, buf, maxLen, kCFStringEncodingUTF8)) {
        free(buf);
        CFRelease(ref);
        return NULL;
    }
    CFRelease(ref);
    return buf;
}
*/
import "C"
import (
	"log"
	"os"
	"unsafe"
)

// machineID returns the hardware serial number as the stable machine
// identifier.  It is read directly from IOKit and is not user-configurable.
func machineID() string {
	ptr := C.readHWSerial()
	if ptr == nil {
		log.Println("config: IOKit serial number unavailable, falling back to hostname")
		h, _ := os.Hostname()
		return sanitiseHostname(h)
	}
	defer C.free(unsafe.Pointer(ptr))
	serial := C.GoString(ptr)
	if serial == "" {
		h, _ := os.Hostname()
		return sanitiseHostname(h)
	}
	return serial
}
