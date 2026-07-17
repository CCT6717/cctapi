<#
.SYNOPSIS
    Credential-gated, paced production soak for the cctapi fallback gateway.

.DESCRIPTION
    Runs a sustained, paced load against a virtual model for 30-60 minutes and
    captures SANITIZED production-capacity evidence: model rotation, provider
    fallback, cooldown recovery, and usage deltas.

    This script is strictly credential-gated. It refuses to run unless
    CCT_API_TOKEN (or -ApiToken) and CCT_ADMIN_TOKEN (or -AdminToken) are
    provided. It never fabricates or substitutes anonymous quota. No token,
    password, raw request body, or raw upstream response body is ever written
    to the evidence file.

    Designed to satisfy the "production capacity acceptance" gate that the
    merged feature candidate could not close without real production credentials.

.PARAMETER BaseUrl
    Gateway base URL. Defaults to $env:CCT_API_BASE_URL or http://localhost:3008.

.PARAMETER ApiToken
    Production API token. Defaults to $env:CCT_API_TOKEN. REQUIRED.

.PARAMETER AdminToken
    Admin token for the observability endpoint. Defaults to $env:CCT_ADMIN_TOKEN. REQUIRED.

.PARAMETER Model
    Virtual model under soak. Default: openrouter/auto. Override with your
    production virtual model.

.PARAMETER DurationMin
    Soak duration in minutes. Default: 45 (covers the required 30-60 min window).

.PARAMETER IntervalSec
    Paced interval between requests in seconds. Default: 5 (matches prior soak cadence).

.PARAMETER TimeoutSec
    Per-request timeout in seconds. Default: 60.

.PARAMETER MinSuccessRate
    Minimum acceptable success rate; the run fails closed below this. Default: 0.95.

.PARAMETER OutputDir
    Directory for the sanitized evidence JSON. Default: docs/evidence.

.PARAMETER SnapshotEvery
    Poll /api/fallback/attempt-observability every N requests. Default: 10.

.PARAMETER IncludeTools
    Also send a structured tool-call request periodically.

.PARAMETER IncludeResponses
    Also send an OpenAI Responses request periodically.

.PARAMETER OutputJson
    Emit only a machine-readable summary on stdout.
#>
param(
  [string]$BaseUrl = $(if ($env:CCT_API_BASE_URL) { $env:CCT_API_BASE_URL } else { "http://localhost:3008" }),
  [string]$ApiToken = $env:CCT_API_TOKEN,
  [string]$AdminToken = $env:CCT_ADMIN_TOKEN,
  [string]$Model = "openrouter/auto",
  [int]$DurationMin = 45,
  [int]$IntervalSec = 5,
  [int]$TimeoutSec = 60,
  [double]$MinSuccessRate = 0.95,
  [string]$OutputDir = "docs/evidence",
  [int]$SnapshotEvery = 10,
  [switch]$IncludeTools,
  [switch]$IncludeResponses,
  [switch]$OutputJson
)

$ErrorActionPreference = "Stop"

# ---------------------------------------------------------------------------
# Credential gate: never run without real production credentials.
# ---------------------------------------------------------------------------
if ([string]::IsNullOrWhiteSpace($ApiToken)) {
  throw "Missing production credential: set CCT_API_TOKEN or pass -ApiToken. This soak must use real production credentials; anonymous quota is not a substitute."
}
if ([string]::IsNullOrWhiteSpace($AdminToken)) {
  throw "Missing admin credential: set CCT_ADMIN_TOKEN or pass -AdminToken. Required to read /api/fallback/attempt-observability."
}

$BaseUrl = $BaseUrl.TrimEnd("/")

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

function Parse-JsonResponse {
  param([object]$Response, [string]$Path)
  $json = $Response.Content | ConvertFrom-Json
  if ($null -eq $json) { throw "Invalid JSON response from $Path." }
  if ($json.PSObject.Properties.Name -contains "success" -and $json.success -ne $true) {
    throw "$Path returned success=false: $($json.message)"
  }
  return $json
}

