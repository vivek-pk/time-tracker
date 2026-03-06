package main

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework CoreLocation -framework Foundation -framework CoreFoundation -framework AppKit

#import <CoreLocation/CoreLocation.h>
#import <Foundation/Foundation.h>
#import <CoreFoundation/CoreFoundation.h>
#import <AppKit/AppKit.h>

typedef struct {
    double lat;
    double lon;
    double accuracy;
    int    ok;
    int    denied;
} LocResult;

@interface _TrackerLocHelper : NSObject <CLLocationManagerDelegate>
@property (nonatomic, strong) CLLocationManager *mgr;
@property (nonatomic, strong) CLLocation *fix;
@property (nonatomic) BOOL done;
@property (nonatomic) BOOL denied;
@property (nonatomic) BOOL requested; // guard: only call requestLocation once
@end

@implementation _TrackerLocHelper

- (void)locationManager:(CLLocationManager *)m
      didUpdateLocations:(NSArray<CLLocation *> *)locs {
    CLLocation *l = locs.lastObject;
    if (l.horizontalAccuracy > 0 && l.horizontalAccuracy < 5000) {
        self.fix = l;
    }
    self.done = YES;
}

- (void)locationManager:(CLLocationManager *)m didFailWithError:(NSError *)e {
    fprintf(stderr, "location-helper: error domain=%s code=%ld desc=%s\n",
            e.domain.UTF8String, (long)e.code, e.localizedDescription.UTF8String);
    // kCLErrorLocationUnknown (0) is transient – keep waiting; anything else is fatal
    if (e.domain == kCLErrorDomain && e.code == kCLErrorLocationUnknown) return;
    self.done = YES;
}

// macOS 11+ unified authorization callback
- (void)locationManagerDidChangeAuthorization:(CLLocationManager *)m {
    [self _handleStatus:m.authorizationStatus manager:m];
}

// Pre-macOS 11 authorization callback
- (void)locationManager:(CLLocationManager *)m
    didChangeAuthorizationStatus:(CLAuthorizationStatus)s {
    [self _handleStatus:s manager:m];
}

- (void)_handleStatus:(CLAuthorizationStatus)s manager:(CLLocationManager *)m {
    if (s == kCLAuthorizationStatusDenied || s == kCLAuthorizationStatusRestricted) {
        self.denied = YES; self.done = YES; return;
    }
    // kCLAuthorizationStatusAuthorized == 3 (macOS < 11 / "Always" on macOS 11+)
    // kCLAuthorizationStatusAuthorizedWhenInUse == 4 (macOS 11+, iOS 8+)
    BOOL authorized = (s == kCLAuthorizationStatusAuthorized);
#ifdef kCLAuthorizationStatusAuthorizedWhenInUse
    authorized = authorized || (s == kCLAuthorizationStatusAuthorizedWhenInUse);
#endif
    if (authorized && !self.requested) {
        self.requested = YES;
        [m startUpdatingLocation]; // restart after auth is confirmed
    }
}
@end

static LocResult fetchGPS(int timeoutSecs) {
    LocResult r = {0, 0, 0, 0, 0};

    if (![CLLocationManager locationServicesEnabled]) {
        fprintf(stderr, "location-helper: Location Services disabled system-wide\n");
        fprintf(stderr, "  Enable: System Settings > Privacy & Security > Location Services\n");
        r.denied = 1; return r;
    }

    // Activate as a foreground app so macOS locationd can show the permission dialog.
    // This is a no-op when already authorized but required for the first-run prompt.
    NSApplication *app = [NSApplication sharedApplication];
    [app setActivationPolicy:NSApplicationActivationPolicyAccessory]; // no dock icon
    [app activateIgnoringOtherApps:YES];

    CLLocationManager  *mgr    = [[CLLocationManager alloc] init];
    _TrackerLocHelper  *helper = [[_TrackerLocHelper alloc] init];
    helper.mgr          = mgr;
    mgr.delegate        = helper;
    mgr.desiredAccuracy = kCLLocationAccuracyKilometer; // fast WiFi fix; plenty for attendance

    // Check current authorization status — if already authorized, kick off
    // requestLocation immediately; otherwise the delegate callback will do it
    // once the user grants permission.
    CLAuthorizationStatus cur = mgr.authorizationStatus;
    BOOL alreadyAuthorized = (cur == kCLAuthorizationStatusAuthorized);
#ifdef kCLAuthorizationStatusAuthorizedWhenInUse
    alreadyAuthorized = alreadyAuthorized || (cur == kCLAuthorizationStatusAuthorizedWhenInUse);
#endif
    if (cur == kCLAuthorizationStatusDenied || cur == kCLAuthorizationStatusRestricted) {
        r.denied = 1; return r;
    }
    // For CLI tools, calling requestLocation directly triggers the macOS
    // permission prompt when status is NotDetermined. The auth delegate
    // callback will call it again once authorized (guarded by helper.requested).
    // Do NOT set helper.requested yet — let _handleStatus do that after auth.
    // Use startUpdatingLocation rather than requestLocation: CLLMs with
    // requestWhenInUse+NotDetermined can silently fail in non-bundle context.
    [mgr startUpdatingLocation];

    // Poll: spin the CF run loop until done or timeout.
    NSDate *deadline = [NSDate dateWithTimeIntervalSinceNow:(double)timeoutSecs];
    while (!helper.done) {
        if ([[NSDate date] compare:deadline] == NSOrderedDescending) break;
        CFRunLoopRunInMode(kCFRunLoopDefaultMode, 0.1, true);
    }

    [mgr stopUpdatingLocation];
    if (helper.fix == nil && !helper.denied) {
        fprintf(stderr, "location-helper: no fix received (authStatus=%d)\n", (int)mgr.authorizationStatus);
    }
    if (helper.denied) { r.denied = 1; return r; }
    if (helper.fix) {
        r.lat      = helper.fix.coordinate.latitude;
        r.lon      = helper.fix.coordinate.longitude;
        r.accuracy = helper.fix.horizontalAccuracy;
        r.ok       = 1;
    }
    return r;
}
*/
import "C"

import (
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"time"

	"github.com/vivek/time-tracker/internal/location"
)

func main() {
	// Lock this goroutine to the OS thread so CGo runs on the true main thread.
	// CLLocationManager requires a run loop on the creating thread.
	runtime.LockOSThread()
	outPath := location.SharedFilePath
	if len(os.Args) > 1 {
		outPath = os.Args[1]
	}

	result := C.fetchGPS(30)

	if result.denied != 0 {
		fmt.Fprintln(os.Stderr, "location-helper: permission denied")
		fmt.Fprintln(os.Stderr, "  Grant access: System Settings > Privacy & Security > Location Services")
		os.Exit(2)
	}
	if result.ok == 0 {
		fmt.Fprintln(os.Stderr, "location-helper: no GPS fix within timeout")
		fmt.Fprintln(os.Stderr, "  If the permission dialog did not appear, grant access to the terminal app:")
		fmt.Fprintln(os.Stderr, "  System Settings > Privacy & Security > Location Services > Terminal (or Code)")
		os.Exit(1)
	}

	info := location.Info{
		Latitude:  float64(result.lat),
		Longitude: float64(result.lon),
		Accuracy:  float64(result.accuracy),
		UpdatedAt: time.Now().UTC(),
	}

	if err := location.WriteToFile(outPath, info); err != nil {
		fmt.Fprintf(os.Stderr, "location-helper: write %s: %v\n", outPath, err)
		os.Exit(1)
	}

	out, _ := json.Marshal(info)
	fmt.Printf("location-helper: ok %s\n", out)
}
