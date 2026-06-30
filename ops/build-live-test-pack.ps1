param(
    [string]$OutputRoot,
    [switch]$ForceRefresh
)

$ErrorActionPreference = "Stop"

function Resolve-FullPath([string]$Path) {
    $executionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath($Path)
}

$repoRoot = Resolve-FullPath (Join-Path $PSScriptRoot "..")
if ([string]::IsNullOrWhiteSpace($OutputRoot)) {
    $OutputRoot = Join-Path $repoRoot "_live-test-pack"
}
$outputRootFull = Resolve-FullPath $OutputRoot
$target = Join-Path $outputRootFull "Archive Center 2.0 Live Test"
$targetFull = Resolve-FullPath $target

if (-not $targetFull.StartsWith($repoRoot, [System.StringComparison]::OrdinalIgnoreCase)) {
    throw "Refusing to write live-test pack outside Archive Center 2.0: $targetFull"
}

if ((Test-Path -LiteralPath $targetFull) -and -not $ForceRefresh) {
    throw "Target already exists: $targetFull. Re-run with -ForceRefresh to rebuild it."
}

if (Test-Path -LiteralPath $targetFull) {
    $resolved = Resolve-FullPath $targetFull
    if (-not $resolved.StartsWith($outputRootFull, [System.StringComparison]::OrdinalIgnoreCase)) {
        throw "Refusing recursive delete outside output root: $resolved"
    }
    Remove-Item -LiteralPath $resolved -Recurse -Force
}

New-Item -ItemType Directory -Force -Path $targetFull | Out-Null

function Copy-File([string]$Source, [string]$DestRelative) {
    $src = Join-Path $repoRoot $Source
    if (-not (Test-Path -LiteralPath $src -PathType Leaf)) {
        throw "Missing source file: $src"
    }
    $dest = Join-Path $targetFull $DestRelative
    New-Item -ItemType Directory -Force -Path (Split-Path -Parent $dest) | Out-Null
    Copy-Item -LiteralPath $src -Destination $dest -Force
}

function Copy-Directory([string]$Source, [string]$DestRelative) {
    $src = Join-Path $repoRoot $Source
    if (-not (Test-Path -LiteralPath $src -PathType Container)) {
        throw "Missing source directory: $src"
    }
    $dest = Join-Path $targetFull $DestRelative
    New-Item -ItemType Directory -Force -Path $dest | Out-Null
    Get-ChildItem -LiteralPath $src -Force | ForEach-Object {
        Copy-Item -LiteralPath $_.FullName -Destination $dest -Recurse -Force
    }
}

Copy-File "Archive Center.js" "Archive Center.js"
Copy-File "README.md" "README.md"
Copy-File ".env.example" ".env.source.example"

New-Item -ItemType Directory -Force -Path (Join-Path $targetFull "go-service") | Out-Null
Copy-File "go-service/go.mod" "go-service/go.mod"
Copy-File "go-service/go.sum" "go-service/go.sum"
Copy-File "go-service/README.md" "go-service/README.md"
Copy-Directory "go-service/cmd" "go-service/cmd"
Copy-Directory "go-service/internal" "go-service/internal"
Copy-Directory "migrations" "migrations"
Copy-Directory "prompts" "prompts"

$templateRoot = Join-Path $repoRoot "ops\live-test-pack"
Copy-File "ops/live-test-pack/README_LIVE_TEST.md" "README_LIVE_TEST.md"
Copy-File "ops/live-test-pack/.env.live.example" ".env.live.example"
Copy-Directory "ops/live-test-pack/scripts" "scripts"

$generated = @{
    pack_name = "Archive Center 2.0 Live Test"
    generated_at = [DateTimeOffset]::UtcNow.ToString("o")
    source_root = $repoRoot
    target_root = $targetFull
    includes_runtime_binaries = $false
    includes_database_files = $false
    includes_chromadb_data = $false
    vector_target = "chromadb"
    canonical_store = "mariadb"
    excluded = @(
        ".git",
        ".runtime",
        ".runtime-cache",
        "go-service/.gocache",
        "go-service/*.exe",
        "runtime data",
        "database files",
        "backup/release/deploy outputs"
    )
}
$generated | ConvertTo-Json -Depth 6 | Set-Content -LiteralPath (Join-Path $targetFull "LIVE_TEST_PACK_MANIFEST.json") -Encoding UTF8

Write-Host "Live-test pack created:"
Write-Host "  $targetFull"
Write-Host ""
Write-Host "Next:"
Write-Host "  1. Copy .env.live.example to .env.live.local"
Write-Host "  2. Fill AC_MARIADB_DSN and AC_CHROMA_ENDPOINT"
Write-Host "  3. Start ChromaDB: powershell -ExecutionPolicy Bypass -File .\scripts\start-chromadb.ps1"
Write-Host "  4. Start backend: powershell -ExecutionPolicy Bypass -File .\scripts\start-backend.ps1 -RunSchema"
Write-Host "  5. Smoke: powershell -ExecutionPolicy Bypass -File .\scripts\smoke-live.ps1"
