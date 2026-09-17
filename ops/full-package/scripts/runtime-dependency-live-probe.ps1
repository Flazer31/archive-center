param(
    [switch]$Execute,
    [switch]$SkipChroma,
    [switch]$SkipMariaDB,
    [string]$ChromaEndpoint = $env:AC_CHROMA_ENDPOINT,
    [string]$MariaDBDSN = $env:AC_MARIADB_DSN,
    [string]$OutputPath = "",
    [Nullable[int]]$TimeoutSeconds = $null,
    [string]$GoCommand = "go"
)

$ErrorActionPreference = "Stop"

function Write-ProbeReport([hashtable]$Report, [int]$ExitCode) {
    $json = $Report | ConvertTo-Json -Depth 20
    if (-not [string]::IsNullOrWhiteSpace($OutputPath)) {
        $parent = Split-Path -Parent $OutputPath
        if (-not [string]::IsNullOrWhiteSpace($parent)) {
            New-Item -ItemType Directory -Force -Path $parent | Out-Null
        }
        $json | Set-Content -LiteralPath $OutputPath -Encoding UTF8
    }
    $json
    exit $ExitCode
}

$report = [ordered]@{
    contract = "archive-center.windows-runtime-dependency-live-probe.v1"
    status = "guarded"
    executed = $false
    evidence_class = "source_guarded_no_live_execution"
    execution_surface = "go_run_from_active_source"
    installed_package_proof = $false
    generated_at = [DateTimeOffset]::UtcNow.ToString("o")
    chroma = $null
    mariadb = $null
    errors = @()
}

if (-not $Execute) {
    $report.errors = @("-Execute is required before temporary ChromaDB collections or MariaDB tables are created")
    Write-ProbeReport $report 2
}
if ($SkipChroma -and $SkipMariaDB) {
    $report.status = "failed"
    $report.executed = $true
    $report.errors = @("both dependency probes were skipped")
    Write-ProbeReport $report 2
}
if ($null -ne $TimeoutSeconds -and $TimeoutSeconds -lt 1) {
    $report.status = "failed"
    $report.executed = $true
    $report.errors = @("TimeoutSeconds must be greater than zero")
    Write-ProbeReport $report 2
}
if (-not $SkipChroma -and [string]::IsNullOrWhiteSpace($ChromaEndpoint)) {
    $report.status = "failed"
    $report.executed = $true
    $report.errors = @("ChromaEndpoint or AC_CHROMA_ENDPOINT is required")
    Write-ProbeReport $report 2
}
if (-not $SkipMariaDB -and [string]::IsNullOrWhiteSpace($MariaDBDSN)) {
    $report.status = "failed"
    $report.executed = $true
    $report.errors = @("MariaDBDSN or AC_MARIADB_DSN is required")
    Write-ProbeReport $report 2
}

$sourceRoot = [System.IO.Path]::GetFullPath((Join-Path $PSScriptRoot "..\..\.."))
$goServiceRoot = Join-Path $sourceRoot "go-service"
$schemaPath = Join-Path $sourceRoot "migrations\001_schema.sql"
if (-not (Test-Path -LiteralPath (Join-Path $goServiceRoot "go.mod") -PathType Leaf)) {
    $report.status = "failed"
    $report.executed = $true
    $report.errors = @("go-service source root was not found")
    Write-ProbeReport $report 2
}
if (-not $SkipMariaDB -and -not (Test-Path -LiteralPath $schemaPath -PathType Leaf)) {
    $report.status = "failed"
    $report.executed = $true
    $report.errors = @("MariaDB schema was not found")
    Write-ProbeReport $report 2
}

$report.status = "running"
$report.executed = $true
$report.evidence_class = "source_driven_live_dependency_probe"
$originalMariaDBDSN = $env:AC_MARIADB_DSN
try {
    Push-Location $goServiceRoot
    try {
        if (-not $SkipChroma) {
            $chromaArgs = @(
                "run", "-buildvcs=false", "./cmd/runtime-dependency-live-probe",
                "-execute",
                "-chroma-endpoint", $ChromaEndpoint
            )
            if ($null -ne $TimeoutSeconds) {
                $chromaArgs += @("-timeout", ("{0}s" -f $TimeoutSeconds))
            }
            $chromaOutput = & $GoCommand @chromaArgs
            $chromaExit = $LASTEXITCODE
            if ($chromaExit -ne 0) {
                throw "ChromaDB live probe exited with code $chromaExit"
            }
            $report.chroma = ($chromaOutput -join [Environment]::NewLine) | ConvertFrom-Json
        }

        if (-not $SkipMariaDB) {
            $env:AC_MARIADB_DSN = $MariaDBDSN
            $mariaArgs = @(
                "run", "-buildvcs=false", "./cmd/mariadb-schema",
                "-schema", $schemaPath,
                "-execute",
                "-app-account-probe"
            )
            if ($null -ne $TimeoutSeconds) {
                $mariaArgs += @("-timeout", ("{0}s" -f $TimeoutSeconds))
            }
            $mariaOutput = & $GoCommand @mariaArgs
            $mariaExit = $LASTEXITCODE
            if ($mariaExit -ne 0) {
                throw "MariaDB schema/application-account probe exited with code $mariaExit"
            }
            $report.mariadb = ($mariaOutput -join [Environment]::NewLine) | ConvertFrom-Json
        }
    } finally {
        Pop-Location
    }
} catch {
    $report.status = "failed"
    $report.errors = @($_.Exception.Message)
    Write-ProbeReport $report 1
} finally {
    $env:AC_MARIADB_DSN = $originalMariaDBDSN
}

$report.status = "ok"
Write-ProbeReport $report 0