function Parse-AttemptObservability {
  param([object]$Response)
  if ($Response.StatusCode -lt 200 -or $Response.StatusCode -ge 300) {
    throw "Observability fetch failed: HTTP $($Response.StatusCode)"
  }
  $json = $Response.Content | ConvertFrom-Json
  if ($json.success -ne $true) { throw "Observability response failed: $($json.message)" }
  return $json.data
}

function Send-ChatRequest {
  param([bool]$Stream, [bool]$WithTools)
  $body = @{
    model = $Model
    messages = @( @{ role = "user"; content = "Reply with one concise sentence. Keep it under 20 words." } )
    temperature = 0.3
    max_tokens = 64
    stream = $Stream
  }
  if ($WithTools) {
    $body["tools"] = @(
      @{ type = "function"; function = @{ name = "get_time"; description = "Get current time"; parameters = @{ type = "object"; properties = @{}; required = @() } } }
    )
    $body["tool_choice"] = "auto"
  }
  $r = Request-Fallback -Method POST -Path "/v1/chat/completions" -Token $ApiToken -Body $body
  if ($r.StatusCode -lt 200 -or $r.StatusCode -ge 300) { throw "Chat failed: HTTP $($r.StatusCode)" }
  if ($Stream) {
    if ($r.Content -notmatch "data:") { throw "Stream response missing SSE data:." }
  } else {
    $j = $r.Content | ConvertFrom-Json
    if (-not $j.choices -or $j.choices.Count -lt 1) { throw "Non-stream response missing choices." }
  }
  return $true
}

function Send-ResponsesRequest {
  $body = @{
    model = $Model
    input = "Reply with one concise sentence."
    temperature = 0.3
    max_output_tokens = 64
  }
  $r = Request-Fallback -Method POST -Path "/v1/responses" -Token $ApiToken -Body $body
  if ($r.StatusCode -lt 200 -or $r.StatusCode -ge 300) { throw "Responses failed: HTTP $($r.StatusCode)" }
  return $true
}

# ---------------------------------------------------------------------------
# Setup
# ---------------------------------------------------------------------------
$startedAt = [DateTime]::UtcNow
$totalIterations = [Math]::Max(1, [int][Math]::Floor(($DurationMin * 60) / $IntervalSec))
$endedBy = $startedAt.AddMinutes($DurationMin)

$stats = [pscustomobject]@{
  totalRequests = 0
  successCount  = 0
  failureCount  = 0
  streamCount   = 0
  toolCount     = 0
  responsesCount = 0
  latenciesMs   = @()
  errors        = @()
}

$distinctProviders = @{}
$distinctModels    = @{}
$observabilitySnapshots = @()
$lastSnapshotAt = [DateTime]::UtcNow.AddMinutes(-1)

if (-not $OutputJson) {
  Write-Host "Production soak starting"
  Write-Host "  Base URL        : $BaseUrl"
  Write-Host "  Model           : $Model"
  Write-Host "  Duration        : $DurationMin min"
  Write-Host "  Interval        : $IntervalSec s"
  Write-Host "  Iterations      : $totalIterations"
  Write-Host "  Min success rate: $($MinSuccessRate.ToString('P0'))"
  Write-Host "  Tools/Responses : $(if ($IncludeTools) {'tools '} else {''})$(if ($IncludeResponses) {'responses'} else {''})"
}

# Warm-up: confirm observability endpoint is reachable before the timed run.
$null = Parse-AttemptObservability (Request-Fallback -Method GET -Path "/api/fallback/attempt-observability" -Token $AdminToken)
if (-not $OutputJson) { Write-Host "Observability endpoint reachable. Beginning paced load..." }

