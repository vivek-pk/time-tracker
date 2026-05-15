# Time Tracker — Single-Command Setup Guide

You can install and uninstall the Time Tracker on any platform using a single line of code in your terminal. This will automatically download the correct architecture (Intel/ARM), extract it, install the background daemon/task, and add the location helper.

## macOS

Open **Terminal** and paste the following command:

**Install:**
```bash
sudo /bin/bash -c "$(curl -fsSL https://github.com/vivek-pk/time-tracker/releases/latest/download/setup.sh)"
```

**⚠️ Important: Location Permission Required**

After installation, you **must grant location permission** for the tracker to log your work location:

1. A permission dialog may appear automatically - click **"Allow"**
2. If no dialog appears, open **System Settings** manually:
   - Go to: **System Settings** (or System Preferences) → **Privacy & Security** → **Location Services**
   - Find: **"time-tracker-location"** in the list
   - Enable: Check the box next to it

You can verify location is working by checking the logs:
```bash
sudo tail -20 /var/log/time-tracker/output.log
```
Look for a line like: `location: lat=... lon=...` with a fresh timestamp (not "stale").

**Uninstall:**
```bash
sudo /bin/bash -c "$(curl -fsSL https://github.com/vivek-pk/time-tracker/releases/latest/download/setup.sh)" -- --uninstall
```
*(The uninstaller will prompt you if you want to keep or delete your local database and logs).*

**View Logs:**
```bash
tail -f /var/log/time-tracker/output.log
```

---

## Windows

Open **PowerShell as Administrator** (Search for PowerShell in Start Menu → Right Click → Run as Administrator) and paste:

**Install:**
```powershell
$f="$env:TEMP\tt-setup.ps1"; Invoke-WebRequest -UseBasicParsing -Uri "https://github.com/vivek-pk/time-tracker/releases/latest/download/setup.ps1" -OutFile $f; & $f; Remove-Item $f
```

**Uninstall:**
```powershell
$f="$env:TEMP\tt-setup.ps1"; Invoke-WebRequest -UseBasicParsing -Uri "https://github.com/vivek-pk/time-tracker/releases/latest/download/setup.ps1" -OutFile $f; $env:TIME_TRACKER_UNINSTALL="1"; & $f; Remove-Item $f
```

**View Logs:**
```powershell
Get-Content "C:\ProgramData\time-tracker\logs\output.log" -Tail 50 -Wait
```

---

## Linux

Open your terminal and paste the following command. You will be prompted for your sudo password:

**Install:**
```bash
curl -fsSL https://github.com/vivek-pk/time-tracker/releases/latest/download/setup-linux.sh | sudo bash
```

**Uninstall:**
```bash
curl -fsSL https://github.com/vivek-pk/time-tracker/releases/latest/download/setup-linux.sh | sudo bash -s -- --uninstall
```

**View Logs:**
```bash
journalctl -u time-tracker -f
```

---

### What these scripts do:
1. Detect your system's architecture (`amd64` vs `arm64`)
2. Download the appropriate compiled release from GitHub.
3. Place the binaries in standard OS locations (`/usr/local/bin` for Unix, `C:\ProgramData` for Windows).
4. Register and start the background daemon/task (`launchd` on macOS, `systemd` on Linux, Scheduled Task on Windows).
5. Clean up all temporary files used during installation.
