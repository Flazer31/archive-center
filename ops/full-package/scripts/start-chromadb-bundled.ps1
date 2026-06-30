param(
    [string]$EnvFile = ".\.env.full.local"
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

function Find-BundledPython([string]$Root) {
    $candidates = @(
        "runtime\Python\python.exe",
        "runtime\python\python.exe",
        "runtime\Python\Scripts\python.exe",
        "runtime\python\Scripts\python.exe",
        "runtime\ChromaDB\python.exe",
        "runtime\chromadb\python.exe",
        "runtime\ChromaDB\Scripts\python.exe",
        "runtime\chromadb\Scripts\python.exe"
    )
    foreach ($rel in $candidates) {
        $path = Join-Path $Root $rel
        if (Test-Path -LiteralPath $path -PathType Leaf) {
            return (Resolve-Path -LiteralPath $path).Path
        }
    }
    $hit = Get-ChildItem -LiteralPath (Join-Path $Root "runtime") -Recurse -File -ErrorAction SilentlyContinue |
        Where-Object { $_.Name -ieq "python.exe" } |
        Sort-Object FullName |
        Select-Object -First 1
    if ($null -eq $hit) {
        return ""
    }
    return $hit.FullName
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

$python = Find-BundledPython $packRoot
if ([string]::IsNullOrWhiteSpace($python)) {
    throw "Bundled ChromaDB Python runtime not found under runtime/. Package is incomplete."
}

& $python -c "import importlib.util, sys; sys.exit(0 if importlib.util.find_spec('chromadb') else 1)"
if ($LASTEXITCODE -ne 0) {
    throw "Bundled Python exists but does not include chromadb."
}

Write-Host "Starting bundled ChromaDB"
Write-Host "  Endpoint: http://$hostName`:$port"
Write-Host "  Data:     $dataDir"
& $python -c "import sys; from chromadb.cli.cli import app; sys.argv = ['chroma', 'run', '--host', sys.argv[1], '--port', sys.argv[2], '--path', sys.argv[3]]; app()" $hostName ([string]$port) $dataDir
