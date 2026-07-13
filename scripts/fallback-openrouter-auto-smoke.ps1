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
    [object]$Body = $null,
    [int]$RequestTimeoutSec = 0,
    [switch]$NoRedirect
  )

  $headers = $null
  if (-not [string]::IsNullOrWhiteSpace($Token)) {
    $headers = @{ Authorization = "Bearer $Token" }
  }

  $effectiveTimeoutSec = if ($RequestTimeoutSec -gt 0) { $RequestTimeoutSec } else { $TimeoutSec }
  $params = @{
    Uri = "$BaseUrl$Path"
    Method = $Method
    TimeoutSec = $effectiveTimeoutSec
    UseBasicParsing = $true
  }
  if ($headers) {
    $headers["Content-Type"] = "application/json"
    $params.Headers = $headers
  }
  if ($null -ne $Body) {
    $params.Body = ($Body | ConvertTo-Json -Depth 8)
  }
  if ($NoRedirect) {
    $params.MaximumRedirection = 0
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
    if ($parts.Count -lt 2) {
      throw "Invalid metric sample: $line"
    }
    [double]$sample = 0
    if (-not [double]::TryParse($parts[1], [System.Globalization.NumberStyles]::Float, [System.Globalization.CultureInfo]::InvariantCulture, [ref]$sample)) {
      throw "Invalid metric sample value '$($parts[1])' in: $line"
    }
    if ([double]::IsNaN($sample) -or [double]::IsInfinity($sample)) {
      throw "Non-finite metric sample value '$($parts[1])' in: $line"
    }
    $metricName = $parts[0]
    $braceIndex = $metricName.IndexOf("{")
    $baseName = if ($braceIndex -ge 0) { $metricName.Substring(0, $braceIndex) } else { $metricName }
    if (-not $result.ContainsKey($baseName)) {
      $result[$baseName] = [double]0
    }
    [double]$aggregate = [double]$result[$baseName] + $sample
    if ([double]::IsNaN($aggregate) -or [double]::IsInfinity($aggregate)) {
      throw "Non-finite aggregate for metric '$baseName'."
    }
    $result[$baseName] = $aggregate
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
    [decimal]$ExpectedMin,
    [decimal]$Actual
  )
  if ($Actual -lt $ExpectedMin) {
    throw "$Label failed: expected >= $ExpectedMin, got $Actual"
  }
}

