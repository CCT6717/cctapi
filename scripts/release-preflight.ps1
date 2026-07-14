[CmdletBinding()]
param()

$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $PSScriptRoot
$go = 'D:\ct\tools\go\bin\go.exe'
$gccBin = 'D:\ct\tools\w64devkit-1.23.0\bin'

function Invoke-Checked {
    param(
        [Parameter(Mandatory = $true)][string]$FilePath,
        [Parameter(Mandatory = $true)][string[]]$Arguments
    )

    Write-Host ("==> {0} {1}" -f $FilePath, ($Arguments -join ' '))
    & $FilePath @Arguments
    if ($LASTEXITCODE -ne 0) {
        throw ("command failed with exit code {0}: {1} {2}" -f $LASTEXITCODE, $FilePath, ($Arguments -join ' '))
    }
}

if (-not (Test-Path -LiteralPath $go)) {
    throw "Go toolchain not found: $go"
}
if (-not (Test-Path -LiteralPath (Join-Path $gccBin 'gcc.exe'))) {
    throw "CGO toolchain not found: $gccBin"
}

$env:CGO_ENABLED = '1'
$env:PATH = $gccBin + ';' + $env:PATH
$env:STORYBOOK_DISABLE_TELEMETRY = '1'

Push-Location (Join-Path $root 'web\default')
try {
    Invoke-Checked 'npm.cmd' @('run', 'lint')
    Invoke-Checked 'npm.cmd' @('test')
    Invoke-Checked 'npm.cmd' @('run', 'build')
    Invoke-Checked 'npm.cmd' @('run', 'build-storybook', '--', '--quiet')
} finally {
    Pop-Location
}

Push-Location $root
try {
    Invoke-Checked $go @('test', './...', '-count=1')
    Invoke-Checked $go @('test', '-race', './fallback', './controller', './middleware', './common', './router', '-count=1')
    Invoke-Checked $go @('vet', './...')
    Invoke-Checked $go @('build', './...')
    Invoke-Checked 'git.exe' @('diff', '--check')
} finally {
    Pop-Location
}

Write-Host 'Local release preflight passed. R2 still requires CI for the exact target SHA; R3 and R4 require runtime acceptance and sanitized evidence.'
