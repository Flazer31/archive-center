param(
    [string]$EnvFile = "..\_dist\Archive Center 2.0 Full Package Windows\.env.full.local",
    [int]$MariaDBPort = 3307
)

$ErrorActionPreference = "Stop"

function Import-DotEnv([string]$Path) {
    if (-not (Test-Path -LiteralPath $Path -PathType Leaf)) {
        throw "Env file not found: $Path"
    }
    Get-Content -LiteralPath $Path | ForEach-Object {
        $line = $_.Trim()
        if ($line -eq "" -or $line.StartsWith("#")) { return }
        $idx = $line.IndexOf("=")
        if ($idx -lt 1) { return }
        [Environment]::SetEnvironmentVariable($line.Substring(0, $idx).Trim(), $line.Substring($idx + 1).Trim(), "Process")
    }
}

function Test-PortOpen([int]$Port) {
    $client = New-Object Net.Sockets.TcpClient
    try {
        $iar = $client.BeginConnect("127.0.0.1", $Port, $null, $null)
        $ok = $iar.AsyncWaitHandle.WaitOne(500, $false)
        if ($ok) { $client.EndConnect($iar) }
        return $ok
    } catch {
        return $false
    } finally {
        $client.Close()
    }
}

$root = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
$goRoot = Join-Path $root "go-service"
$resolvedEnv = (Resolve-Path (Join-Path $PSScriptRoot $EnvFile)).Path

Import-DotEnv $resolvedEnv

if ([string]::IsNullOrWhiteSpace($env:AC_MARIADB_DSN)) {
    $env:AC_MARIADB_DSN = "archive_center:archive-center-local-pass@tcp(127.0.0.1:${MariaDBPort})/archive_center?parseTime=true"
}
if ([string]::IsNullOrWhiteSpace($env:AC_BIND_ADDR)) {
    $env:AC_BIND_ADDR = "127.0.0.1:28080"
}
if ([string]::IsNullOrWhiteSpace($env:AC_CHROMA_ENDPOINT)) {
    $env:AC_CHROMA_ENDPOINT = "http://127.0.0.1:8000"
}
if ([string]::IsNullOrWhiteSpace($env:AC_CHROMA_COLLECTION)) {
    $env:AC_CHROMA_COLLECTION = "archive_center_vectors"
}
if ([string]::IsNullOrWhiteSpace($env:AC_CHROMA_API_PATH)) {
    $env:AC_CHROMA_API_PATH = "/api/v2"
}

$env:AC_MODE = "live"
$env:AC_STORE_MODE = "mariadb_authority"
$env:AC_PROMPT_DIR = Join-Path $root "prompts"

if (-not (Test-PortOpen $MariaDBPort)) {
    throw "MariaDB is not listening on 127.0.0.1:$MariaDBPort. Start the full package services first."
}

$chromaUri = [Uri]$env:AC_CHROMA_ENDPOINT
$chromaPort = if ($chromaUri.Port -gt 0) { $chromaUri.Port } else { 8000 }
if (-not (Test-PortOpen $chromaPort)) {
    throw "ChromaDB is not listening on 127.0.0.1:$chromaPort. Start the full package services first."
}

Write-Host "Starting Archive Center source backend with full-package services"
Write-Host "  Go:      $($env:AC_BIND_ADDR)"
Write-Host "  MariaDB: 127.0.0.1:$MariaDBPort"
Write-Host "  Chroma:  $($env:AC_CHROMA_ENDPOINT)"
Write-Host "  Store:   $($env:AC_STORE_MODE)"
Write-Host ""
Write-Host "Stop with Ctrl+C."

Set-Location $goRoot
go run -buildvcs=false ./cmd/archive-center-go