# ---------------------------------------------------------------------------
# Soak loop
# ---------------------------------------------------------------------------
for ($i = 1; $i -le $totalIterations; $i++) {
  $now = [DateTime]::UtcNow
  if ($now -ge $endedBy) { break }

  # Rotate request shape across iterations.
  $useStream = ($i % 2) -eq 0
  $useTools  = $IncludeTools -and ($i % 7) -eq 0
  $useResp   = $IncludeResponses -and ($i % 11) -eq 0

  $sw = [System.Diagnostics.Stopwatch]::StartNew()
  $ok = $false
  try {
    if ($useResp) {
      $ok = Send-ResponsesRequest
      if ($ok) { $stats.responsesCount++ }
    } else {
      $ok = Send-ChatRequest -Stream $useStream -WithTools $useTools
      if ($ok) {
        if ($useStream) { $stats.streamCount++ }
        if ($useTools)  { $stats.toolCount++ }
      }
    }
  } catch {
    $msg = $_.Exception.Message
    $stats.errors += [pscustomobject]@{ iteration = $i; at = [DateTime]::UtcNow.ToString("o"); error = $msg }
    if ($stats.errors.Count -gt 50) { $stats.errors = $stats.errors[0..49] }
  }
  $sw.Stop()
  $stats.latenciesMs += $sw.ElapsedMilliseconds
  $stats.totalRequests++

  if (-not $OutputJson) {
    $tag = if ($useResp) { "responses" } elseif ($useTools) { "tools" } elseif ($useStream) { "stream" } else { "chat" }
    $status = if ($ok) { "OK" } else { "FAIL" }
    Write-Host ("[{0}/{1}] {2} {3} ({4}ms)" -f $i, $totalIterations, $tag, $status, $sw.ElapsedMilliseconds)
  }

  # Periodic observability snapshot (captures model rotation / provider fallback / cooldown).
  if (($i % $SnapshotEvery) -eq 0 -or ([DateTime]::UtcNow - $lastSnapshotAt).TotalSeconds -ge 60) {
    try {
      $snap = Parse-AttemptObservability (Request-Fallback -Method GET -Path "/api/fallback/attempt-observability" -Token $AdminToken)
      $snapObj = [pscustomobject]@{
        at = [DateTime]::UtcNow.ToString("o")
        iteration = $i
        failure_event_count = $snap.failure_event_count
        skip_event_count = $snap.skip_event_count
        top_providers = $snap.top_providers
        top_models = $snap.top_models
        error_categories = $snap.error_categories
        outcomes = $snap.outcomes
        recent_chains = $snap.recent_chains
      }
      $observabilitySnapshots += $snapObj
      $lastSnapshotAt = [DateTime]::UtcNow

      # Accumulate distinct providers / models observed in recent chains.
      foreach ($chain in $snap.recent_chains) {
        foreach ($step in $chain.steps) {
          if (-not [string]::IsNullOrWhiteSpace($step.provider)) { $distinctProviders[$step.provider] = $true }
          if (-not [string]::IsNullOrWhiteSpace($step.real_model)) { $distinctModels[$step.real_model] = $true }
        }
      }
    } catch {
      if (-not $OutputJson) { Write-Warning "Observability snapshot $i failed: $($_.Exception.Message)" }
    }
  }

  # Pace until next iteration or the deadline.
  $remainingMs = ($endedBy - [DateTime]::UtcNow).TotalMilliseconds
  if ($remainingMs -le 0) { break }
  $sleepMs = [Math]::Min($IntervalSec * 1000, [Math]::Max(0, [int]$remainingMs))
  if ($sleepMs -gt 0) { Start-Sleep -Milliseconds $sleepMs }
}

# ---------------------------------------------------------------------------
# Aggregate + sanitize evidence
# ---------------------------------------------------------------------------
$endedAt = [DateTime]::UtcNow
$successRate = if ($stats.totalRequests -gt 0) { $stats.successCount / $stats.totalRequests } else { 0 }
if ($stats.latenciesMs.Count -gt 0) {
  $sorted = $stats.latenciesMs | Sort-Object
  $p50 = $sorted[[Math]::Floor($sorted.Count * 0.50)]
  $p95 = $sorted[[Math]::Floor($sorted.Count * 0.95)]
  $avgLatency = ($sorted | Measure-Object -Average).Average
} else {
  $p50 = $p95 = $avgLatency = 0
}

