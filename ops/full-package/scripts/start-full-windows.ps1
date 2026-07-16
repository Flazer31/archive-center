param(
    [string]$EnvFile = ".\.env.full.local",
    [string]$BindAddr = "",
    [string]$RuntimeProfile = "",
    [string]$VectorMode = "",
    [int]$MariaDBPort = 3307,
    [switch]$KeepServices
)

$ErrorActionPreference = "Stop"
$packagedBuildVersion = "__ARCHIVE_CENTER_PACKAGE_VERSION__"

function ConvertFrom-ProtectedEnvText([string]$ProtectedPath) {
    $cipherText = (Get-Content -LiteralPath $ProtectedPath -Raw).Trim()
    $secureText = ConvertTo-SecureString $cipherText
    $bstr = [Runtime.InteropServices.Marshal]::SecureStringToBSTR($secureText)
    try {
        [Runtime.InteropServices.Marshal]::PtrToStringBSTR($bstr)
    } finally {
        if ($bstr -ne [IntPtr]::Zero) {
            [Runtime.InteropServices.Marshal]::ZeroFreeBSTR($bstr)
        }
    }
}

function Read-DotEnvContent([string]$Path) {
    $protectedPath = "$Path.protected"
    if (Test-Path -LiteralPath $protectedPath -PathType Leaf) {
        return ConvertFrom-ProtectedEnvText $protectedPath
    } elseif (Test-Path -LiteralPath $Path -PathType Leaf) {
        return Get-Content -LiteralPath $Path -Raw
    }
    throw "Env file not found: $Path or $protectedPath. Copy .env.full.example to .env.full.local first."
}

function Get-DotEnvValue([string]$Path, [string]$Name) {
    $content = Read-DotEnvContent $Path
    foreach ($rawLine in ($content -split "\r?\n")) {
        $line = $rawLine.Trim()
        if ($line -eq "" -or $line.StartsWith("#")) { continue }
        $idx = $line.IndexOf("=")
        if ($idx -lt 1) { continue }
        if ($line.Substring(0, $idx).Trim() -eq $Name) {
            return $line.Substring($idx + 1).Trim()
        }
    }
    return ""
}

function Import-DotEnv([string]$Path) {
    $protectedPath = "$Path.protected"
    if (Test-Path -LiteralPath $protectedPath -PathType Leaf) {
        Write-Host "Using protected env: $protectedPath"
    }
    $content = Read-DotEnvContent $Path
    $content -split "\r?\n" | ForEach-Object {
        $line = $_.Trim()
        if ($line -eq "" -or $line.StartsWith("#")) { return }
        $idx = $line.IndexOf("=")
        if ($idx -lt 1) { return }
        [Environment]::SetEnvironmentVariable($line.Substring(0, $idx).Trim(), $line.Substring($idx + 1).Trim(), "Process")
    }
}

function Test-PortOpen([int]$Port) {
    $client = New-Object Net.Sockets.TcpClient
    try {
        $iar = $client.BeginConnect("127.0.0.1", $Port, $null, $null)
        $ok = $iar.AsyncWaitHandle.WaitOne(500, $false)
        if ($ok) { $client.EndConnect($iar) }
        return $ok
    } catch {
        return $false
    } finally {
        $client.Close()
    }
}

function Wait-Port([int]$Port, [int]$TimeoutSeconds = 60) {
    for ($i = 0; $i -lt $TimeoutSeconds; $i++) {
        if (Test-PortOpen $Port) { return }
        Start-Sleep -Seconds 1
    }
    throw "Port did not become ready on 127.0.0.1:$Port"
}

function Join-Args([string[]]$ArgList) {
    ($ArgList | ForEach-Object {
        if ($_ -match '[\s"]') { '"' + ($_ -replace '"', '\"') + '"' } else { $_ }
    }) -join " "
}

function Unblock-PackageFile([string]$Path) {
    if ([string]::IsNullOrWhiteSpace($Path) -or -not (Test-Path -LiteralPath $Path -PathType Leaf)) {
        return
    }
    try {
        Unblock-File -LiteralPath $Path -ErrorAction SilentlyContinue
    } catch {
        # Best effort only. The process start path below still reports a clear error.
    }
}

