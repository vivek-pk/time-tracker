//go:build ignore

package main

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework IOKit -framework CoreFoundation

#include <stdlib.h>
#include <stdint.h>
#include <string.h>
#include <CoreFoundation/CoreFoundation.h>
#include <IOKit/IOKitLib.h>

double hidIdleSeconds(void) {
    io_iterator_t iter = 0;
    io_registry_entry_t entry = 0;
    CFMutableDictionaryRef props = NULL;
    double result = -1.0;

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
import "fmt"

func main() {
	fmt.Printf("Idle: %f\n", float64(C.hidIdleSeconds()))
}
