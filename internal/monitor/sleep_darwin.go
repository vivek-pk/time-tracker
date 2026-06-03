//go:build darwin

package monitor

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework IOKit -framework CoreFoundation

#include <IOKit/IOKitLib.h>
#include <IOKit/pwr_mgt/IOPMLib.h>
#include <IOKit/IOMessage.h>
#include <CoreFoundation/CoreFoundation.h>

// Forward declarations for the Go callback.
extern void goSleepCallback(int messageType);

static io_connect_t root_port;
static IONotificationPortRef notifyPortRef;
static io_object_t notifierObject;

// sleepCallbackC is invoked by IOKit on sleep/wake events.
static void sleepCallbackC(void *refCon, io_service_t service,
                           natural_t messageType, void *messageArgument) {
    switch (messageType) {
    case kIOMessageCanSystemSleep:
        // Idle sleep — allow it immediately.
        IOAllowPowerChange(root_port, (long)messageArgument);
        break;
    case kIOMessageSystemWillSleep:
        // Forced sleep (lid close, user action, or scheduled).
        goSleepCallback(1); // 1 = going to sleep
        IOAllowPowerChange(root_port, (long)messageArgument);
        break;
    case kIOMessageSystemHasPoweredOn:
        // Full user-initiated wake.
        goSleepCallback(2); // 2 = woke up
        break;
    }
}

// registerSleepWake registers for IOKit power notifications and runs
// CFRunLoop on the current thread. This function blocks forever.
// Returns 0 on success setup, -1 on failure (before entering run loop).
static int registerSleepWake(void) {
    root_port = IORegisterForSystemPower(
        NULL, &notifyPortRef, sleepCallbackC, &notifierObject);
    if (root_port == 0) {
        return -1;
    }
    CFRunLoopAddSource(
        CFRunLoopGetCurrent(),
        IONotificationPortGetRunLoopSource(notifyPortRef),
        kCFRunLoopDefaultMode);
    CFRunLoopRun(); // blocks forever
    return 0; // unreachable
}
*/
import "C"

import (
	"log"
	"sync/atomic"
)

// sleepFlag is set to 1 when the system is sleeping, 0 when awake.
// Accessed atomically from the monitor poll goroutine and the IOKit
// callback goroutine.
var sleepFlag atomic.Bool

//export goSleepCallback
func goSleepCallback(messageType C.int) {
	switch messageType {
	case 1: // going to sleep
		log.Println("monitor: system going to sleep (IOKit)")
		sleepFlag.Store(true)
	case 2: // woke up
		log.Println("monitor: system woke up (IOKit)")
		sleepFlag.Store(false)
	}
}

// startSleepWatcher launches a goroutine that listens for IOKit
// sleep/wake notifications. It sets the global sleepFlag accordingly.
// On failure, the flag stays false (awake) and the monitor falls back
// to its existing poll-gap heuristic.
func startSleepWatcher() {
	go func() {
		log.Println("monitor: starting IOKit sleep/wake watcher")
		ret := C.registerSleepWake()
		if ret != 0 {
			log.Println("monitor: IOKit sleep watcher failed to register (falling back to gap heuristic)")
		}
		// If CFRunLoopRun returns (shouldn't), log it.
		log.Println("monitor: IOKit sleep watcher exited unexpectedly")
	}()
}

// isSystemSleeping returns true if IOKit has reported the system is
// currently sleeping and has not yet delivered a wake notification.
func isSystemSleeping() bool {
	return sleepFlag.Load()
}
