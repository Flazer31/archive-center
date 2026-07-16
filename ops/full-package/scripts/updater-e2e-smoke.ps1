param(
    [string]$PackageRoot = ""
)

$ErrorActionPreference = "Stop"
Set-StrictMode -Version 2.0

function Assert-True([bool]$Condition, [string]$Message) {
    if (-not $Condition) {
        throw $Message
    }
}

function Assert-Equal($Actual, $Expected, [string]$Message) {
    if ($Actual -ne $Expected) {
        throw "$Message (actual='$Actual', expected='$Expected')"
    }
}

function Get-OptionalProperty($Object, [string]$Name) {
    if ($null -eq $Object) { return $null }
    $property = $Object.PSObject.Properties[$Name]
    if ($null -eq $property) { return $null }
    return $property.Value
}

function Get-OptionalArrayCount($Object, [string]$Name) {
    $value = Get-OptionalProperty $Object $Name
    if ($null -eq $value) { return 0 }
    return @($value).Count
}

function Write-Utf8NoBom([string]$Path, [string]$Text) {
    $parent = Split-Path -Parent $Path
    if (-not [string]::IsNullOrWhiteSpace($parent)) {
        New-Item -ItemType Directory -Force -Path $parent | Out-Null
    }
    [System.IO.File]::WriteAllText($Path, $Text, (New-Object System.Text.UTF8Encoding($false)))
}

function Copy-Bytes([string]$Source, [string]$Destination) {
    New-Item -ItemType Directory -Force -Path (Split-Path -Parent $Destination) | Out-Null
    [System.IO.File]::WriteAllBytes($Destination, [System.IO.File]::ReadAllBytes($Source))
}

function Get-LowerSHA256([string]$Path) {
    (Get-FileHash -LiteralPath $Path -Algorithm SHA256).Hash.ToLowerInvariant()
}