function Get-UsageTotal {
  param(
    [object[]]$Rows,
    [string]$Property
  )

  [decimal]$total = 0
  foreach ($row in @($Rows)) {
    $propertyValue = $row.PSObject.Properties[$Property]
    if ($null -eq $propertyValue -or $null -eq $propertyValue.Value) {
      continue
    }

    [long]$value = 0
    $text = [string]$propertyValue.Value
    if (-not [long]::TryParse($text, [System.Globalization.NumberStyles]::Integer, [System.Globalization.CultureInfo]::InvariantCulture, [ref]$value)) {
      throw "Usage property '$Property' is not a valid Int64 value: '$text'."
    }
    $total += [decimal]$value
  }
  return $total
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
$providerRowsBefore = @($usageBefore | Where-Object { $_.provider -eq $ExpectedProvider })
$usageRequestCountBefore = Get-UsageTotal -Rows $providerRowsBefore -Property "request_count"
$usageSuccessCountBefore = Get-UsageTotal -Rows $providerRowsBefore -Property "success_count"

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
$expectedUsageDelta = 2
$metricsAfterRaw = (Request-Fallback -Method GET -Path "/metrics" -Token $AdminToken).Content
$metricsAfter = Parse-Metrics -Text $metricsAfterRaw
$usageDeadline = [DateTime]::UtcNow.AddSeconds($TimeoutSec)
do {
  $usageRemainingMs = ($usageDeadline - [DateTime]::UtcNow).TotalMilliseconds
  if ($usageRemainingMs -le 0) {
    break
  }
  $usageRequestTimeoutSec = [Math]::Max(1, [int][Math]::Ceiling($usageRemainingMs / 1000))
  $usageAfterRaw = Request-Fallback -Method GET -Path "/api/fallback/free-pool/usage?provider=$ExpectedProvider" -Token $AdminToken -RequestTimeoutSec $usageRequestTimeoutSec
  $usageAfter = Parse-UsageResponse -Text $usageAfterRaw.Content -Path "/api/fallback/free-pool/usage?provider=$ExpectedProvider"
  $usageAfterCount = @($usageAfter | Measure-Object).Count
  $providerRowsAfter = @($usageAfter | Where-Object { $_.provider -eq $ExpectedProvider })
  $usageRequestCountAfter = Get-UsageTotal -Rows $providerRowsAfter -Property "request_count"
  $usageSuccessCountAfter = Get-UsageTotal -Rows $providerRowsAfter -Property "success_count"
  $usageRequestDelta = $usageRequestCountAfter - $usageRequestCountBefore
  $usageSuccessDelta = $usageSuccessCountAfter - $usageSuccessCountBefore
  if ($providerRowsAfter.Count -ge 1 -and $usageRequestDelta -ge $expectedUsageDelta -and $usageSuccessDelta -ge $expectedUsageDelta) {
    break
  }
  $usageRemainingMs = ($usageDeadline - [DateTime]::UtcNow).TotalMilliseconds
  if ($usageRemainingMs -le 0) {
    break
  }
  Start-Sleep -Milliseconds ([Math]::Min(250, [int][Math]::Floor($usageRemainingMs)))
} while ($true)

if ($usageAfterCount -le 0) {
  throw "Usage query returned zero rows for provider=$ExpectedProvider."
}
if ($providerRowsAfter.Count -lt 1) {
  throw "Usage query has data but no provider=$ExpectedProvider row."
}
Assert-Int "usage request_count" 1 $usageRequestCountAfter
if ($usageRequestDelta -lt $expectedUsageDelta) {
  throw "usage request_count did not record both smoke requests (expected delta >= $expectedUsageDelta, before=$usageRequestCountBefore, after=$usageRequestCountAfter)."
}
if ($usageSuccessDelta -lt $expectedUsageDelta) {
  throw "usage success_count did not record both smoke requests (expected delta >= $expectedUsageDelta, before=$usageSuccessCountBefore, after=$usageSuccessCountAfter)."
}
Write-Host "Usage recorded for provider '$ExpectedProvider' (rows=$($providerRowsAfter.Count))."
Write-Host "Usage deltas: request_count +$usageRequestDelta, success_count +$usageSuccessDelta"

$totalReqBefore = if ($metricsBefore.ContainsKey("fallback_requests_total")) { [double]$metricsBefore["fallback_requests_total"] } else { 0 }
$totalReqAfter = if ($metricsAfter.ContainsKey("fallback_requests_total")) { [double]$metricsAfter["fallback_requests_total"] } else { 0 }
$delta = $totalReqAfter - $totalReqBefore
if ([double]::IsNaN($delta) -or [double]::IsInfinity($delta)) {
  throw "fallback_requests_total delta is not finite (before=$totalReqBefore, after=$totalReqAfter)."
}
if ($delta -le 0) {
  throw "fallback_requests_total did not increase (before=$totalReqBefore, after=$totalReqAfter)."
}
Write-Host "Metrics delta: fallback_requests_total +$delta"

if ($usageBeforeCount -eq $usageAfterCount) {
  Write-Warning "Usage row count unchanged. Model counters still indicate usage; row replacement may be expected."
}

Write-Host "==> 8) Verify free-pool page reachability"
$pageReachable = $false
$pageResp = Request-Fallback -Method GET -Path "/fallback/free-pool" -Token $AdminToken -NoRedirect
if ($pageResp.StatusCode -lt 200 -or $pageResp.StatusCode -ge 300) {
  throw "Free-pool page check failed: HTTP $($pageResp.StatusCode)"
}
$pageReachable = $true
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
    usageRequestCount = [long]$usageRequestCountAfter
    usageSuccessCount = [long]$usageSuccessCountAfter
    usageRequestDelta = [long]$usageRequestDelta
    usageSuccessDelta = [long]$usageSuccessDelta
    fallbackRequestsDelta = $delta
    pageReachable = $pageReachable
    pageContainsOpenRouterAuto = $pageHasOpenRouterAuto
  }
  $summary | ConvertTo-Json -Depth 3 | Write-Output
  exit 0
}

Write-Host ""
Write-Host "OpenRouter auto stability smoke passed."
Write-Host "Runtime deployment used: $deploymentId"
Write-Host "Non-stream response model: $($chatJson.model)"
