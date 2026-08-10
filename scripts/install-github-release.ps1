param(
    [string]$Repo = "Flazer31/archive-center",
    [string]$InstallDir = "",
    [Nullable[int]]$ExternalOperationTimeoutSeconds = $null,
    [switch]$Start
)

$ErrorActionPreference = "Stop"

if ($null -eq $ExternalOperationTimeoutSeconds -and -not [string]::IsNullOrWhiteSpace($env:AC_EXTERNAL_OPERATION_TIMEOUT_SECONDS)) {
    $parsedExternalOperationTimeoutSeconds = 0
    if (-not [int]::TryParse($env:AC_EXTERNAL_OPERATION_TIMEOUT_SECONDS, [ref]$parsedExternalOperationTimeoutSeconds) -or
        $parsedExternalOperationTimeoutSeconds -lt 1) {
        throw "AC_EXTERNAL_OPERATION_TIMEOUT_SECONDS must be a positive integer."
    }
    $ExternalOperationTimeoutSeconds = $parsedExternalOperationTimeoutSeconds
}
if ($null -eq $ExternalOperationTimeoutSeconds -or $ExternalOperationTimeoutSeconds -lt 1) {
    throw "Supply -ExternalOperationTimeoutSeconds or AC_EXTERNAL_OPERATION_TIMEOUT_SECONDS. Archive Center does not invent a hidden download deadline."
}

if ([string]::IsNullOrWhiteSpace($InstallDir)) {
    $InstallDir = Join-Path $env:LOCALAPPDATA "ArchiveCenter"
}

function Get-ArchiveCenterPlatform {
    if ([System.Runtime.InteropServices.RuntimeInformation]::IsOSPlatform([System.Runtime.InteropServices.OSPlatform]::OSX)) {
        if ([System.Runtime.InteropServices.RuntimeInformation]::ProcessArchitecture -eq [System.Runtime.InteropServices.Architecture]::Arm64) {
            return "macos-apple-silicon"
        }
        return "macos-intel"
    }
    if ([System.Runtime.InteropServices.RuntimeInformation]::IsOSPlatform([System.Runtime.InteropServices.OSPlatform]::Linux)) {
        if ([System.Runtime.InteropServices.RuntimeInformation]::ProcessArchitecture -eq [System.Runtime.InteropServices.Architecture]::Arm64) {
            return "linux-arm64"
        }
        return "linux-x64"
    }
    return "windows-x64"
}

function Get-AssetNeedle([string]$Platform) {
    switch ($Platform) {
        "windows-x64" { return "windows auto install" }
        "linux-x64" { return "linux x64" }
        "linux-arm64" { return "linux arm64" }
        "macos-intel" { return "macos intel" }
        "macos-apple-silicon" { return "macos apple silicon" }
        default { throw "Unsupported platform: $Platform" }
    }
}

function ConvertTo-ComparableAssetName([string]$Value) {
    $normalized = ($Value.ToLowerInvariant() -replace '[^a-z0-9]+', ' ').Trim()
    return (($normalized -split '\s+') -join ' ')
}

function Find-ReleaseAsset($Release, [string]$Needle) {
    $needleComparable = ConvertTo-ComparableAssetName $Needle
    foreach ($asset in $Release.assets) {
        $name = [string]$asset.name
        $comparable = ConvertTo-ComparableAssetName $name
        if ($name.ToLowerInvariant().EndsWith(".zip") -and $comparable.Contains($needleComparable) -and $comparable.Contains("archive center")) {
            return $asset
        }
    }
    return $null
}

function Test-LocalPortOpen([int]$Port, [string]$ConnectHost = "127.0.0.1") {
    try {
        $addresses = [System.Net.Dns]::GetHostAddresses($ConnectHost)
    } catch {
        return $false
    }
    foreach ($address in $addresses) {
        $client = [System.Net.Sockets.TcpClient]::new($address.AddressFamily)
        try {
            $client.Connect($address, $Port)
            if ($client.Connected) {
                return $true
            }
        } catch {
            continue
        } finally {
            $client.Close()
        }
    }
    return $false
}