function Start-ArchiveChildProcess {
    param(
        [Parameter(Mandatory = $true)][string]$FilePath,
        [string[]]$ArgumentList = @(),
        [string]$WorkingDirectory = ""
    )

    if ([string]::IsNullOrWhiteSpace($FilePath) -or -not (Test-Path -LiteralPath $FilePath -PathType Leaf)) {
        throw "Executable not found: $FilePath"
    }

    Unblock-PackageFile $FilePath

    $psi = New-Object System.Diagnostics.ProcessStartInfo
    $psi.FileName = $FilePath
    $psi.Arguments = Join-Args $ArgumentList
    if ([string]::IsNullOrWhiteSpace($WorkingDirectory)) {
        $psi.WorkingDirectory = Split-Path -Parent $FilePath
    } else {
        $psi.WorkingDirectory = $WorkingDirectory
    }
    $psi.UseShellExecute = $false
    $psi.CreateNoWindow = $true

    try {
        $proc = [System.Diagnostics.Process]::Start($psi)
        if ($null -eq $proc) {
            throw "Process.Start returned null."
        }
        return $proc
    } catch {
        $nativeCode = $null
        if ($_.Exception -is [System.ComponentModel.Win32Exception]) {
            $nativeCode = $_.Exception.NativeErrorCode
        } elseif ($_.Exception.InnerException -is [System.ComponentModel.Win32Exception]) {
            $nativeCode = $_.Exception.InnerException.NativeErrorCode
        }
        $message = $_.Exception.Message
        $hint = "Failed to start bundled runtime executable."
        if ($nativeCode -eq 1223 -or $message -match "(?i)cancel|cancell|operation.*canceled|user.*cancel") {
            $hint = "Windows cancelled the bundled runtime executable launch. This is commonly caused by Mark-of-the-Web, SmartScreen, Defender quarantine, or a cancelled security prompt. From the package root, run: Get-ChildItem -Recurse -File | Unblock-File"
        }
        throw "$hint`nFile: $FilePath`nOriginal error: $message"
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

function Start-BundledChromaDB {
    param(
        [Parameter(Mandatory = $true)][string]$PackageRoot,
        [Parameter(Mandatory = $true)][Uri]$Endpoint
    )

    $hostName = if ([string]::IsNullOrWhiteSpace($Endpoint.Host)) { "127.0.0.1" } else { $Endpoint.Host }
    $port = if ($Endpoint.Port -gt 0) { $Endpoint.Port } else { 8000 }
    $dataDir = Join-Path $PackageRoot ".runtime\chromadb"
    New-Item -ItemType Directory -Force -Path $dataDir | Out-Null

    $python = Find-BundledPython $PackageRoot
    if ([string]::IsNullOrWhiteSpace($python)) {
        throw "Bundled ChromaDB Python runtime not found under runtime/. Package is incomplete."
    }
    Unblock-PackageFile $python

    & $python -c "import importlib.util, sys; sys.exit(0 if importlib.util.find_spec('chromadb') else 1)"
    if ($LASTEXITCODE -ne 0) {
        throw "Bundled Python exists but does not include chromadb."
    }

    Write-Host "Starting bundled ChromaDB"
    Write-Host "  Endpoint: http://$hostName`:$port"
    Write-Host "  Data:     $dataDir"

    $chromaCode = "import sys; from chromadb.cli.cli import app; sys.argv = ['chroma', 'run', '--host', sys.argv[1], '--port', sys.argv[2], '--path', sys.argv[3]]; app()"
    Start-ArchiveChildProcess -FilePath $python -ArgumentList @("-c", $chromaCode, $hostName, ([string]$port), $dataDir) -WorkingDirectory $PackageRoot
}

function Find-MariaDBTool([string]$Root, [string[]]$Names) {
    foreach ($name in $Names) {
        $hit = Get-ChildItem -LiteralPath $Root -Recurse -File -ErrorAction SilentlyContinue |
            Where-Object { $_.Name -ieq $name } |
            Sort-Object FullName |
            Select-Object -First 1
        if ($null -ne $hit) {
            return $hit.FullName
        }
    }
    return ""
}

function Normalize-ProcessPathForStartProcess {
    $envs = [System.Environment]::GetEnvironmentVariables("Process")
    $pathValue = ""
    foreach ($key in @("Path", "PATH")) {
        if ($envs.Contains($key) -and -not [string]::IsNullOrWhiteSpace([string]$envs[$key])) {
            $pathValue = [string]$envs[$key]
            break
        }
    }
    if ([string]::IsNullOrWhiteSpace($pathValue)) {
        $machinePath = [System.Environment]::GetEnvironmentVariable("Path", "Machine")
        $userPath = [System.Environment]::GetEnvironmentVariable("Path", "User")
        $pathValue = @($machinePath, $userPath) -join ";"
    }
    [System.Environment]::SetEnvironmentVariable("PATH", $null, "Process")
    [System.Environment]::SetEnvironmentVariable("Path", $null, "Process")
    [System.Environment]::SetEnvironmentVariable("Path", $pathValue, "Process")
}

function Test-AllowedRuntimeProfile([string]$Value) {
    @("client_only", "core_lite", "vector_external", "vector_local_native", "full_local") -contains $Value
}

function Test-AllowedVectorMode([string]$Value) {
    @("off", "fallback", "external", "local_native", "local_proot", "bundled") -contains $Value
}

function Test-VectorRequiresChroma([string]$Value) {
    @("external", "local_native", "local_proot", "bundled") -contains $Value
}

function Test-LocalChromaRequested([string]$Value) {
    @("local_native", "local_proot", "bundled") -contains $Value
}

function Invoke-ArchiveUpdater {
    param(
        [Parameter(Mandatory = $true)][string]$RunnerPath,
        [Parameter(Mandatory = $true)][string]$Command,
        [Parameter(Mandatory = $true)][string]$PackageRoot
    )

    $output = @(& $RunnerPath $Command --root $PackageRoot 2>&1)
    $exitCode = $LASTEXITCODE
    $text = ($output | ForEach-Object { [string]$_ }) -join [Environment]::NewLine
    try {
        $result = $text | ConvertFrom-Json
    } catch {
        throw "Archive Center updater returned an invalid response for '$Command' (exit $exitCode). Recovery is required before startup to avoid a mixed package.`n$text"
    }
    if ($null -eq $result -or [string]::IsNullOrWhiteSpace([string]$result.status)) {
        throw "Archive Center updater response for '$Command' had no status. Recovery is required before startup to avoid a mixed package."
    }
    return [pscustomobject]@{
        ExitCode = $exitCode
        Status = ([string]$result.status).Trim().ToLowerInvariant()
        Result = $result
    }
}

function Test-UpdaterSafeBaselineStatus([string]$Status) {
    $Status -in @("no_pending", "rolled_back", "nothing_to_rollback")
}

function Wait-BackendMainReady {
    param(
        [Parameter(Mandatory = $true)][System.Diagnostics.Process]$Process,
        [Parameter(Mandatory = $true)][int]$Port,
        [Parameter(Mandatory = $true)][string]$ExpectedVersion,
        [int]$TimeoutSeconds = 60
    )

    $lastError = ""
    $readyStreak = 0
    for ($i = 0; $i -lt $TimeoutSeconds; $i++) {
        if ($Process.HasExited) {
            return [pscustomobject]@{ Ready = $false; Detail = "backend exited with code $($Process.ExitCode)" }
        }
        try {
            $ready = Invoke-RestMethod -Method GET -Uri "http://127.0.0.1:$Port/ready" -TimeoutSec 2
            # Reference-vector degradation is intentionally not a failure here.
            if ($ready.ready -eq $true) {
                $version = Invoke-RestMethod -Method GET -Uri "http://127.0.0.1:$Port/version" -TimeoutSec 2
                if ([string]$version.version -eq $ExpectedVersion) {
                    $Process.Refresh()
                    if (-not $Process.HasExited) {
                        $readyStreak++
                        if ($readyStreak -ge 2) {
                            return [pscustomobject]@{ Ready = $true; Detail = "main ready at expected version $ExpectedVersion" }
                        }
                    }
                } else {
                    $readyStreak = 0
                    $lastError = "version mismatch: expected $ExpectedVersion, got $($version.version)"
                }
            } else {
                $readyStreak = 0
                $lastError = "main ready=false"
            }
        } catch {
            $readyStreak = 0
            $lastError = $_.Exception.Message
        }
        Start-Sleep -Seconds 1
    }
    return [pscustomobject]@{ Ready = $false; Detail = "ready timeout: $lastError" }
}

function Stop-ArchiveChildProcess([System.Diagnostics.Process]$Process) {
    if ($null -ne $Process -and -not $Process.HasExited) {
        $Process.Kill()
        $Process.WaitForExit()
    }
}

$packRoot = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
Set-Location $packRoot

$backendExe = Join-Path $packRoot "bin\archive-center-go.exe"
$profileBeforeApply = if (-not [string]::IsNullOrWhiteSpace($RuntimeProfile)) {
    $RuntimeProfile
} else {
    Get-DotEnvValue $EnvFile "AC_RUNTIME_PROFILE"
}
if ([string]::IsNullOrWhiteSpace($profileBeforeApply)) {
    $profileBeforeApply = "core_lite"
}
$profileBeforeApply = $profileBeforeApply.Trim().ToLowerInvariant()

$updaterExe = Join-Path $packRoot "bin\archive-center-updater.exe"
$updaterRunnerDir = Join-Path $packRoot ".updates\runner"
$pendingMarker = Join-Path $packRoot ".updates\pending-update.json"
$stateMarker = Join-Path $packRoot ".updates\update-state.json"
$statePreviousMarker = "$stateMarker.previous"
$pendingMarkerPresent = Test-Path -LiteralPath $pendingMarker -PathType Leaf
$stateMarkerPresent = (Test-Path -LiteralPath $stateMarker -PathType Leaf) -or (Test-Path -LiteralPath $statePreviousMarker -PathType Leaf)
$stateStatusSource = if (Test-Path -LiteralPath $stateMarker -PathType Leaf) { $stateMarker } else { $statePreviousMarker }
$updateStatePresent = $pendingMarkerPresent -or $stateMarkerPresent
$updaterRunner = ""
$pendingApplyStatus = "no_pending"
$pendingTargetVersion = ""
$pendingCurrentVersion = ""
$observedStateStatus = ""
if ($stateMarkerPresent) {
    try {
        $observedState = Get-Content -LiteralPath $stateStatusSource -Raw -Encoding UTF8 | ConvertFrom-Json
        $observedStateStatus = ([string]$observedState.status).Trim().ToLowerInvariant()
    } catch {
        $observedStateStatus = "invalid"
    }
}

if ($profileBeforeApply -eq "client_only") {
    $clientStateStatus = "no_state"
    if ($stateMarkerPresent) {
        try {
            $clientState = Get-Content -LiteralPath $stateStatusSource -Raw -Encoding UTF8 | ConvertFrom-Json
            $clientStateStatus = ([string]$clientState.status).Trim().ToLowerInvariant()
        } catch {
            throw "client_only profile found unreadable update state. Startup stopped because local backend health commit is unavailable."
        }
    }
    if ($clientStateStatus -in @("applying", "applied_pending_health")) {
        $clientPreservedRunner = Get-ChildItem -LiteralPath $updaterRunnerDir -Filter "archive-center-updater-*.exe" -File -ErrorAction SilentlyContinue |
            Sort-Object LastWriteTimeUtc -Descending |
            Select-Object -First 1
        if ($null -ne $clientPreservedRunner) {
            $updaterRunner = $clientPreservedRunner.FullName
        } elseif (Test-Path -LiteralPath $updaterExe -PathType Leaf) {
            New-Item -ItemType Directory -Force -Path $updaterRunnerDir | Out-Null
            $updaterRunner = Join-Path $updaterRunnerDir ("archive-center-updater-{0}.exe" -f $PID)
            Copy-Item -LiteralPath $updaterExe -Destination $updaterRunner -Force
        } else {
            $recoveryRunner = Get-ChildItem -LiteralPath $updaterRunnerDir -Filter "archive-center-updater-*.exe" -File -ErrorAction SilentlyContinue |
                Sort-Object LastWriteTimeUtc -Descending |
                Select-Object -First 1
            if ($null -eq $recoveryRunner) {
                throw "client_only profile found active update state '$clientStateStatus', but no updater recovery runner is available."
            }
            $updaterRunner = $recoveryRunner.FullName
        }
        Unblock-PackageFile $updaterRunner
        $clientRollback = Invoke-ArchiveUpdater -RunnerPath $updaterRunner -Command "rollback" -PackageRoot $packRoot
        if ($clientRollback.ExitCode -ne 0 -or $clientRollback.Status -notin @("rolled_back", "nothing_to_rollback")) {
            throw "client_only profile could not safely roll back active update state '$clientStateStatus'. Startup stopped to avoid a mixed package."
        }
        Write-Host "client_only profile restored the baseline package instead of entering an unsupported backend health gate."
    } elseif ($stateMarkerPresent -and $clientStateStatus -notin @("committed", "rolled_back", "no_state")) {
        throw "client_only profile found unsupported update state '$clientStateStatus'. Startup stopped because local backend health commit is unavailable."
    } elseif ($updateStatePresent) {
        Write-Host "client_only profile: pending package apply is deferred because no local backend health gate is available."
    }
} else {
    $preservedRunner = Get-ChildItem -LiteralPath $updaterRunnerDir -Filter "archive-center-updater-*.exe" -File -ErrorAction SilentlyContinue |
        Sort-Object LastWriteTimeUtc -Descending |
        Select-Object -First 1
    if ($observedStateStatus -in @("applying", "applied_pending_health") -and $null -ne $preservedRunner) {
        $updaterRunner = $preservedRunner.FullName
        Write-Host "Using preserved updater recovery runner: $updaterRunner"
    } elseif (Test-Path -LiteralPath $updaterExe -PathType Leaf) {
        New-Item -ItemType Directory -Force -Path $updaterRunnerDir | Out-Null
        $updaterRunner = Join-Path $updaterRunnerDir ("archive-center-updater-{0}.exe" -f $PID)
        Copy-Item -LiteralPath $updaterExe -Destination $updaterRunner -Force
    } elseif ($updateStatePresent) {
        $recoveryRunner = Get-ChildItem -LiteralPath $updaterRunnerDir -Filter "archive-center-updater-*.exe" -File -ErrorAction SilentlyContinue |
            Sort-Object LastWriteTimeUtc -Descending |
            Select-Object -First 1
        if ($null -eq $recoveryRunner) {
            throw "Archive Center updater is missing while pending update state exists. No recovery runner is available; startup stopped to avoid a mixed package."
        }
        $updaterRunner = $recoveryRunner.FullName
        Write-Host "Using preserved updater recovery runner: $updaterRunner"
    } else {
        Write-Host "Warning: Archive Center updater is not installed. No pending state exists, so normal startup will continue."
    }

    if (-not [string]::IsNullOrWhiteSpace($updaterRunner)) {
        Unblock-PackageFile $updaterRunner
        $applyFailure = ""
        try {
            $pendingApply = Invoke-ArchiveUpdater -RunnerPath $updaterRunner -Command "apply-pending" -PackageRoot $packRoot
            $pendingApplyStatus = $pendingApply.Status
            $pendingTargetVersion = ([string]$pendingApply.Result.target_version).Trim()
            $pendingCurrentVersion = ([string]$pendingApply.Result.current_version).Trim()
            if ($pendingApply.ExitCode -ne 0 -or ($pendingApplyStatus -ne "applied_pending_health" -and -not (Test-UpdaterSafeBaselineStatus $pendingApplyStatus))) {
                $applyFailure = "status '$pendingApplyStatus' (exit $($pendingApply.ExitCode))"
            }
        } catch {
            $applyFailure = $_.Exception.Message
        }

        if (-not [string]::IsNullOrWhiteSpace($applyFailure)) {
            try {
                $safety = Invoke-ArchiveUpdater -RunnerPath $updaterRunner -Command "status" -PackageRoot $packRoot
            } catch {
                throw "Updater apply-pending failed ($applyFailure), and update state could not be verified. Startup stopped to avoid a mixed package.`n$($_.Exception.Message)"
            }
            if ($safety.ExitCode -eq 0 -and $safety.Status -in @("no_state", "rolled_back", "nothing_to_rollback")) {
                $pendingApplyStatus = "no_pending"
                Write-Host "Warning: pending update was rejected before a live package mutation ($applyFailure). State is '$($safety.Status)'; continuing with the existing package."
            } else {
                throw "Updater apply-pending failed ($applyFailure), and state is '$($safety.Status)'. Startup stopped to avoid a mixed package."
            }
        } elseif ($pendingApplyStatus -eq "applied_pending_health") {
            Write-Host "Applied a verified pending package. Main readiness will be checked before commit."
        } elseif (Test-UpdaterSafeBaselineStatus $pendingApplyStatus) {
            Write-Host "Updater reported '$pendingApplyStatus'; continuing with the verified baseline package."
        }
    }
}

Import-DotEnv $EnvFile
if ($pendingApplyStatus -eq "applied_pending_health" -and -not [string]::IsNullOrWhiteSpace($pendingTargetVersion)) {
    $env:AC_BUILD_VERSION = $pendingTargetVersion
} elseif ($packagedBuildVersion -notmatch '^__ARCHIVE_CENTER_' -and -not [string]::IsNullOrWhiteSpace($packagedBuildVersion)) {
    # Build identity belongs to the package, not to a preserved user env file.
    $env:AC_BUILD_VERSION = $packagedBuildVersion
}
if (-not [string]::IsNullOrWhiteSpace($BindAddr)) {
    $env:AC_BIND_ADDR = $BindAddr
}
$profileCandidate = if (-not [string]::IsNullOrWhiteSpace($RuntimeProfile)) { $RuntimeProfile } elseif (-not [string]::IsNullOrWhiteSpace($env:AC_RUNTIME_PROFILE)) { $env:AC_RUNTIME_PROFILE } else { "core_lite" }
$profileCandidate = $profileCandidate.Trim().ToLowerInvariant()
if (-not (Test-AllowedRuntimeProfile $profileCandidate)) {
    throw "Unsupported runtime profile: $profileCandidate"
}
$env:AC_RUNTIME_PROFILE = $profileCandidate

$vectorCandidate = if (-not [string]::IsNullOrWhiteSpace($VectorMode)) { $VectorMode } elseif (-not [string]::IsNullOrWhiteSpace($env:AC_VECTOR_MODE)) { $env:AC_VECTOR_MODE } else { "" }
if ([string]::IsNullOrWhiteSpace($vectorCandidate)) {
    switch ($env:AC_RUNTIME_PROFILE) {
        "client_only" { $vectorCandidate = "off" }
        "vector_external" { $vectorCandidate = "external" }
        "vector_local_native" { $vectorCandidate = "bundled" }
        "full_local" { $vectorCandidate = "bundled" }
        default { $vectorCandidate = "fallback" }
    }
}
$vectorCandidate = $vectorCandidate.Trim().ToLowerInvariant()
if (-not (Test-AllowedVectorMode $vectorCandidate)) {
    throw "Unsupported vector mode: $vectorCandidate"
}
if ($env:AC_RUNTIME_PROFILE -eq "client_only" -and $vectorCandidate -ne "off") {
    throw "client_only requires AC_VECTOR_MODE=off"
}
if ($env:AC_RUNTIME_PROFILE -eq "core_lite" -and $vectorCandidate -notin @("fallback", "off")) {
    throw "core_lite supports only fallback or off vector modes"
}
if ($env:AC_RUNTIME_PROFILE -eq "vector_external" -and $vectorCandidate -ne "external") {
    throw "vector_external requires AC_VECTOR_MODE=external"
}
$env:AC_VECTOR_MODE = $vectorCandidate

if ($env:AC_RUNTIME_PROFILE -eq "client_only") {
    Write-Host "Archive Center client_only profile selected."
    Write-Host "No local backend, MariaDB, or ChromaDB service will be started on this device."
    Write-Host "Configure the RisuAI plugin Bridge URL to the PC/NAS Archive Center backend."
    Remove-Item -LiteralPath $updaterRunner -Force -ErrorAction SilentlyContinue
    exit 0
}
Normalize-ProcessPathForStartProcess

$runtimeRoot = Join-Path $packRoot "runtime"
$mariadbd = Find-MariaDBTool $runtimeRoot @("mariadbd.exe", "mysqld.exe")
$installDb = Find-MariaDBTool $runtimeRoot @("mariadb-install-db.exe", "mysql_install_db.exe")
$client = Find-MariaDBTool $runtimeRoot @("mariadb.exe", "mysql.exe")
$admin = Find-MariaDBTool $runtimeRoot @("mariadb-admin.exe", "mysqladmin.exe")
foreach ($tool in @($mariadbd, $installDb, $client, $admin)) {
    if ([string]::IsNullOrWhiteSpace($tool) -or -not (Test-Path -LiteralPath $tool -PathType Leaf)) {
        throw "Bundled MariaDB runtime is incomplete."
    }
    Unblock-PackageFile $tool
}
Unblock-PackageFile $backendExe
Unblock-PackageFile (Join-Path $packRoot "bin\mariadb-schema.exe")

$dataDir = Join-Path $packRoot ".runtime\mariadb"
$logDir = Join-Path $packRoot ".runtime\logs"
New-Item -ItemType Directory -Force -Path $dataDir, $logDir | Out-Null

if (-not (Test-Path -LiteralPath (Join-Path $dataDir "mysql") -PathType Container)) {
    & $installDb "--datadir=$dataDir" "--password="
    if ($LASTEXITCODE -ne 0) {
        throw "MariaDB data directory initialization failed."
    }
}

$startedMariaDB = $null
$startedChroma = $null
try {
    if (-not (Test-PortOpen $MariaDBPort)) {
        $mariaArgs = @(
            "--no-defaults",
            "--datadir=$dataDir",
            "--port=$MariaDBPort",
            "--socket=$(Join-Path $dataDir "mysql.sock")",
            "--skip-networking=0",
            "--bind-address=127.0.0.1",
            "--pid-file=$(Join-Path $dataDir "mysqld.pid")",
            "--console"
        )
        $startedMariaDB = Start-ArchiveChildProcess -FilePath $mariadbd -ArgumentList $mariaArgs -WorkingDirectory $dataDir
        Start-Sleep -Seconds 2
        if ($startedMariaDB.HasExited) {
            throw "MariaDB exited early with code $($startedMariaDB.ExitCode). Check .runtime\mariadb and .runtime\logs for details."
        }
    }
    Wait-Port $MariaDBPort 60

    $dbName = "archive_center"
    $dbUser = "archive_center"
    $dbPassword = "archive-center-local-pass"
    if ([string]::IsNullOrWhiteSpace($env:AC_MARIADB_DSN)) {
        $env:AC_MARIADB_DSN = "${dbUser}:${dbPassword}@tcp(127.0.0.1:${MariaDBPort})/${dbName}?parseTime=true"
    }
    $sqlPassword = $dbPassword.Replace("'", "''")
    $sql = "CREATE DATABASE IF NOT EXISTS $dbName CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci; CREATE USER IF NOT EXISTS '$dbUser'@'127.0.0.1' IDENTIFIED BY '$sqlPassword'; GRANT ALL PRIVILEGES ON $dbName.* TO '$dbUser'@'127.0.0.1'; CREATE USER IF NOT EXISTS '$dbUser'@'localhost' IDENTIFIED BY '$sqlPassword'; GRANT ALL PRIVILEGES ON $dbName.* TO '$dbUser'@'localhost'; FLUSH PRIVILEGES;"
    & $client --protocol=tcp --ssl=0 -h 127.0.0.1 -P $MariaDBPort -u root -e $sql
    if ($LASTEXITCODE -ne 0) {
        throw "MariaDB bootstrap SQL failed."
    }

    if (Test-VectorRequiresChroma $env:AC_VECTOR_MODE) {
        if ($env:AC_VECTOR_MODE -eq "external") {
            if ([string]::IsNullOrWhiteSpace($env:AC_CHROMA_ENDPOINT)) {
                throw "AC_CHROMA_ENDPOINT is required for vector_external."
            }
        } else {
            if ([string]::IsNullOrWhiteSpace($env:AC_CHROMA_ENDPOINT)) {
                $env:AC_CHROMA_ENDPOINT = "http://127.0.0.1:8000"
            }
            $chromaUri = [Uri]$env:AC_CHROMA_ENDPOINT
            $chromaPort = if ($chromaUri.Port -gt 0) { $chromaUri.Port } else { 8000 }
            if (-not (Test-PortOpen $chromaPort)) {
                $startedChroma = Start-BundledChromaDB -PackageRoot $packRoot -Endpoint $chromaUri
            }
            try {
                Wait-Port $chromaPort 60
            } catch {
                Write-Host "ChromaDB failed to open port $chromaPort."
                throw
            }
        }
    } else {
        $env:AC_CHROMA_ENDPOINT = ""
    }

    $env:AC_MODE = "live"
    $env:AC_STORE_MODE = "mariadb_authority"
    if ([string]::IsNullOrWhiteSpace($env:AC_BIND_ADDR)) {
        $env:AC_BIND_ADDR = "0.0.0.0:28080"
    }
    if ([string]::IsNullOrWhiteSpace($env:AC_CHROMA_COLLECTION)) {
        $env:AC_CHROMA_COLLECTION = "archive_center_vectors"
    }
    if ([string]::IsNullOrWhiteSpace($env:AC_CHROMA_API_PATH)) {
        $env:AC_CHROMA_API_PATH = "/api/v2"
    }
    $env:AC_PROMPT_DIR = Join-Path $packRoot "prompts"

    $schemaPath = Join-Path $packRoot "migrations\001_schema.sql"
    & (Join-Path $packRoot "bin\mariadb-schema.exe") -dsn $env:AC_MARIADB_DSN -schema $schemaPath -execute
    if ($LASTEXITCODE -ne 0) {
        throw "mariadb-schema failed."
    }

Write-Host "Starting Archive Center 2.1 full package"
    Write-Host "  Go:      $($env:AC_BIND_ADDR)"
    Write-Host "  MariaDB: 127.0.0.1:$MariaDBPort"
    if (Test-VectorRequiresChroma $env:AC_VECTOR_MODE) {
        Write-Host "  Chroma:  $($env:AC_CHROMA_ENDPOINT)"
    } else {
        Write-Host "  Chroma:  disabled ($($env:AC_VECTOR_MODE))"
    }
    Write-Host "  Store:   $($env:AC_STORE_MODE)"
    Write-Host "  Profile: $($env:AC_RUNTIME_PROFILE)"
    Write-Host "  Vector:  $($env:AC_VECTOR_MODE)"
    Write-Host ""
    Write-Host "Stop with Ctrl+C."
    if ($pendingApplyStatus -eq "applied_pending_health") {
        $backendPort = 28080
        if ($env:AC_BIND_ADDR -match ':(\d+)$') {
            $backendPort = [int]$Matches[1]
        }
        $candidateBackend = Start-ArchiveChildProcess -FilePath $backendExe -WorkingDirectory $packRoot
        $health = Wait-BackendMainReady -Process $candidateBackend -Port $backendPort -ExpectedVersion $pendingTargetVersion -TimeoutSeconds 60
        if ($health.Ready) {
            $commitFailure = ""
            try {
                $commit = Invoke-ArchiveUpdater -RunnerPath $updaterRunner -Command "commit" -PackageRoot $packRoot
                if ($commit.ExitCode -ne 0 -or $commit.Status -ne "committed") {
                    $commitFailure = "status '$($commit.Status)' (exit $($commit.ExitCode))"
                }
            } catch {
                $commitFailure = $_.Exception.Message
            }
            if ([string]::IsNullOrWhiteSpace($commitFailure)) {
                Write-Host "Pending Archive Center package committed after main readiness passed."
                $candidateBackend.WaitForExit()
            } else {
                Stop-ArchiveChildProcess $candidateBackend
                $restartManagedMariaDB = $null -ne $startedMariaDB
                $restartManagedChroma = $null -ne $startedChroma
                Stop-ArchiveChildProcess $startedChroma
                Stop-ArchiveChildProcess $startedMariaDB
                $rollback = Invoke-ArchiveUpdater -RunnerPath $updaterRunner -Command "rollback" -PackageRoot $packRoot
                if (-not (Test-UpdaterSafeBaselineStatus $rollback.Status)) {
                    throw "Update commit failed ($commitFailure) and rollback was not reported safe (status '$($rollback.Status)'). Startup stopped to avoid a mixed package."
                }
                if ($rollback.Status -eq "rolled_back" -and -not [string]::IsNullOrWhiteSpace($pendingCurrentVersion)) {
                    $env:AC_BUILD_VERSION = $pendingCurrentVersion
                }
                if ($restartManagedMariaDB) {
                    $startedMariaDB = Start-ArchiveChildProcess -FilePath $mariadbd -ArgumentList $mariaArgs -WorkingDirectory $dataDir
                    Wait-Port $MariaDBPort 60
                }
                if ($restartManagedChroma) {
                    $startedChroma = Start-BundledChromaDB -PackageRoot $packRoot -Endpoint $chromaUri
                    Wait-Port $chromaPort 60
                }
                Write-Host "Update commit did not return a clean acknowledgement ($commitFailure). Recovery is safe; starting the verified current backend."
                & $backendExe
            }
        } else {
            Stop-ArchiveChildProcess $candidateBackend
            $restartManagedMariaDB = $null -ne $startedMariaDB
            $restartManagedChroma = $null -ne $startedChroma
            Stop-ArchiveChildProcess $startedChroma
            Stop-ArchiveChildProcess $startedMariaDB
            $rollback = Invoke-ArchiveUpdater -RunnerPath $updaterRunner -Command "rollback" -PackageRoot $packRoot
            if (-not (Test-UpdaterSafeBaselineStatus $rollback.Status)) {
                throw "Updated backend failed main readiness ($($health.Detail)) and rollback was not reported safe (status '$($rollback.Status)'). Startup stopped to avoid a mixed package."
            }
            if ($rollback.Status -eq "rolled_back" -and -not [string]::IsNullOrWhiteSpace($pendingCurrentVersion)) {
                $env:AC_BUILD_VERSION = $pendingCurrentVersion
            }
            if ($restartManagedMariaDB) {
                $startedMariaDB = Start-ArchiveChildProcess -FilePath $mariadbd -ArgumentList $mariaArgs -WorkingDirectory $dataDir
                Wait-Port $MariaDBPort 60
            }
            if ($restartManagedChroma) {
                $startedChroma = Start-BundledChromaDB -PackageRoot $packRoot -Endpoint $chromaUri
                Wait-Port $chromaPort 60
            }
            Write-Host "Updated backend failed main readiness ($($health.Detail)). The verified baseline was restored; starting the old backend."
            & $backendExe
        }
    } else {
        & $backendExe
    }
} catch {
    $startupError = $_
    if ($pendingApplyStatus -eq "applied_pending_health" -and -not [string]::IsNullOrWhiteSpace($updaterRunner)) {
        Stop-ArchiveChildProcess $startedChroma
        Stop-ArchiveChildProcess $startedMariaDB
        try {
            $startupRollback = Invoke-ArchiveUpdater -RunnerPath $updaterRunner -Command "rollback" -PackageRoot $packRoot
            if ($startupRollback.ExitCode -ne 0 -or $startupRollback.Status -notin @("rolled_back", "nothing_to_rollback")) {
                throw "rollback status '$($startupRollback.Status)' (exit $($startupRollback.ExitCode))"
            }
            Write-Host "Updated package preparation failed before main readiness. Managed package files were rolled back."
        } catch {
            throw "Updated package preparation failed, and rollback could not be proven safe. Startup stopped to avoid a mixed package.`nOriginal: $($startupError.Exception.Message)`nRollback: $($_.Exception.Message)"
        }
    }
    throw $startupError
} finally {
    if ($updaterRunner -and (Test-Path -LiteralPath $updaterRunner -PathType Leaf)) {
        Remove-Item -LiteralPath $updaterRunner -Force -ErrorAction SilentlyContinue
    }
    if (-not $KeepServices) {
        if ($startedChroma -and -not $startedChroma.HasExited) {
            $startedChroma.Kill()
        }
        if ($startedMariaDB -and -not $startedMariaDB.HasExited) {
            $startedMariaDB.Kill()
        }
    }
}
