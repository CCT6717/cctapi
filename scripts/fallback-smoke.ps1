param(
  [string]$BaseUrl = $(if ($env:CCT_API_BASE_URL) { $env:CCT_API_BASE_URL } else { 'http://localhost:3007' }),
  [string]$ApiToken = $env:CCT_API_TOKEN,
  [string]$Model = $env:CCT_API_MODEL,
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

function Invoke-ChatCompletion {
  param(
    [bool]$Stream
  )

  $body = @{
    model = $Model
    messages = @(
      @{
        role = 'user'
        content = 'Reply with one short sentence for a fallback smoke test.'
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

Write-Host "Base URL: $BaseUrl"
Write-Host "Model: $Model"

Write-Host 'Running non-stream chat completion...'
$nonStream = Invoke-ChatCompletion -Stream:$false
if ($nonStream.StatusCode -lt 200 -or $nonStream.StatusCode -ge 300) {
  throw "Non-stream request failed with HTTP $($nonStream.StatusCode)."
}
$nonStreamJson = $nonStream.Content | ConvertFrom-Json
if (-not $nonStreamJson.choices -or $nonStreamJson.choices.Count -lt 1) {
  throw 'Non-stream response does not contain choices.'
}
Write-Host 'Non-stream request passed.'

Write-Host 'Running stream chat completion...'
$stream = Invoke-ChatCompletion -Stream:$true
if ($stream.StatusCode -lt 200 -or $stream.StatusCode -ge 300) {
  throw "Stream request failed with HTTP $($stream.StatusCode)."
}
if ($stream.Content -notmatch 'data:') {
  throw 'Stream response does not look like an SSE response.'
}
Write-Host 'Stream request passed.'

Write-Host 'Checking fallback metrics endpoint...'
$metrics = Invoke-WebRequest `
  -Uri "$BaseUrl/metrics" `
  -Method Get `
  -TimeoutSec $TimeoutSec `
  -UseBasicParsing
if ($metrics.Content -notmatch 'fallback_requests_total') {
  throw 'Metrics response does not contain fallback_requests_total.'
}
Write-Host 'Metrics endpoint passed.'

Write-Host 'Fallback smoke test passed.'
