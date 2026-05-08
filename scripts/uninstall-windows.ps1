# uninstall-windows.ps1 - Remove the time-tracker Scheduled Task and files.
#
# Must be run as Administrator.

param(
    [switch]$KeepData
)

$ErrorActionPreference = "Stop"

$TaskName    = "TimeTracker"
$InstallDir  = "$env:ProgramData\time-tracker"

# -- Admin check ---------------------------------------------------------
$isAdmin = ([Security.Principal.WindowsPrincipal] [Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
if (-not $isAdmin) {
    Write-Error "This script must be run as Administrator."
    exit 1
}

# -- Stop and remove scheduled task ----------------------------------------
$existingTask = Get-ScheduledTask -TaskName $TaskName -ErrorAction SilentlyContinue
if ($existingTask) {
    Write-Host '  [+] Stopping task...'
    Stop-ScheduledTask -TaskName $TaskName -ErrorAction SilentlyContinue
    Start-Sleep -Seconds 2
    Write-Host '  [+] Removing task...'
    Unregister-ScheduledTask -TaskName $TaskName -Confirm:$false
    Write-Host '  [+] Task removed'
} else {
    Write-Host ('  [!] Task ''{0}'' not found - skipping' -f $TaskName)
}

# Also clean up any old Windows Service from previous installs
$existingSvc = Get-Service -Name $TaskName -ErrorAction SilentlyContinue
if ($existingSvc) {
    Write-Host '  [+] Removing old Windows Service...'
    Stop-Service -Name $TaskName -Force -ErrorAction SilentlyContinue
    Start-Sleep -Seconds 2
    sc.exe delete $TaskName | Out-Null
}

# -- Remove binary --------------------------------------------------------
$binaryPath = "$InstallDir\time-tracker.exe"
if (Test-Path $binaryPath) {
    Write-Host '  [+] Removing binary'
    Remove-Item -Path $binaryPath -Force
}

# -- Remove data ----------------------------------------------------------
if (-not $KeepData) {
    $response = Read-Host ('  [?] Delete all data, logs, and config at {0}? [y/N]' -f $InstallDir)
    if ($response -match '^[Yy]') {
        Write-Host ('  [+] Removing {0}' -f $InstallDir)
        Remove-Item -Path $InstallDir -Recurse -Force -ErrorAction SilentlyContinue
    } else {
        Write-Host ('  [!] Data retained at {0}' -f $InstallDir)
    }
} else {
    Write-Host ('  [!] -KeepData specified - data retained at {0}' -f $InstallDir)
}

Write-Host ''
Write-Host 'Uninstall complete.'