# Detect rotation / fallback signals from the last observability snapshot only
# (defensive: the endpoint already sanitizes; we never add tokens or raw bodies).
$lastSnap = if ($observabilitySnapshots.Count -gt 0) { $observabilitySnapshots[-1] } else { $null }
$rateLimitOutcomes = 0
$multiProviderChains = 0
if ($lastSnap) {
  foreach ($oc in $lastSnap.outcomes) {
    if ($oc.outcome -eq "model_rate_limited") { $rateLimitOutcomes += $oc.count }
  }
  foreach ($chain in $lastSnap.recent_chains) {
    $provs = @($chain.steps | ForEach-Object { $_.provider } | Where-Object { -not [string]::IsNullOrWhiteSpace($_) } | Sort-Object -Unique)
    if ($provs.Count -gt 1) { $multiProviderChains++ }
  }
}

$evidence = [pscustomobject]@{
  generated_at = $endedAt.ToString("o")
  kind = "production-capacity-soak"
  credential_source = "real-production-credentials"
  base_url = $BaseUrl
  model = $Model
  duration_minutes = [Math]::Round(($endedAt - $startedAt).TotalMinutes, 2)
  interval_sec = $IntervalSec
  summary = [pscustomobject]@{
    total_requests = $stats.totalRequests
    success_count = $stats.successCount
    failure_count = $stats.failureCount
    success_rate = [Math]::Round($successRate, 4)
    stream_requests = $stats.streamCount
    tool_requests = $stats.toolCount
    responses_requests = $stats.responsesCount
    distinct_providers = @($distinctProviders.Keys | Sort-Object)
    distinct_real_models = @($distinctModels.Keys | Sort-Object)
    latency_ms_avg = [Math]::Round($avgLatency, 1)
    latency_ms_p50 = $p50
    latency_ms_p95 = $p95
    rate_limit_outcomes_last_snapshot = $rateLimitOutcomes
    multi_provider_chains_last_snapshot = $multiProviderChains
  }
  # The endpoint already omits keys / raw bodies. We store the sanitized snapshots as-is.
  observability_snapshots = $observabilitySnapshots
  request_errors = $stats.errors
}

# Write sanitized evidence file (no tokens, no passwords, no raw bodies).
if (-not (Test-Path $OutputDir)) { New-Item -ItemType Directory -Path $OutputDir -Force | Out-Null }
$stamp = $startedAt.ToString("yyyy-MM-dd-HHmm")
$evidencePath = Join-Path $OutputDir "production-soak-$stamp.json"
$evidence | ConvertTo-Json -Depth 12 | Set-Content -Encoding utf8 $evidencePath

$passed = $successRate -ge $MinSuccessRate -and $stats.totalRequests -gt 0

if ($OutputJson) {
  [pscustomobject]@{
    pass = $passed
    evidence_path = $evidencePath
    success_rate = [Math]::Round($successRate, 4)
    total_requests = $stats.totalRequests
    distinct_providers = $evidence.summary.distinct_providers
    distinct_real_models = $evidence.summary.distinct_real_models
  } | ConvertTo-Json -Depth 4 | Write-Output
} else {
  Write-Host ""
  Write-Host "==== Production soak complete ===="
  Write-Host ("  Requests         : {0}" -f $stats.totalRequests)
  Write-Host ("  Successes        : {0}" -f $stats.successCount)
  Write-Host ("  Failures         : {0}" -f $stats.failureCount)
  Write-Host ("  Success rate     : {0:P2} (min {1:P0})" -f $successRate, $MinSuccessRate)
  Write-Host ("  Distinct providers : {0}" -f ($evidence.summary.distinct_providers -join ", "))
  Write-Host ("  Distinct real models: {0}" -f ($evidence.summary.distinct_real_models -join ", "))
  Write-Host ("  Latency p50/p95  : {0}ms / {1}ms" -f $p50, $p95)
  Write-Host ("  Sanitized evidence: {0}" -f $evidencePath)
  Write-Host ("  RESULT           : {0}" -f $(if ($passed) { "PASS" } else { "FAIL (success rate below threshold)" }))
}

exit $(if ($passed) { 0 } else { 1 })
