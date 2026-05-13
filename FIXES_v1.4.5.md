# Fixes in v1.4.5

## Critical Bugs Fixed

### 1. Location Capture Broken After Multi-Platform Support
**Problem:** Location helper was writing to user-specific temp directory (`/var/folders/.../T/`) while the root daemon was looking in a different temp directory. This caused the "location file not updated after 35s timeout" error.

**Root Cause:** 
- `os.TempDir()` returns different paths for different users and processes
- Location helper (runs as logged-in user) writes to `/var/folders/{user}/T/time-tracker-location.json`
- Daemon (runs as root) reads from `/tmp/` or root's temp directory
- File isolation prevented communication between the two processes

**Solution:**
- Changed `SharedFilePath` from `filepath.Join(os.TempDir(), "time-tracker-location.json")` to hardcoded `/tmp/time-tracker-location.json`
- This ensures both root daemon and user LaunchAgent access the same shared file

### 2. CoreLocation Permission Inconsistency
**Problem:** LaunchAgent was running the standalone binary at `/usr/local/bin/time-tracker-location`, which had a different code signature than the app bundle that received permission.

**Root Cause:**
- macOS TCC (Transparency, Consent, and Control) system caches permissions by **bundle ID + code signature hash (CDHash)**
- Permission was granted to app bundle at `/Applications/time-tracker-location.app`
- LaunchAgent ran standalone binary with different CDHash
- Result: `authStatus=0` (permission denied) despite UI showing "granted"

**Solution:**
- Updated `LaunchAgent` plist to run `/Applications/time-tracker-location.app/Contents/MacOS/time-tracker-location` instead of `/usr/local/bin/time-tracker-location`
- Modified `install.sh` to copy the signed app bundle to `/Applications/`
- This maintains consistent code signature across all invocations

### 3. Missing Go Build Tags
**Problem:** All platform-specific files (darwin, linux, windows) were being compiled together, causing symbol conflicts for `idleSeconds()` and `refreshAndRefreshAndReadLocation()`.

**Solution:**
- Added `//go:build darwin` to `internal/monitor/idle_darwin.go` and `location_darwin.go`
- Added `//go:build linux` to `internal/monitor/idle_linux.go`  
- Added `//go:build windows` to `internal/monitor/idle_windows.go`

### 4. Confusing Lack of Poll Logging
**Problem:** The monitor was polling every 30 seconds correctly, but only logged when state changed. This made it appear that polling wasn't working when the user stayed in the same state (active/idle) for extended periods.

**Example:** User stays active for 10 minutes → only 1 log line (session creation), creating the impression that monitoring stopped.

**Solution:**
- Added verbose logging for **every poll cycle** showing:
  - Current idle time (e.g., `idle=84.1s`)
  - Current state (e.g., `state=active`)
  - Session ID (e.g., `session=10`)
  - Whether state changed or not (e.g., `(no change)`)
  
**Example Logs:**
```
[time-tracker] 2026/05/14 01:35:01 monitor: new session id=10 state=active idle=77.1s
[time-tracker] 2026/05/14 01:35:31 monitor: poll idle=29.1s state=active session=10 (no change)
[time-tracker] 2026/05/14 01:36:01 monitor: poll idle=59.1s state=active session=10 (no change)
[time-tracker] 2026/05/14 01:36:31 monitor: poll idle=89.1s state=active session=10 (no change)
[time-tracker] 2026/05/14 01:41:31 monitor: poll idle=389.1s state=active session=10 (no change)
[time-tracker] 2026/05/14 01:42:01 monitor: state changed active->idle, closing session id=10
[time-tracker] 2026/05/14 01:42:01 monitor: new session id=11 state=idle idle=419.1s
```

Now you can clearly see:
- ✅ Monitor **IS** polling every 30 seconds
- ✅ Idle time is being tracked continuously
- ✅ State transitions happen when idle exceeds 5 minutes (300 seconds)

## Files Changed

### Core Files
- `internal/location/location.go` - Changed SharedFilePath to `/tmp/time-tracker-location.json`
- `launchd/com.timetracker.locationhelper.plist` - Updated ProgramArguments to use app bundle path
- `scripts/install.sh` - Added app bundle installation to `/Applications/`

### Platform-Specific Files (Build Tags)
- `internal/monitor/idle_darwin.go` - Added `//go:build darwin`
- `internal/monitor/idle_linux.go` - Added `//go:build linux`
- `internal/monitor/idle_windows.go` - Added `//go:build windows`
- `internal/monitor/location_darwin.go` - Added `//go:build darwin`

### Documentation
- `scripts/install.sh` - Enhanced permission instructions
- `scripts/setup.sh` - Added 5-attempt retry logic for permissions
- `installation_guide.md` - Added prominent permission warning section

## Testing & Verification

### How Activity Tracking Works

The monitor uses a **state machine** with three states:

**1. ACTIVE State**
- **Trigger:** Keyboard/mouse activity detected within last 5 minutes
- **Idle time:** < 300 seconds (5 minutes)
- **What it means:** User is actively working

