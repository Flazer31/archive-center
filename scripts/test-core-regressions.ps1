param()

$ErrorActionPreference = "Stop"
$root = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
$goRoot = Join-Path $root "go-service"
$manifestPath = Join-Path $root "testdata\core-regression-suite.json"

if (-not (Test-Path -LiteralPath $manifestPath -PathType Leaf)) {
    throw "Core regression manifest not found: $manifestPath"
}
$nodeCandidates = @()
if (-not [string]::IsNullOrWhiteSpace($env:ARCHIVE_CENTER_NODE_BINARY)) {
    $nodeCandidates += $env:ARCHIVE_CENTER_NODE_BINARY
}
$nodeCommand = Get-Command node -ErrorAction SilentlyContinue
if ($nodeCommand) {
    $nodeCandidates += $nodeCommand.Source
}
$nodeCandidates += @(
    (Join-Path $HOME ".cache\codex-runtimes\codex-primary-runtime\dependencies\node\bin\node.exe"),
    (Join-Path $env:LOCALAPPDATA "Programs\nodejs\node.exe"),
    (Join-Path $env:ProgramFiles "nodejs\node.exe")
)

$nodePath = $nodeCandidates |
    Where-Object { -not [string]::IsNullOrWhiteSpace($_) -and (Test-Path -LiteralPath $_ -PathType Leaf) } |
    Select-Object -First 1
if (-not $nodePath) {
    throw "Node.js is required so JavaScript runtime fixtures cannot be silently skipped"
}
$env:ARCHIVE_CENTER_NODE_BINARY = (Resolve-Path -LiteralPath $nodePath).Path

$goCache = Join-Path ([IO.Path]::GetTempPath()) "archive-center-core-regression-go-build"
if (-not (Test-Path -LiteralPath $goCache -PathType Container)) {
    New-Item -ItemType Directory -Path $goCache -Force | Out-Null
}
$env:GOCACHE = (Resolve-Path -LiteralPath $goCache).Path

$manifest = Get-Content -LiteralPath $manifestPath -Raw -Encoding UTF8 | ConvertFrom-Json
if ($manifest.isolation.real_database_allowed -or $manifest.isolation.real_vector_store_allowed -or $manifest.isolation.real_user_session_allowed) {
    throw "Core regression isolation contract must forbid real runtime data"
}

Push-Location $goRoot
try {
    foreach ($suite in $manifest.suites) {
        Write-Host ("[core-regression] " + $suite.name)
        $listed = @(& go test $suite.package -list $suite.run | Where-Object { $_ -match '^Test' })
        if ($LASTEXITCODE -ne 0) {
            throw "Core regression discovery failed: $($suite.name)"
        }
        if ($listed.Count -ne [int]$suite.expected_tests) {
            throw "Core regression discovery mismatch: $($suite.name) found $($listed.Count), expected $($suite.expected_tests)"
        }
        & go test $suite.package -run $suite.run -count=1
        if ($LASTEXITCODE -ne 0) {
            throw "Core regression suite failed: $($suite.name)"
        }
    }
} finally {
    Pop-Location
}

Write-Host "[core-regression] all isolated suites passed"
