param(
    [string]$SourceDb = "",
    [switch]$Execute
)

$ErrorActionPreference = "Stop"

$packRoot = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
Set-Location $packRoot

$tool = Join-Path $packRoot "bin\legacy10-migrate.exe"
if (-not (Test-Path -LiteralPath $tool -PathType Leaf)) {
    throw "Missing migration tool: $tool"
}

if ([string]::IsNullOrWhiteSpace($SourceDb)) {
    Write-Host ""
    Write-Host "Archive Center 1.0 -> 2.0 DB migration"
    Write-Host "Enter the full path to the old 1.0 memory.db file."
    Write-Host "The source DB is opened read-only."
    Write-Host ""
    $SourceDb = Read-Host "Source memory.db"
}

if ([string]::IsNullOrWhiteSpace($SourceDb) -or -not (Test-Path -LiteralPath $SourceDb -PathType Leaf)) {
    throw "Source DB not found: $SourceDb"
}

$workRoot = Join-Path $packRoot ".runtime\legacy-migration"
New-Item -ItemType Directory -Force -Path $workRoot | Out-Null
$stamp = Get-Date -Format "yyyyMMdd-HHmmss"
$workDir = Join-Path $workRoot $stamp
$reportPath = Join-Path $workDir "legacy10-migrate-report.json"

if ([string]::IsNullOrWhiteSpace($env:AC_MARIADB_DSN)) {
    $env:AC_MARIADB_DSN = "archive_center:archive-center-local-pass@tcp(127.0.0.1:3307)/archive_center?parseTime=true"
}

Write-Host ""
Write-Host "Step 1/2: dry-run validation"
& $tool -sqlite-db $SourceDb -work-dir $workDir -out $reportPath
if ($LASTEXITCODE -ne 0) {
    throw "Dry-run migration validation failed. Report: $reportPath"
}
Write-Host "Dry-run report:"
Write-Host "  $reportPath"

if (-not $Execute) {
    Write-Host ""
    $answer = Read-Host "Import into the running 2.0 MariaDB now? Type YES to continue"
    if ($answer -ne "YES") {
        Write-Host "Stopped after dry-run. No MariaDB rows were changed."
        return
    }
}

Write-Host ""
Write-Host "Step 2/2: MariaDB import"
$executeReportPath = Join-Path $workDir "legacy10-migrate-execute-report.json"
& $tool -sqlite-db $SourceDb -work-dir $workDir -dsn $env:AC_MARIADB_DSN -execute -out $executeReportPath
if ($LASTEXITCODE -ne 0) {
    throw "MariaDB import failed. Report: $executeReportPath"
}

Write-Host "Migration import completed."
Write-Host "Report:"
Write-Host "  $executeReportPath"
Write-Host ""
Write-Host "Next: open Archive Center 2.1, confirm sessions/timeline, then run vector reindex if imported memories need ChromaDB search."
