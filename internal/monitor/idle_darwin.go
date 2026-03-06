package monitor

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework IOKit -framework CoreFoundation

#include <stdlib.h>
#include <stdint.h>
#include <string.h>
#include <CoreFoundation/CoreFoundation.h>
#include <IOKit/IOKitLib.h>

// hidIdleSeconds returns seconds since the last HID (keyboard/mouse) event.
// Uses the IOHIDSystem kernel service which is readable from any process,
// including a root launchd system daemon with no display session.
// Returns -1 on any error.
double hidIdleSeconds(void) {
    io_iterator_t iter = 0;
    io_registry_entry_t entry = 0;
    CFMutableDictionaryRef props = NULL;
    double result = -1.0;

    // kIOMainPortDefault replaces kIOMasterPortDefault (deprecated in macOS 12).
    // Both are defined as 0, so this cast is always safe.
    mach_port_t mainPort = (mach_port_t)0;

    kern_return_t kr = IOServiceGetMatchingServices(
        mainPort,
        IOServiceMatching("IOHIDSystem"),
        &iter);
    if (kr != KERN_SUCCESS) return result;

    entry = IOIteratorNext(iter);
    IOObjectRelease(iter);
    if (!entry) return result;

    kr = IORegistryEntryCreateCFProperties(entry, &props, kCFAllocatorDefault, 0);
    IOObjectRelease(entry);
    if (kr != KERN_SUCCESS) return result;

    CFTypeRef obj = CFDictionaryGetValue(props, CFSTR("HIDIdleTime"));
    if (obj) {
        uint64_t ns = 0;
        CFTypeID typeID = CFGetTypeID(obj);
        if (typeID == CFDataGetTypeID()) {
            const UInt8 *bytes = CFDataGetBytePtr((CFDataRef)obj);
            memcpy(&ns, bytes, sizeof(ns));
        } else {
            CFNumberGetValue((CFNumberRef)obj, kCFNumberSInt64Type, &ns);
        }
        result = (double)ns / 1.0e9;
    }
    CFRelease(props);
    return result;
}
*/
import "C"

// idleSeconds returns the system-wide HID idle time in seconds.
// A negative return value means the probe failed; callers treat that as active.
func idleSeconds() float64 {
	return float64(C.hidIdleSeconds())
}
