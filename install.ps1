#!/usr/bin/env pwsh
param(
  [ValidateSet('install','uninstall','status','help')]
  [string]$Command = 'help'
)

if (-not ([Security.Principal.WindowsPrincipal]::new(
  [Security.Principal.WindowsIdentity]::GetCurrent()
).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator))) {
  Write-Warning "Not running as administrator. Scheduled task registration may fail."
}

$AppName = "clipboardsync"
$TaskName = "ClipboardSync"
$Version = "0.1.0"
$BinDir = "$env:LOCALAPPDATA\$AppName"
$BinPath = "$BinDir\$AppName.exe"

function Get-ArchBinary {
  $pattern = Join-Path $PSScriptRoot "${AppName}.exe"
  if (-not (Test-Path $pattern)) {
    Write-Error "Binary not found: $pattern. Re-download the package."
    exit 1
  }
  return $pattern
}

function Show-Help {
  @"
Usage: .\install.ps1 -Command {install|uninstall|status|help}

Commands:
  install    Copy binary, register scheduled task (login trigger)
  uninstall  Remove scheduled task and binary
  status     Show scheduled task status
  help       Show this help
"@
}

function Install-Service {
  Write-Host "==> Installing Clipboard Sync..."

  $src = Get-ArchBinary
  Write-Host "  Binary: $src"

  Write-Host "  Copying to $BinPath..."
  $null = New-Item -ItemType Directory -Force -Path $BinDir
  Copy-Item -Path $src -Destination $BinPath -Force

  Write-Host "  Creating config..."
  $configDir = "$env:APPDATA\clipboardsync"
  $null = New-Item -ItemType Directory -Force -Path $configDir
  $configFile = "$configDir\trusted.json"
  if (-not (Test-Path $configFile)) {
    Set-Content $configFile '{"trusted_uuids":[],"devices":{}}'
  }
  Write-Host "  Run 'clipboardsync trust' to configure trusted devices."

  Write-Host "  Creating scheduled task (login trigger)..."
  $action = New-ScheduledTaskAction -Execute $BinPath
  $trigger = New-ScheduledTaskTrigger -AtLogOn -User $env:USERNAME
  $settings = New-ScheduledTaskSettingsSet -AllowStartIfOnBatteries -DontStopIfGoingOnBatteries -StartWhenAvailable
  $principal = New-ScheduledTaskPrincipal -UserId $env:USERNAME -LogonType Interactive -RunLevel Limited
  $null = Register-ScheduledTask -TaskName $TaskName -Action $action -Trigger $trigger -Settings $settings -Principal $principal -Force

  Write-Host "  Starting now..."
  $proc = Start-Process -WindowStyle Hidden -FilePath $BinPath -PassThru
  Write-Host "  PID: $($proc.Id)"

  Write-Host "==> Done! ClipboardSync is running (background, no window)."
  Write-Host "    Task Manager: look for clipboardsync.exe (PID $($proc.Id))"
  Write-Host "    Auto-start:   will start automatically at next login (scheduled task)"
  Write-Host "    Stop:         Taskkill /PID $($proc.Id) /F"
}

function Uninstall-Service {
  Write-Host "==> Uninstalling Clipboard Sync..."

  Write-Host "  Removing scheduled task..."
  $null = Stop-ScheduledTask -TaskName $TaskName -ErrorAction SilentlyContinue
  $null = Unregister-ScheduledTask -TaskName $TaskName -Confirm:$false -ErrorAction SilentlyContinue

  Write-Host "  Removing binary..."
  if (Test-Path $BinPath) {
    Remove-Item -Path $BinPath -Force
  }
  if (Test-Path $BinDir -And -not (Get-ChildItem $BinDir)) {
    Remove-Item -Path $BinDir -Force
  }

  Write-Host "==> Done."
}

function Get-Status {
  Write-Host "==> Scheduled task status:"
  $task = Get-ScheduledTask -TaskName $TaskName -ErrorAction SilentlyContinue
  if ($task) {
    Write-Host "  State: $($task.State)"
    $nextRun = ($task | Get-ScheduledTaskInfo).NextRunTime
    Write-Host "  Next Run Time: $(if ($nextRun) { $nextRun } else { 'N/A' })"
  } else {
    Write-Host "  Not registered"
  }
  Write-Host ""
  if (Test-Path $BinPath) {
    Write-Host "  Binary: installed"
  } else {
    Write-Host "  Binary: not found"
  }
}

switch ($Command) {
  'install'   { Install-Service }
  'uninstall' { Uninstall-Service }
  'status'    { Get-Status }
  'help'      { Show-Help }
}
