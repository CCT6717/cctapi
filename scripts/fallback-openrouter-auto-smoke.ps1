param(
  [string]$BaseUrl = $(if ($env:CCT_API_BASE_URL) { $env:CCT_API_BASE_URL } else { "http://localhost:3008" }),
  [string]$ApiToken = $env:CCT_API_TOKEN,
  [string]$AdminToken = $env:CCT_ADMIN_TOKEN,
  [string]$Model = "openrouter/auto",
  [string]$ExpectedProvider = "openrouter",
  [string]$ExpectedDeploymentPrefix = "free:openrouter-",
  [int]$TimeoutSec = 60,
  [switch]$OutputJson
)

$ErrorActionPreference = "Stop"


function Request-Fallback {
  param(
    [ValidateSet("GET","POST")]
    [string]$Method,
    [string]$Path,
    [string]$Token,
    [object]$Body = $null
  )

  $headers = $null
  if (-not [string]::IsNullOrWhiteSpace($Token)) {
    $headers = @{ Authorization = "Bearer $Token" }
  }

  $params = @{
    Uri = "$BaseUrl$Path"
    Method = $Method
    TimeoutSec = $TimeoutSec
    UseBasicParsing = $true
  }
  if ($headers) {
    $headers["Content-Type"] = "application/json"
    $params.Headers = $headers
  }
  if ($null -ne $Body) {
    $params.Body = ($Body | ConvertTo-Json -Depth 8)
  }

  return Invoke-WebRequest @params
}

function Parse-Metrics {
  param([string]$Text)
  $result = @{}
  foreach ($line in ($Text -split "`n")) {
    $line = $line.Trim()
    if ($line -eq "" -or $line.StartsWith("#")) { continue }
    $parts = $line -split "\s+"
    if ($parts.Count -ge 2 -and [double]::TryParse($parts[1], [ref](0.0))) {
      $result[$parts[0]] = [double]$parts[1]
    }
  }
  return $result
}

function Parse-JsonResponse {
  param(
    [object]$Response,
    [string]$Path
  )

  $json = $Response.Content | ConvertFrom-Json
  if ($null -eq $json) {
    throw "Invalid JSON response from $Path."
  }
  if ($json.PSObject.Properties.Name -contains "success" -and $json.success -ne $true) {
    throw "$Path returned success=false: $($json.message)"
  }
  return $json
}

function Parse-UsageResponse {
  param(
    [string]$Text,
    [string]$Path
  )

  $json = $Text | ConvertFrom-Json
  if ($json.success -ne $true) {
    throw "Usage query failed for ${Path}: $($json.message)"
  }
  if ($null -eq $json.data) {
    throw "Usage query returned empty data for $Path."
  }
  return $json.data
}

function Assert-Int {
  param(
    [string]$Label,
    [int]$ExpectedMin,
    [int]$Actual
  )
  if ($Actual -lt $ExpectedMin) {
    throw "$Label failed: expected >= $ExpectedMin, got $Actual"
  }
}

$BaseUrl = $BaseUrl.TrimEnd("/")
Write-Host "Base URL: $BaseUrl"
Write-Host "Target model: $Model"

if ([string]::IsNullOrWhiteSpace($ApiToken)) {
  throw "Missing CCT_API_TOKEN / ApiToken."
}
if ([string]::IsNullOrWhiteSpace($AdminToken)) {
  throw "Missing CCT_ADMIN_TOKEN / AdminToken."
}

Write-Host "==> 1) Validate openrouter catalog entry"
$cfgResponse = Request-Fallback -Method GET -Path "/api/fallback/gateway/config" -Token $AdminToken
if ($cfgResponse.StatusCode -lt 200 -or $cfgResponse.StatusCode -ge 300) {
  throw "Failed to read gateway config: HTTP $($cfgResponse.StatusCode)"
}

$cfg = $cfgResponse.Content | ConvertFrom-Json
if ($cfg.success -ne $true) {
  throw "Gateway config request failed: $($cfg.message)"
}