function New-ManagedManifest([string]$Root, [string[]]$RelativePaths) {
    $items = @()
    foreach ($relative in @($RelativePaths | Sort-Object)) {
        $path = Join-Path $Root ($relative.Replace('/', '\'))
        Assert-True (Test-Path -LiteralPath $path -PathType Leaf) "Managed fixture file is missing: $path"
        $item = Get-Item -LiteralPath $path
        $items += [ordered]@{
            path = $relative.Replace('\', '/')
            size_bytes = [int64]$item.Length
            sha256 = Get-LowerSHA256 $path
        }
    }
    [ordered]@{
        schema_version = "archive-center.package-file-manifest.v1"
        scope = "managed_package_payloads"
        files = @($items)
    }
}

function Write-Manifest([string]$Root, [string[]]$RelativePaths) {
    $manifest = New-ManagedManifest $Root $RelativePaths
    $json = $manifest | ConvertTo-Json -Depth 8
    $path = Join-Path $Root "PACKAGE_FILE_MANIFEST.json"
    Write-Utf8NoBom $path ($json + "`n")
    return $path
}

function New-CandidateArchive([string]$ScenarioDir, [string]$PackageRoot, [string]$UpdaterExe, [string]$ScenarioName) {
    $sourceParent = Join-Path $ScenarioDir "candidate-source"
    $wrapper = Join-Path $sourceParent "archive-center-candidate"
    New-Item -ItemType Directory -Force -Path $wrapper | Out-Null
    Write-Utf8NoBom (Join-Path $wrapper "Archive Center.js") "candidate-js-$ScenarioName`n"
    Write-Utf8NoBom (Join-Path $wrapper "bin\archive-center-go.exe") "candidate-backend-$ScenarioName`n"
    Copy-Bytes $UpdaterExe (Join-Path $wrapper "bin\archive-center-updater.exe")
    Write-Utf8NoBom (Join-Path $wrapper "scripts\new-tool.ps1") "Write-Output 'candidate-$ScenarioName'`n"
    $managed = @(
        "Archive Center.js",
        "bin/archive-center-go.exe",
        "bin/archive-center-updater.exe",
        "scripts/new-tool.ps1"
    )
    $manifestPath = Write-Manifest $wrapper $managed
    $manifestBytes = [System.IO.File]::ReadAllBytes($manifestPath)
    $asset = Join-Path $PackageRoot ".updates\candidate.zip"
    New-Item -ItemType Directory -Force -Path (Split-Path -Parent $asset) | Out-Null
    Add-Type -AssemblyName System.IO.Compression.FileSystem
    [System.IO.Compression.ZipFile]::CreateFromDirectory($sourceParent, $asset, [System.IO.Compression.CompressionLevel]::Optimal, $false)
    [pscustomobject]@{
        Asset = $asset
        SHA256 = Get-LowerSHA256 $asset
        Managed = $managed
        ManifestBytes = $manifestBytes
        ExpectedJS = "candidate-js-$ScenarioName`n"
        ExpectedBackend = "candidate-backend-$ScenarioName`n"
        ExpectedScript = "Write-Output 'candidate-$ScenarioName'`n"
    }
}

function New-Scenario([string]$RunRoot, [string]$UpdaterExe, [string]$Name) {
    $scenarioDir = Join-Path $RunRoot $Name
    $root = Join-Path $scenarioDir "package-root"
    New-Item -ItemType Directory -Force -Path $root | Out-Null
    Write-Utf8NoBom (Join-Path $root "Archive Center.js") "baseline-js-$Name`n"
    Write-Utf8NoBom (Join-Path $root "bin\archive-center-go.exe") "baseline-backend-$Name`n"
    Copy-Bytes $UpdaterExe (Join-Path $root "bin\archive-center-updater.exe")
    $baselineManaged = @("Archive Center.js", "bin/archive-center-go.exe", "bin/archive-center-updater.exe")
    $baselineManifestPath = Write-Manifest $root $baselineManaged
    $baseline = [ordered]@{}
    foreach ($relative in $baselineManaged) {
        $path = Join-Path $root ($relative.Replace('/', '\'))
        $baseline[$relative] = Get-LowerSHA256 $path
    }
    $baselineManifestBytes = [System.IO.File]::ReadAllBytes($baselineManifestPath)
    Write-Utf8NoBom (Join-Path $root ".runtime\sentinel.txt") "runtime-sentinel-$Name`n"
    Write-Utf8NoBom (Join-Path $root ".env.full.local") "secret-sentinel-$Name`n"
    $candidate = New-CandidateArchive $scenarioDir $root $UpdaterExe $Name
    $runner = Join-Path $root ".updates\runner\archive-center-updater-e2e.exe"
    Copy-Bytes $UpdaterExe $runner
    $pending = [ordered]@{
        contract_version = "archive-center.pending-update.v1"
        current_version = "e2e-baseline-$Name"
        target_version = "e2e-candidate-$Name"
        asset_path = ".updates/candidate.zip"
        sha256 = $candidate.SHA256
        required_files = @($candidate.Managed)
    }
    Write-Utf8NoBom (Join-Path $root ".updates\pending-update.json") (($pending | ConvertTo-Json -Depth 8) + "`n")
    [pscustomobject]@{
        Name = $Name
        Root = $root
        Runner = $runner
        Candidate = $candidate
        BaselineHashes = $baseline
        BaselineManifestBytes = $baselineManifestBytes
        RuntimeSentinel = "runtime-sentinel-$Name`n"
        EnvSentinel = "secret-sentinel-$Name`n"
    }
}

function Quote-ProcessArgument([string]$Value) {
    if ($Value -notmatch '[\s"]') {
        return $Value
    }
    return '"' + ($Value -replace '(\\*)"', '$1$1\"' -replace '(\\+)$', '$1$1') + '"'
}

function Invoke-Updater([string]$Runner, [string]$Command, [string]$Root, [int]$ExpectedExit, [string]$ExpectedStatus, [string]$ExpectedCode = "") {
    Assert-True (Test-Path -LiteralPath $Runner -PathType Leaf) "Updater runner is missing: $Runner"
    $arguments = (Quote-ProcessArgument $Command) + " --root " + (Quote-ProcessArgument $Root)
    $startInfo = New-Object System.Diagnostics.ProcessStartInfo
    $startInfo.FileName = $Runner
    $startInfo.Arguments = $arguments
    $startInfo.WorkingDirectory = $Root
    $startInfo.UseShellExecute = $false
    $startInfo.CreateNoWindow = $true
    $startInfo.RedirectStandardOutput = $true
    $startInfo.RedirectStandardError = $true
    $process = New-Object System.Diagnostics.Process
    $process.StartInfo = $startInfo
    Assert-True $process.Start() "$Command failed to start"
    $stdoutTask = $process.StandardOutput.ReadToEndAsync()
    $stderrTask = $process.StandardError.ReadToEndAsync()
    $process.WaitForExit()
    $stdoutRaw = $stdoutTask.GetAwaiter().GetResult()
    $stderrRaw = $stderrTask.GetAwaiter().GetResult()
    $stdout = if ($null -eq $stdoutRaw) { "" } else { ([string]$stdoutRaw).Trim() }
    $stderr = if ($null -eq $stderrRaw) { "" } else { ([string]$stderrRaw).Trim() }
    Assert-Equal $process.ExitCode $ExpectedExit "$Command exit code"
    if ($ExpectedExit -eq 0) {
        Assert-True ([string]::IsNullOrWhiteSpace($stderr)) "$Command emitted unexpected stderr: $stderr"
        Assert-True (-not [string]::IsNullOrWhiteSpace($stdout)) "$Command emitted no success JSON"
        $payload = $stdout | ConvertFrom-Json
        Assert-Equal $payload.contract_version "archive-center.updater-result.v1" "$Command result contract"
        Assert-Equal $payload.action $Command "$Command result action"
        Assert-Equal $payload.status $ExpectedStatus "$Command result status"
    } else {
        Assert-True ([string]::IsNullOrWhiteSpace($stdout)) "$Command emitted unexpected stdout: $stdout"
        Assert-True (-not [string]::IsNullOrWhiteSpace($stderr)) "$Command emitted no error JSON"
        $payload = $stderr | ConvertFrom-Json
        Assert-Equal $payload.contract_version "archive-center.updater-result.v1" "$Command error contract"
        Assert-Equal $payload.action $Command "$Command error action"
        Assert-Equal $payload.status "error" "$Command error status"
        Assert-Equal $payload.code $ExpectedCode "$Command stable error code"
    }
    return $payload
}

function Assert-Sentinels([object]$Scenario) {
    Assert-Equal ([System.IO.File]::ReadAllText((Join-Path $Scenario.Root ".runtime\sentinel.txt"))) $Scenario.RuntimeSentinel "$($Scenario.Name) runtime sentinel changed"
    Assert-Equal ([System.IO.File]::ReadAllText((Join-Path $Scenario.Root ".env.full.local"))) $Scenario.EnvSentinel "$($Scenario.Name) environment sentinel changed"
}

function Assert-BytesEqual([byte[]]$Actual, [byte[]]$Expected, [string]$Message) {
    Assert-Equal $Actual.Length $Expected.Length "$Message length"
    for ($i = 0; $i -lt $Actual.Length; $i++) {
        if ($Actual[$i] -ne $Expected[$i]) {
            throw "$Message differs at byte $i"
        }
    }
}

function Assert-BackupCleaned([string]$Root, [string]$BackupDir) {
    Assert-True ([string]::IsNullOrWhiteSpace($BackupDir)) "Final state retained backup_dir: $BackupDir"
    $backups = Join-Path $Root ".updates\backups"
    if (Test-Path -LiteralPath $backups -PathType Container) {
        $backupChildren = @(Get-ChildItem -LiteralPath $backups -Force)
        Assert-Equal $backupChildren.Count 0 "Backup directory is not empty"
    }
}

if ([string]::IsNullOrWhiteSpace($PackageRoot)) {
    $PackageRoot = Join-Path $PSScriptRoot ".."
}
$packageRootResolved = (Resolve-Path -LiteralPath $PackageRoot).Path
$updaterExe = Join-Path $packageRootResolved "bin\archive-center-updater.exe"
Assert-True (Test-Path -LiteralPath $updaterExe -PathType Leaf) "Packaged updater not found: $updaterExe"

$tempBase = [System.IO.Path]::GetFullPath([System.IO.Path]::GetTempPath()).TrimEnd('\')
$runRoot = Join-Path $tempBase ("archive-center-updater-e2e-" + [guid]::NewGuid().ToString("N"))
$runRootResolved = [System.IO.Path]::GetFullPath($runRoot)
$expectedPrefix = $tempBase + '\archive-center-updater-e2e-'
Assert-True $runRootResolved.StartsWith($expectedPrefix, [System.StringComparison]::OrdinalIgnoreCase) "Unsafe E2E root: $runRootResolved"
New-Item -ItemType Directory -Path $runRootResolved | Out-Null

$report = $null
try {
    $success = New-Scenario $runRootResolved $updaterExe "success"
    $successApply = Invoke-Updater $success.Runner "apply-pending" $success.Root 0 "applied_pending_health"
    Assert-True ([bool]$successApply.health_required) "Success apply did not require health"
    Assert-Equal ([System.IO.File]::ReadAllText((Join-Path $success.Root "Archive Center.js"))) $success.Candidate.ExpectedJS "Success JS was not updated"
    Assert-Equal ([System.IO.File]::ReadAllText((Join-Path $success.Root "bin\archive-center-go.exe"))) $success.Candidate.ExpectedBackend "Success backend was not updated"
    Assert-Equal ([System.IO.File]::ReadAllText((Join-Path $success.Root "scripts\new-tool.ps1"))) $success.Candidate.ExpectedScript "Success new managed file was not installed"
    Assert-BytesEqual ([System.IO.File]::ReadAllBytes((Join-Path $success.Root "PACKAGE_FILE_MANIFEST.json"))) $success.Candidate.ManifestBytes "Success managed manifest was not installed"
    Assert-Sentinels $success
    $null = Invoke-Updater $success.Runner "status" $success.Root 0 "applied_pending_health"
    $null = Invoke-Updater $success.Runner "commit" $success.Root 0 "committed"
    $null = Invoke-Updater $success.Runner "status" $success.Root 0 "committed"
    Assert-True (-not (Test-Path -LiteralPath (Join-Path $success.Root ".updates\pending-update.json"))) "Commit retained pending marker"
    $successState = Get-Content -LiteralPath (Join-Path $success.Root ".updates\update-state.json") -Raw -Encoding UTF8 | ConvertFrom-Json
    Assert-Equal (Get-OptionalArrayCount $successState "journal") 0 "Commit retained journal"
    Assert-BackupCleaned $success.Root ([string](Get-OptionalProperty $successState "backup_dir"))
    Assert-Sentinels $success

    $rollback = New-Scenario $runRootResolved $updaterExe "rollback"
    $null = Invoke-Updater $rollback.Runner "apply-pending" $rollback.Root 0 "applied_pending_health"
    $null = Invoke-Updater $rollback.Runner "rollback" $rollback.Root 0 "rolled_back"
    $null = Invoke-Updater $rollback.Runner "status" $rollback.Root 0 "rolled_back"
    foreach ($relative in $rollback.BaselineHashes.Keys) {
        Assert-Equal (Get-LowerSHA256 (Join-Path $rollback.Root ($relative.Replace('/', '\')))) $rollback.BaselineHashes[$relative] "Rollback failed to restore $relative"
    }
    Assert-BytesEqual ([System.IO.File]::ReadAllBytes((Join-Path $rollback.Root "PACKAGE_FILE_MANIFEST.json"))) $rollback.BaselineManifestBytes "Rollback failed to restore manifest"
    Assert-True (-not (Test-Path -LiteralPath (Join-Path $rollback.Root "scripts\new-tool.ps1"))) "Rollback retained newly-created managed file"
    Assert-True (-not (Test-Path -LiteralPath (Join-Path $rollback.Root ".updates\pending-update.json"))) "Rollback retained pending marker"
    $rollbackState = Get-Content -LiteralPath (Join-Path $rollback.Root ".updates\update-state.json") -Raw -Encoding UTF8 | ConvertFrom-Json
    Assert-Equal (Get-OptionalArrayCount $rollbackState "journal") 0 "Rollback retained journal"
    Assert-BackupCleaned $rollback.Root ([string](Get-OptionalProperty $rollbackState "backup_dir"))
    Assert-Sentinels $rollback

    $tampered = New-Scenario $runRootResolved $updaterExe "tampered"
    $tamperedPendingPath = Join-Path $tampered.Root ".updates\pending-update.json"
    $tamperedPending = Get-Content -LiteralPath $tamperedPendingPath -Raw -Encoding UTF8 | ConvertFrom-Json
    $tamperedPending.sha256 = ("0" * 64)
    Write-Utf8NoBom $tamperedPendingPath (($tamperedPending | ConvertTo-Json -Depth 8) + "`n")
    $null = Invoke-Updater $tampered.Runner "apply-pending" $tampered.Root 1 "error" "asset_verification_failed"
    foreach ($relative in $tampered.BaselineHashes.Keys) {
        Assert-Equal (Get-LowerSHA256 (Join-Path $tampered.Root ($relative.Replace('/', '\')))) $tampered.BaselineHashes[$relative] "Tampered archive changed $relative"
    }
    Assert-BytesEqual ([System.IO.File]::ReadAllBytes((Join-Path $tampered.Root "PACKAGE_FILE_MANIFEST.json"))) $tampered.BaselineManifestBytes "Tampered archive changed manifest"
    Assert-True (-not (Test-Path -LiteralPath (Join-Path $tampered.Root ".updates\update-state.json"))) "Tampered archive wrote update state"
    Assert-Sentinels $tampered

    $report = [ordered]@{
        schema_version = "archive-center.updater-e2e-smoke.v1"
        status = "ok"
        scope = "production updater CLI transactions"
        package_root = $packageRootResolved
        production_updater_sha256 = Get-LowerSHA256 $updaterExe
        scenarios = [ordered]@{
            apply_status_commit_cli = "passed"
            apply_explicit_rollback_cli = "passed"
            tampered_asset_rejected_before_mutation = "passed"
        }
        launcher_integration = [ordered]@{
            status = "not_tested"
            includes = @(
                "start-full-windows.ps1",
                "backend /ready and /version",
                "automatic health-failure rollback"
            )
        }
        staged_asset_present_before_temp_cleanup = [bool](Test-Path -LiteralPath $success.Candidate.Asset -PathType Leaf)
    }
    $report | ConvertTo-Json -Depth 8
} finally {
    $cleanupPath = [System.IO.Path]::GetFullPath($runRootResolved)
    $cleanupParent = [System.IO.Path]::GetDirectoryName($cleanupPath)
    $cleanupLeaf = [System.IO.Path]::GetFileName($cleanupPath)
    if (-not $cleanupParent.Equals($tempBase, [System.StringComparison]::OrdinalIgnoreCase) -or -not $cleanupLeaf.StartsWith("archive-center-updater-e2e-", [System.StringComparison]::Ordinal)) {
        throw "Refusing unsafe E2E cleanup: $cleanupPath"
    }
    if (Test-Path -LiteralPath $cleanupPath) {
        Remove-Item -LiteralPath $cleanupPath -Recurse -Force
    }
}
