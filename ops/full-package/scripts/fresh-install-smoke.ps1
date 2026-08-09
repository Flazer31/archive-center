param(
    [switch]$SkipRuntimePayloadCheck,
    [int]$Port = 28180,
    [string]$RuntimeProfile = "",
    [string]$VectorMode = "",
    [string]$ChromaEndpoint = "",
    [Nullable[int]]$ReadinessTimeoutSeconds = $null,
    [Nullable[int]]$ReadinessPollIntervalMilliseconds = $null,
    [Nullable[int]]$RequestTimeoutSeconds = $null
)

$ErrorActionPreference = "Stop"
foreach ($timeoutSetting in @($ReadinessTimeoutSeconds, $ReadinessPollIntervalMilliseconds, $RequestTimeoutSeconds)) {
    if ($null -ne $timeoutSetting -and $timeoutSetting -lt 1) {
        throw "Explicit timeout and polling values must be greater than zero."
    }
}
if (($null -eq $ReadinessTimeoutSeconds) -ne ($null -eq $ReadinessPollIntervalMilliseconds)) {
    throw "ReadinessTimeoutSeconds and ReadinessPollIntervalMilliseconds must be supplied together."
}
if ($null -eq $RequestTimeoutSeconds) {
    throw "RequestTimeoutSeconds is required for bounded package HTTP smoke probes."
}

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
    $invokeArgs = @{ Method = "GET"; Uri = $Uri }
    if ($null -ne $RequestTimeoutSeconds) {
        $invokeArgs.TimeoutSec = $RequestTimeoutSeconds
    }
    Invoke-RestMethod @invokeArgs
}

$packRoot = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
$failures = [System.Collections.Generic.List[string]]::new()
$warnings = [System.Collections.Generic.List[string]]::new()

foreach ($rel in @("bin\archive-center-go.exe", "bin\archive-center-updater.exe", "bin\mariadb-schema.exe", "Archive Center.js", "LICENSE", "NOTICE", "THIRD_PARTY_NOTICES.md", "licenses\Apache-2.0.txt", "migrations", "prompts", "scripts", "tools\install-windows.ps1", "01_start_archive_center_windows.bat", ".env.full.example", "FULL_PACKAGE_MANIFEST.json", "PACKAGE_FILE_MANIFEST.json", "SHA256SUMS.txt")) {
    $path = Join-Path $packRoot $rel
    if (-not (Test-Path -LiteralPath $path)) {
        [void]$failures.Add("missing:$rel")
    }
}

