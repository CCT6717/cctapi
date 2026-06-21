param(
  [string]$TaskName = 'CCT API Local Server',
  [int]$Port = 3007,
  [string]$ProjectRoot = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
)

$ErrorActionPreference = 'Stop'

$scriptPath = Join-Path $ProjectRoot 'scripts\start-cctapi.ps1'
if (-not (Test-Path $scriptPath)) {
  throw "Start script not found: $scriptPath"
}

$action = New-ScheduledTaskAction `
  -Execute 'powershell.exe' `
  -Argument "-NoProfile -ExecutionPolicy Bypass -File `"$scriptPath`" -Port $Port -NoBrowser"

$trigger = New-ScheduledTaskTrigger -AtLogOn
$principal = New-ScheduledTaskPrincipal -UserId $env:USERNAME -LogonType Interactive -RunLevel Limited
$settings = New-ScheduledTaskSettingsSet `
  -AllowStartIfOnBatteries `
  -DontStopIfGoingOnBatteries `
  -ExecutionTimeLimit (New-TimeSpan -Hours 0) `
  -MultipleInstances IgnoreNew

Register-ScheduledTask `
  -TaskName $TaskName `
  -Action $action `
  -Trigger $trigger `
  -Principal $principal `
  -Settings $settings `
  -Force | Out-Null

Write-Host "Installed autostart task '$TaskName' for http://localhost:$Port"
Write-Host "Run now: powershell -ExecutionPolicy Bypass -File `"$scriptPath`" -Port $Port"