function Get-DataFileMap([string]$Root) {
    $rootFull = [System.IO.Path]::GetFullPath($Root).TrimEnd('\')
    $result = @{}
    if (-not (Test-Path -LiteralPath $rootFull -PathType Container)) {
        return $result
    }
    foreach ($file in @(Get-ChildItem -LiteralPath $rootFull -Recurse -File -Force | Sort-Object FullName)) {
        $relative = $file.FullName.Substring($rootFull.Length).TrimStart('\').Replace('\', '/')
        $result[$relative] = "$($file.Length):$((Get-FileHash -LiteralPath $file.FullName -Algorithm SHA256).Hash.ToLowerInvariant())"
    }
    return $result
}

function Copy-LegacyDataVerified([string]$Source, [string]$Destination) {
    $destinationFull = [System.IO.Path]::GetFullPath($Destination)
    $destinationParent = Split-Path -Parent $destinationFull
    $destinationLeaf = Split-Path -Leaf $destinationFull
    New-Item -ItemType Directory -Force -Path $destinationParent | Out-Null
    $staging = Join-Path $destinationParent (".$destinationLeaf.import-" + [Guid]::NewGuid().ToString("N"))
    New-Item -ItemType Directory -Path $staging | Out-Null
    $sourceFull = [System.IO.Path]::GetFullPath($Source).TrimEnd('\')
    try {
        foreach ($sourceFile in @(Get-ChildItem -LiteralPath $sourceFull -Recurse -File -Force | Sort-Object FullName)) {
            $relative = $sourceFile.FullName.Substring($sourceFull.Length).TrimStart('\')
            $stagedFile = Join-Path $staging $relative
            New-Item -ItemType Directory -Force -Path (Split-Path -Parent $stagedFile) | Out-Null
            Copy-Item -LiteralPath $sourceFile.FullName -Destination $stagedFile
        }
        $sourceMap = Get-DataFileMap $Source
        $stagingMap = Get-DataFileMap $staging
        if ($sourceMap.Count -ne $stagingMap.Count) {
            throw "Persistent data import staging count mismatch for $Destination"
        }
        foreach ($relative in $sourceMap.Keys) {
            if (-not $stagingMap.ContainsKey($relative) -or $sourceMap[$relative] -cne $stagingMap[$relative]) {
                throw "Persistent data import staging mismatch: $relative"
            }
        }
        if (Test-Path -LiteralPath $destinationFull) {
            $destinationMap = Get-DataFileMap $destinationFull
            if ($sourceMap.Count -ne $destinationMap.Count) {
                throw "Persistent data import found a conflicting destination directory: $Destination"
            }
            foreach ($relative in $sourceMap.Keys) {
                if (-not $destinationMap.ContainsKey($relative) -or $sourceMap[$relative] -cne $destinationMap[$relative]) {
                    throw "Persistent data import found a conflicting destination file: $relative"
                }
            }
            return
        }
        Move-Item -LiteralPath $staging -Destination $destinationFull
        $staging = ""
        $destinationMap = Get-DataFileMap $destinationFull
        if ($sourceMap.Count -ne $destinationMap.Count) {
            throw "Persistent data import verification count mismatch for $Destination"
        }
        foreach ($relative in $sourceMap.Keys) {
            if (-not $destinationMap.ContainsKey($relative) -or $sourceMap[$relative] -cne $destinationMap[$relative]) {
                throw "Persistent data import verification mismatch: $relative"
            }
        }
    } finally {
        if (-not [string]::IsNullOrWhiteSpace($staging) -and (Test-Path -LiteralPath $staging)) {
            Remove-Item -LiteralPath $staging -Recurse -Force
        }
    }
}

function Get-PreviousPackageChromaTargets([int]$DefaultPort = 8000) {
    $targets = [ordered]@{}
    foreach ($defaultHost in @("127.0.0.1", "::1")) {
        $targets["$defaultHost`:$DefaultPort"] = [pscustomobject]@{
            Host = $defaultHost
            Port = $DefaultPort
        }
    }
    $endpointText = [string]$env:AC_CHROMA_ENDPOINT
    if (-not [string]::IsNullOrWhiteSpace($endpointText)) {
        $endpoint = $null
        if ([Uri]::TryCreate($endpointText.Trim(), [UriKind]::Absolute, [ref]$endpoint)) {
            $endpointHost = $endpoint.DnsSafeHost
            $endpointAddress = $null
            $isLoopback = $endpointHost.Equals("localhost", [System.StringComparison]::OrdinalIgnoreCase)
            if ([System.Net.IPAddress]::TryParse($endpointHost, [ref]$endpointAddress)) {
                $isLoopback = [System.Net.IPAddress]::IsLoopback($endpointAddress)
            }
        }
        if ($null -ne $endpoint -and $isLoopback) {
            $endpointPort = $(if ($endpoint.IsDefaultPort) { 8000 } else { $endpoint.Port })
            $targets["$($endpointHost.ToLowerInvariant())`:$endpointPort"] = [pscustomobject]@{
                Host = $endpointHost
                Port = $endpointPort
            }
        }
    }
    @($targets.Values | Where-Object { $_.Port -gt 0 -and $_.Port -le 65535 })
}

function Assert-PreviousPackageChromaOffline([string]$Source, [int]$DefaultPort = 8000) {
    foreach ($target in @(Get-PreviousPackageChromaTargets -DefaultPort $DefaultPort)) {
        if (Test-LocalPortOpen -Port $target.Port -ConnectHost $target.Host) {
            $displayHost = $(if ($target.Host.Contains(":")) { "[$($target.Host)]" } else { $target.Host })
            throw "Legacy ChromaDB data may be active at $displayHost`:$($target.Port). Stop ChromaDB before importing raw data files from $Source."
        }
    }
}

function Import-PreviousPackageRuntimeOnce([string]$PreviousPackageRoot, [string]$DataRoot, [int]$MariaDBPort = 3307, [int]$ChromaPort = 8000) {
    if ([string]::IsNullOrWhiteSpace($PreviousPackageRoot)) {
        return
    }
    $legacyRoot = Join-Path $PreviousPackageRoot ".runtime"
    if (-not (Test-Path -LiteralPath $legacyRoot -PathType Container)) {
        return
    }
    $dataRootFull = [System.IO.Path]::GetFullPath($DataRoot)
    $markerPath = Join-Path $dataRootFull ".archive-center-legacy-runtime-import-v1.json"
    if (Test-Path -LiteralPath $markerPath -PathType Leaf) {
        try {
            $marker = Get-Content -LiteralPath $markerPath -Raw -Encoding UTF8 | ConvertFrom-Json -ErrorAction Stop
        } catch {
            throw "Persistent data import marker is unreadable: $markerPath"
        }
        if ([string]$marker.contract_version -cne "archive-center.legacy-runtime-import.v1" -or
            ([string]$marker.data_root).Trim() -cne $dataRootFull) {
            throw "Persistent data import marker contract/path mismatch."
        }
        return
    }

    $mariaCandidates = @(
        (Join-Path $legacyRoot "mariadb"),
        (Join-Path $legacyRoot "mariadb-data")
    ) | Where-Object { Test-Path -LiteralPath $_ -PathType Container }
    if (@($mariaCandidates).Count -gt 1) {
        throw "Both legacy MariaDB data layouts exist; refusing an ambiguous automatic import."
    }
    $imports = @()
    if (@($mariaCandidates).Count -eq 1) {
        $imports += [pscustomobject]@{ Name = "mariadb"; Source = [string]$mariaCandidates[0]; Destination = (Join-Path $dataRootFull "mariadb") }
    }
    $legacyChroma = Join-Path $legacyRoot "chromadb"
    if (Test-Path -LiteralPath $legacyChroma -PathType Container) {
        $imports += [pscustomobject]@{ Name = "chromadb"; Source = $legacyChroma; Destination = (Join-Path $dataRootFull "chromadb") }
    }
    if ($imports.Count -eq 0) {
        return
    }

    New-Item -ItemType Directory -Force -Path $dataRootFull | Out-Null
    $completed = @()
    foreach ($item in $imports) {
        if ($item.Name -eq "mariadb") {
            $mariaProcesses = @(Get-Process -Name "mariadbd", "mysqld" -ErrorAction SilentlyContinue)
            if ((Test-LocalPortOpen $MariaDBPort) -or $mariaProcesses.Count -gt 0) {
                throw "Legacy MariaDB data may be active. Stop MariaDB and ensure port $MariaDBPort is closed before importing raw data files."
            }
        } elseif ($item.Name -eq "chromadb") {
            Assert-PreviousPackageChromaOffline -Source $item.Source -DefaultPort $ChromaPort
        }
        Copy-LegacyDataVerified -Source $item.Source -Destination $item.Destination
        $completed += [ordered]@{
            name = $item.Name
            source = [System.IO.Path]::GetFullPath($item.Source)
            destination = [System.IO.Path]::GetFullPath($item.Destination)
            file_count = (Get-DataFileMap $item.Source).Count
        }
    }
    $marker = [ordered]@{
        contract_version = "archive-center.legacy-runtime-import.v1"
        data_root = $dataRootFull
        source_preserved = $true
        verified = $true
        completed_at = [DateTimeOffset]::UtcNow.ToString("o")
        imports = @($completed)
    }
    $temporaryPath = "$markerPath.tmp"
    [System.IO.File]::WriteAllText(
        $temporaryPath,
        ($marker | ConvertTo-Json -Depth 6) + [Environment]::NewLine,
        (New-Object System.Text.UTF8Encoding($false))
    )
    Move-Item -LiteralPath $temporaryPath -Destination $markerPath -Force
    foreach ($volatile in @("mariadb\mysql.sock", "mariadb\mariadb.pid", "mariadb\mysqld.pid")) {
        Remove-Item -LiteralPath (Join-Path $dataRootFull $volatile) -Force -ErrorAction SilentlyContinue
    }
    Write-Host "Legacy runtime data was copied and verified. The source package remains unchanged."
}

if ($Repo -notmatch '^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$') {
    throw "Repo must be OWNER/REPO."
}

$platform = Get-ArchiveCenterPlatform
$needle = Get-AssetNeedle $platform
$apiUrl = "https://api.github.com/repos/$Repo/releases/latest"
$headers = @{ "Accept" = "application/vnd.github+json"; "User-Agent" = "Archive-Center-Installer" }
$currentPackagePointer = Join-Path $InstallDir "current-package.txt"
$previousPackageRoot = ""
if (Test-Path -LiteralPath $currentPackagePointer -PathType Leaf) {
    $previousPackageRoot = ((Get-Content -LiteralPath $currentPackagePointer -Raw) -as [string]).Trim()
}
$persistentDataDir = if ([string]::IsNullOrWhiteSpace($env:ARCHIVE_CENTER_DATA_DIR)) {
    Join-Path $InstallDir "data"
} else {
    $env:ARCHIVE_CENTER_DATA_DIR
}
$release = Invoke-RestMethod -Method Get -Uri $apiUrl -Headers $headers -TimeoutSec $ExternalOperationTimeoutSeconds
$asset = Find-ReleaseAsset $release $needle
if ($null -eq $asset) {
    throw "No release package asset matched platform $platform."
}
$workDir = Join-Path ([System.IO.Path]::GetTempPath()) ("archive-center-update-" + [guid]::NewGuid().ToString("N"))
New-Item -ItemType Directory -Force -Path $workDir | Out-Null
try {
    $zipPath = Join-Path $workDir ([string]$asset.name)
    Invoke-WebRequest -Uri ([string]$asset.browser_download_url) -Headers $headers -OutFile $zipPath -TimeoutSec $ExternalOperationTimeoutSeconds

    $versionDirName = ([string]$release.tag_name) -replace '[^A-Za-z0-9_.-]', '_'
    if ([string]::IsNullOrWhiteSpace($versionDirName)) {
        $versionDirName = "latest"
    }
    $targetDir = Join-Path (Join-Path $InstallDir "releases") $versionDirName
    New-Item -ItemType Directory -Force -Path $targetDir | Out-Null
    Expand-Archive -LiteralPath $zipPath -DestinationPath $targetDir -Force

    $packageRoot = Get-ChildItem -LiteralPath $targetDir -Recurse -File |
        Where-Object { $_.Name -in @("01_start_archive_center_windows.bat", "start-archive-center-linux.sh", "Start Archive Center macOS.command") } |
        Sort-Object FullName |
        Select-Object -First 1 |
        ForEach-Object { Split-Path -Parent $_.FullName }
    if ([string]::IsNullOrWhiteSpace($packageRoot)) {
        throw "Extracted package launcher was not found."
    }

    New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
    $persistentDataDir = [System.IO.Path]::GetFullPath($persistentDataDir)
    $env:ARCHIVE_CENTER_DATA_DIR = $persistentDataDir
    New-Item -ItemType Directory -Force -Path $persistentDataDir | Out-Null
    Import-PreviousPackageRuntimeOnce -PreviousPackageRoot $previousPackageRoot -DataRoot $persistentDataDir

    Set-Content -LiteralPath $currentPackagePointer -Value $packageRoot -Encoding UTF8
    Set-Content -LiteralPath (Join-Path $InstallDir "current-version.txt") -Value ([string]$release.tag_name) -Encoding UTF8

    Write-Host "Installed Archive Center $($release.tag_name)"
    Write-Host "  Platform: $platform"
    Write-Host "  Package:  $packageRoot"
    Write-Host "  Data:     $persistentDataDir"
    Write-Host "  Pointer:  $(Join-Path $InstallDir "current-package.txt")"

    if ($Start) {
        if ($platform -eq "windows-x64") {
            $launcher = Join-Path $packageRoot "01_start_archive_center_windows.bat"
            Start-Process -FilePath $launcher -WorkingDirectory $packageRoot
        } elseif ($platform.StartsWith("linux-")) {
            & sh (Join-Path $packageRoot "start-archive-center-linux.sh")
        } elseif ($platform.StartsWith("macos-")) {
            & sh (Join-Path $packageRoot "scripts/start-full-macos.sh")
        }
    }
} finally {
    Remove-Item -LiteralPath $workDir -Recurse -Force -ErrorAction SilentlyContinue
}
