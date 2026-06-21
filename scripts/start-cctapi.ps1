param(
  [int]$Port = 3007,
  [string]$ProjectRoot = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path,
  [switch]$NoBrowser
)

$ErrorActionPreference = 'Stop'

$exePath = Join-Path $ProjectRoot 'one-api.exe'
$logDir = Join-Path $ProjectRoot 'logs'
$stdoutLog = Join-Path $logDir 'one-api.stdout.log'
$stderrLog = Join-Path $logDir 'one-api.stderr.log'

if (-not (Test-Path $exePath)) {
  throw "one-api.exe not found: $exePath"
}

if (-not (Test-Path $logDir)) {
  New-Item -ItemType Directory -Path $logDir | Out-Null
}

$listeners = Get-NetTCPConnection -LocalPort $Port -State Listen -ErrorAction SilentlyContinue
if ($listeners) {
  $pids = $listeners | Select-Object -ExpandProperty OwningProcess -Unique
  Write-Host "CCT API is already listening on port $Port. PID: $($pids -join ', ')"
  if (-not $NoBrowser) {
    Start-Process "http://localhost:$Port"
  }
  exit 0
}

$env:PORT = [string]$Port
Start-Process `
  -FilePath $exePath `
  -WorkingDirectory $ProjectRoot `
  -WindowStyle Hidden `
  -RedirectStandardOutput $stdoutLog `
  -RedirectStandardError $stderrLog

Start-Sleep -Seconds 2

$started = Get-NetTCPConnection -LocalPort $Port -State Listen -ErrorAction SilentlyContinue
if (-not $started) {
  throw "CCT API did not start on port $Port. Check logs: $stdoutLog and $stderrLog"
}

$startedPids = $started | Select-Object -ExpandProperty OwningProcess -Unique
Write-Host "CCT API started on http://localhost:$Port. PID: $($startedPids -join ', ')"

if (-not $NoBrowser) {
  Start-Process "http://localhost:$Port"
}
