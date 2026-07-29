param(
    [Parameter(Mandatory = $true)][string]$PackageRoot,
    [string]$UpdateZip = "",
    [string]$CurrentVersion = ""
)

$ErrorActionPreference = "Stop"
Set-StrictMode -Version 2.0

function Get-LowerSHA256([string]$Path) {
    (Get-FileHash -LiteralPath $Path -Algorithm SHA256).Hash.ToLowerInvariant()
}

function Write-Utf8NoBomAtomic([string]$Path, $Value) {
    $temporary = "$Path.tmp"
    New-Item -ItemType Directory -Force -Path (Split-Path -Parent $Path) | Out-Null
    [System.IO.File]::WriteAllText(
        $temporary,
        ($Value | ConvertTo-Json -Depth 8) + [Environment]::NewLine,
        (New-Object System.Text.UTF8Encoding($false))
    )
    Move-Item -LiteralPath $temporary -Destination $Path -Force
}

function Get-PackageVersion([string]$Root) {
    $envExample = Join-Path $Root ".env.full.example"
    if (-not (Test-Path -LiteralPath $envExample -PathType Leaf)) {
        return ""
    }
    foreach ($line in Get-Content -LiteralPath $envExample -Encoding UTF8) {
        if ($line -match '^\s*AC_BUILD_VERSION\s*=\s*(\S+)\s*$') {
            return $Matches[1].Trim()
        }
    }
    return ""
}

function Normalize-Version([string]$Value) {
    $Value.Trim().TrimStart("v", "V")
}

function Quote-ProcessArgument([string]$Value) {
    if ($Value -notmatch '[\s"]') {
        return $Value
    }
    return '"' + ($Value -replace '(\\*)"', '$1$1\"' -replace '(\\+)$', '$1$1') + '"'
}

function Invoke-BridgeUpdater([string]$Runner, [string]$Root) {
    $startInfo = New-Object System.Diagnostics.ProcessStartInfo
    $startInfo.FileName = $Runner
    $startInfo.Arguments = (Quote-ProcessArgument "apply-pending") + " --root " + (Quote-ProcessArgument $Root)
    $startInfo.WorkingDirectory = $Root
    $startInfo.UseShellExecute = $false
    $startInfo.CreateNoWindow = $true
    $startInfo.RedirectStandardOutput = $true
    $startInfo.RedirectStandardError = $true
    $process = New-Object System.Diagnostics.Process
    $process.StartInfo = $startInfo
    if (-not $process.Start()) {
        throw "Compatibility bridge updater failed to start."
    }
    $stdoutTask = $process.StandardOutput.ReadToEndAsync()
    $stderrTask = $process.StandardError.ReadToEndAsync()
    $process.WaitForExit()
    $stdout = ([string]$stdoutTask.GetAwaiter().GetResult()).Trim()
    $stderr = ([string]$stderrTask.GetAwaiter().GetResult()).Trim()
    if ($process.ExitCode -ne 0) {
        if (-not [string]::IsNullOrWhiteSpace($stdout) -or [string]::IsNullOrWhiteSpace($stderr)) {
            throw "Compatibility bridge updater violated failure IPC (exit $($process.ExitCode))."
        }
        try {
            $failure = $stderr | ConvertFrom-Json -ErrorAction Stop
        } catch {
            throw "Compatibility bridge updater returned invalid failure JSON (exit $($process.ExitCode))."
        }
        if ([string]$failure.contract_version -cne "archive-center.updater-result.v1" -or
            [string]$failure.action -cne "apply-pending" -or
            [string]$failure.status -cne "error") {
            throw "Compatibility bridge updater returned an invalid failure contract."
        }
        $failureMessage = ([string]$failure.message) -replace '[\r\n]+', ' '
        if ($failureMessage.Length -gt 512) {
            $failureMessage = $failureMessage.Substring(0, 512) + "...<truncated>"
        }
        throw "Compatibility bridge updater rejected the transaction: $([string]$failure.code) ($failureMessage)"
    }
    if ([string]::IsNullOrWhiteSpace($stdout) -or -not [string]::IsNullOrWhiteSpace($stderr)) {
        throw "Compatibility bridge updater violated success IPC."
    }
    try {
        $result = $stdout | ConvertFrom-Json -ErrorAction Stop
    } catch {
        throw "Compatibility bridge updater returned invalid success JSON."
    }
    if ([string]$result.contract_version -cne "archive-center.updater-result.v1" -or
        [string]$result.action -cne "apply-pending" -or
        [string]$result.status -cne "applied_pending_health") {
        throw "Compatibility bridge updater did not enter applied_pending_health."
    }
    return $result
}

$bridgeRoot = (Resolve-Path -LiteralPath $PSScriptRoot).Path
$manifestPath = Join-Path $bridgeRoot "BRIDGE_MANIFEST.json"
$updaterSource = Join-Path $bridgeRoot "archive-center-updater.exe"
$scriptPath = $MyInvocation.MyCommand.Path
if (-not (Test-Path -LiteralPath $manifestPath -PathType Leaf) -or
    -not (Test-Path -LiteralPath $updaterSource -PathType Leaf)) {
    throw "Compatibility bridge bundle is incomplete."
}
try {
    $manifest = Get-Content -LiteralPath $manifestPath -Raw -Encoding UTF8 | ConvertFrom-Json -ErrorAction Stop
} catch {
    throw "Compatibility bridge manifest is unreadable."
}
if ([string]$manifest.contract_version -cne "archive-center.external-update-bridge.v1") {
    throw "Compatibility bridge manifest contract mismatch."
}
if ((Get-LowerSHA256 $updaterSource) -cne ([string]$manifest.updater_sha256).ToLowerInvariant() -or
    (Get-LowerSHA256 $scriptPath) -cne ([string]$manifest.script_sha256).ToLowerInvariant()) {
    throw "Compatibility bridge executable or script hash mismatch."
}

