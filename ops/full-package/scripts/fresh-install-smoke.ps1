param(
    [switch]$SkipRuntimePayloadCheck,
    [int]$Port = 28180,
    [string]$RuntimeProfile = "",
    [string]$VectorMode = "",
    [string]$ChromaEndpoint = ""
)

$ErrorActionPreference = "Stop"

function Find-MariaDBProvider([string]$Root) {
    $hit = Get-ChildItem -LiteralPath $Root -Recurse -File -ErrorAction SilentlyContinue |
        Where-Object { $_.Name -ieq "mariadbd.exe" -or $_.Name -ieq "mysqld.exe" } |
        Sort-Object FullName |
        Select-Object -First 1
    if ($null -eq $hit) { return "" }
    return $hit.FullName
}

function Find-ChromaRuntime([string]$Root) {
    $python = Get-ChildItem -LiteralPath $Root -Recurse -File -ErrorAction SilentlyContinue |
        Where-Object { $_.Name -ieq "python.exe" } |
        Sort-Object FullName |
        Select-Object -First 1
    if ($null -ne $python) { return $python.FullName }
    $chroma = Get-ChildItem -LiteralPath $Root -Recurse -Directory -ErrorAction SilentlyContinue |
        Where-Object { $_.Name -ieq "chromadb" -or $_.Name -ieq "ChromaDB" } |
        Sort-Object FullName |
        Select-Object -First 1
    if ($null -ne $chroma) { return $chroma.FullName }
    return ""
}

function Invoke-Json($Uri) {
    Invoke-RestMethod -Method GET -Uri $Uri -TimeoutSec 2
}

$packRoot = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
$failures = [System.Collections.Generic.List[string]]::new()
$warnings = [System.Collections.Generic.List[string]]::new()

foreach ($rel in @("bin\archive-center-go.exe", "bin\mariadb-schema.exe", "Archive Center.js", "migrations", "prompts", "scripts")) {
    $path = Join-Path $packRoot $rel
    if (-not (Test-Path -LiteralPath $path)) {
        [void]$failures.Add("missing:$rel")
    }
}

if (-not $SkipRuntimePayloadCheck) {
    if ([string]::IsNullOrWhiteSpace((Find-MariaDBProvider (Join-Path $packRoot "runtime")))) {
        [void]$failures.Add("missing:mariadb_runtime")
    }
    if ([string]::IsNullOrWhiteSpace((Find-ChromaRuntime (Join-Path $packRoot "runtime")))) {
        [void]$failures.Add("missing:chromadb_runtime")
    }
}

$node = Get-Command node -ErrorAction SilentlyContinue
if ($node) {
    & $node.Source --check (Join-Path $packRoot "Archive Center.js")
    if ($LASTEXITCODE -ne 0) {
        [void]$failures.Add("node_check_failed")
    }
} else {
    [void]$warnings.Add("node_not_available")
}

$forbidden = @(".git", ".runtime-cache", "go-service", "milvus.db", "milvus_data")
foreach ($rel in $forbidden) {
    if (Test-Path -LiteralPath (Join-Path $packRoot $rel)) {
        [void]$failures.Add("forbidden_payload:$rel")
    }
}

$backend = Join-Path $packRoot "bin\archive-center-go.exe"
$process = $null
$ready = $null
if (Test-Path -LiteralPath $backend -PathType Leaf) {
    $env:AC_MODE = "shadow"
    $env:AC_STORE_MODE = "noop"
    $env:AC_BIND_ADDR = "127.0.0.1:$Port"
    $env:AC_PROMPT_DIR = Join-Path $packRoot "prompts"
    if (-not [string]::IsNullOrWhiteSpace($RuntimeProfile)) {
        $env:AC_RUNTIME_PROFILE = $RuntimeProfile
    }
    if (-not [string]::IsNullOrWhiteSpace($VectorMode)) {
        $env:AC_VECTOR_MODE = $VectorMode
    }
    if (-not [string]::IsNullOrWhiteSpace($ChromaEndpoint)) {
        $env:AC_CHROMA_ENDPOINT = $ChromaEndpoint
    }
    $process = Start-Process -FilePath $backend -WorkingDirectory $packRoot -WindowStyle Hidden -PassThru
    try {
        $ok = $false
        for ($i = 0; $i -lt 30; $i++) {
            if ($process.HasExited) {
                break
            }
            try {
                $health = Invoke-Json "http://127.0.0.1:$Port/health"
                if ($health.status -eq "ok") {
                    $ok = $true
                    break
                }
            } catch {
            }
            Start-Sleep -Seconds 1
        }
        if (-not $ok) {
            [void]$failures.Add("backend_shadow_health_failed")
        } else {
            try {
                $ready = Invoke-Json "http://127.0.0.1:$Port/ready"
                if (-not $ready.ready) {
                    [void]$failures.Add("backend_ready_false")
                }
                if (-not [string]::IsNullOrWhiteSpace($RuntimeProfile) -and $ready.runtime_profile -ne $RuntimeProfile) {
                    [void]$failures.Add("runtime_profile_mismatch:$($ready.runtime_profile)")
                }
                if (-not [string]::IsNullOrWhiteSpace($VectorMode) -and $ready.vector_mode -ne $VectorMode) {
                    [void]$failures.Add("vector_mode_mismatch:$($ready.vector_mode)")
                }
            } catch {
                [void]$failures.Add("backend_ready_probe_failed:$($_.Exception.Message)")
            }
        }
    } finally {
        if ($process -and -not $process.HasExited) {
            $process.Kill()
            $process.WaitForExit()
        }
    }
}

$sizeBytes = (Get-ChildItem -LiteralPath $packRoot -Recurse -File -ErrorAction SilentlyContinue | Measure-Object -Property Length -Sum).Sum
$report = [ordered]@{
    schema_version = "archive-center.full-package.fresh-smoke.v1"
    generated_at = [DateTimeOffset]::UtcNow.ToString("o")
    package_root = $packRoot
    size_bytes = [int64]$sizeBytes
    skipped_runtime_payload_check = [bool]$SkipRuntimePayloadCheck
    requested_runtime_profile = $RuntimeProfile
    requested_vector_mode = $VectorMode
    ready = if ($null -eq $ready) {
        $null
    } else {
        [ordered]@{
            ready = $ready.ready
            mode = $ready.mode
            runtime_profile = $ready.runtime_profile
            vector_mode = $ready.vector_mode
            degraded = $ready.degraded
            checks = $ready.checks
        }
    }
    status = if ($failures.Count -eq 0) { "ok" } else { "blocked" }
    warnings = @($warnings)
    failures = @($failures)
}

$outDir = Join-Path $packRoot ".runtime\reports"
New-Item -ItemType Directory -Force -Path $outDir | Out-Null
$outFile = Join-Path $outDir "fresh-install-smoke.json"
$report | ConvertTo-Json -Depth 8 | Set-Content -LiteralPath $outFile -Encoding UTF8
$report | ConvertTo-Json -Depth 8
if ($failures.Count -gt 0) {
    exit 1
}
