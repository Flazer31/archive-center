param(
    [switch]$Preflight,
    [switch]$InstallMariaDBRuntime,
    [switch]$InstallChromaDBRuntime,
    [switch]$StageMariaDBProvider,
    [switch]$VerifyBundle,
    [string]$Out = "",
    [string]$DataDir = "",
    [string]$InstallDir = "",
    [string]$ProviderArchive = "",
    [string]$BundlePath = "",
    [string]$MariaDBVersion = "12.3.2",
    [string]$MariaDBDownloadUrl = "https://downloads.mariadb.org/rest-api/mariadb/12.3.2/mariadb-12.3.2-winx64.zip",
    [string]$MariaDBSha256 = "67347c129eb9c5923d002ea34fbfa27c60eb95d36dd73b85af2651cdeceecac5",
    [string]$PythonVersion = "3.11.9",
    [string]$PythonDownloadUrl = "https://www.python.org/ftp/python/3.11.9/python-3.11.9-amd64.exe",
    [string]$PythonSha256 = "5ee42c4eee1e6b4464bb23722f90b45303f79442df63083f05322f1785f5fdde",
    [string]$ChromaDBVersion = "1.5.9"
)

Set-StrictMode -Version 3.0
$ErrorActionPreference = "Stop"

function Resolve-ExistingPathOrRaw {
    param([string]$Path)
    if ([string]::IsNullOrWhiteSpace($Path)) {
        return ""
    }
    if (Test-Path -LiteralPath $Path) {
        return (Resolve-Path -LiteralPath $Path).Path
    }
    return [System.IO.Path]::GetFullPath($Path)
}