$packageRootFull = (Resolve-Path -LiteralPath $PackageRoot).Path
if ([string]::IsNullOrWhiteSpace($CurrentVersion)) {
    $CurrentVersion = Get-PackageVersion $packageRootFull
}
$currentNormalized = Normalize-Version $CurrentVersion
$supported = @($manifest.sources | ForEach-Object { Normalize-Version ([string]$_.version) })
if ([string]::IsNullOrWhiteSpace($currentNormalized) -or $currentNormalized -notin $supported) {
    throw "Installed package version '$CurrentVersion' is not an authenticated bridge source."
}

if ([string]::IsNullOrWhiteSpace($UpdateZip)) {
    $UpdateZip = Join-Path (Split-Path -Parent $bridgeRoot) ([string]$manifest.update_asset_name)
}
$updateZipFull = (Resolve-Path -LiteralPath $UpdateZip).Path
if ([System.IO.Path]::GetFileName($updateZipFull) -cne [string]$manifest.update_asset_name -or
    (Get-LowerSHA256 $updateZipFull) -cne ([string]$manifest.update_sha256).ToLowerInvariant()) {
    throw "The selected 3.7 update ZIP does not match the authenticated bridge manifest."
}

$updatesRoot = Join-Path $packageRootFull ".updates"
$pendingPath = Join-Path $updatesRoot "pending-update.json"
$statePath = Join-Path $updatesRoot "update-state.json"
if (Test-Path -LiteralPath $pendingPath -PathType Leaf) {
    throw "An update is already pending. Resolve it before running the compatibility bridge."
}
if (Test-Path -LiteralPath $statePath -PathType Leaf) {
    try {
        $existingState = Get-Content -LiteralPath $statePath -Raw -Encoding UTF8 | ConvertFrom-Json -ErrorAction Stop
    } catch {
        throw "Existing update state is unreadable."
    }
    if ([string]$existingState.contract_version -cne "archive-center.update-state.v1" -or
        ([string]$existingState.status).Trim().ToLowerInvariant() -notin @("committed", "rolled_back")) {
        throw "Existing update state is not a safe terminal state."
    }
}

$targetVersion = ([string]$manifest.target_version).Trim()
$stagedAsset = Join-Path $updatesRoot ("external-bridge-" + ($targetVersion -replace '[^A-Za-z0-9_.-]', '_') + ".zip")
New-Item -ItemType Directory -Force -Path $updatesRoot | Out-Null
Copy-Item -LiteralPath $updateZipFull -Destination $stagedAsset -Force
if ((Get-LowerSHA256 $stagedAsset) -cne ([string]$manifest.update_sha256).ToLowerInvariant()) {
    throw "Staged update ZIP hash verification failed."
}

$updaterSHA = ([string]$manifest.updater_sha256).ToLowerInvariant()
$runner = Join-Path $updatesRoot "runner\archive-center-updater-bridge.exe"
New-Item -ItemType Directory -Force -Path (Split-Path -Parent $runner) | Out-Null
Copy-Item -LiteralPath $updaterSource -Destination $runner -Force
if ((Get-LowerSHA256 $runner) -cne $updaterSHA) {
    throw "Preserved bridge updater hash verification failed."
}

$pending = [ordered]@{
    contract_version = "archive-center.pending-update.v1"
    current_version = $currentNormalized
    target_version = $targetVersion
    asset_path = ".updates/" + [System.IO.Path]::GetFileName($stagedAsset)
    sha256 = ([string]$manifest.update_sha256).ToLowerInvariant()
    required_files = @(
        "bin/archive-center-go.exe",
        "bin/archive-center-updater.exe",
        "Archive Center.js",
        "PACKAGE_MIGRATION_UPDATE.json"
    )
}
Write-Utf8NoBomAtomic -Path $pendingPath -Value $pending

$runnerIdentity = [ordered]@{
    contract_version = "archive-center.updater-runner-identity.v1"
    target_version = $targetVersion
    runner_path = ".updates/runner/" + [System.IO.Path]::GetFileName($runner)
    runner_sha256 = $updaterSHA
    written_at = [DateTimeOffset]::UtcNow.ToString("o")
}
Write-Utf8NoBomAtomic -Path (Join-Path $updatesRoot "runner-identity.json") -Value $runnerIdentity

$result = Invoke-BridgeUpdater -Runner $runner -Root $packageRootFull
[ordered]@{
    contract_version = "archive-center.external-update-bridge-result.v1"
    status = "applied_pending_health"
    current_version = $currentNormalized
    target_version = $targetVersion
    runner_sha256 = $updaterSHA
    next_step = "start Archive Center normally; the packaged launcher will run backend readiness and commit or rollback"
    updater_result = $result
} | ConvertTo-Json -Depth 8