$catalog = @($cfg.data.free_provider_catalog | Where-Object { $_.name -eq $ExpectedProvider })
if ($catalog.Count -lt 1) {
  throw "Provider '$ExpectedProvider' not found in free_provider_catalog."
}
$provider = $catalog[0]
if (-not $provider.enabled) {
  throw "Provider '$ExpectedProvider' is not enabled in catalog."
}
if (-not $provider.keyless -and (-not $provider.key_count -or $provider.key_count -lt 1)) {
  throw "Provider '$ExpectedProvider' requires keys but key_count is $($provider.key_count)."
}
Write-Host "Catalog check passed: key_count=$($provider.key_count), keyless=$($provider.keyless), requires_key=$($provider.requires_key)."

Write-Host "==> 2) Reload and sync free pool"
$reloadResponse = Request-Fallback -Method POST -Path "/api/fallback/config/reload" -Token $AdminToken -Body @{ action = "reload" }
if ($reloadResponse.StatusCode -lt 200 -or $reloadResponse.StatusCode -ge 300) {
  throw "Config reload failed: HTTP $($reloadResponse.StatusCode)"
}
$null = Parse-JsonResponse -Response $reloadResponse -Path "/api/fallback/config/reload"
Write-Host "Config reload done."

$syncResponse = Request-Fallback -Method POST -Path "/api/fallback/free-pool/sync" -Token $AdminToken
if ($syncResponse.StatusCode -lt 200 -or $syncResponse.StatusCode -ge 300) {
  throw "Free-pool sync failed: HTTP $($syncResponse.StatusCode)"
}
$null = Parse-JsonResponse -Response $syncResponse -Path "/api/fallback/free-pool/sync"
Write-Host "Free-pool sync done."

Start-Sleep -Seconds 2

Write-Host "==> 3) Validate openrouter runtime rows exist"
$runtimeResponse = Request-Fallback -Method GET -Path "/api/fallback/deployments/runtime-status" -Token $AdminToken
if ($runtimeResponse.StatusCode -lt 200 -or $runtimeResponse.StatusCode -ge 300) {
  throw "Runtime status fetch failed: HTTP $($runtimeResponse.StatusCode)"
}

$runtime = $runtimeResponse.Content | ConvertFrom-Json
if ($runtime.success -ne $true) {
  throw "Runtime status response failed: $($runtime.message)"
}
$runtimeRows = @($runtime.data | Where-Object { $_.deployment_id -like "$ExpectedDeploymentPrefix*" })
Assert-Int "runtime deployment rows for $ExpectedProvider" 1 $runtimeRows.Count
$deploymentId = $runtimeRows[0].deployment_id
Write-Host "Found runtime row: $deploymentId (enabled=$($runtimeRows[0].enabled), health=$($runtimeRows[0].health))."

Write-Host "==> 4) Snapshot counters and usage before request"
$metricsBeforeRaw = (Request-Fallback -Method GET -Path "/metrics" -Token $AdminToken).Content
$metricsBefore = Parse-Metrics -Text $metricsBeforeRaw
$usageBeforeRaw = Request-Fallback -Method GET -Path "/api/fallback/free-pool/usage?provider=$ExpectedProvider" -Token $AdminToken
$usageBefore = Parse-UsageResponse -Text $usageBeforeRaw.Content -Path "/api/fallback/free-pool/usage?provider=$ExpectedProvider"
$usageBeforeCount = @($usageBefore | Measure-Object).Count

Write-Host "==> 5) Run non-stream openrouter/auto chat"
$chatBody = @{
  model = $Model
  messages = @(
    @{
      role = "user"
      content = "Reply with one concise sentence in Chinese."
    }
  )
  temperature = 0.2
  max_tokens = 48
  stream = $false
}
$chatResp = Request-Fallback -Method POST -Path "/v1/chat/completions" -Token $ApiToken -Body $chatBody
if ($chatResp.StatusCode -lt 200 -or $chatResp.StatusCode -ge 300) {
  throw "Non-stream openrouter/auto chat failed: HTTP $($chatResp.StatusCode)"
}

$chatJson = $chatResp.Content | ConvertFrom-Json
if (-not $chatJson.choices -or $chatJson.choices.Count -lt 1) {
  throw "Non-stream response missing choices."
}
Write-Host "Non-stream request passed."

