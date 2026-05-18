# Time Tracker

A background daemon that monitors keyboard/mouse activity, records work sessions, and syncs attendance data to a remote API. Runs silently as a system service with automatic sleep/wake detection and optional GPS location tracking.

## Features

- **Activity Monitoring** — Detects active, idle, and offline (sleep) states via OS-native idle APIs
- **Session Recording** — Stores sessions in a local SQLite database
- **Automatic Sync** — Pushes attendance data at configurable morning and evening times
- **Location Tracking** — Captures GPS coordinates at session start (CoreLocation on macOS, GeoClue2 on Linux, Windows Location API, IP geolocation fallback)
- **Tamper-Proof** — Runs as a system-level service that auto-restarts on failure
- **Cross-Platform** — macOS, Linux, and Windows

## Release Downloads

| Platform | Architecture | Download |
|----------|-------------|----------|
| **macOS** | Universal (Apple Silicon + Intel) | `time-tracker-darwin-universal.tar.gz` |
| **Linux** | x86_64 (amd64) | `time-tracker-linux-amd64.tar.gz` |
| **Linux** | ARM64 (aarch64) | `time-tracker-linux-arm64.tar.gz` |
| **Windows** | x86_64 (amd64) | `time-tracker-windows-amd64.zip` |
| **Windows** | ARM64 | `time-tracker-windows-arm64.zip` |