$managedManifestPath = Join-Path $packRoot "PACKAGE_FILE_MANIFEST.json"
if (Test-Path -LiteralPath $managedManifestPath -PathType Leaf) {
    try {
        $managedManifest = Get-Content -LiteralPath $managedManifestPath -Raw -Encoding UTF8 | ConvertFrom-Json
        if ($managedManifest.scope -ne "managed_package_payloads") {
            [void]$failures.Add("managed_manifest_scope_invalid:$($managedManifest.scope)")
        }
        if ([string]::IsNullOrWhiteSpace([string]$managedManifest.package_version)) {
            [void]$failures.Add("managed_manifest_package_version_missing")
        }
        if ([string]$managedManifest.source_commit -ne "unknown" -and [string]$managedManifest.source_commit -notmatch '^[0-9a-f]{40}$') {
            [void]$failures.Add("managed_manifest_source_commit_invalid:$($managedManifest.source_commit)")
        }
        if ($managedManifest.source_dirty -isnot [bool] -and [string]$managedManifest.source_dirty -ne "unknown") {
            [void]$failures.Add("managed_manifest_source_dirty_invalid:$($managedManifest.source_dirty)")
        }
        if ([string]::IsNullOrWhiteSpace([string]$managedManifest.source_dirty_scope)) {
            [void]$failures.Add("managed_manifest_source_dirty_scope_missing")
        }
        if ([string]$managedManifest.build_command -notmatch '^ops/build-full-package\.ps1 -PackageKind \S+ -PackageVersion \S+ -Zip:(True|False) -UpdateZip:(True|False) -CodeSigning:(True|False)$') {
            [void]$failures.Add("managed_manifest_build_descriptor_invalid")
        }
        $managedPaths = @($managedManifest.files | ForEach-Object { ([string]$_.path).Replace('\', '/') })
        foreach ($requiredManagedPath in @("bin/archive-center-go.exe", "bin/archive-center-updater.exe", "bin/mariadb-schema.exe", "scripts/start-full-windows.ps1", "01_start_archive_center_windows.bat", "tools/install-windows.ps1", "PACKAGE_MIGRATION_UPDATE.json", "PACKAGE_RELEASE_STATUS.json", "Archive Center.js")) {
            if ($managedPaths -notcontains $requiredManagedPath) {
                [void]$failures.Add("managed_manifest_missing:$requiredManagedPath")
            }
        }
        foreach ($requiredPrefix in @("migrations/", "prompts/", "scripts/")) {
            if (@($managedPaths | Where-Object { $_.StartsWith($requiredPrefix, [System.StringComparison]::OrdinalIgnoreCase) }).Count -eq 0) {
                [void]$failures.Add("managed_manifest_missing_prefix:$requiredPrefix")
            }
        }
        foreach ($managedPath in $managedPaths) {
            if ($managedPath -match '^(?i)(\.runtime|\.runtime-cache|\.updates)/' -or $managedPath -in @(".env.full.local", ".env.full.local.protected")) {
                [void]$failures.Add("managed_manifest_contains_user_data:$managedPath")
            }
        }
    } catch {
        [void]$failures.Add("managed_manifest_invalid:$($_.Exception.Message)")
    }
}

$fullManifestPath = Join-Path $packRoot "FULL_PACKAGE_MANIFEST.json"
if (Test-Path -LiteralPath $fullManifestPath -PathType Leaf) {
    try {
        $fullManifest = Get-Content -LiteralPath $fullManifestPath -Raw -Encoding UTF8 | ConvertFrom-Json
        if ($fullManifest.package_kind -eq "managed") {
            if ($fullManifest.runtime_profile_default -ne "full_local") {
                [void]$failures.Add("managed_runtime_profile_default_invalid:$($fullManifest.runtime_profile_default)")
            }
            if ($fullManifest.vector_mode_default -ne "bundled") {
                [void]$failures.Add("managed_vector_mode_default_invalid:$($fullManifest.vector_mode_default)")
            }
            if ($fullManifest.chromadb_distribution -ne "separate_pinned_runtime_install") {
                [void]$failures.Add("managed_chromadb_distribution_invalid:$($fullManifest.chromadb_distribution)")
            }
        }
    } catch {
        [void]$failures.Add("full_package_manifest_invalid:$($_.Exception.Message)")
    }
}

$standardLauncherPath = Join-Path $packRoot "01_start_archive_center_windows.bat"
if (Test-Path -LiteralPath $standardLauncherPath -PathType Leaf) {
    $standardLauncherText = Get-Content -LiteralPath $standardLauncherPath -Raw -Encoding UTF8
    if ($standardLauncherText -notmatch '(?im)powershell[^\r\n]*-RuntimeProfile\s+"full_local"[^\r\n]*-VectorMode\s+"bundled"') {
        [void]$failures.Add("standard_launcher_full_local_vector_contract_missing")
    }
}

$fullEnvExamplePath = Join-Path $packRoot ".env.full.example"
if (Test-Path -LiteralPath $fullEnvExamplePath -PathType Leaf) {
    $fullEnvExampleText = Get-Content -LiteralPath $fullEnvExamplePath -Raw -Encoding UTF8
    if ($fullEnvExampleText -notmatch '(?m)^AC_RUNTIME_PROFILE=full_local\s*$') {
        [void]$failures.Add("full_env_runtime_profile_default_invalid")
    }
    if ($fullEnvExampleText -notmatch '(?m)^AC_VECTOR_MODE=bundled\s*$') {
        [void]$failures.Add("full_env_vector_mode_default_invalid")
    }
}

$launcherScriptPath = Join-Path $packRoot "scripts\start-full-windows.ps1"
if (Test-Path -LiteralPath $launcherScriptPath -PathType Leaf) {
    $launcherScriptText = Get-Content -LiteralPath $launcherScriptPath -Raw -Encoding UTF8
    $unstampedVersionToken = "__ARCHIVE_CENTER_" + "PACKAGE_VERSION__"
    if ($launcherScriptText.Contains($unstampedVersionToken)) {
        [void]$failures.Add("launcher_package_version_not_stamped")
    }
    foreach ($marker in @("archive-center-updater.exe", "apply-pending", "applied_pending_health", "Wait-BackendMainReady", "/version", "statePreviousMarker", "Using preserved updater recovery runner", 'Invoke-ArchiveUpdater -RunnerPath $updaterRunner -Command "status"', 'safety.Status -in @("no_state", "rolled_back", "nothing_to_rollback")', 'Invoke-ArchiveUpdater -RunnerPath $updaterRunner -Command "commit"', 'Invoke-ArchiveUpdater -RunnerPath $updaterRunner -Command "rollback"', 'No pending Archive Center update exists. Normal startup will continue.', 'Warning: Archive Center updater is not installed. No pending state exists, so normal startup will continue.', 'AC_UPDATE_APPLY_MODE = "managed_launcher_exit_75"', 'AC_UPDATE_STAGING_DIR = Join-Path $packRoot ".updates"', 'AC_UPDATE_LAUNCHER_TOKEN', 'launcher-session.json', 'Start-ArchiveBackendProcess', '$backendExitCode -eq 75', '& $PSCommandPath @launcherParameters', '-schema (Join-Path $packRoot "migrations")')) {
        if (-not $launcherScriptText.Contains($marker)) {
            [void]$failures.Add("launcher_update_marker_missing:$marker")
        }
    }
    foreach ($marker in @('function Wait-Port([int]$Port, [int]$TimeoutSeconds = 60)', 'Start-Sleep -Seconds 1', '$Process.HasExited', '$ready.ready -eq $true', '$process.WaitForExit()', 'This Windows full package requires an active ChromaDB vector mode')) {
        if (-not $launcherScriptText.Contains($marker)) {
            [void]$failures.Add("launcher_signal_readiness_marker_missing:$marker")
        }
    }
    foreach ($forbiddenPattern in @('AC_EXTERNAL_OPERATION_TIMEOUT_SECONDS', 'External child operation requires', 'Managed ChromaDB installation requires', 'Invoke-BoundedArchiveChildProcess', 'SpinWait', 'adaptiveWait')) {
        if ($launcherScriptText -match $forbiddenPattern) {
            [void]$failures.Add("launcher_hidden_fixed_time_policy_present:$forbiddenPattern")
        }
    }
    if ($launcherScriptText.Contains("archive-center-go.new.exe")) {
        [void]$failures.Add("launcher_legacy_partial_update_path_present")
    }
    foreach ($marker in @("AC_MARIADB_RUNTIME_DIR", "-InstallMariaDBRuntime", "-managed-bootstrap", "-expected-datadir", "AC_CHROMA_RUNTIME_DIR", "-InstallChromaDBRuntime", "Test-ChromaRuntimeVersion", "chromadb==`$managedChromaDBVersion", "chromaRuntimeReady", "Repairing the per-user runtime", "Start-ManagedChromaDB", "LocalApplicationData")) {
        if (-not $launcherScriptText.Contains($marker)) {
            [void]$failures.Add("launcher_managed_runtime_marker_missing:$marker")
        }
    }
    foreach ($marker in @(
        'else { "full_local" }',
        '"full_local" { $vectorCandidate = "bundled" }',
        '$env:AC_CHROMA_ENDPOINT = "http://127.0.0.1:8000"'
    )) {
        if (-not $launcherScriptText.Contains($marker)) {
            [void]$failures.Add("launcher_full_local_vector_contract_missing:$marker")
        }
    }
    foreach ($forbiddenMarker in @("CREATE USER IF NOT EXISTS 'archive_center'", "GRANT ALL PRIVILEGES ON archive_center.*")) {
        if ($launcherScriptText.Contains($forbiddenMarker)) {
            [void]$failures.Add("launcher_direct_mariadb_bootstrap_sql_present:$forbiddenMarker")
        }
    }
    if ($launcherScriptText -notmatch '(?s)\$chromaRuntimeReady\s*=\s*Test-ChromaRuntimeVersion.*?if\s*\(-not\s+\$chromaRuntimeReady\).*?-InstallChromaDBRuntime.*?\$chromaRuntimeReady\s*=\s*Test-ChromaRuntimeVersion') {
        [void]$failures.Add("launcher_chromadb_health_repair_recheck_flow_missing")
    }
    if ($launcherScriptText -notmatch '(?s)if \(\$pendingApplyStatus -eq "applied_pending_health"\).*?\}\s*else\s*\{\s*\$backendProcess\s*=\s*Start-ArchiveBackendProcess.*?Wait-ArchiveBackendLifetime') {
        [void]$failures.Add("launcher_no_pending_managed_backend_path_missing")
    }
    if ($launcherScriptText -notmatch '(?s)function Stop-ArchiveChildProcess\(\[System\.Diagnostics\.Process\[\]\]\$Process\).*?\$shutdownDeadline\s*=\s*\[DateTime\]::UtcNow\.AddSeconds\(10\).*?\$target\.Kill\(\)') {
        [void]$failures.Add("launcher_group_shutdown_deadline_missing")
    }
    if ($launcherScriptText -notmatch '(?s)JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE.*?AssignProcessToJobObject.*?ExitWhenProcessEnds.*?\$script:archiveProcessJob\.AddProcess\(\$proc\.Handle\).*?\$archiveProcessJob\.ExitWhenProcessEnds\(\[ArchiveCenter\.ManagedProcessJob\]::GetCurrentParentProcessId\(\)\)') {
        [void]$failures.Add("launcher_job_object_lifetime_binding_missing")
    }
    if ($launcherScriptText -notmatch '(?s)Wait-Process -InputObject \$Process.*?finally \{.*?Stop-ArchiveChildProcess -Process @\(\s*\$backendProcess,\s*\$candidateBackend,\s*\$restoredBackend,\s*\$startedChroma,\s*\$startedMariaDB\s*\).*?\$archiveProcessJob\.Dispose\(\)') {
        [void]$failures.Add("launcher_ctrl_c_full_process_cleanup_missing")
    }
    if ($launcherScriptText.Contains('KeepServices')) {
        [void]$failures.Add("launcher_keep_services_option_must_not_exist")
    }
}

foreach ($rel in @(
    "bin\sqlite-export.exe",
    "bin\dry-run-validator.exe",
    "bin\compare-dry-run.exe",
    "bin\mariadb-dry-run-import.exe",
    "bin\mariadb-import.exe",
    "bin\legacy10-migrate.exe",
    "06_migrate_1_0_to_2_0_windows.bat",
    "scripts\migrate-legacy-1.0-windows.ps1"
)) {
    if (Test-Path -LiteralPath (Join-Path $packRoot $rel)) {
        [void]$failures.Add("forbidden_legacy_migration_payload:$rel")
    }
}

$installerScriptPath = Join-Path $packRoot "tools\install-windows.ps1"
if (Test-Path -LiteralPath $installerScriptPath -PathType Leaf) {
    $installerScriptText = Get-Content -LiteralPath $installerScriptPath -Raw -Encoding UTF8
    foreach ($marker in @(
        "-InstallMariaDBRuntime",
        "-InstallChromaDBRuntime",
        "https://downloads.mariadb.org/rest-api/mariadb/12.3.2/mariadb-12.3.2-winx64.zip",
        "67347c129eb9c5923d002ea34fbfa27c60eb95d36dd73b85af2651cdeceecac5",
        "MariaDB archive SHA-256 mismatch",
        "https://www.python.org/ftp/python/3.11.9/python-3.11.9-amd64.exe",
        "5ee42c4eee1e6b4464bb23722f90b45303f79442df63083f05322f1785f5fdde",
        "Python installer Authenticode verification failed",
        "Test-CompatiblePythonBootstrap",
        "Python registration exists but the runtime is incomplete",
        "Start-Process",
        "-Wait",
        "Invoke-WebRequest",
        "chromadb==`$ChromaDBVersion",
        "package_bundled = `$false"
    )) {
        if (-not $installerScriptText.Contains($marker)) {
            [void]$failures.Add("installer_separate_mariadb_marker_missing:$marker")
        }
    }
}

if (-not $SkipRuntimePayloadCheck) {
    if (-not [string]::IsNullOrWhiteSpace((Find-MariaDBProvider (Join-Path $packRoot "runtime")))) {
        [void]$failures.Add("forbidden_payload:mariadb_runtime")
    }
    if (-not [string]::IsNullOrWhiteSpace((Find-ChromaRuntime (Join-Path $packRoot "runtime")))) {
        [void]$failures.Add("forbidden_payload:chromadb_runtime")
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
        $watch = [System.Diagnostics.Stopwatch]::StartNew()
        while (-not $process.HasExited) {
            try {
                $health = Invoke-Json "http://127.0.0.1:$Port/health"
                if ($health.status -eq "ok") {
                    $ok = $true
                    break
                }
            } catch {
            }
            if ($null -eq $ReadinessTimeoutSeconds -or $null -eq $ReadinessPollIntervalMilliseconds) {
                break
            }
            if ($null -ne $ReadinessTimeoutSeconds -and $watch.Elapsed.TotalSeconds -ge $ReadinessTimeoutSeconds) {
                break
            }
            $remainingMilliseconds = [int][Math]::Ceiling(($ReadinessTimeoutSeconds - $watch.Elapsed.TotalSeconds) * 1000)
            if ($remainingMilliseconds -le 0) {
                break
            }
            $waitMilliseconds = [Math]::Min($ReadinessPollIntervalMilliseconds, $remainingMilliseconds)
            if ($process.WaitForExit($waitMilliseconds)) {
                break
            }
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
            $null = $process.WaitForExit([int64]$RequestTimeoutSeconds * 1000)
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
