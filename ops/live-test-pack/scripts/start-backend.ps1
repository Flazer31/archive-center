param(
    [string]$EnvFile = ".\.env.live.local",
    [switch]$RunSchema,
    [string]$Go = "go"
)

$ErrorActionPreference = "Stop"

function Import-DotEnv([string]$Path) {
    if (-not (Test-Path -LiteralPath $Path -PathType Leaf)) {
        throw "Env file not found: $Path. Copy .env.live.example to .env.live.local first."
    }
    Get-Content -LiteralPath $Path | ForEach-Object {
        $line = $_.Trim()
        if ($line -eq "" -or $line.StartsWith("#")) { return }
        $idx = $line.IndexOf("=")
        if ($idx -lt 1) { return }
        $key = $line.Substring(0, $idx).Trim()
        $value = $line.Substring($idx + 1).Trim()
        [Environment]::SetEnvironmentVariable($key, $value, "Process")
    }
}

$packRoot = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
Set-Location $packRoot
Import-DotEnv $EnvFile

if ([string]::IsNullOrWhiteSpace($env:AC_MARIADB_DSN)) {
    throw "AC_MARIADB_DSN is required for live-test backend."
}
if ([string]::IsNullOrWhiteSpace($env:AC_CHROMA_ENDPOINT)) {
    throw "AC_CHROMA_ENDPOINT is required for live-test backend."
}

$env:AC_MODE = "live"
$env:AC_STORE_MODE = "mariadb_authority"
if ([string]::IsNullOrWhiteSpace($env:AC_BIND_ADDR)) {
    $env:AC_BIND_ADDR = "127.0.0.1:28080"
}
if ([string]::IsNullOrWhiteSpace($env:AC_CHROMA_COLLECTION)) {
    $env:AC_CHROMA_COLLECTION = "archive_center_vectors"
}
if ([string]::IsNullOrWhiteSpace($env:AC_CHROMA_API_PATH)) {
    $env:AC_CHROMA_API_PATH = "/api/v2"
}
if ([string]::IsNullOrWhiteSpace($env:AC_PROMPT_DIR)) {
    $env:AC_PROMPT_DIR = Join-Path $packRoot "prompts"
}

Set-Location (Join-Path $packRoot "go-service")

if ($RunSchema) {
    & $Go run -buildvcs=false ./cmd/mariadb-schema -dsn $env:AC_MARIADB_DSN -execute
    if ($LASTEXITCODE -ne 0) {
        throw "mariadb-schema failed."
    }
}

Write-Host "Starting Archive Center 2.0 backend"
Write-Host "  Bind:    $($env:AC_BIND_ADDR)"
Write-Host "  Store:   $($env:AC_STORE_MODE)"
Write-Host "  Chroma:  $($env:AC_CHROMA_ENDPOINT)"
Write-Host "  Prompt:  $($env:AC_PROMPT_DIR)"
Write-Host ""
Write-Host "Stop with Ctrl+C."

& $Go run -buildvcs=false ./cmd/archive-center-go