function Test-PathInside {
    param([string]$Child, [string]$Parent)
    if ([string]::IsNullOrWhiteSpace($Child) -or [string]::IsNullOrWhiteSpace($Parent)) {
        return $false
    }
    $childFull = [System.IO.Path]::GetFullPath($Child).TrimEnd('\') + '\'
    $parentFull = [System.IO.Path]::GetFullPath($Parent).TrimEnd('\') + '\'
    return $childFull.StartsWith($parentFull, [System.StringComparison]::OrdinalIgnoreCase)
}

function Add-ListItem {
    param([System.Collections.Generic.List[string]]$List, [string]$Item)
    if (-not [string]::IsNullOrWhiteSpace($Item)) {
        [void]$List.Add($Item)
    }
}

function Test-PortFree {
    param([int]$Port)
    $listener = $null
    try {
        $listener = [System.Net.Sockets.TcpListener]::new([System.Net.IPAddress]::Parse("127.0.0.1"), $Port)
        $listener.Start()
        return "free"
    } catch {
        return "busy"
    } finally {
        if ($null -ne $listener) {
            $listener.Stop()
        }
    }
}

function Test-RunningOnWindows {
    $platform = ""
    if ($PSVersionTable.ContainsKey("Platform")) {
        $platform = [string]$PSVersionTable.Platform
    }
    if ($platform -eq "Win32NT") {
        return $true
    }
    return [System.Runtime.InteropServices.RuntimeInformation]::IsOSPlatform([System.Runtime.InteropServices.OSPlatform]::Windows)
}

function Find-CommandPath {
    param([string[]]$Names)
    foreach ($name in $Names) {
        $cmd = Get-Command $name -ErrorAction SilentlyContinue | Select-Object -First 1
        if ($null -ne $cmd -and -not [string]::IsNullOrWhiteSpace($cmd.Source)) {
            return $cmd.Source
        }
    }
    return ""
}

function Find-BundledMariaDB {
    param([string]$Root)
    $candidates = @(
        "runtime\MariaDB\bin\mariadbd.exe",
        "runtime\mariadb\bin\mariadbd.exe",
        "vendor\MariaDB\bin\mariadbd.exe",
        "vendor\mariadb\bin\mariadbd.exe",
        "resources\MariaDB\bin\mariadbd.exe",
        "resources\mariadb\bin\mariadbd.exe",
        "mariadb\bin\mariadbd.exe",
        "MariaDB\bin\mariadbd.exe",
        "runtime\MariaDB\bin\mysqld.exe",
        "runtime\mariadb\bin\mysqld.exe"
    )
    foreach ($rel in $candidates) {
        $path = Join-Path $Root $rel
        if (Test-Path -LiteralPath $path -PathType Leaf) {
            return (Resolve-Path -LiteralPath $path).Path
        }
    }
    # Fallback: search any nested */bin/mariadbd.exe or */bin/mysqld.exe
    $fallback = Get-ChildItem -Path $Root -Recurse -File -ErrorAction SilentlyContinue |
        Where-Object { ($_.Name -ieq "mariadbd.exe" -or $_.Name -ieq "mysqld.exe") -and $_.DirectoryName -match "(^|[\\/])bin$" } |
        Sort-Object FullName |
        Select-Object -First 1
    if ($null -ne $fallback) {
        return (Resolve-Path -LiteralPath $fallback.FullName).Path
    }
    return ""
}

function Find-ExtractedMariaDBProvider {
    param([string]$Root)

    $direct = Find-BundledMariaDB $Root
    if (-not [string]::IsNullOrWhiteSpace($direct)) {
        return $direct
    }

    $runtimeRoot = Join-Path $Root "runtime\MariaDB"
    if (-not (Test-Path -LiteralPath $runtimeRoot -PathType Container)) {
        return ""
    }

    $found = Get-ChildItem -LiteralPath $runtimeRoot -Recurse -File -ErrorAction SilentlyContinue |
        Where-Object { ($_.Name -ieq "mariadbd.exe" -or $_.Name -ieq "mysqld.exe") -and $_.DirectoryName -match "(^|[\\/])bin$" } |
        Sort-Object FullName |
        Select-Object -First 1
    if ($null -eq $found) {
        return ""
    }
    return (Resolve-Path -LiteralPath $found.FullName).Path
}

function Find-BundledProviderArchive {
    param([string]$SearchRoot)
    if ([string]::IsNullOrWhiteSpace($SearchRoot)) {
        return ""
    }
    $candidates = @(
        "mariadb-provider.zip",
        "MariaDB.zip",
        "runtime\MariaDB.zip",
        "runtime\mariadb.zip",
        "bundled\mariadb-provider.zip",
        "bundled\MariaDB.zip"
    )
    foreach ($rel in $candidates) {
        $path = Join-Path $SearchRoot $rel
        if (Test-Path -LiteralPath $path -PathType Leaf) {
            return (Resolve-Path -LiteralPath $path).Path
        }
    }
    return ""
}

function Find-GoBackendBinary {
    param([string]$Root)
    $candidates = @(
        "go-service\archive-center-go.exe",
        "archive-center-go.exe",
        "bin\archive-center-go.exe",
        "runtime\archive-center-go.exe"
    )
    foreach ($rel in $candidates) {
        $path = Join-Path $Root $rel
        if (Test-Path -LiteralPath $path -PathType Leaf) {
            return (Resolve-Path -LiteralPath $path).Path
        }
    }
    $fallback = Get-ChildItem -LiteralPath $Root -Recurse -File -ErrorAction SilentlyContinue |
        Where-Object { $_.Name -ieq "archive-center-go.exe" } |
        Sort-Object FullName |
        Select-Object -First 1
    if ($null -ne $fallback) {
        return (Resolve-Path -LiteralPath $fallback.FullName).Path
    }
    return ""
}

function Find-ChromaDBPython {
    param([string]$Root)
    if ([string]::IsNullOrWhiteSpace($Root) -or -not (Test-Path -LiteralPath $Root -PathType Container)) {
        return ""
    }
    $candidates = @(
        "runtime\ChromaDB\$ChromaDBVersion\Scripts\python.exe",
        "runtime\chromadb\$ChromaDBVersion\Scripts\python.exe",
        "runtime\ChromaDB\Scripts\python.exe",
        "runtime\chromadb\Scripts\python.exe",
        "runtime\ChromaDB\python.exe",
        "runtime\chromadb\python.exe"
    )
    foreach ($rel in $candidates) {
        $path = Join-Path $Root $rel
        if (Test-Path -LiteralPath $path -PathType Leaf) {
            return (Resolve-Path -LiteralPath $path).Path
        }
    }
    $found = Get-ChildItem -LiteralPath $Root -Recurse -File -ErrorAction SilentlyContinue |
        Where-Object { $_.Name -ieq "python.exe" -and $_.FullName -match "(?i)[\\/]runtime[\\/]chromadb[\\/]" } |
        Sort-Object FullName |
        Select-Object -First 1
    if ($null -ne $found) {
        return (Resolve-Path -LiteralPath $found.FullName).Path
    }
    return ""
}

function Find-ChromaDBRuntime {
    param([string]$Root)
    $python = Find-ChromaDBPython $Root
    if ([string]::IsNullOrWhiteSpace($python)) {
        return ""
    }
    $runtime = Split-Path -Parent (Split-Path -Parent $python)
    return (Resolve-Path -LiteralPath $runtime).Path
}

function Test-ChromaDBRuntimeVersion {
    param([string]$PythonPath)
    if ([string]::IsNullOrWhiteSpace($PythonPath) -or -not (Test-Path -LiteralPath $PythonPath -PathType Leaf)) {
        return $false
    }
    $previousErrorActionPreference = $ErrorActionPreference
    $probeExitCode = -1
    try {
        $ErrorActionPreference = "Continue"
        & $PythonPath -c "import sys; from importlib.metadata import version; import chromadb; sys.exit(0 if version('chromadb') == sys.argv[1] else 1)" $ChromaDBVersion *> $null
        $probeExitCode = $LASTEXITCODE
    } catch {
        $probeExitCode = -1
    } finally {
        $ErrorActionPreference = $previousErrorActionPreference
    }
    return $probeExitCode -eq 0
}

function Test-CompatiblePythonBootstrap {
    param([string]$PythonPath)
    if ([string]::IsNullOrWhiteSpace($PythonPath) -or -not (Test-Path -LiteralPath $PythonPath -PathType Leaf)) {
        return $false
    }
    try {
        $signature = Get-AuthenticodeSignature -LiteralPath $PythonPath -ErrorAction Stop
    } catch {
        # An inaccessible or policy-blocked system Python is not a usable
        # bootstrap candidate. Continue to the next candidate or install the
        # verified managed Python runtime.
        return $false
    }
    $signerSubject = if ($null -ne $signature.SignerCertificate) { [string]$signature.SignerCertificate.Subject } else { "" }
    if ($signature.Status -ne [System.Management.Automation.SignatureStatus]::Valid -or $signerSubject -notmatch "(?i)(^|,\s*)O=Python Software Foundation(,|$)") {
        return $false
    }
    $previousErrorActionPreference = $ErrorActionPreference
    $probeExitCode = -1
    try {
        $ErrorActionPreference = "Continue"
        & $PythonPath -c "import struct, sys; sys.exit(0 if (3, 9) <= sys.version_info[:2] < (3, 13) and struct.calcsize('P') * 8 == 64 else 1)" *> $null
        $probeExitCode = $LASTEXITCODE
    } catch {
        $probeExitCode = -1
    } finally {
        $ErrorActionPreference = $previousErrorActionPreference
    }
    return $probeExitCode -eq 0
}

function Get-PythonRuntimeVersion {
    param([string]$PythonPath)
    if ([string]::IsNullOrWhiteSpace($PythonPath) -or -not (Test-Path -LiteralPath $PythonPath -PathType Leaf)) {
        return ""
    }
    $productVersion = (Get-Item -LiteralPath $PythonPath).VersionInfo.ProductVersion
    if (-not [string]::IsNullOrWhiteSpace($productVersion) -and $productVersion -match "\d+\.\d+\.\d+") {
        return $Matches[0]
    }
    $versionText = (& $PythonPath -c "import platform; print(platform.python_version())" 2>$null | Select-Object -First 1)
    if ($LASTEXITCODE -ne 0) {
        return ""
    }
    return ([string]$versionText).Trim()
}

function Find-CompatiblePythonBootstrap {
    param([string]$PreferredPath)

    $candidates = [System.Collections.Generic.List[string]]::new()
    if (-not [string]::IsNullOrWhiteSpace($PreferredPath)) {
        [void]$candidates.Add($PreferredPath)
    }

    $pyLauncher = Get-Command py.exe -ErrorAction SilentlyContinue | Select-Object -First 1
    if ($null -ne $pyLauncher -and -not [string]::IsNullOrWhiteSpace($pyLauncher.Source)) {
        foreach ($selector in @("-3.12", "-3.11", "-3.10", "-3.9")) {
            $previousErrorActionPreference = $ErrorActionPreference
            try {
                $ErrorActionPreference = "Continue"
                $located = (& $pyLauncher.Source $selector -c "import sys; print(sys.executable)" 2>$null | Select-Object -First 1)
                $launcherExitCode = $LASTEXITCODE
            } finally {
                $ErrorActionPreference = $previousErrorActionPreference
            }
            if ($launcherExitCode -eq 0 -and -not [string]::IsNullOrWhiteSpace([string]$located)) {
                [void]$candidates.Add(([string]$located).Trim())
            }
        }
    }

    $localAppData = [Environment]::GetFolderPath("LocalApplicationData")
    $programFiles = [Environment]::GetFolderPath("ProgramFiles")
    $programFilesX86 = [Environment]::GetFolderPath("ProgramFilesX86")
    foreach ($candidate in @(
        (Join-Path $localAppData "Programs\Python\Python312\python.exe"),
        (Join-Path $localAppData "Programs\Python\Python311\python.exe"),
        (Join-Path $localAppData "Programs\Python\Python310\python.exe"),
        (Join-Path $localAppData "Programs\Python\Python39\python.exe"),
        (Join-Path $programFiles "Python312\python.exe"),
        (Join-Path $programFiles "Python311\python.exe"),
        (Join-Path $programFiles "Python310\python.exe"),
        (Join-Path $programFiles "Python39\python.exe"),
        (Join-Path $programFilesX86 "Python312\python.exe"),
        (Join-Path $programFilesX86 "Python311\python.exe"),
        (Join-Path $programFilesX86 "Python310\python.exe"),
        (Join-Path $programFilesX86 "Python39\python.exe")
    )) {
        if (-not [string]::IsNullOrWhiteSpace($candidate)) {
            [void]$candidates.Add($candidate)
        }
    }

    $seen = @{}
    foreach ($candidate in $candidates) {
        if ([string]::IsNullOrWhiteSpace($candidate)) {
            continue
        }
        $full = [System.IO.Path]::GetFullPath($candidate)
        if ($seen.ContainsKey($full)) {
            continue
        }
        $seen[$full] = $true
        if (Test-CompatiblePythonBootstrap $full) {
            return $full
        }
    }
    return ""
}

function Write-JsonReport {
    param([object]$Report, [string]$Path)
    $json = $Report | ConvertTo-Json -Depth 8
    if ([string]::IsNullOrWhiteSpace($Path)) {
        Write-Output $json
        return
    }
    $parent = Split-Path -Parent $Path
    if (-not [string]::IsNullOrWhiteSpace($parent)) {
        New-Item -ItemType Directory -Force -Path $parent | Out-Null
    }
    Set-Content -LiteralPath $Path -Value $json -Encoding UTF8
}

function Invoke-Preflight {
    $scriptDir = Split-Path -Parent $PSCommandPath
    $repoRoot = Resolve-Path -LiteralPath (Join-Path $scriptDir "..")
    $repoRoot = $repoRoot.Path

    $effectiveInstallDir = $InstallDir
    if ([string]::IsNullOrWhiteSpace($effectiveInstallDir)) {
        $effectiveInstallDir = $repoRoot
    }
    $effectiveInstallDir = Resolve-ExistingPathOrRaw $effectiveInstallDir

    $effectiveDataDir = $DataDir
    if ([string]::IsNullOrWhiteSpace($effectiveDataDir)) {
        $localAppData = [Environment]::GetFolderPath("LocalApplicationData")
        if ([string]::IsNullOrWhiteSpace($localAppData)) {
            $localAppData = Join-Path $env:USERPROFILE "AppData\Local"
        }
        $effectiveDataDir = Join-Path $localAppData "ArchiveCenter"
    }
    $effectiveDataDir = Resolve-ExistingPathOrRaw $effectiveDataDir

    $warnings = [System.Collections.Generic.List[string]]::new()
    $failures = [System.Collections.Generic.List[string]]::new()

    $isWindows = Test-RunningOnWindows
    if (-not $isWindows) {
        Add-ListItem $failures "not_running_on_windows"
    }

    $arch = [System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture.ToString().ToLowerInvariant()
    $archSupported = $arch -in @("x64", "arm64")
    if (-not $archSupported) {
        Add-ListItem $failures "unsupported_architecture"
    }

    $dataOutsideSource = -not (Test-PathInside $effectiveDataDir $repoRoot)
    if (-not $dataOutsideSource) {
        Add-ListItem $failures "data_dir_inside_source_tree"
    }

    $writeProbeOK = $false
    $writeProbeTarget = ""
    if ($dataOutsideSource) {
        $target = if (Test-Path -LiteralPath $effectiveDataDir -PathType Container) {
            $effectiveDataDir
        } else {
            Split-Path -Parent $effectiveDataDir
        }
        if (-not [string]::IsNullOrWhiteSpace($target) -and (Test-Path -LiteralPath $target -PathType Container)) {
            $writeProbeTarget = $target
            $probe = Join-Path $target ".archive-center-preflight-$PID.tmp"
            try {
                Set-Content -LiteralPath $probe -Value "ok" -Encoding ASCII
                Remove-Item -LiteralPath $probe -Force
                $writeProbeOK = $true
            } catch {
                Add-ListItem $failures "install_target_not_writable"
            }
        } else {
            Add-ListItem $failures "install_target_parent_missing"
        }
    }

    $goBinary = if ($env:ARCHIVE_CENTER_GO_BINARY) { $env:ARCHIVE_CENTER_GO_BINARY } else { Find-GoBackendBinary $effectiveInstallDir }
    if ([string]::IsNullOrWhiteSpace($goBinary)) {
        $goBinary = Join-Path $effectiveInstallDir "bin\archive-center-go.exe"
    }
    $goBinaryPresent = Test-Path -LiteralPath $goBinary -PathType Leaf
    if (-not $goBinaryPresent) {
        Add-ListItem $warnings "go_backend_binary_not_found"
    }
    $goToolAvailable = [bool](Get-Command go -ErrorAction SilentlyContinue)

    $localAppData = [Environment]::GetFolderPath([Environment+SpecialFolder]::LocalApplicationData)
    if ([string]::IsNullOrWhiteSpace($localAppData)) {
        $localAppData = $env:LOCALAPPDATA
    }
    $defaultMariaDBInstallRoot = Join-Path $localAppData "ArchiveCenter"
    $mariaDBSearchRoot = if (-not [string]::IsNullOrWhiteSpace($env:AC_MARIADB_RUNTIME_DIR)) {
        Resolve-ExistingPathOrRaw $env:AC_MARIADB_RUNTIME_DIR
    } else {
        $defaultMariaDBInstallRoot
    }
    $separateProvider = Find-ExtractedMariaDBProvider $mariaDBSearchRoot
    $bundleEmbeddedArchive = Find-BundledProviderArchive $effectiveInstallDir
    $providerArchivePresent = -not [string]::IsNullOrWhiteSpace($ProviderArchive) -and (Test-Path -LiteralPath $ProviderArchive -PathType Leaf)
    if (-not $providerArchivePresent -and -not [string]::IsNullOrWhiteSpace($bundleEmbeddedArchive)) {
        $providerArchivePresent = $true
    }
    $systemProvider = Find-CommandPath @("mariadbd.exe", "mysqld.exe", "mariadbd", "mysqld")
    $providerMode = "official_download_required"
    $providerPath = ""
    $installerManagedRequired = $true
    $requiredAction = "installer downloads the verified official MariaDB runtime into the per-user ArchiveCenter runtime directory"

    if (-not [string]::IsNullOrWhiteSpace($separateProvider)) {
        $providerMode = "separate_runtime"
        $providerPath = $separateProvider
        $installerManagedRequired = $false
        $requiredAction = "use the installed separate MariaDB runtime"
    } elseif (-not [string]::IsNullOrWhiteSpace($bundleEmbeddedArchive)) {
        $providerMode = "legacy_embedded_archive"
        $providerPath = $bundleEmbeddedArchive
        $requiredAction = "migrate the legacy embedded archive to the separate runtime directory"
        $installerManagedRequired = $false
    } elseif (-not [string]::IsNullOrWhiteSpace($systemProvider)) {
        $providerMode = "system_command"
        $providerPath = $systemProvider
        $installerManagedRequired = $false
        $requiredAction = "use the detected system MariaDB provider"
    } elseif ($providerArchivePresent) {
        $providerMode = "installer_bundle_available"
        $providerPath = Resolve-ExistingPathOrRaw $ProviderArchive
        $requiredAction = "run -StageMariaDBProvider with this archive into a non-source install directory"
    } else {
        Add-ListItem $warnings "mariadb_separate_runtime_install_required"
    }

    $chromaSearchRoot = if (-not [string]::IsNullOrWhiteSpace($env:AC_CHROMA_RUNTIME_DIR)) {
        Resolve-ExistingPathOrRaw $env:AC_CHROMA_RUNTIME_DIR
    } else {
        $defaultMariaDBInstallRoot
    }
    $chromaRuntime = Find-ChromaDBRuntime $chromaSearchRoot
    $chromaRuntimePresent = -not [string]::IsNullOrWhiteSpace($chromaRuntime)
    if (-not $chromaRuntimePresent) {
        Add-ListItem $warnings "chromadb_separate_runtime_install_required"
    }

    $supportLevel = "green"
    $preflightStatus = "ok"
    $fallbackProfile = "none"
    if ($failures.Count -gt 0) {
        $supportLevel = "red"
        $preflightStatus = "unsupported"
    } elseif ($installerManagedRequired -or -not $goBinaryPresent -or -not $chromaRuntimePresent) {
        $supportLevel = "yellow"
        $preflightStatus = "degraded"
        $fallbackProfile = "windows_separate_runtime_install_required"
    }

    return [ordered]@{
        schema_version = "archive-center.preflight.v1"
        target = "windows"
        preflight_only = $true
        platform = "Windows"
        arch = $arch
        support_level = $supportLevel
        preflight_status = $preflightStatus
        install_status = "not_run_preflight_only"
        fallback_profile = $fallbackProfile
        paths = [ordered]@{
            repo_root = $repoRoot
            install_dir = $effectiveInstallDir
            data_dir = $effectiveDataDir
            data_path_outside_source = $dataOutsideSource
            write_probe_target = $writeProbeTarget
            write_probe_ok = $writeProbeOK
        }
        go_backend = [ordered]@{
            binary_path = $goBinary
            binary_present = $goBinaryPresent
            go_tool_available = $goToolAvailable
            health_status = "not_run_preflight_only"
            ready_status = "not_run_preflight_only"
            version_status = "not_run_preflight_only"
        }
        mariadb = [ordered]@{
            provider_mode = $providerMode
            provider_path = $providerPath
            provider_archive_present = $providerArchivePresent
            installer_managed_required = $installerManagedRequired
            normal_user_manual_mariadb_required = $false
            required_action = $requiredAction
            schema_status = "not_run_preflight_only"
            smoke_status = "not_run_preflight_only"
        }
        chromadb = [ordered]@{
            runtime_present = $chromaRuntimePresent
            runtime_path = $chromaRuntime
            installer_managed_required = -not $chromaRuntimePresent
            normal_user_manual_chromadb_required = $false
            required_action = if ($chromaRuntimePresent) { "use the installed separate ChromaDB runtime" } else { "installer downloads verified official Python and installs pinned ChromaDB into the per-user ArchiveCenter runtime directory" }
            smoke_status = "not_run_preflight_only"
        }
        ports = [ordered]@{
            go_28080 = Test-PortFree 28080
            mariadb_3307 = Test-PortFree 3307
            chromadb_8000 = Test-PortFree 8000
        }
        warnings = @($warnings)
        failures = @($failures)
    }
}

function Invoke-StageMariaDBProvider {
    $scriptDir = Split-Path -Parent $PSCommandPath
    $repoRoot = (Resolve-Path -LiteralPath (Join-Path $scriptDir "..")).Path
    $effectiveInstallDir = if ([string]::IsNullOrWhiteSpace($InstallDir)) { $repoRoot } else { Resolve-ExistingPathOrRaw $InstallDir }
    if ([string]::IsNullOrWhiteSpace($ProviderArchive)) {
        $autoArchive = Find-BundledProviderArchive $effectiveInstallDir
        if ([string]::IsNullOrWhiteSpace($autoArchive)) {
            throw "ProviderArchive is required for -StageMariaDBProvider"
        }
        $ProviderArchive = $autoArchive
    }
    if (-not (Test-Path -LiteralPath $ProviderArchive -PathType Leaf)) {
        throw "ProviderArchive not found: $ProviderArchive"
    }
    $scriptDir = Split-Path -Parent $PSCommandPath
    $repoRoot = (Resolve-Path -LiteralPath (Join-Path $scriptDir "..")).Path
    $effectiveInstallDir = if ([string]::IsNullOrWhiteSpace($InstallDir)) { $repoRoot } else { Resolve-ExistingPathOrRaw $InstallDir }
    if (Test-PathInside $effectiveInstallDir $repoRoot) {
        throw "Refusing to stage MariaDB provider into the source tree. Pass -InstallDir outside the repository."
    }
    $targetDir = Join-Path $effectiveInstallDir "runtime\MariaDB"
    New-Item -ItemType Directory -Force -Path $targetDir | Out-Null
    Expand-Archive -LiteralPath $ProviderArchive -DestinationPath $targetDir -Force
    $provider = Find-ExtractedMariaDBProvider $effectiveInstallDir
    if ([string]::IsNullOrWhiteSpace($provider)) {
        throw "Archive was extracted, but mariadbd.exe/mysqld.exe was not found under runtime/MariaDB"
    }
    return [ordered]@{
        schema_version = "archive-center.provider-stage.v1"
        target = "windows"
        status = "ok"
        install_dir = $effectiveInstallDir
        provider_path = $provider
        normal_user_manual_mariadb_required = $false
        authority_switch = $false
        go_default_switch = $false
    }
}

function Invoke-InstallMariaDBRuntime {
    $scriptDir = Split-Path -Parent $PSCommandPath
    $repoRoot = (Resolve-Path -LiteralPath (Join-Path $scriptDir "..")).Path
    $effectiveInstallDir = $InstallDir
    if ([string]::IsNullOrWhiteSpace($effectiveInstallDir)) {
        $localAppData = [Environment]::GetFolderPath("LocalApplicationData")
        if ([string]::IsNullOrWhiteSpace($localAppData)) {
            $localAppData = Join-Path $env:USERPROFILE "AppData\Local"
        }
        $effectiveInstallDir = Join-Path $localAppData "ArchiveCenter"
    }
    $effectiveInstallDir = Resolve-ExistingPathOrRaw $effectiveInstallDir
    if (Test-PathInside $effectiveInstallDir $repoRoot) {
        throw "Refusing to install the separate MariaDB runtime inside the Archive Center package or source tree."
    }

    $existing = Find-ExtractedMariaDBProvider $effectiveInstallDir
    if (-not [string]::IsNullOrWhiteSpace($existing)) {
        return [ordered]@{
            schema_version = "archive-center.mariadb-runtime-install.v1"
            status = "already_installed"
            version = $MariaDBVersion
            install_dir = $effectiveInstallDir
            provider_path = $existing
            downloaded_by_archive_center = $false
        }
    }

    if ([string]::IsNullOrWhiteSpace($MariaDBDownloadUrl) -or [string]::IsNullOrWhiteSpace($MariaDBSha256)) {
        throw "MariaDB download URL and SHA-256 are required."
    }

    $runtimeRoot = Join-Path $effectiveInstallDir "runtime\MariaDB"
    $versionRoot = Join-Path $runtimeRoot $MariaDBVersion
    New-Item -ItemType Directory -Force -Path $versionRoot | Out-Null
    $archivePath = Join-Path ([System.IO.Path]::GetTempPath()) ("archive-center-mariadb-$MariaDBVersion-$PID.zip")
    try {
        [Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
        Write-Host "Downloading MariaDB $MariaDBVersion from the official MariaDB distribution service."
        Invoke-WebRequest -UseBasicParsing -Uri $MariaDBDownloadUrl -OutFile $archivePath
        $actualSha256 = (Get-FileHash -LiteralPath $archivePath -Algorithm SHA256).Hash.ToLowerInvariant()
        $expectedSha256 = $MariaDBSha256.Trim().ToLowerInvariant()
        if ($actualSha256 -ne $expectedSha256) {
            throw "MariaDB archive SHA-256 mismatch. Expected $expectedSha256, got $actualSha256."
        }
        Expand-Archive -LiteralPath $archivePath -DestinationPath $versionRoot -Force
        $provider = Find-ExtractedMariaDBProvider $effectiveInstallDir
        if ([string]::IsNullOrWhiteSpace($provider)) {
            throw "MariaDB archive was verified and extracted, but mariadbd.exe/mysqld.exe was not found."
        }
        return [ordered]@{
            schema_version = "archive-center.mariadb-runtime-install.v1"
            status = "installed"
            version = $MariaDBVersion
            install_dir = $effectiveInstallDir
            runtime_root = $runtimeRoot
            provider_path = $provider
            source_url = $MariaDBDownloadUrl
            sha256 = $actualSha256
            downloaded_by_archive_center = $true
            package_bundled = $false
        }
    } finally {
        if (Test-Path -LiteralPath $archivePath -PathType Leaf) {
            Remove-Item -LiteralPath $archivePath -Force -ErrorAction SilentlyContinue
        }
    }
}

function Invoke-InstallChromaDBRuntime {
    $scriptDir = Split-Path -Parent $PSCommandPath
    $repoRoot = (Resolve-Path -LiteralPath (Join-Path $scriptDir "..")).Path
    $effectiveInstallDir = $InstallDir
    if ([string]::IsNullOrWhiteSpace($effectiveInstallDir)) {
        $localAppData = [Environment]::GetFolderPath("LocalApplicationData")
        if ([string]::IsNullOrWhiteSpace($localAppData)) {
            $localAppData = Join-Path $env:USERPROFILE "AppData\Local"
        }
        $effectiveInstallDir = Join-Path $localAppData "ArchiveCenter"
    }
    $effectiveInstallDir = Resolve-ExistingPathOrRaw $effectiveInstallDir
    if (Test-PathInside $effectiveInstallDir $repoRoot) {
        throw "Refusing to install the separate Python/ChromaDB runtime inside the Archive Center package or source tree."
    }

    $arch = [System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture.ToString().ToLowerInvariant()
    if ($arch -ne "x64") {
        throw "The managed Windows ChromaDB installer currently supports x64 only. Detected architecture: $arch"
    }

    $existingPython = Find-ChromaDBPython $effectiveInstallDir
    if (Test-ChromaDBRuntimeVersion $existingPython) {
        $existingRuntimePythonVersion = Get-PythonRuntimeVersion $existingPython
        return [ordered]@{
            schema_version = "archive-center.chromadb-runtime-install.v1"
            status = "already_installed"
            version = $ChromaDBVersion
            python_version = $existingRuntimePythonVersion
            install_dir = $effectiveInstallDir
            runtime_root = Find-ChromaDBRuntime $effectiveInstallDir
            python_path = $existingPython
            downloaded_by_archive_center = $false
            package_bundled = $false
        }
    }

    if ([string]::IsNullOrWhiteSpace($PythonDownloadUrl) -or [string]::IsNullOrWhiteSpace($PythonSha256)) {
        throw "Python download URL and SHA-256 are required."
    }

    $pythonRoot = Join-Path $effectiveInstallDir "runtime\Python\$PythonVersion"
    $pythonExe = Join-Path $pythonRoot "python.exe"
    $chromaRoot = Join-Path $effectiveInstallDir "runtime\ChromaDB\$ChromaDBVersion"
    $chromaPython = Join-Path $chromaRoot "Scripts\python.exe"
    $installerPath = Join-Path ([System.IO.Path]::GetTempPath()) ("archive-center-python-$PythonVersion-$PID.exe")
    $downloadedPython = $false
    $bootstrapPython = Find-CompatiblePythonBootstrap $pythonExe
    try {
        if ([string]::IsNullOrWhiteSpace($bootstrapPython)) {
            [Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
            Write-Host "Downloading Python $PythonVersion from python.org for the managed ChromaDB runtime."
            Invoke-WebRequest -UseBasicParsing -Uri $PythonDownloadUrl -OutFile $installerPath
            $downloadedPython = $true
            $actualSha256 = (Get-FileHash -LiteralPath $installerPath -Algorithm SHA256).Hash.ToLowerInvariant()
            $expectedSha256 = $PythonSha256.Trim().ToLowerInvariant()
            if ($actualSha256 -ne $expectedSha256) {
                throw "Python installer SHA-256 mismatch. Expected $expectedSha256, got $actualSha256."
            }
            $signature = Get-AuthenticodeSignature -LiteralPath $installerPath
            $signerSubject = if ($null -ne $signature.SignerCertificate) { [string]$signature.SignerCertificate.Subject } else { "" }
            if ($signature.Status -ne [System.Management.Automation.SignatureStatus]::Valid -or $signerSubject -notmatch "(?i)(^|,\s*)O=Python Software Foundation(,|$)") {
                throw "Python installer Authenticode verification failed. Status: $($signature.Status); signer: $signerSubject"
            }

            New-Item -ItemType Directory -Force -Path $pythonRoot | Out-Null
            $pythonArgs = @(
                "/quiet",
                "InstallAllUsers=0",
                ('TargetDir="{0}"' -f $pythonRoot),
                "PrependPath=0",
                "Include_launcher=0",
                "Include_test=0",
                "Include_doc=0",
                "Include_tcltk=0",
                "Include_pip=1"
            )
            $installProcess = Start-Process -FilePath $installerPath -ArgumentList $pythonArgs -Wait -PassThru -WindowStyle Hidden
            if ($installProcess.ExitCode -ne 0) {
                throw "Python installer failed with exit code $($installProcess.ExitCode)."
            }
            $bootstrapPython = Find-CompatiblePythonBootstrap $pythonExe
            if ([string]::IsNullOrWhiteSpace($bootstrapPython)) {
                Write-Host "Python registration exists but the runtime is incomplete. Running the verified installer repair path."
                $repairProcess = Start-Process -FilePath $installerPath -ArgumentList @("/quiet", "/repair") -Wait -PassThru -WindowStyle Hidden
                if ($repairProcess.ExitCode -ne 0) {
                    throw "Python installer repair failed with exit code $($repairProcess.ExitCode)."
                }
                $bootstrapPython = Find-CompatiblePythonBootstrap $pythonExe
            }
        }
        if ([string]::IsNullOrWhiteSpace($bootstrapPython)) {
            throw "Python installation completed without producing a signed compatible Python 3.9-3.12 x64 runtime."
        }
        $bootstrapPythonVersion = Get-PythonRuntimeVersion $bootstrapPython

        New-Item -ItemType Directory -Force -Path (Split-Path -Parent $chromaRoot) | Out-Null
        & $bootstrapPython -m venv $chromaRoot 2>&1 | Out-Host
        if ($LASTEXITCODE -ne 0 -or -not (Test-Path -LiteralPath $chromaPython -PathType Leaf)) {
            throw "Managed ChromaDB virtual environment creation failed."
        }
        & $chromaPython -m pip install --disable-pip-version-check --no-input --upgrade pip wheel setuptools 2>&1 | Out-Host
        if ($LASTEXITCODE -ne 0) {
            throw "Managed ChromaDB pip bootstrap failed."
        }
        & $chromaPython -m pip install --disable-pip-version-check --no-input --upgrade "chromadb==$ChromaDBVersion" 2>&1 | Out-Host
        if ($LASTEXITCODE -ne 0) {
            throw "pip install chromadb==$ChromaDBVersion failed."
        }
        if (-not (Test-ChromaDBRuntimeVersion $chromaPython)) {
            throw "ChromaDB $ChromaDBVersion was installed but the runtime verification failed."
        }

        return [ordered]@{
            schema_version = "archive-center.chromadb-runtime-install.v1"
            status = "installed"
            version = $ChromaDBVersion
            python_version = $bootstrapPythonVersion
            install_dir = $effectiveInstallDir
            runtime_root = $chromaRoot
            python_path = $chromaPython
            bootstrap_python_path = $bootstrapPython
            python_source_url = if ($downloadedPython) { $PythonDownloadUrl } else { "" }
            python_sha256 = if ($downloadedPython) { $PythonSha256.Trim().ToLowerInvariant() } else { "" }
            python_authenticode_signer = "Python Software Foundation"
            downloaded_by_archive_center = $downloadedPython
            package_bundled = $false
        }
    } finally {
        if (Test-Path -LiteralPath $installerPath -PathType Leaf) {
            Remove-Item -LiteralPath $installerPath -Force -ErrorAction SilentlyContinue
        }
    }
}

function Invoke-VerifyBundle {
    if ([string]::IsNullOrWhiteSpace($BundlePath)) {
        throw "BundlePath is required for -VerifyBundle"
    }
    if (-not (Test-Path -LiteralPath $BundlePath -PathType Leaf)) {
        throw "BundlePath not found: $BundlePath"
    }
    $bundleFull = (Resolve-Path -LiteralPath $BundlePath).Path
    $tempRoot = Join-Path ([System.IO.Path]::GetTempPath()) ("archive-center-bundle-verify-" + $PID)
    if (Test-Path -LiteralPath $tempRoot) {
        Remove-Item -LiteralPath $tempRoot -Recurse -Force
    }
    New-Item -ItemType Directory -Force -Path $tempRoot | Out-Null

    $warnings = [System.Collections.Generic.List[string]]::new()
    $failures = [System.Collections.Generic.List[string]]::new()
    try {
        Expand-Archive -LiteralPath $bundleFull -DestinationPath $tempRoot -Force
        $goBinary = Find-GoBackendBinary $tempRoot
        $mariadbProvider = Find-ExtractedMariaDBProvider $tempRoot
        $chromaRuntime = Find-ChromaDBRuntime $tempRoot

        if ([string]::IsNullOrWhiteSpace($goBinary)) {
            Add-ListItem $failures "go_backend_binary_missing"
        }
        if (-not [string]::IsNullOrWhiteSpace($mariadbProvider)) {
            Add-ListItem $failures "mariadb_runtime_must_not_be_bundled"
        }
        if (-not [string]::IsNullOrWhiteSpace($chromaRuntime)) {
            Add-ListItem $failures "chromadb_runtime_must_not_be_bundled"
        }
        $runtimeInstallerFile = Get-ChildItem -LiteralPath $tempRoot -Recurse -File -ErrorAction SilentlyContinue |
            Where-Object { $_.Name -ieq "install-windows.ps1" -and $_.DirectoryName -match "(?i)[\\/]tools$" } |
            Sort-Object FullName |
            Select-Object -First 1
        if ($null -eq $runtimeInstallerFile) {
            Add-ListItem $failures "managed_runtime_installer_missing"
        } else {
            $runtimeInstaller = $runtimeInstallerFile.FullName
            $runtimeInstallerText = Get-Content -LiteralPath $runtimeInstaller -Raw -Encoding UTF8
            foreach ($marker in @("-InstallMariaDBRuntime", "-InstallChromaDBRuntime", "chromadb==`$ChromaDBVersion", "Python installer Authenticode verification failed")) {
                if (-not $runtimeInstallerText.Contains($marker)) {
                    Add-ListItem $failures "managed_runtime_installer_marker_missing:$marker"
                }
            }
        }

        $status = "ok"
        $supportLevel = "green"
        if ($failures.Count -gt 0) {
            $status = "blocked"
            $supportLevel = "red"
        } elseif ($warnings.Count -gt 0) {
            $status = "degraded"
            $supportLevel = "yellow"
        }

        return [ordered]@{
            schema_version = "archive-center.single-file-bundle.v1"
            target = "windows"
            status = $status
            support_level = $supportLevel
            bundle_path = $bundleFull
            single_file_bundle = $true
            extracted_to_temp = $true
            mariadb_distribution = "separate_official_runtime_install"
            normal_user_manual_mariadb_required = $false
            normal_user_manual_chromadb_required = $false
            authority_switch = $false
            go_default_switch = $false
            components = [ordered]@{
                go_backend_binary_present = -not [string]::IsNullOrWhiteSpace($goBinary)
                go_backend_binary_path = $goBinary
                mariadb_provider_present = $false
                mariadb_provider_path = ""
                chromadb_runtime_present = $false
                chromadb_runtime_path = ""
            }
            warnings = @($warnings)
            failures = @($failures)
        }
    } finally {
        if (Test-Path -LiteralPath $tempRoot) {
            Remove-Item -LiteralPath $tempRoot -Recurse -Force -ErrorAction SilentlyContinue
        }
    }
}

if (-not $Preflight -and -not $InstallMariaDBRuntime -and -not $InstallChromaDBRuntime -and -not $StageMariaDBProvider -and -not $VerifyBundle) {
    throw "Use -Preflight, -InstallMariaDBRuntime, -InstallChromaDBRuntime, -StageMariaDBProvider, or -VerifyBundle"
}

if ($Preflight) {
    Write-JsonReport (Invoke-Preflight) $Out
    exit 0
}

if ($VerifyBundle) {
    Write-JsonReport (Invoke-VerifyBundle) $Out
    exit 0
}

if ($InstallMariaDBRuntime) {
    Write-JsonReport (Invoke-InstallMariaDBRuntime) $Out
    exit 0
}

if ($InstallChromaDBRuntime) {
    Write-JsonReport (Invoke-InstallChromaDBRuntime) $Out
    exit 0
}

Write-JsonReport (Invoke-StageMariaDBProvider) $Out
exit 0