**2. IDLE State**
- **Trigger:** No keyboard/mouse activity for more than 5 minutes
- **Idle time:** ≥ 300 seconds (5 minutes)
- **What it means:** User is away from keyboard (break, meeting, etc.)

**3. OFFLINE State**
- **Trigger:** System sleep detected (>60 second gap between polls)
- **What it means:** Machine was asleep/suspended

**State Transitions:**
```
active (idle<5min) ──> idle (idle≥5min) ──> active (keyboard press)
     │                                            │
     └──> offline (sleep) ──> active (wake) ─────┘
```

**Polling Behavior:**
- Monitor polls **every 30 seconds**
- Checks current idle time using HID system
- Compares idle time against 5-minute threshold
- Creates new session if state changes
- Keeps same session open if state doesn't change

### Understanding the Logs

**Normal Operation (No State Change):**
```
[time-tracker] monitor: poll idle=45.2s state=active session=10 (no change)
```
- `idle=45.2s` - User last interacted 45 seconds ago
- `state=active` - Current state is ACTIVE
- `session=10` - Currently tracking session ID 10
- `(no change)` - State hasn't changed, session continues

**State Transition (Active → Idle):**
```
[time-tracker] monitor: poll idle=315.8s state=active session=10 (no change)
[time-tracker] monitor: state changed active->idle, closing session id=10
[time-tracker] monitor: new session id=11 state=idle idle=315.8s
```
- Idle time exceeded 300 seconds (5 minutes)
- Monitor closes active session (ID 10)
- Creates new idle session (ID 11)

**Sleep/Wake Detection:**
```
[time-tracker] monitor: sleep gap detected 242s
[time-tracker] monitor: new session id=12 state=active
```
- Gap > 60 seconds detected between polls
- System was asleep for ~4 minutes
- Creates offline session for sleep period (immediately closed)
- Creates new active session after wake

### Verify Location Capture Works
```bash
# 1. Trigger location helper manually
launchctl kickstart -k gui/$(id -u)/com.timetracker.locationhelper

# 2. Wait 30 seconds for GPS fix, then check file
cat /tmp/time-tracker-location.json

# 3. Should show recent timestamp (within last minute):
# {
#   "lat": 11.188939...,
#   "lon": 75.808179...,
#   "accuracy_m": 40,
#   "updated_at": "2026-05-13T19:33:58..."
# }
```

### Verify Daemon Reads Location
```bash
# Restart daemon and check logs
sudo launchctl kickstart -k system/com.timetracker.daemon
sleep 3
sudo tail -20 /var/log/time-tracker/output.log

# Should see:
# [time-tracker] location: lat=11.18894 lon=75.80818 accuracy=40m (fixed Xs ago)
```

### Verify Idle Detection Works
```bash
# Check daemon logs for idle detection
sudo tail -20 /var/log/time-tracker/output.log | grep "idle detection"

# Should see:
# [time-tracker] monitor: idle detection OK (current idle: X.Xs)
```

## Important Notes

### Code Signing and Permissions
- The app bundle at `/Applications/time-tracker-location.app` must maintain a **stable code signature**
- Running `make install` rebuilds and re-signs, creating a **new CDHash**
- macOS TCC remembers the old CDHash, causing permission denial
- **Solution:** Don't run `make install` repeatedly during development

### Permission Grant Process
1. First installation triggers permission dialog via `open -W /Applications/time-tracker-location.app`
2. User grants permission in System Settings
3. macOS stores permission for bundle ID + CDHash pair
4. All subsequent runs with same signature inherit permission
5. Re-signing invalidates permission, requiring new grant

### Cleanup for Fresh Installation
If permissions get corrupted, perform complete cleanup:
```bash
# 1. Unload services
launchctl bootout gui/$(id -u)/com.timetracker.locationhelper
sudo launchctl unload /Library/LaunchDaemons/com.timetracker.daemon.plist

# 2. Remove all files
sudo rm -rf /Applications/time-tracker-location.app
sudo rm -f /usr/local/bin/time-tracker*
sudo rm -f /Library/LaunchDaemons/com.timetracker.daemon.plist
sudo rm -f /Library/LaunchAgents/com.timetracker.locationhelper.plist
rm -f /tmp/time-tracker-location*

# 3. Remove from System Settings
# System Settings → Privacy & Security → Location Services
# Find "time-tracker-location" and click (-) to remove

# 4. Reinstall
sudo make install
```

## Release Information

**Version:** v1.4.5  
**Tag:** `git tag v1.4.5`  
**Commits:**
- `5b0c12e` Monitor: Add verbose logging for every poll cycle
- `6beb7f9` Install: Copy location app bundle to /Applications
- `2eb29a2` Fix: Use shared /tmp path for location file and app bundle in LaunchAgent  
- `99ebe9f` Fix: Add missing build tags for platform-specific files

**Tested On:** macOS Sonoma (ARM64)  
**Status:** ✅ Location capture working, idle detection working, daemon successfully reading location, **verbose poll logging added**
