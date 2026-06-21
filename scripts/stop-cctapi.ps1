param(
  [int]$Port = 3007
)

$ErrorActionPreference = 'Stop'

$listeners = Get-NetTCPConnection -LocalPort $Port -State Listen -ErrorAction SilentlyContinue
if (-not $listeners) {
  Write-Host "No CCT API process is listening on port $Port."
  exit 0
}

$pids = $listeners | Select-Object -ExpandProperty OwningProcess -Unique
foreach ($pidValue in $pids) {
  Stop-Process -Id $pidValue -Force -ErrorAction SilentlyContinue
}

Write-Host "Stopped CCT API on port $Port. PID: $($pids -join ', ')"
