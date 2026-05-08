# install-windows.ps1 — Install time-tracker as a Windows Service.
#
# Must be run as Administrator.
# Usage: .\scripts\install-windows.ps1 [-BinaryPath .\bin\time-tracker.exe]

param(
    [string]$BinaryPath = ".\bin\time-tracker.exe"
)

$ErrorActionPreference = "Stop"

# ── Configuration ─────────────────────────────────────────────────────────────
$ServiceName = "TimeTracker"
$DisplayName = "Time Tracker - Activity Monitor"
$Description = "Monitors keyboard/mouse activity and syncs attendance data."
$InstallDir  = "$env:ProgramData\time-tracker"
$BinaryDst   = "$InstallDir\time-tracker.exe"
$EnvFile     = "$InstallDir\.env"
$DbDir       = "$InstallDir"
$LogDir      = "$InstallDir\logs"

# ── Admin check ───────────────────────────────────────────────────────────────
$isAdmin = ([Security.Principal.WindowsPrincipal] [Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
if (-not $isAdmin) {
    Write-Error "This script must be run as Administrator."
    exit 1
}

# ── Sanity check ──────────────────────────────────────────────────────────────
if (-not (Test-Path $BinaryPath)) {
    Write-Error "Binary not found at $BinaryPath — run 'make build' or 'make build-windows' first."
    exit 1
}

# ── Stop existing service ─────────────────────────────────────────────────────
$existingService = Get-Service -Name $ServiceName -ErrorAction SilentlyContinue
if ($existingService) {
    Write-Host "  [+] Stopping existing service..."
    Stop-Service -Name $ServiceName -Force -ErrorAction SilentlyContinue
    Start-Sleep -Seconds 2
    Write-Host "  [+] Removing existing service..."
    sc.exe delete $ServiceName | Out-Null
    Start-Sleep -Seconds 1
}

# ── Create directories ────────────────────────────────────────────────────────
Write-Host '  [+] Creating directories'
New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
New-Item -ItemType Directory -Path $LogDir -Force | Out-Null

# ── Install binary ────────────────────────────────────────────────────────────
Write-Host ('  [+] Installing binary -> {0}' -f $BinaryDst)
Copy-Item -Path $BinaryPath -Destination $BinaryDst -Force

# ── Install config ────────────────────────────────────────────────────────────
if (Test-Path $EnvFile) {
    Write-Host ('  [!] Config {0} already exists - not overwriting.' -f $EnvFile)
} else {
    Write-Host ('  [+] Installing default config -> {0}' -f $EnvFile)
    @"
# $InstallDir\.env  (OPTIONAL OVERRIDE FILE)
# All configuration is embedded in the binary (config.json).
# Uncomment variables below ONLY if you need to override them.

# SYNC_API_URL=https://your-api-endpoint.example.com/attendance
# SYNC_API_KEY=
# MORNING_SYNC_HOUR=6
# EVENING_SYNC_HOUR=18
# EVENING_SYNC_MINUTE=30
# IDLE_THRESHOLD_MINUTES=5
# POLL_INTERVAL_SECONDS=30
# DB_PATH=$InstallDir\tracker.db
# LOG_PATH=$LogDir
# RETENTION_DAYS=3
# SYNC_TIMEOUT_SECONDS=30
"@ | Set-Content -Path $EnvFile -Encoding UTF8
    Write-Host ('  [!] IMPORTANT: edit {0} and set SYNC_API_URL before starting the service' -f $EnvFile)
}

# ── Create Windows Service ───────────────────────────────────────────────────
Write-Host ('  [+] Creating Windows Service ''{0}''' -f $ServiceName)

# Use sc.exe to create the service with environment variable
sc.exe create $ServiceName `
    binPath= "$BinaryDst" `
    start= auto `
    DisplayName= "$DisplayName" | Out-Null

# Set description
sc.exe description $ServiceName "$Description" | Out-Null

# Set recovery: restart on failure after 10 seconds
sc.exe failure $ServiceName reset= 86400 actions= restart/10000/restart/10000/restart/30000 | Out-Null

# Set the ENV_FILE environment variable for the service via registry
$regPath = "HKLM:\SYSTEM\CurrentControlSet\Services\$ServiceName"
$envValue = "ENV_FILE=$EnvFile"
# Create or update the Environment multi-string value
Set-ItemProperty -Path $regPath -Name "Environment" -Value @($envValue) -Type MultiString

# ── Start service ─────────────────────────────────────────────────────────────
Write-Host '  [+] Starting service...'
Start-Service -Name $ServiceName
Start-Sleep -Seconds 2

$svc = Get-Service -Name $ServiceName
if ($svc.Status -eq 'Running') {
    Write-Host '  [+] Service is running'
} else {
    Write-Host ('  [!] Service may not have started. Check: Get-Service {0}' -f $ServiceName)
    Write-Host ('  [!] Logs: Get-EventLog -LogName Application -Source {0}' -f $ServiceName)
}

Write-Host ''
Write-Host 'Installation complete.'
Write-Host ('  Config file : {0}' -f $EnvFile)
Write-Host ('  Database    : {0}\tracker.db' -f $DbDir)
Write-Host ('  Logs        : {0}\' -f $LogDir)
Write-Host ''
Write-Host ('Edit {0} with the correct SYNC_API_URL,' -f $EnvFile)
Write-Host ('then restart: Restart-Service {0}' -f $ServiceName)
