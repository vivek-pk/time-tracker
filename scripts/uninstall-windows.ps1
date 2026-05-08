# uninstall-windows.ps1 — Remove the time-tracker Windows Service.
#
# Must be run as Administrator.

param(
    [switch]$KeepData
)

$ErrorActionPreference = "Stop"

$ServiceName = "TimeTracker"
$InstallDir  = "$env:ProgramData\time-tracker"

# ── Admin check ───────────────────────────────────────────────────────────────
$isAdmin = ([Security.Principal.WindowsPrincipal] [Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
if (-not $isAdmin) {
    Write-Error "This script must be run as Administrator."
    exit 1
}

# ── Stop and remove service ───────────────────────────────────────────────────
$existingService = Get-Service -Name $ServiceName -ErrorAction SilentlyContinue
if ($existingService) {
    Write-Host "  [+] Stopping service..."
    Stop-Service -Name $ServiceName -Force -ErrorAction SilentlyContinue
    Start-Sleep -Seconds 2
    Write-Host "  [+] Removing service..."
    sc.exe delete $ServiceName | Out-Null
    Write-Host "  [+] Service removed"
} else {
    Write-Host "  [!] Service '$ServiceName' not found — skipping"
}

# ── Remove binary ─────────────────────────────────────────────────────────────
$binaryPath = "$InstallDir\time-tracker.exe"
if (Test-Path $binaryPath) {
    Write-Host "  [+] Removing binary"
    Remove-Item -Path $binaryPath -Force
}

# ── Remove data ───────────────────────────────────────────────────────────────
if (-not $KeepData) {
    $response = Read-Host "  [?] Delete all data, logs, and config at $InstallDir? [y/N]"
    if ($response -match '^[Yy]') {
        Write-Host "  [+] Removing $InstallDir"
        Remove-Item -Path $InstallDir -Recurse -Force -ErrorAction SilentlyContinue
    } else {
        Write-Host "  [!] Data retained at $InstallDir"
    }
} else {
    Write-Host "  [!] -KeepData specified — data retained at $InstallDir"
}

Write-Host ""
Write-Host "Uninstall complete."
