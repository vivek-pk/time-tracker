# install-windows.ps1 - Install time-tracker as a Windows Service.
#
# Must be run as Administrator.
# Usage: .\scripts\install-windows.ps1 [-BinaryPath .\bin\time-tracker.exe]

param(
    [string]$BinaryPath = ".\bin\time-tracker.exe"
)

$ErrorActionPreference = "Stop"

# -- Configuration -------------------------------------------------------
$ServiceName = "TimeTracker"
$DisplayName = "Time Tracker - Activity Monitor"
$Description = "Monitors keyboard/mouse activity and syncs attendance data."
$InstallDir  = "$env:ProgramData\time-tracker"
$BinaryDst   = "$InstallDir\time-tracker.exe"
$DbDir       = "$InstallDir"
$LogDir      = "$InstallDir\logs"

# -- Admin check ---------------------------------------------------------
$isAdmin = ([Security.Principal.WindowsPrincipal] [Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
if (-not $isAdmin) {
    Write-Error "This script must be run as Administrator."
    exit 1
}

# -- Sanity check ---------------------------------------------------------
if (-not (Test-Path $BinaryPath)) {
    Write-Error "Binary not found at $BinaryPath - run 'make build' or 'make build-windows' first."
    exit 1
}

# -- Stop existing service ------------------------------------------------
$existingService = Get-Service -Name $ServiceName -ErrorAction SilentlyContinue
if ($existingService) {
    Write-Host '  [+] Stopping existing service...'
    Stop-Service -Name $ServiceName -Force -ErrorAction SilentlyContinue
    Start-Sleep -Seconds 2
    Write-Host '  [+] Removing existing service...'
    sc.exe delete $ServiceName | Out-Null
    Start-Sleep -Seconds 1
}

# -- Create directories ---------------------------------------------------
Write-Host '  [+] Creating directories'
New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
New-Item -ItemType Directory -Path $LogDir -Force | Out-Null

# -- Install binary -------------------------------------------------------
Write-Host ('  [+] Installing binary -> {0}' -f $BinaryDst)
Copy-Item -Path $BinaryPath -Destination $BinaryDst -Force

# -- Create Windows Service -----------------------------------------------
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

# -- Start service --------------------------------------------------------
Write-Host '  [+] Starting service...'
Start-Service -Name $ServiceName
Start-Sleep -Seconds 2

$svc = Get-Service -Name $ServiceName
if ($svc.Status -eq 'Running') {
    Write-Host '  [+] Service is running'
} else {
    Write-Host ('  [!] Service may not have started. Check: Get-Service {0}' -f $ServiceName)
    Write-Host ('  [!] Logs: {0}\' -f $LogDir)
}

Write-Host ''
Write-Host 'Installation complete.'
Write-Host '  Config      : embedded in binary (config.json at build time)'
Write-Host ('  Database    : {0}\tracker.db' -f $DbDir)
Write-Host ('  Logs        : {0}\' -f $LogDir)
Write-Host ''
Write-Host ('To check status: Get-Service {0}' -f $ServiceName)
Write-Host ('To view logs:    Get-Content "{0}\output.log" -Tail 50 -Wait' -f $LogDir)

