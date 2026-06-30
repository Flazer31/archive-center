param(
    [string]$EnvFile = ".\.env.live.local",
    [string]$Python = "python",
    [switch]$InstallChroma
)

$ErrorActionPreference = "Stop"

function Import-DotEnv([string]$Path) {
    if (Test-Path -LiteralPath $Path -PathType Leaf) {
        Get-Content -LiteralPath $Path | ForEach-Object {
            $line = $_.Trim()
            if ($line -eq "" -or $line.StartsWith("#")) { return }
            $idx = $line.IndexOf("=")
            if ($idx -lt 1) { return }
            [Environment]::SetEnvironmentVariable($line.Substring(0, $idx).Trim(), $line.Substring($idx + 1).Trim(), "Process")
        }
    }
}

$packRoot = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
Set-Location $packRoot
Import-DotEnv $EnvFile

if ([string]::IsNullOrWhiteSpace($env:AC_CHROMA_ENDPOINT)) {
    $env:AC_CHROMA_ENDPOINT = "http://127.0.0.1:8000"
}

$uri = [Uri]$env:AC_CHROMA_ENDPOINT
$hostName = if ([string]::IsNullOrWhiteSpace($uri.Host)) { "127.0.0.1" } else { $uri.Host }
$port = if ($uri.Port -gt 0) { $uri.Port } else { 8000 }
$dataDir = Join-Path $packRoot ".runtime\chromadb"
New-Item -ItemType Directory -Force -Path $dataDir | Out-Null

if ($InstallChroma) {
    & $Python -m pip install chromadb
    if ($LASTEXITCODE -ne 0) {
        throw "pip install chromadb failed."
    }
}

$chromaCmd = Get-Command chroma -ErrorAction SilentlyContinue
if ($chromaCmd) {
    Write-Host "Starting ChromaDB through chroma CLI"
    Write-Host "  Endpoint: http://$hostName`:$port"
    Write-Host "  Data:     $dataDir"
    & $chromaCmd.Source run --host $hostName --port $port --path $dataDir
    exit $LASTEXITCODE
}

Write-Host "Starting ChromaDB through Python module fallback"
Write-Host "  Endpoint: http://$hostName`:$port"
Write-Host "  Data:     $dataDir"
& $Python -c "import sys; from chromadb.cli.cli import app; sys.argv = ['chroma', 'run', '--host', sys.argv[1], '--port', sys.argv[2], '--path', sys.argv[3]]; app()" $hostName ([string]$port) $dataDir
