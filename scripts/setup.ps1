# setup.ps1 — Unified Windows Setup Script
#
# Install or uninstall with a single command from PowerShell (Run as Administrator):
#
# Install:
#   Invoke-Expression (Invoke-WebRequest -Uri "https://github.com/vivek-pk/time-tracker/releases/latest/download/setup.ps1").Content
#
# Uninstall:
#   $env:TIME_TRACKER_UNINSTALL="1"; Invoke-Expression (Invoke-WebRequest -Uri "https://github.com/vivek-pk/time-tracker/releases/latest/download/setup.ps1").Content

$ErrorActionPreference = "Stop"

# ── Configuration ─────────────────────────────────────────────────────────────
$repoUrl = "https://github.com/vivek-pk/time-tracker"
$releaseUrl = "$repoUrl/releases/latest/download"
$InstallDir = "$env:ProgramData\time-tracker"

# ── Admin check ───────────────────────────────────────────────────────────────
$isAdmin = ([Security.Principal.WindowsPrincipal] [Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
if (-not $isAdmin) {
    Write-Error "This script must be run as Administrator."
    exit 1
}

# ── Uninstall flow ────────────────────────────────────────────────────────────
if ($env:TIME_TRACKER_UNINSTALL -eq "1") {
    Write-Host "`n  [+] Time Tracker Uninstall Started`n" -ForegroundColor Cyan
    
    $UninstallScript = "$InstallDir\uninstall-windows.ps1"
    if (Test-Path $UninstallScript) {
        & $UninstallScript
    } else {
        Write-Host "  [!] Uninstall script not found at $UninstallScript. Cannot proceed automatically." -ForegroundColor Yellow
        Write-Host "  [!] You can manually stop and delete the 'TimeTracker' service and remove the $InstallDir folder." -ForegroundColor Yellow
    }
    exit 0
}

# ── Install flow ──────────────────────────────────────────────────────────────
Write-Host "`n  [+] Time Tracker Install Started`n" -ForegroundColor Cyan

# Determine Architecture
$arch = $env:PROCESSOR_ARCHITECTURE
$fileArch = "amd64"
if ($arch -eq "ARM64") {
    $fileArch = "arm64"
}

$zipName = "time-tracker-windows-$fileArch.zip"
$zipUrl = "$releaseUrl/$zipName"
$tempDir = Join-Path $env:TEMP "time-tracker-install"
$zipPath = Join-Path $tempDir $zipName

Write-Host "  [+] Architecture detected: $fileArch"

# Clean temp directory if exists
if (Test-Path $tempDir) {
    Remove-Item -Path $tempDir -Recurse -Force
}
New-Item -ItemType Directory -Path $tempDir | Out-Null

# Download
Write-Host "  [+] Downloading $zipUrl ..."
try {
    Invoke-WebRequest -Uri $zipUrl -OutFile $zipPath
} catch {
    Write-Error "Failed to download release. Ensure you have internet access and the release exists."
    exit 1
}

# Extract
Write-Host "  [+] Extracting to $tempDir ..."
Expand-Archive -Path $zipPath -DestinationPath $tempDir -Force

# Run Installer
$installerScript = Join-Path $tempDir "scripts/install-windows.ps1"
$binaryPath = Join-Path $tempDir "time-tracker-windows-$fileArch.exe"
$locationBinary = Join-Path $tempDir "time-tracker-location-windows-$fileArch.exe"

if (-not (Test-Path $installerScript)) {
    Write-Error "Installer script not found inside the downloaded archive."
    exit 1
}

Write-Host "  [+] Running installer script..."
& $installerScript -BinaryPath $binaryPath

# Install location helper
if (Test-Path $locationBinary) {
    Write-Host "  [+] Installing location helper..."
    Copy-Item $locationBinary "$InstallDir\time-tracker-location.exe" -Force
    
    # Add to system PATH so the daemon can find it
    $currentPath = [Environment]::GetEnvironmentVariable("Path", "Machine")
    if ($currentPath -notlike "*time-tracker*") {
        Write-Host "  [+] Adding $InstallDir to System PATH..."
        [Environment]::SetEnvironmentVariable("Path", "$currentPath;$InstallDir", "Machine")
    }
} else {
    Write-Host "  [!] Location helper binary not found in archive. Falling back to IP-based location." -ForegroundColor Yellow
}

# Cleanup
Write-Host "  [+] Cleaning up temp files..."
Remove-Item -Path $tempDir -Recurse -Force

Write-Host "`n  [+] Time Tracker Installation Complete! 🚀" -ForegroundColor Green
Write-Host "      - To check status: Get-Service TimeTracker"
Write-Host "      - Logs and config: $InstallDir"
Write-Host "      - Please ensure Location Services are enabled in Windows Settings if you want GPS/WiFi accuracy.`n"
