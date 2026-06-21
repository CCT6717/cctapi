param(
  [string]$BaseUrl = $(if ($env:CCT_API_BASE_URL) { $env:CCT_API_BASE_URL } else { 'http://localhost:3007' }),
  [string]$ApiToken = $env:CCT_API_TOKEN,
  [string]$Model = $env:CCT_API_MODEL,
  [string]$AdminToken = $env:CCT_ADMIN_TOKEN,
  [string]$PrimaryDeployment = $env:CCT_PRIMARY_DEPLOYMENT,
  [string[]]$FallbackDeployments = $(if ($env:CCT_FALLBACK_DEPLOYMENTS) { $env:CCT_FALLBACK_DEPLOYMENTS -split ',' } else { @() }),
  [switch]$RunFaultScenarios,
  [int]$CooldownSeconds = 120,
  [int]$TimeoutSec = 60
)

$ErrorActionPreference = 'Stop'

if ([string]::IsNullOrWhiteSpace($ApiToken)) {
  throw 'Missing API token. Set CCT_API_TOKEN or pass -ApiToken.'
}

if ([string]::IsNullOrWhiteSpace($Model)) {
  throw 'Missing model. Set CCT_API_MODEL to a fallback virtual model or pass -Model.'
}

$BaseUrl = $BaseUrl.TrimEnd('/')
$headers = @{
  Authorization = "Bearer $ApiToken"
  'Content-Type' = 'application/json'
}

$adminHeaders = $null
if (-not [string]::IsNullOrWhiteSpace($AdminToken)) {
  $adminHeaders = @{
    Authorization = "Bearer $AdminToken"
    'Content-Type' = 'application/json'
  }
}

function Write-Step {
  param([string]$Message)
  Write-Host ""
  Write-Host "==> $Message"
}

function Invoke-ChatCompletion {
  param(
    [bool]$Stream,
    [string]$Prompt = 'Reply with one short sentence for a fallback smoke test.'
  )

  $body = @{
    model = $Model
    messages = @(
      @{
        role = 'user'
        content = $Prompt
      }
    )
    max_tokens = 24
    stream = $Stream
  } | ConvertTo-Json -Depth 8

  Invoke-WebRequest `
    -Uri "$BaseUrl/v1/chat/completions" `
    -Method Post `
    -Headers $headers `
    -Body $body `
    -TimeoutSec $TimeoutSec `
    -UseBasicParsing
}

function Invoke-FallbackAdmin {
  param(
    [ValidateSet('GET', 'POST')]
    [string]$Method,
    [string]$Path,
    [object]$Body = $null
  )

  if ($null -eq $adminHeaders) {
    throw 'Missing admin token. Set CCT_ADMIN_TOKEN or pass -AdminToken.'
  }

  $request = @{
    Uri = "$BaseUrl$Path"
    Method = $Method
    Headers = $adminHeaders
    TimeoutSec = $TimeoutSec
    UseBasicParsing = $true
  }

  if ($null -ne $Body) {
    $request.Body = ($Body | ConvertTo-Json -Depth 8)
  }

  Invoke-WebRequest @request
}

function Get-FallbackMetricsSnapshot {
  $metrics = Invoke-WebRequest `
    -Uri "$BaseUrl/metrics" `
    -Method Get `
    -TimeoutSec $TimeoutSec `
    -UseBasicParsing

  $parsed = @{}
  foreach ($line in ($metrics.Content -split "`n")) {
    $line = $line.Trim()
    if ($line -eq '' -or $line.StartsWith('#')) {
      continue
    }
    $parts = $line -split '\s+'
    if ($parts.Count -ge 2) {
      $parsed[$parts[0]] = $parts[1]
    }
  }

  [pscustomobject]@{
    Raw = $metrics.Content
    Parsed = $parsed
  }
}

function Show-FallbackMetricsDelta {
  param(
    [hashtable]$Before,
    [hashtable]$After
  )

  $names = @(
    'fallback_requests_total',
    'fallback_success_total',
    'fallback_failed_total',
    'fallback_switch_total'
  )

  foreach ($name in $names) {
    $beforeValue = 0
    $afterValue = 0
    if ($Before.ContainsKey($name)) { [double]$beforeValue = $Before[$name] }
    if ($After.ContainsKey($name)) { [double]$afterValue = $After[$name] }
    Write-Host ("{0}: {1} -> {2} (delta {3})" -f $name, $beforeValue, $afterValue, ($afterValue - $beforeValue))
  }
}

function Invoke-ExpectedFailureChat {
  param([string]$ScenarioName)

  try {
    $response = Invoke-ChatCompletion -Stream:$false -Prompt "Trigger fallback smoke scenario: $ScenarioName"
    Write-Host "$ScenarioName returned HTTP $($response.StatusCode)."
    return $false
  } catch {
    Write-Host "$ScenarioName failed as expected: $($_.Exception.Message)"
    return $true
  }
}

Write-Host "Base URL: $BaseUrl"
Write-Host "Model: $Model"

