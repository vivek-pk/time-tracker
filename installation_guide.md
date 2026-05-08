# Time Tracker — Single-Command Setup Guide

You can install and uninstall the Time Tracker on any platform using a single line of code in your terminal. This will automatically download the correct architecture (Intel/ARM), extract it, install the background service, and add the location helper.

## macOS

Open **Terminal** and paste the following command:

**Install:**
```bash
sudo /bin/bash -c "$(curl -fsSL https://github.com/vivek-pk/time-tracker/releases/latest/download/setup.sh)"
```

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
Invoke-Expression (Invoke-WebRequest -Uri "https://github.com/vivek-pk/time-tracker/releases/latest/download/setup.ps1").Content
```

**Uninstall:**
```powershell
$env:TIME_TRACKER_UNINSTALL="1"; Invoke-Expression (Invoke-WebRequest -Uri "https://github.com/vivek-pk/time-tracker/releases/latest/download/setup.ps1").Content
```

**View Logs:**
```powershell
Get-EventLog -LogName Application -Source TimeTracker -Newest 50
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
4. Register and start the background daemon (`launchd` on macOS, `systemd` on Linux, `Windows Services` on Windows).
5. Clean up all temporary files used during installation.