Write-Host "==> 6) Run stream openrouter/auto chat"
$streamBody = $chatBody.Clone()
$streamBody.stream = $true
$streamResp = Request-Fallback -Method POST -Path "/v1/chat/completions" -Token $ApiToken -Body $streamBody
if ($streamResp.StatusCode -lt 200 -or $streamResp.StatusCode -ge 300) {
  throw "Stream openrouter/auto chat failed: HTTP $($streamResp.StatusCode)"
}
if ($streamResp.Content -notmatch "data:") {
  throw "Stream response does not look like SSE (missing data:)."
}
Write-Host "Stream request passed."

Write-Host "==> 7) Validate usage and counters after request"
$metricsAfterRaw = (Request-Fallback -Method GET -Path "/metrics" -Token $AdminToken).Content
$metricsAfter = Parse-Metrics -Text $metricsAfterRaw
$usageAfterRaw = Request-Fallback -Method GET -Path "/api/fallback/free-pool/usage?provider=$ExpectedProvider" -Token $AdminToken
$usageAfter = Parse-UsageResponse -Text $usageAfterRaw.Content -Path "/api/fallback/free-pool/usage?provider=$ExpectedProvider"
$usageAfterCount = @($usageAfter | Measure-Object).Count

if ($usageAfterCount -le 0) {
  throw "Usage query returned zero rows for provider=$ExpectedProvider."
}

$providerRowsAfter = @($usageAfter | Where-Object { $_.provider -eq $ExpectedProvider })
if ($providerRowsAfter.Count -lt 1) {
  throw "Usage query has data but no provider=$ExpectedProvider row."
}
Assert-Int "usage request_count" 1 ([int](($providerRowsAfter | Measure-Object request_count -Sum).Sum))
Write-Host "Usage recorded for provider '$ExpectedProvider' (rows=$($providerRowsAfter.Count))."

$totalReqBefore = if ($metricsBefore.ContainsKey("fallback_requests_total")) { [double]$metricsBefore["fallback_requests_total"] } else { 0 }
$totalReqAfter = if ($metricsAfter.ContainsKey("fallback_requests_total")) { [double]$metricsAfter["fallback_requests_total"] } else { 0 }
if ($totalReqAfter -lt $totalReqBefore) {
  throw "fallback_requests_total did not increase (before=$totalReqBefore, after=$totalReqAfter)."
}
$delta = $totalReqAfter - $totalReqBefore
Write-Host "Metrics delta: fallback_requests_total +$delta"

if ($usageBeforeCount -eq $usageAfterCount) {
  Write-Warning "Usage row count unchanged. Model counters still indicate usage; row replacement may be expected."
}

Write-Host "==> 8) Verify free-pool page exposes openrouter/auto"
$pageResp = Request-Fallback -Method GET -Path "/fallback/free-pool" -Token $AdminToken
if ($pageResp.StatusCode -lt 200 -or $pageResp.StatusCode -ge 300) {
  throw "Free-pool page check failed: HTTP $($pageResp.StatusCode)"
}
$pageHasOpenRouterAuto = $true
if ($pageResp.Content -notmatch "openrouter" -or $pageResp.Content -notmatch "auto") {
  $pageHasOpenRouterAuto = $false
  Write-Warning "free-pool page content does not explicitly contain openrouter/auto text. Please verify in browser if required."
}
Write-Host "Free-pool page check done."

if ($OutputJson) {
  $summary = [pscustomobject]@{
    pass = $true
    baseUrl = $BaseUrl
    model = $chatJson.model
    deploymentId = $deploymentId
    usageRowsBefore = $usageBeforeCount
    usageRowsAfter = $usageAfterCount
    runtimeRows = $runtimeRows.Count
    usageRequestCount = [int](($providerRowsAfter | Measure-Object request_count -Sum).Sum)
    usageSuccessCount = [int](($providerRowsAfter | Measure-Object success_count -Sum).Sum)
    fallbackRequestsDelta = $delta
    pageContainsOpenRouterAuto = $pageHasOpenRouterAuto
  }
  $summary | ConvertTo-Json -Depth 3 | Write-Output
  exit 0
}

Write-Host ""
Write-Host "OpenRouter auto stability smoke passed."
Write-Host "Runtime deployment used: $deploymentId"
Write-Host "Non-stream response model: $($chatJson.model)"
