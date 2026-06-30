param(
    [switch]$Preflight,
    [switch]$StageMariaDBProvider,
    [switch]$VerifyBundle,
    [string]$Out = "",
    [string]$DataDir = "",
    [string]$InstallDir = "",
    [string]$ProviderArchive = "",
    [string]$BundlePath = ""
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

function Find-ChromaDBRuntime {
    param([string]$Root)
    $candidates = @(
        "runtime\ChromaDB",
        "runtime\chromadb",
        "runtime\Python\Lib\site-packages\chromadb",
        "resources\ChromaDB",
        "resources\chromadb",
        "vendor\ChromaDB",
        "vendor\chromadb"
    )
    foreach ($rel in $candidates) {
        $path = Join-Path $Root $rel
        if (Test-Path -LiteralPath $path -PathType Container) {
            return (Resolve-Path -LiteralPath $path).Path
        }
    }
    $found = Get-ChildItem -LiteralPath $Root -Recurse -Directory -ErrorAction SilentlyContinue |
        Where-Object { $_.Name -ieq "chromadb" -or $_.Name -ieq "ChromaDB" -or $_.Name -ieq "chroma" } |
        Sort-Object FullName |
        Select-Object -First 1
    if ($null -ne $found) {
        return (Resolve-Path -LiteralPath $found.FullName).Path
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

    $bundledProvider = Find-ExtractedMariaDBProvider $effectiveInstallDir
    $bundleEmbeddedArchive = Find-BundledProviderArchive $effectiveInstallDir
    $providerArchivePresent = -not [string]::IsNullOrWhiteSpace($ProviderArchive) -and (Test-Path -LiteralPath $ProviderArchive -PathType Leaf)
    if (-not $providerArchivePresent -and -not [string]::IsNullOrWhiteSpace($bundleEmbeddedArchive)) {
        $providerArchivePresent = $true
    }
    $systemProvider = Find-CommandPath @("mariadbd.exe", "mysqld.exe", "mariadbd", "mysqld")
    $providerMode = "installer_bundle_required"
    $providerPath = ""
    $installerManagedRequired = $true
    $requiredAction = "installer must stage a bundled MariaDB runtime under runtime/MariaDB; normal users must not install MariaDB manually"

    if (-not [string]::IsNullOrWhiteSpace($bundledProvider)) {
        $providerMode = "bundled_runtime"
        $providerPath = $bundledProvider
        $installerManagedRequired = $false
        $requiredAction = "use bundled MariaDB provider"
    } elseif (-not [string]::IsNullOrWhiteSpace($bundleEmbeddedArchive)) {
        $providerMode = "bundle_embedded"
        $providerPath = $bundleEmbeddedArchive
        $requiredAction = "auto-stage embedded MariaDB provider on first run"
        $installerManagedRequired = $false
    } elseif (-not [string]::IsNullOrWhiteSpace($systemProvider)) {
        $providerMode = "system_command"
        $providerPath = $systemProvider
        $installerManagedRequired = $false
        $requiredAction = "use detected MariaDB server command for validation; packaged normal path should still prefer bundled runtime"
    } elseif ($providerArchivePresent) {
        $providerMode = "installer_bundle_available"
        $providerPath = Resolve-ExistingPathOrRaw $ProviderArchive
        $requiredAction = "run -StageMariaDBProvider with this archive into a non-source install directory"
    } else {
        Add-ListItem $warnings "mariadb_provider_bundle_required"
    }

    $chromaRuntime = Find-ChromaDBRuntime $effectiveInstallDir
    $chromaRuntimePresent = -not [string]::IsNullOrWhiteSpace($chromaRuntime)
    if (-not $chromaRuntimePresent) {
        Add-ListItem $warnings "chromadb_runtime_bundle_required"
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
        $fallbackProfile = "windows_full_package_runtime_required"
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
            required_action = if ($chromaRuntimePresent) { "use bundled ChromaDB runtime" } else { "installer must stage a bundled ChromaDB/Python runtime under runtime/ChromaDB or runtime/Python" }
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
        if ([string]::IsNullOrWhiteSpace($mariadbProvider)) {
            Add-ListItem $failures "mariadb_bundled_provider_missing"
        }
        if ([string]::IsNullOrWhiteSpace($chromaRuntime)) {
            Add-ListItem $failures "chromadb_runtime_missing"
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
            normal_user_manual_mariadb_required = $false
            normal_user_manual_chromadb_required = $false
            authority_switch = $false
            go_default_switch = $false
            components = [ordered]@{
                go_backend_binary_present = -not [string]::IsNullOrWhiteSpace($goBinary)
                go_backend_binary_path = $goBinary
                mariadb_provider_present = -not [string]::IsNullOrWhiteSpace($mariadbProvider)
                mariadb_provider_path = $mariadbProvider
                chromadb_runtime_present = -not [string]::IsNullOrWhiteSpace($chromaRuntime)
                chromadb_runtime_path = $chromaRuntime
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

if (-not $Preflight -and -not $StageMariaDBProvider -and -not $VerifyBundle) {
    throw "Use -Preflight, -StageMariaDBProvider, or -VerifyBundle"
}

if ($Preflight) {
    Write-JsonReport (Invoke-Preflight) $Out
    exit 0
}

if ($VerifyBundle) {
    Write-JsonReport (Invoke-VerifyBundle) $Out
    exit 0
}

Write-JsonReport (Invoke-StageMariaDBProvider) $Out
exit 0