Download the latest release from the [Releases page](https://github.com/vivek-pk/time-tracker/releases/latest).

Each package includes:
- `time-tracker` — the main daemon binary
- `time-tracker-location` — the location helper binary
- Platform-specific install/uninstall scripts

---

## macOS

### Quick Install (One Command)

The easiest way to install on macOS — downloads the latest release and sets everything up:

```bash
sudo /bin/bash -c "$(curl -fsSL https://github.com/vivek-pk/time-tracker/releases/latest/download/setup.sh)"
```

This will:
1. Download the latest universal binary (works on both Apple Silicon and Intel Macs)
2. Install the daemon and location helper
3. Prompt for Location Services permission
4. Set up the launchd system daemon (auto-starts on boot)
5. Display your **Machine ID** for HRMS registration

### Manual Install (From Release)

**1. Download and extract**

```bash
curl -fsSL https://github.com/vivek-pk/time-tracker/releases/latest/download/time-tracker-darwin-universal.tar.gz -o time-tracker.tar.gz
tar -xzf time-tracker.tar.gz
```

**2. Install the binaries**

```bash
# Copy the main daemon
sudo cp time-tracker /usr/local/bin/time-tracker
sudo chmod 755 /usr/local/bin/time-tracker

# Copy the location helper app bundle
sudo cp -R time-tracker-location.app /Applications/time-tracker-location.app
```

**3. Grant location permission**

Open the location helper once to trigger the macOS permission dialog:

```bash
open /Applications/time-tracker-location.app
```

Click **"Allow"** when prompted. You can verify permission was granted:

```
System Settings → Privacy & Security → Location Services → time-tracker-location ✓
```

**4. Create the launchd daemon plist**

```bash
sudo tee /Library/LaunchDaemons/com.timetracker.daemon.plist > /dev/null << 'EOF'
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN"
    "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.timetracker.daemon</string>
    <key>ProgramArguments</key>
    <array>
        <string>/usr/local/bin/time-tracker</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>ThrottleInterval</key>
    <integer>10</integer>
    <key>ProcessType</key>
    <string>Background</string>
    <key>Nice</key>
    <integer>10</integer>
    <key>StandardOutPath</key>
    <string>/var/log/time-tracker/output.log</string>
    <key>StandardErrorPath</key>
    <string>/var/log/time-tracker/error.log</string>
    <key>AbandonProcessGroup</key>
    <true/>
</dict>
</plist>
EOF

sudo chown root:wheel /Library/LaunchDaemons/com.timetracker.daemon.plist
sudo chmod 644 /Library/LaunchDaemons/com.timetracker.daemon.plist
```

**5. Create directories and start**

```bash
sudo mkdir -p /var/lib/time-tracker /var/log/time-tracker
sudo launchctl load -w /Library/LaunchDaemons/com.timetracker.daemon.plist
```

**6. Verify it's running**

```bash
sudo launchctl list | grep timetracker
tail -f /var/log/time-tracker/output.log
```

### macOS Uninstall

**Quick uninstall (one command):**

```bash
sudo /bin/bash -c "$(curl -fsSL https://github.com/vivek-pk/time-tracker/releases/latest/download/setup.sh)" -- --uninstall
```

**Manual uninstall:**

```bash
# Stop and unload the daemon
sudo launchctl unload -w /Library/LaunchDaemons/com.timetracker.daemon.plist

# Remove files
sudo rm -f /Library/LaunchDaemons/com.timetracker.daemon.plist
sudo rm -f /Library/LaunchAgents/com.timetracker.locationhelper.plist
sudo rm -f /usr/local/bin/time-tracker
sudo rm -rf /Applications/time-tracker-location.app

# Remove data (optional — prompts you)
sudo rm -rf /var/lib/time-tracker    # database
sudo rm -rf /var/log/time-tracker    # logs
```

---

## Linux

### Install (From Release)

**1. Download and extract**

For x86_64 (most desktops/servers):

```bash
curl -fsSL https://github.com/vivek-pk/time-tracker/releases/latest/download/time-tracker-linux-amd64.tar.gz -o time-tracker.tar.gz
tar -xzf time-tracker.tar.gz
```

For ARM64 (Raspberry Pi 4/5, AWS Graviton, etc.):

```bash
curl -fsSL https://github.com/vivek-pk/time-tracker/releases/latest/download/time-tracker-linux-arm64.tar.gz -o time-tracker.tar.gz
tar -xzf time-tracker.tar.gz
```

**2. Install the binaries**

```bash
sudo cp time-tracker-linux-amd64 /usr/local/bin/time-tracker
sudo chmod 755 /usr/local/bin/time-tracker

# Location helper (optional — provides better accuracy than IP geolocation)
sudo cp time-tracker-location-linux-amd64 /usr/local/bin/time-tracker-location
sudo chmod 755 /usr/local/bin/time-tracker-location
```

> **Note:** Replace `amd64` with `arm64` if you downloaded the ARM64 package.

**3. Install the systemd service**

```bash
sudo cp systemd/time-tracker.service /etc/systemd/system/time-tracker.service
sudo chmod 644 /etc/systemd/system/time-tracker.service
```

Or create the unit file manually:

```bash
sudo tee /etc/systemd/system/time-tracker.service > /dev/null << 'EOF'
[Unit]
Description=Time Tracker — Activity Monitoring Daemon
After=network.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=/usr/local/bin/time-tracker
Restart=always
RestartSec=10
Nice=10
StandardOutput=append:/var/log/time-tracker/output.log
StandardError=append:/var/log/time-tracker/error.log

NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/var/lib/time-tracker /var/log/time-tracker /tmp
PrivateTmp=false

[Install]
WantedBy=multi-user.target
EOF
```

**4. Create directories**

```bash
sudo mkdir -p /var/lib/time-tracker /var/log/time-tracker
```

**5. Enable and start**

```bash
sudo systemctl daemon-reload
sudo systemctl enable time-tracker
sudo systemctl start time-tracker
```

**6. Verify it's running**

```bash
systemctl status time-tracker
journalctl -u time-tracker -f
```

### Location Helper — Linux

The location helper provides better accuracy than the built-in IP geolocation fallback by using **GeoClue2** (WiFi/cell triangulation, available on GNOME/KDE desktops).

| Method | Accuracy | Requirement |
|--------|----------|-------------|
| GeoClue2 (via helper) | ~100m–1km | Desktop with `geoclue-2.0` installed |
| IP Geolocation (built-in fallback) | ~5km (city) | Internet connection |

**Requirements for GeoClue2:**

```bash
# Debian/Ubuntu
sudo apt install geoclue-2.0

# Fedora/RHEL
sudo dnf install geoclue2

# Arch
sudo pacman -S geoclue
```

The location helper is automatically called by the daemon if it's found in `PATH`. No extra configuration needed.

> **Headless servers:** If GeoClue2 is not available (no desktop environment), the daemon automatically falls back to IP geolocation — no setup required.

### Linux Uninstall

```bash
# Stop and disable the service
sudo systemctl stop time-tracker
sudo systemctl disable time-tracker

# Remove service file and reload
sudo rm -f /etc/systemd/system/time-tracker.service
sudo systemctl daemon-reload

# Remove binaries
sudo rm -f /usr/local/bin/time-tracker
sudo rm -f /usr/local/bin/time-tracker-location

# Remove data (optional)
sudo rm -rf /var/lib/time-tracker    # database
sudo rm -rf /var/log/time-tracker    # logs
```

---

## Windows

### Quick Install (One Command)

The easiest way to install on Windows — downloads the latest release, extracts it, and installs the service automatically.

Open **PowerShell as Administrator** and run:

```powershell
$f="$env:TEMP\tt-setup.ps1"; Invoke-WebRequest -UseBasicParsing -Uri "https://github.com/vivek-pk/time-tracker/releases/latest/download/setup.ps1" -OutFile $f; & $f; Remove-Item $f
```

This script will automatically:
1. Detect your architecture (amd64 or arm64)
2. Download and extract the latest release
3. Install the daemon binary and location helper
4. Add the installation directory to your System PATH
5. Create and start a background **Scheduled Task** named `TimeTracker`

### Manual Install (From Release)

**1. Download and extract**

Download the appropriate zip file from the [Releases page](https://github.com/vivek-pk/time-tracker/releases/latest):

- **x86_64 (most PCs):** `time-tracker-windows-amd64.zip`
- **ARM64 (Surface Pro X, etc.):** `time-tracker-windows-arm64.zip`

Right-click the downloaded zip → **Extract All** to a folder (e.g., `C:\time-tracker-setup`).

**2. Run the install script**

Open **PowerShell as Administrator** (right-click PowerShell → "Run as administrator"):

```powershell
cd C:\time-tracker-setup
.\scripts\install-windows.ps1 -BinaryPath .\time-tracker-windows-amd64.exe
```

This will:
1. Create `C:\ProgramData\time-tracker\` directory
2. Copy the binary to `C:\ProgramData\time-tracker\time-tracker.exe`
3. Create a **Scheduled Task** named "TimeTracker" (starts at user logon)
4. Start the task in the current desktop session

**3. Install the location helper (optional)**

```powershell
Copy-Item .\time-tracker-location-windows-amd64.exe "C:\ProgramData\time-tracker\time-tracker-location.exe"

# Add to system PATH so the daemon can find it
$currentPath = [Environment]::GetEnvironmentVariable("Path", "Machine")
if ($currentPath -notlike "*time-tracker*") {
    [Environment]::SetEnvironmentVariable("Path", "$currentPath;C:\ProgramData\time-tracker", "Machine")
}
```

**4. Enable Location Services (for best accuracy)**

For the location helper to use WiFi/GPS (not just IP geolocation):

```
Settings → Privacy & Security → Location → Turn ON "Location services"
```

**5. Verify it's running**

```powershell
Get-ScheduledTask -TaskName TimeTracker
```

You should see `State: Running`.

### Location Helper — Windows

The location helper provides better accuracy than the built-in IP geolocation fallback by using the **Windows Location API** (WiFi triangulation, cell towers, GPS).

| Method | Accuracy | Requirement |
|--------|----------|-------------|
| Windows Location API (via helper) | ~100m–1km | Location Services enabled in Settings |
| IP Geolocation (built-in fallback) | ~5km (city) | Internet connection |

**Enabling Location Services:**

```
Settings → Privacy & Security → Location
  ✓ Location services: ON
  ✓ Let apps access your location: ON
```

If Location Services is disabled, the daemon automatically falls back to IP geolocation.

### Windows Uninstall

Open **PowerShell as Administrator**:

```powershell
cd C:\time-tracker-setup
.\scripts\uninstall-windows.ps1
```

**Manual uninstall:**

```powershell
# Stop and remove the scheduled task
Stop-ScheduledTask -TaskName TimeTracker
Unregister-ScheduledTask -TaskName TimeTracker -Confirm:$false

# Remove files (prompts for confirmation)
Remove-Item -Path "C:\ProgramData\time-tracker" -Recurse -Force

# Remove from PATH (optional)
$currentPath = [Environment]::GetEnvironmentVariable("Path", "Machine")
$newPath = ($currentPath -split ";" | Where-Object { $_ -notlike "*time-tracker*" }) -join ";"
[Environment]::SetEnvironmentVariable("Path", $newPath, "Machine")
```

---

## Configuration

All runtime configuration comes from the embedded `internal/config/config.json` file baked into the binary at build time. Runtime `.env` overrides are intentionally disabled so every platform uses the same config source.

### Configuration Options

```json
{
  "sync_api_url": "https://your-api-endpoint.example.com/attendance",
  "sync_api_key": "your-api-key",
  "morning_sync_hour": 6,
  "evening_sync_hour": 18,
  "evening_sync_minute": 30,
  "idle_threshold_minutes": 5,
  "poll_interval_seconds": 30,
  "retention_days": 15,
  "sync_timeout_seconds": 30,
  "realtime_sync": false
}
```

After editing the config, restart the service:

```bash
# macOS
sudo launchctl kickstart -k system/com.timetracker.daemon

# Linux
sudo systemctl restart time-tracker

# Windows (PowerShell as Admin)
Stop-ScheduledTask -TaskName TimeTracker
Start-ScheduledTask -TaskName TimeTracker
```

---

## Location Tracking Summary

Location is captured at the start of each session. The system uses a layered fallback:

```
┌─────────────────────────────────────────────────┐
│  1. Platform-Native API  (~10m–1km accuracy)    │
│     • macOS: CoreLocation (GPS/WiFi)            │
│     • Linux: GeoClue2 D-Bus (WiFi/cell)         │
│     • Windows: Location API (WiFi/cell/GPS)     │
├─────────────────────────────────────────────────┤
│  2. IP Geolocation       (~5km, city-level)     │
│     • Built into the daemon (all platforms)     │
│     • No setup required, just needs internet    │
├─────────────────────────────────────────────────┤
│  3. No Location                                 │
│     • Sessions recorded with 0,0 coordinates    │
│     • Happens if offline + no cached fix        │
└─────────────────────────────────────────────────┘
```

The location helper binary is **optional**. If not installed, the daemon uses IP geolocation automatically.

---

## Service Management Quick Reference

| Action | macOS | Linux | Windows |
|--------|-------|-------|---------|
| **Start** | `sudo launchctl load -w /Library/LaunchDaemons/com.timetracker.daemon.plist` | `sudo systemctl start time-tracker` | `Start-ScheduledTask -TaskName TimeTracker` |
| **Stop** | `sudo launchctl unload /Library/LaunchDaemons/com.timetracker.daemon.plist` | `sudo systemctl stop time-tracker` | `Stop-ScheduledTask -TaskName TimeTracker` |
| **Restart** | `sudo launchctl kickstart -k system/com.timetracker.daemon` | `sudo systemctl restart time-tracker` | `Stop-ScheduledTask -TaskName TimeTracker; Start-ScheduledTask -TaskName TimeTracker` |
| **Status** | `sudo launchctl list \| grep timetracker` | `systemctl status time-tracker` | `Get-ScheduledTask -TaskName TimeTracker` |
| **Logs** | `tail -f /var/log/time-tracker/output.log` | `journalctl -u time-tracker -f` | `Get-Content "C:\ProgramData\time-tracker\logs\output.log" -Tail 50 -Wait` |

---

## Building From Source

Requires [Go 1.24+](https://go.dev/dl/).

```bash
git clone https://github.com/vivek-pk/time-tracker.git
cd time-tracker

# Build for current OS
make build

# Cross-compile for all platforms
make build-all

# Platform-specific builds
make build-linux          # Linux amd64 + location helper
make build-linux-arm64    # Linux arm64 + location helper
make build-windows        # Windows amd64 + location helper
make build-windows-arm64  # Windows arm64 + location helper

# macOS-specific
make build-universal      # Universal binary (arm64 + amd64) + signed location helper
make sign-location        # Sign the CoreLocation helper for macOS

# Install on current OS
sudo make install
```

> **Note:** macOS builds require CGo (for IOKit idle detection). Linux and Windows builds are pure Go — no CGo, no C compiler needed.

### Local Debug Build

A debug build embeds a separate config (`config.debug.json`) that points to `http://localhost:3000/api/machine-sync/sync` and enables `realtime_sync: true`. Use this to test against a local API server without touching the production binary or config.

**macOS**

```bash
# Build the debug binary (./bin/time-tracker-debug)
make build-debug

# Install it as the running daemon (replaces the current daemon)
sudo make install-debug

# Verify it is using localhost
sudo launchctl list | grep timetracker
tail -f /var/log/time-tracker/output.log
```

To switch back to production:

```bash
sudo make install
```

**Linux**

```bash
# Build the debug binary (./bin/time-tracker-debug)
make build-debug

# Install it
sudo make install-debug

# Verify
systemctl status time-tracker
journalctl -u time-tracker -f
```

To switch back to production:

```bash
sudo make install
```

**Windows** (PowerShell as Administrator)

```powershell
# Build the debug binary
go build -tags debug -ldflags "-s -w" -o bin\time-tracker-debug.exe .\cmd\tracker

# Stop the running task and replace the binary
Stop-ScheduledTask -TaskName TimeTracker
Copy-Item bin\time-tracker-debug.exe "C:\ProgramData\time-tracker\time-tracker.exe" -Force
Start-ScheduledTask -TaskName TimeTracker

# Verify
Get-ScheduledTask -TaskName TimeTracker
Get-Content "C:\ProgramData\time-tracker\logs\output.log" -Tail 20 -Wait
```

To switch back to production:

```powershell
Stop-ScheduledTask -TaskName TimeTracker
Copy-Item bin\time-tracker-windows-amd64.exe "C:\ProgramData\time-tracker\time-tracker.exe" -Force
Start-ScheduledTask -TaskName TimeTracker
```

> The debug binary is identical to production except for the embedded config — same binary size optimisations, same location helper, same service setup.

---

## License

MIT
