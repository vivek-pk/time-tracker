# install-windows.ps1 - Install time-tracker as a Scheduled Task.
#
# Must be run as Administrator.
# Usage: .\scripts\install-windows.ps1 [-BinaryPath .\bin\time-tracker.exe]

param(
    [string]$BinaryPath = ".\bin\time-tracker.exe"
)

$ErrorActionPreference = "Stop"

# -- Configuration -------------------------------------------------------
$TaskName    = "TimeTracker"
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

# -- Stop existing task/service -------------------------------------------
# Remove old scheduled task if it exists
$existingTask = Get-ScheduledTask -TaskName $TaskName -ErrorAction SilentlyContinue
if ($existingTask) {
    Write-Host '  [+] Stopping existing scheduled task...'
    Stop-ScheduledTask -TaskName $TaskName -ErrorAction SilentlyContinue
    Start-Sleep -Seconds 1
    Write-Host '  [+] Removing existing scheduled task...'
    Unregister-ScheduledTask -TaskName $TaskName -Confirm:$false
}

# Also clean up any old Windows Service from previous installs
$existingSvc = Get-Service -Name $TaskName -ErrorAction SilentlyContinue
if ($existingSvc) {
    Write-Host '  [+] Removing old Windows Service...'
    Stop-Service -Name $TaskName -Force -ErrorAction SilentlyContinue
    Start-Sleep -Seconds 2
    sc.exe delete $TaskName | Out-Null
}

# -- Create directories ---------------------------------------------------
Write-Host '  [+] Creating directories'
New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
New-Item -ItemType Directory -Path $LogDir -Force | Out-Null

# -- Install binary -------------------------------------------------------
Write-Host ('  [+] Installing binary -> {0}' -f $BinaryDst)
Copy-Item -Path $BinaryPath -Destination $BinaryDst -Force

# -- Create Scheduled Task ------------------------------------------------
# Runs in the user's desktop session (NOT Session 0) so idle detection works.
Write-Host '  [+] Creating scheduled task (runs at user logon)...'

$action = New-ScheduledTaskAction -Execute $BinaryDst -WorkingDirectory $InstallDir

# Trigger: at logon of any user
$trigger = New-ScheduledTaskTrigger -AtLogOn

# Settings: restart on failure, don't stop on idle, run indefinitely
$settings = New-ScheduledTaskSettingsSet `
    -AllowStartIfOnBatteries `
    -DontStopIfGoingOnBatteries `
    -StartWhenAvailable `
    -RestartCount 999 `
    -RestartInterval (New-TimeSpan -Seconds 30) `
    -ExecutionTimeLimit (New-TimeSpan -Days 0)

# Run as SYSTEM but allow interaction with desktop is not needed;
# instead, run as the current logged-in user's context.
# Using SYSTEM with -RunLevel Highest ensures it auto-starts.
$principal = New-ScheduledTaskPrincipal -GroupId "BUILTIN\Users" -RunLevel Limited

Register-ScheduledTask `
    -TaskName $TaskName `
    -Action $action `
    -Trigger $trigger `
    -Settings $settings `
    -Principal $principal `
    -Description $Description `
    -Force | Out-Null

# -- Start the task now ---------------------------------------------------
Write-Host '  [+] Starting task...'
Start-ScheduledTask -TaskName $TaskName
Start-Sleep -Seconds 3

$task = Get-ScheduledTask -TaskName $TaskName
if ($task.State -eq 'Running') {
    Write-Host '  [+] Task is running'
} else {
    Write-Host ('  [!] Task state: {0}. It will start automatically at next logon.' -f $task.State)
}

# -- Read Machine ID (same logic as the Go binary) ----------------------------
$machineId = ""
try {
    $regKey = Get-ItemProperty -Path "HKLM:\SOFTWARE\Microsoft\Cryptography" -Name "MachineGuid" -ErrorAction Stop
    $machineId = $regKey.MachineGuid
} catch {
    $machineId = $env:COMPUTERNAME
}

Write-Host ''
Write-Host '==========================================================='
Write-Host 'Installation complete.'
Write-Host '==========================================================='
Write-Host ''
Write-Host ('  Machine ID  : {0}' -f $machineId)
Write-Host '  Config      : embedded in binary (config.json at build time)'
Write-Host ('  Database    : {0}\tracker.db' -f $DbDir)
Write-Host ('  Logs        : {0}\' -f $LogDir)
Write-Host ''
Write-Host '  ** Copy the Machine ID above to register this machine in your HRMS **'
Write-Host ''
Write-Host ('To check status: Get-ScheduledTask -TaskName {0}' -f $TaskName)
Write-Host ('To view logs:    Get-Content "{0}\output.log" -Tail 50 -Wait' -f $LogDir)
