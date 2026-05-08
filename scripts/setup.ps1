# setup.ps1 - Unified Windows Setup Script
#
# Install or uninstall with a single command from PowerShell (Run as Administrator):
#
# Install:
#   $f="$env:TEMP\tt-setup.ps1"; Invoke-WebRequest -UseBasicParsing -Uri "https://github.com/vivek-pk/time-tracker/releases/latest/download/setup.ps1" -OutFile $f; & $f; Remove-Item $f
#
# Uninstall:
#   $f="$env:TEMP\tt-setup.ps1"; Invoke-WebRequest -UseBasicParsing -Uri "https://github.com/vivek-pk/time-tracker/releases/latest/download/setup.ps1" -OutFile $f; $env:TIME_TRACKER_UNINSTALL="1"; & $f; Remove-Item $f

$ErrorActionPreference = "Stop"

# -- Configuration -------------------------------------------------------
$repoUrl = "https://github.com/vivek-pk/time-tracker"
$releaseUrl = "$repoUrl/releases/latest/download"
$InstallDir = "$env:ProgramData\time-tracker"

# -- Admin check ---------------------------------------------------------
$isAdmin = ([Security.Principal.WindowsPrincipal] [Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
if (-not $isAdmin) {
    Write-Error "This script must be run as Administrator."
    exit 1
}

# -- Uninstall flow -------------------------------------------------------
if ($env:TIME_TRACKER_UNINSTALL -eq "1") {
    Write-Host ""
    Write-Host '  [+] Time Tracker Uninstall Started' -ForegroundColor Cyan
    Write-Host ""

    $UninstallScript = "$InstallDir\uninstall-windows.ps1"
    if (Test-Path $UninstallScript) {
        & $UninstallScript
    } else {
        Write-Host ('  [!] Uninstall script not found at {0}. Cannot proceed automatically.' -f $UninstallScript) -ForegroundColor Yellow
        Write-Host ('  [!] You can manually stop and delete the TimeTracker service and remove the {0} folder.' -f $InstallDir) -ForegroundColor Yellow
    }
    exit 0
}

# -- Install flow ---------------------------------------------------------
Write-Host ""
Write-Host '  [+] Time Tracker Install Started' -ForegroundColor Cyan
Write-Host ""

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

Write-Host ('  [+] Architecture detected: {0}' -f $fileArch)

# Clean temp directory if exists
if (Test-Path $tempDir) {
    Remove-Item -Path $tempDir -Recurse -Force
}
New-Item -ItemType Directory -Path $tempDir | Out-Null

# Download
Write-Host ('  [+] Downloading {0} ...' -f $zipUrl)
try {
    Invoke-WebRequest -Uri $zipUrl -OutFile $zipPath -UseBasicParsing
} catch {
    Write-Error "Failed to download release. Ensure you have internet access and the release exists."
    exit 1
}

# Extract
Write-Host ('  [+] Extracting to {0} ...' -f $tempDir)
Expand-Archive -Path $zipPath -DestinationPath $tempDir -Force

# Run Installer
$installerScript = Join-Path $tempDir "scripts\install-windows.ps1"
$binaryPath = Join-Path $tempDir "time-tracker-windows-$fileArch.exe"
$locationBinary = Join-Path $tempDir "time-tracker-location-windows-$fileArch.exe"

if (-not (Test-Path $installerScript)) {
    Write-Error "Installer script not found inside the downloaded archive."
    exit 1
}

Write-Host '  [+] Running installer script...'
& $installerScript -BinaryPath $binaryPath

# Install location helper
if (Test-Path $locationBinary) {
    Write-Host '  [+] Installing location helper...'
    Copy-Item $locationBinary "$InstallDir\time-tracker-location.exe" -Force

    # Add to system PATH so the daemon can find it
    $currentPath = [Environment]::GetEnvironmentVariable("Path", "Machine")
    if ($currentPath -notlike "*time-tracker*") {
        Write-Host ('  [+] Adding {0} to System PATH...' -f $InstallDir)
        [Environment]::SetEnvironmentVariable("Path", "$currentPath;$InstallDir", "Machine")
    }
} else {
    Write-Host '  [!] Location helper binary not found in archive. Falling back to IP-based location.' -ForegroundColor Yellow
}

# Cleanup
Write-Host '  [+] Cleaning up temp files...'
Remove-Item -Path $tempDir -Recurse -Force

Write-Host ""
Write-Host '  [+] Time Tracker Installation Complete!' -ForegroundColor Green
Write-Host '      - To check status: Get-Service TimeTracker'
Write-Host ('      - Logs and config: {0}' -f $InstallDir)
Write-Host '      - Please ensure Location Services are enabled in Windows Settings if you want GPS/WiFi accuracy.'
Write-Host ""