Write-Step 'Running non-stream chat completion'
$metricsBefore = Get-FallbackMetricsSnapshot
$nonStream = Invoke-ChatCompletion -Stream:$false
if ($nonStream.StatusCode -lt 200 -or $nonStream.StatusCode -ge 300) {
  throw "Non-stream request failed with HTTP $($nonStream.StatusCode)."
}
$nonStreamJson = $nonStream.Content | ConvertFrom-Json
if (-not $nonStreamJson.choices -or $nonStreamJson.choices.Count -lt 1) {
  throw 'Non-stream response does not contain choices.'
}
Write-Host 'Non-stream request passed.'

Write-Step 'Running stream chat completion'
$stream = Invoke-ChatCompletion -Stream:$true
if ($stream.StatusCode -lt 200 -or $stream.StatusCode -ge 300) {
  throw "Stream request failed with HTTP $($stream.StatusCode)."
}
if ($stream.Content -notmatch 'data:') {
  throw 'Stream response does not look like an SSE response.'
}
Write-Host 'Stream request passed.'

Write-Step 'Checking fallback metrics endpoint'
$metricsAfter = Get-FallbackMetricsSnapshot
if ($metricsAfter.Raw -notmatch 'fallback_requests_total') {
  throw 'Metrics response does not contain fallback_requests_total.'
}
Write-Host 'Metrics endpoint passed.'
Show-FallbackMetricsDelta -Before $metricsBefore.Parsed -After $metricsAfter.Parsed

if (-not $RunFaultScenarios) {
  Write-Host ''
  Write-Host 'Safe fallback smoke test passed.'
  Write-Host 'Fault scenarios were skipped. Add -RunFaultScenarios with CCT_ADMIN_TOKEN, CCT_PRIMARY_DEPLOYMENT and CCT_FALLBACK_DEPLOYMENTS to test switching/recovery.'
  exit 0
}

if ($null -eq $adminHeaders) {
  throw 'RunFaultScenarios requires CCT_ADMIN_TOKEN or -AdminToken.'
}
if ([string]::IsNullOrWhiteSpace($PrimaryDeployment)) {
  throw 'RunFaultScenarios requires CCT_PRIMARY_DEPLOYMENT or -PrimaryDeployment.'
}
if ($FallbackDeployments.Count -lt 1) {
  throw 'RunFaultScenarios requires at least one fallback deployment via CCT_FALLBACK_DEPLOYMENTS or -FallbackDeployments.'
}

$allDeployments = @($PrimaryDeployment) + $FallbackDeployments

try {
  Write-Step "Simulating primary 429/upstream failure by cooling down $PrimaryDeployment"
  $beforeFault = Get-FallbackMetricsSnapshot
  Invoke-FallbackAdmin -Method POST -Path "/api/fallback/deployments/$PrimaryDeployment/cooldown?duration_seconds=$CooldownSeconds" | Out-Null
  $fallbackResponse = Invoke-ChatCompletion -Stream:$false -Prompt 'The primary deployment is cooled down. Reply with one short sentence.'
  if ($fallbackResponse.StatusCode -lt 200 -or $fallbackResponse.StatusCode -ge 300) {
    throw "Fallback request after primary cooldown failed with HTTP $($fallbackResponse.StatusCode)."
  }
  $afterFault = Get-FallbackMetricsSnapshot
  Show-FallbackMetricsDelta -Before $beforeFault.Parsed -After $afterFault.Parsed
  Write-Host 'Primary cooldown fallback scenario passed.'

  Write-Step 'Simulating all deployments failed/cooling down'
  $beforeAllFailed = Get-FallbackMetricsSnapshot
  Invoke-FallbackAdmin `
    -Method POST `
    -Path '/api/fallback/deployments/batch-cooldown' `
    -Body @{ deployment_ids = $allDeployments; duration_seconds = $CooldownSeconds } | Out-Null
  $failedAsExpected = Invoke-ExpectedFailureChat -ScenarioName 'all deployments cooled down'
  if (-not $failedAsExpected) {
    throw 'All-failed scenario did not fail. Check whether CCT_FALLBACK_DEPLOYMENTS includes every deployment in the virtual model.'
  }
  $afterAllFailed = Get-FallbackMetricsSnapshot
  Show-FallbackMetricsDelta -Before $beforeAllFailed.Parsed -After $afterAllFailed.Parsed

  Write-Step 'Recovering deployments and testing request again'
  Invoke-FallbackAdmin `
    -Method POST `
    -Path '/api/fallback/deployments/batch-recover' `
    -Body @{ deployment_ids = $allDeployments } | Out-Null
  $recoverResponse = Invoke-ChatCompletion -Stream:$false -Prompt 'Deployments were recovered. Reply with one short sentence.'
  if ($recoverResponse.StatusCode -lt 200 -or $recoverResponse.StatusCode -ge 300) {
    throw "Recovery request failed with HTTP $($recoverResponse.StatusCode)."
  }
  Write-Host 'Recovery scenario passed.'
} finally {
  Write-Step 'Cleaning up deployment states'
  foreach ($deploymentID in $allDeployments) {
    try {
      Invoke-FallbackAdmin -Method POST -Path "/api/fallback/deployments/$deploymentID/recover" | Out-Null
      Write-Host "Recovered $deploymentID."
    } catch {
      Write-Warning "Failed to recover $deploymentID`: $($_.Exception.Message)"
    }
  }
}

Write-Host ''
Write-Host 'Fallback smoke and fault scenarios passed.'
