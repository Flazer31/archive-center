param(
    [string]$OutputRoot,
    [string[]]$TargetFilter = @(),
    [string]$PackageVersion = "4.3.0",
    [switch]$Zip,
    [switch]$ForceRefresh
)

$ErrorActionPreference = "Stop"

function Resolve-FullPath([string]$Path) {
    $executionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath($Path)
}

function Test-PathInside([string]$Child, [string]$Parent) {
    if ([string]::IsNullOrWhiteSpace($Child) -or [string]::IsNullOrWhiteSpace($Parent)) {
        return $false
    }
    $childFull = [System.IO.Path]::GetFullPath($Child).TrimEnd('\') + '\'
    $parentFull = [System.IO.Path]::GetFullPath($Parent).TrimEnd('\') + '\'
    return $childFull.StartsWith($parentFull, [System.StringComparison]::OrdinalIgnoreCase)
}

function Copy-File([string]$Source, [string]$Destination) {
    if (-not (Test-Path -LiteralPath $Source -PathType Leaf)) {
        throw "Missing source file: $Source"
    }
    New-Item -ItemType Directory -Force -Path (Split-Path -Parent $Destination) | Out-Null
    Copy-Item -LiteralPath $Source -Destination $Destination -Force
}

function Copy-DirectoryContents([string]$Source, [string]$Destination) {
    if (-not (Test-Path -LiteralPath $Source -PathType Container)) {
        throw "Missing source directory: $Source"
    }
    New-Item -ItemType Directory -Force -Path $Destination | Out-Null
    Get-ChildItem -LiteralPath $Source -Force | ForEach-Object {
        Copy-Item -LiteralPath $_.FullName -Destination $Destination -Recurse -Force
    }
}

function Write-TextFile([string]$Path, [string]$Value) {
    New-Item -ItemType Directory -Force -Path (Split-Path -Parent $Path) | Out-Null
    $utf8NoBom = New-Object System.Text.UTF8Encoding($false)
    [System.IO.File]::WriteAllText($Path, $Value, $utf8NoBom)
}

function Normalize-POSIXPackageLineEndings([string]$Root) {
    $utf8NoBom = New-Object System.Text.UTF8Encoding($false)
    foreach ($pattern in @("*.sh", "*.command")) {
        Get-ChildItem -LiteralPath $Root -Filter $pattern -File -Recurse -ErrorAction SilentlyContinue | ForEach-Object {
            $text = [System.IO.File]::ReadAllText($_.FullName, [System.Text.Encoding]::UTF8)
            $normalized = $text.Replace("`r`n", "`n").Replace("`r", "`n")
            [System.IO.File]::WriteAllText($_.FullName, $normalized, $utf8NoBom)
            if ([System.Array]::IndexOf([System.IO.File]::ReadAllBytes($_.FullName), [byte]13) -ge 0) {
                throw "POSIX package text still contains CR after LF normalization: $($_.FullName)"
            }
        }
    }
}

function Set-CopiedPackageVersionText([string]$Root, [string]$PackageVersion) {
    $version = if ([string]::IsNullOrWhiteSpace($PackageVersion)) { "4.3.0" } else { $PackageVersion.Trim() }
    $suffix = "archivecenter" + (($version -replace '\s+', '').ToLowerInvariant())
    $utf8NoBom = New-Object System.Text.UTF8Encoding($false)
    foreach ($pattern in @("*.md", "*.txt", "*.sh", "*.command")) {
        Get-ChildItem -LiteralPath $Root -Filter $pattern -File -Recurse -ErrorAction SilentlyContinue | ForEach-Object {
            $text = [System.IO.File]::ReadAllText($_.FullName, [System.Text.Encoding]::UTF8)
            $next = $text.Replace("Archive Center 2.1", "Archive Center $version")
            $next = $next.Replace("archivecenter2.1", $suffix)
            $next = $next.Replace("__ARCHIVE_CENTER_PACKAGE_VERSION__", $version)
            if ($next -ne $text) {
                [System.IO.File]::WriteAllText($_.FullName, $next, $utf8NoBom)
            }
        }
    }
    $pluginPath = Join-Path $Root "Archive Center.js"
    if (Test-Path -LiteralPath $pluginPath -PathType Leaf) {
        $text = [System.IO.File]::ReadAllText($pluginPath, [System.Text.Encoding]::UTF8)
        $next = [regex]::Replace($text, '(?m)^//@display-name Archive Center .+$', "//@display-name Archive Center $version")
        $next = [regex]::Replace($next, '(?m)^//@version .+$', "//@version $version")
        $next = [regex]::Replace($next, '(?m)^(\s*const VERSION = )"[^"]+";', ('$1"' + $version + '";'))
        if ($next -ne $text) {
            [System.IO.File]::WriteAllText($pluginPath, $next, $utf8NoBom)
        }
    }
}

function Get-RelativePackagePath([string]$Path, [string]$Root) {
    $rootFull = [System.IO.Path]::GetFullPath($Root).TrimEnd('\', '/')
    $pathFull = [System.IO.Path]::GetFullPath($Path)
    return $pathFull.Substring($rootFull.Length).TrimStart([char[]]@('\', '/')).Replace('\', '/')
}

function Write-ManagedPackageManifest([string]$Root, [string]$PackageVersion) {
    $selfFiles = @("PACKAGE_FILE_MANIFEST.json", "SHA256SUMS.txt")
    $excludedRoots = @(".runtime", ".runtime-cache", ".updates", "runtime")
    $excludedLocalFiles = @(".env.full.local", ".env.full.local.protected")
    $files = Get-ChildItem -LiteralPath $Root -Recurse -File -Force -ErrorAction SilentlyContinue |
        Where-Object {
            $rel = Get-RelativePackagePath $_.FullName $Root
            $top = ($rel -split '/', 2)[0]
            ($selfFiles -notcontains $rel) -and
                ($excludedRoots -notcontains $top) -and
                ($excludedLocalFiles -notcontains $rel)
        } |
        Sort-Object FullName
    $items = @()
    $sumLines = @()
    foreach ($file in $files) {
        $rel = Get-RelativePackagePath $file.FullName $Root
        $sha = (Get-FileHash -LiteralPath $file.FullName -Algorithm SHA256).Hash.ToLowerInvariant()
        $items += [ordered]@{ path = $rel; size_bytes = [int64]$file.Length; sha256 = $sha }
        $sumLines += "$sha  $rel"
    }
    $manifest = [ordered]@{
        schema_version = "archive-center.package-file-manifest.v1"
        package_version = $PackageVersion.Trim()
        generated_at = [DateTimeOffset]::UtcNow.ToString("o")
        scope = "managed_package_payloads"
        excluded_runtime_roots = $excludedRoots
        excluded_local_files = $excludedLocalFiles
        files = $items
    }
    [System.IO.File]::WriteAllText(
        (Join-Path $Root "PACKAGE_FILE_MANIFEST.json"),
        ($manifest | ConvertTo-Json -Depth 6) + [Environment]::NewLine,
        (New-Object System.Text.UTF8Encoding($false))
    )
    [System.IO.File]::WriteAllLines(
        (Join-Path $Root "SHA256SUMS.txt"),
        $sumLines,
        (New-Object System.Text.UTF8Encoding($false))
    )
}

function Write-PackageMigrationUpdateManifest([string]$Root, [string]$TargetVersion) {
    if ([string]::IsNullOrWhiteSpace($TargetVersion)) {
        throw "PackageVersion is required for the migration update contract."
    }
    $target = @()
    $migrationFiles = @(Get-ChildItem -LiteralPath (Join-Path $Root "migrations") -File -Filter "*.sql" | Sort-Object Name)
    $expectedRevision = 1
    foreach ($file in $migrationFiles) {
        if ($file.Name -notmatch '^(\d{3})_.+\.sql$' -or [int]$Matches[1] -ne $expectedRevision) {
            throw ("Cumulative migration inventory must contain every sequential revision from 001; expected {0:D3}, found {1}." -f $expectedRevision, $file.Name)
        }
        $expectedRevision++
        $target += [ordered]@{
            path = "migrations/$($file.Name)"
            size_bytes = [int64]$file.Length
            sha256 = (Get-FileHash -LiteralPath $file.FullName -Algorithm SHA256).Hash.ToLowerInvariant()
        }
    }
    if ($target.Count -eq 0) {
        throw "The package migration inventory is empty."
    }
    $schemaTool = Get-Item -LiteralPath (Join-Path $Root "bin\mariadb-schema")
    $target += [ordered]@{
        path = "bin/mariadb-schema"
        size_bytes = [int64]$schemaTool.Length
        sha256 = (Get-FileHash -LiteralPath $schemaTool.FullName -Algorithm SHA256).Hash.ToLowerInvariant()
    }
    $contract = [ordered]@{
        contract_version = "archive-center.package-migration-update.v2"
        target_version = $TargetVersion.Trim()
        target = @($target)
        managed_files = "complete_manifest"
        database_policy = "expand_first_old_backend_compatible"
        minimum_source_version = "3.9.9"
        direct_update_supported = $true
        migration_inventory = "cumulative_complete"
    }
    [System.IO.File]::WriteAllText(
        (Join-Path $Root "PACKAGE_MIGRATION_UPDATE.json"),
        ($contract | ConvertTo-Json -Depth 8) + [Environment]::NewLine,
        (New-Object System.Text.UTF8Encoding($false))
    )
}

function Write-PackageReleaseStatus([string]$Root, [string]$TargetVersion, [bool]$ReleaseReady) {
    $status = [ordered]@{
        contract_version = "archive-center.package-release-status.v1"
        target_version = $TargetVersion.Trim()
        release_ready = $ReleaseReady
        automatic_update_apply = $true
    }
    [System.IO.File]::WriteAllText(
        (Join-Path $Root "PACKAGE_RELEASE_STATUS.json"),
        ($status | ConvertTo-Json -Depth 4) + [Environment]::NewLine,
        (New-Object System.Text.UTF8Encoding($false))
    )
}

function Set-RuntimeDefaultsInEnvExample([string]$Path, [string]$RuntimeProfile, [string]$VectorMode, [string]$PackageVersion) {
    if (-not (Test-Path -LiteralPath $Path -PathType Leaf)) {
        throw "Missing env example file: $Path"
    }
    $text = Get-Content -LiteralPath $Path -Raw
    $text = [regex]::Replace($text, '(?m)^AC_RUNTIME_PROFILE=.*$', "AC_RUNTIME_PROFILE=$RuntimeProfile")
    $text = [regex]::Replace($text, '(?m)^AC_VECTOR_MODE=.*$', "AC_VECTOR_MODE=$VectorMode")
    $text = [regex]::Replace($text, '(?m)^AC_BUILD_VERSION=.*$', "AC_BUILD_VERSION=$PackageVersion")
    $utf8NoBom = New-Object System.Text.UTF8Encoding($false)
    [System.IO.File]::WriteAllText($Path, $text, $utf8NoBom)
}

function Compress-DirectoryPortable([string]$SourceDir, [string]$DestinationZip) {
    Add-Type -AssemblyName System.IO.Compression
    Add-Type -AssemblyName System.IO.Compression.FileSystem

    $sourceFull = [System.IO.Path]::GetFullPath($SourceDir).TrimEnd('\', '/')
    New-Item -ItemType Directory -Force -Path (Split-Path -Parent $DestinationZip) | Out-Null
    if (Test-Path -LiteralPath $DestinationZip -PathType Leaf) {
        Remove-Item -LiteralPath $DestinationZip -Force
    }

    $zipStream = [System.IO.File]::Open($DestinationZip, [System.IO.FileMode]::CreateNew)
    try {
        $archive = New-Object System.IO.Compression.ZipArchive($zipStream, [System.IO.Compression.ZipArchiveMode]::Create)
        try {
            Get-ChildItem -LiteralPath $SourceDir -Recurse -File -Force | ForEach-Object {
                $relative = $_.FullName.Substring($sourceFull.Length).TrimStart([char[]]@('\', '/'))
                $entryName = $relative.Replace('\', '/')
                $entry = $archive.CreateEntry($entryName, [System.IO.Compression.CompressionLevel]::Optimal)
                $isExecutable = $entryName.StartsWith("bin/", [System.StringComparison]::Ordinal) -or
                    $entryName.EndsWith(".sh", [System.StringComparison]::OrdinalIgnoreCase) -or
                    $entryName.EndsWith(".command", [System.StringComparison]::OrdinalIgnoreCase)
                $unixMode = if ($isExecutable) { 0x81ED } else { 0x81A4 }
                $entry.ExternalAttributes = $unixMode -shl 16
                $inputStream = [System.IO.File]::OpenRead($_.FullName)
                try {
                    $entryStream = $entry.Open()
                    try {
                        $inputStream.CopyTo($entryStream)
                    } finally {
                        $entryStream.Dispose()
                    }
                } finally {
                    $inputStream.Dispose()
                }
            }
        } finally {
            $archive.Dispose()
        }
    } finally {
        $zipStream.Dispose()
    }

    # ZipArchive writes archives as DOS-hosted on Windows. POSIX extractors then
    # ignore the Unix mode stored in ExternalAttributes. Rewrite only the
    # central-directory host byte so those already-recorded 0755/0644 modes are
    # interpreted as Unix permissions after extraction.
    $zipBytes = [System.IO.File]::ReadAllBytes($DestinationZip)
    $eocdOffset = -1
    $searchStart = [Math]::Max(0, $zipBytes.Length - 65557)
    for ($offset = $zipBytes.Length - 22; $offset -ge $searchStart; $offset--) {
        if ($zipBytes[$offset] -eq 0x50 -and $zipBytes[$offset + 1] -eq 0x4b -and
            $zipBytes[$offset + 2] -eq 0x05 -and $zipBytes[$offset + 3] -eq 0x06) {
            $eocdOffset = $offset
            break
        }
    }
    if ($eocdOffset -lt 0) {
        throw "Generated ZIP has no end-of-central-directory record: $DestinationZip"
    }
    $entryCount = [System.BitConverter]::ToUInt16($zipBytes, $eocdOffset + 10)
    $centralOffset = [int][System.BitConverter]::ToUInt32($zipBytes, $eocdOffset + 16)
    $cursor = $centralOffset
    for ($entryIndex = 0; $entryIndex -lt $entryCount; $entryIndex++) {
        if ($cursor + 46 -gt $zipBytes.Length -or
            $zipBytes[$cursor] -ne 0x50 -or $zipBytes[$cursor + 1] -ne 0x4b -or
            $zipBytes[$cursor + 2] -ne 0x01 -or $zipBytes[$cursor + 3] -ne 0x02) {
            throw "Generated ZIP central directory is invalid at entry $entryIndex`: $DestinationZip"
        }
        $zipBytes[$cursor + 5] = 3
        $nameLength = [System.BitConverter]::ToUInt16($zipBytes, $cursor + 28)
        $extraLength = [System.BitConverter]::ToUInt16($zipBytes, $cursor + 30)
        $commentLength = [System.BitConverter]::ToUInt16($zipBytes, $cursor + 32)
        $cursor += 46 + $nameLength + $extraLength + $commentLength
    }
    [System.IO.File]::WriteAllBytes($DestinationZip, $zipBytes)
}

function Build-GoBinary([string]$GoServiceRoot, [string]$Goos, [string]$Goarch, [string]$Package, [string]$Output) {
    Push-Location $GoServiceRoot
    try {
        $oldGoos = $env:GOOS
        $oldGoarch = $env:GOARCH
        $oldCgo = $env:CGO_ENABLED
        $env:GOOS = $Goos
        $env:GOARCH = $Goarch
        $env:CGO_ENABLED = "0"
        & go build -buildvcs=false -trimpath -ldflags "-s -w" -o $Output $Package
        if ($LASTEXITCODE -ne 0) {
            throw "go build failed for $Package ($Goos/$Goarch)"
        }
    } finally {
        $env:GOOS = $oldGoos
        $env:GOARCH = $oldGoarch
        $env:CGO_ENABLED = $oldCgo
        Pop-Location
    }
}

$repoRoot = Resolve-FullPath (Join-Path $PSScriptRoot "..")
if ([string]::IsNullOrWhiteSpace($OutputRoot)) {
    $OutputRoot = Join-Path $repoRoot "_dist"
}
$outputRootFull = Resolve-FullPath $OutputRoot
if (-not (Test-PathInside $outputRootFull $repoRoot)) {
    throw "Refusing to write outside the Archive Center workspace: $outputRootFull"
}

$goServiceRoot = Join-Path $repoRoot "go-service"
$targets = @(
    [ordered]@{
        PackageName = "Archive Center 2.1 Linux x64 Auto Install Package"
        Target = "linux-amd64"
        Goos = "linux"
        Goarch = "amd64"
        PackageKind = "full"
        PackageProfile = "managed_full_local_candidate"
        Status = "full_package_candidate_runtime_unverified"
        Launcher = "start-archive-center-linux.sh"
        Script = "start-full-linux.sh"
        InstallScript = "install-linux.sh"
        RuntimeProfileDefault = "full_local"
        VectorModeDefault = "local_native"
        RuntimeMode = "installer_managed_mariadb_full_local_chromadb"
    },
    [ordered]@{
        PackageName = "Archive Center 2.1 Linux arm64 Auto Install Package"
        Target = "linux-arm64"
        Goos = "linux"
        Goarch = "arm64"
        PackageKind = "full"
        PackageProfile = "managed_full_local_candidate"
        Status = "full_package_candidate_runtime_unverified"
        Launcher = "start-archive-center-linux.sh"
        Script = "start-full-linux.sh"
        InstallScript = "install-linux.sh"
        RuntimeProfileDefault = "full_local"
        VectorModeDefault = "local_native"
        RuntimeMode = "installer_managed_mariadb_full_local_chromadb"
    },
    [ordered]@{
        PackageName = "Archive Center 2.1 macOS Intel Auto Install Package"
        Target = "macos-amd64"
        Goos = "darwin"
        Goarch = "amd64"
        PackageKind = "full"
        PackageProfile = "managed_full_local_candidate"
        Status = "full_package_candidate_runtime_unverified"
        Launcher = "Start Archive Center macOS.command"
        Script = "start-full-macos.sh"
        InstallScript = "install-macos.sh"
        RuntimeProfileDefault = "full_local"
        VectorModeDefault = "local_native"
        RuntimeMode = "homebrew_managed_mariadb_full_local_chromadb"
    },
    [ordered]@{
        PackageName = "Archive Center 2.1 macOS Apple Silicon Auto Install Package"
        Target = "macos-arm64"
        Goos = "darwin"
        Goarch = "arm64"
        PackageKind = "full"
        PackageProfile = "managed_full_local_candidate"
        Status = "full_package_candidate_runtime_unverified"
        Launcher = "Start Archive Center macOS.command"
        Script = "start-full-macos.sh"
        InstallScript = "install-macos.sh"
        RuntimeProfileDefault = "full_local"
        VectorModeDefault = "local_native"
        RuntimeMode = "homebrew_managed_mariadb_full_local_chromadb"
    },
    [ordered]@{
        PackageName = "Archive Center 2.1 Termux arm64 Auto Install Package"
        Target = "termux-arm64"
        Goos = "android"
        Goarch = "arm64"
        PackageKind = "full"
        PackageProfile = "auto_install_full_local_candidate"
        Status = "full_auto_install_candidate_runtime_unverified"
        Launcher = "install-and-start-termux.sh"
        Script = "install-and-start-termux.sh"
        InstallScript = "install-termux.sh"
        RuntimeProfileDefault = "full_local"
        VectorModeDefault = "local_proot"
        RuntimeMode = "termux_pkg_managed_mariadb_full_local_proot_chromadb"
    }
)

$packageVersionLabel = if ([string]::IsNullOrWhiteSpace($PackageVersion)) { "4.3.0" } else { $PackageVersion.Trim() }
foreach ($target in $targets) {
    $target.PackageName = ([string]$target.PackageName).Replace("Archive Center 2.1", "Archive Center $packageVersionLabel")
}

# Current packaging has one standard package line. Runtime profiles such as
# core_lite remain available inside it, but separate Lite ZIPs are no longer
# built.
$targets = @($targets | Where-Object { ([string]$_.PackageKind).ToLowerInvariant() -eq "full" })

if ($TargetFilter.Count -gt 0) {
    $wanted = @{}
    foreach ($item in $TargetFilter) {
        $value = [string]$item
        if (-not [string]::IsNullOrWhiteSpace($value)) {
            $wanted[$value.Trim().ToLowerInvariant()] = $true
        }
    }
    if ($wanted.Count -gt 0) {
        $targets = @($targets | Where-Object {
            $wanted.ContainsKey(([string]$_.Target).ToLowerInvariant()) -or
            $wanted.ContainsKey(([string]$_.PackageKind).ToLowerInvariant()) -or
            $wanted.ContainsKey(([string]$_.PackageName).ToLowerInvariant())
        })
        if ($targets.Count -eq 0) {
            throw "TargetFilter did not match any POSIX package target."
        }
    }
}

foreach ($target in $targets) {
    $targetRoot = Resolve-FullPath (Join-Path $outputRootFull $target.PackageName)
    if (-not (Test-PathInside $targetRoot $outputRootFull)) {
        throw "Refusing to write package outside output root: $targetRoot"
    }
    if ((Test-Path -LiteralPath $targetRoot) -and -not $ForceRefresh) {
        throw "Target already exists: $targetRoot. Re-run with -ForceRefresh."
    }
    if (Test-Path -LiteralPath $targetRoot) {
        Remove-Item -LiteralPath $targetRoot -Recurse -Force
    }

    New-Item -ItemType Directory -Force -Path (Join-Path $targetRoot "bin") | Out-Null
    Build-GoBinary $goServiceRoot $target.Goos $target.Goarch "./cmd/archive-center-go" (Join-Path $targetRoot "bin\archive-center-go")
    Build-GoBinary $goServiceRoot $target.Goos $target.Goarch "./cmd/archive-center-updater" (Join-Path $targetRoot "bin\archive-center-updater")
    Build-GoBinary $goServiceRoot $target.Goos $target.Goarch "./cmd/mariadb-schema" (Join-Path $targetRoot "bin\mariadb-schema")
    Copy-File (Join-Path $repoRoot "Archive Center.js") (Join-Path $targetRoot "Archive Center.js")
    Copy-File (Join-Path $repoRoot "LICENSE") (Join-Path $targetRoot "LICENSE")
    Copy-File (Join-Path $repoRoot "NOTICE") (Join-Path $targetRoot "NOTICE")
    Copy-File (Join-Path $repoRoot "THIRD_PARTY_NOTICES.md") (Join-Path $targetRoot "THIRD_PARTY_NOTICES.md")
    Copy-DirectoryContents (Join-Path $repoRoot "licenses") (Join-Path $targetRoot "licenses")
    $readmeSource = "ops\full-package-posix\README_POSIX_FULL_PACKAGE.md"
    $readFirstSource = "ops\full-package-posix\00_README_FIRST_POSIX_FULL.md"
    Copy-File (Join-Path $repoRoot $readmeSource) (Join-Path $targetRoot "README.md")
    Copy-File (Join-Path $repoRoot ".env.example") (Join-Path $targetRoot ".env.source.example")
    Copy-File (Join-Path $repoRoot "ops\full-package\.env.full.example") (Join-Path $targetRoot ".env.full.example")
    Set-RuntimeDefaultsInEnvExample (Join-Path $targetRoot ".env.full.example") $target.RuntimeProfileDefault $target.VectorModeDefault $packageVersionLabel
    Copy-DirectoryContents (Join-Path $repoRoot "migrations") (Join-Path $targetRoot "migrations")
    Copy-File (Join-Path $repoRoot "prompts\critic_system.txt") (Join-Path $targetRoot "prompts\critic_system.txt")
    Copy-File (Join-Path $repoRoot "prompts\supervisor_system.txt") (Join-Path $targetRoot "prompts\supervisor_system.txt")
    Copy-DirectoryContents (Join-Path $repoRoot "ops\full-package-posix") (Join-Path $targetRoot "scripts")
    Get-ChildItem -LiteralPath (Join-Path $targetRoot "scripts") -File -ErrorAction SilentlyContinue |
        Where-Object { $_.Name -like "README_POSIX_*PACKAGE.md" -or $_.Name -like "00_README_FIRST_POSIX*.md" } |
        Remove-Item -Force -ErrorAction SilentlyContinue

    New-Item -ItemType Directory -Force -Path (Join-Path $targetRoot "ops") | Out-Null
    Copy-File (Join-Path $repoRoot "ops\platform-proof.sh") (Join-Path $targetRoot "ops\platform-proof.sh")
    Copy-File (Join-Path $repoRoot ("ops\" + $target.InstallScript)) (Join-Path $targetRoot ("ops\" + $target.InstallScript))

    New-Item -ItemType Directory -Force -Path (Join-Path $targetRoot "docs") | Out-Null
    Copy-File (Join-Path $repoRoot "ops\full-package-posix\README_POSIX_FULL_PACKAGE.md") (Join-Path $targetRoot "README_POSIX_FULL_PACKAGE.md")
    Copy-File (Join-Path $repoRoot $readFirstSource) (Join-Path $targetRoot "00_README_FIRST_POSIX.md")
    if (Test-Path -LiteralPath (Join-Path $repoRoot "docs\platform-cross-build-preflight-2026-06-19.md")) {
        Copy-File (Join-Path $repoRoot "docs\platform-cross-build-preflight-2026-06-19.md") (Join-Path $targetRoot "docs\platform-cross-build-preflight-2026-06-19.md")
    }

    $launcherPath = Join-Path $targetRoot $target.Launcher
    $launcherBody = (@(
        '#!/usr/bin/env sh'
        'set -eu'
        'export AC_EXTERNAL_OPERATION_TIMEOUT_SECONDS="${AC_EXTERNAL_OPERATION_TIMEOUT_SECONDS:-1800}"'
        'export AC_REQUEST_TIMEOUT_SECONDS="${AC_REQUEST_TIMEOUT_SECONDS:-30}"'
        'export AC_READINESS_TIMEOUT_SECONDS="${AC_READINESS_TIMEOUT_SECONDS:-180}"'
        'export AC_READINESS_POLL_INTERVAL_SECONDS="${AC_READINESS_POLL_INTERVAL_SECONDS:-1}"'
        'SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd -P)'
        'ARCHIVE_CENTER_PACKAGE_ROOT="$SCRIPT_DIR"'
        'export ARCHIVE_CENTER_PACKAGE_ROOT'
        ('exec sh "$SCRIPT_DIR/scripts/{0}" --profile "{1}" --vector-mode "{2}" "$@"' -f $target.Script, $target.RuntimeProfileDefault, $target.VectorModeDefault)
    ) -join "`n") + "`n"
    Write-TextFile $launcherPath $launcherBody
    Set-CopiedPackageVersionText $targetRoot $packageVersionLabel
    Normalize-POSIXPackageLineEndings $targetRoot

    $sizeBytes = (Get-ChildItem -LiteralPath $targetRoot -Recurse -File -ErrorAction SilentlyContinue | Measure-Object -Property Length -Sum).Sum
    $releaseReady = $true
    $limitations = @(
        "Built on Windows by cross-compilation.",
        "POSIX MariaDB is installer-managed when not bundled.",
        "This distribution has one standard package line; core_lite and vector_external remain runtime profile options, not separate package artifacts.",
        "Termux proot/local ChromaDB is full_local/local_proot by default for the standard package."
    )
    $manifest = [ordered]@{
        package_name = $target.PackageName
        package_kind = $target.PackageKind
        target = $target.Target
        goos = $target.Goos
        goarch = $target.Goarch
        package_profile = $target.PackageProfile
        status = "green"
        release_ready = $releaseReady
        generated_at = [DateTimeOffset]::UtcNow.ToString("o")
        source_root = "release-source"
        target_root = "."
        size_bytes = [int64]$sizeBytes
        canonical_store = "mariadb"
        vector_engine = "optional_chromadb"
        runtime_payloads_included = $false
        runtime_profile_default = $target.RuntimeProfileDefault
        vector_mode_default = $target.VectorModeDefault
        runtime_mode = $target.RuntimeMode
        normal_user_manual_mariadb_required = $false
        normal_user_manual_chromadb_required = $false
        real_device_proof_required = $false
        release_verification_basis = "managed_package_contract_complete"
        automatic_update_apply = $true
        automatic_update_timing = "backend_exit_75_immediate"
        one_click_entry = $target.Launcher
        managed_scripts = @(
            "scripts/start-full-posix.sh",
            "scripts/process-lifetime.py",
            "scripts/start-full-linux.sh",
            "scripts/start-full-macos.sh",
            "scripts/install-and-start-termux.sh"
        )
        included = @(
            "bin/archive-center-go",
            "bin/archive-center-updater",
            "bin/mariadb-schema",
            "PACKAGE_MIGRATION_UPDATE.json",
            "Archive Center.js",
            "LICENSE",
            "NOTICE",
            "THIRD_PARTY_NOTICES.md",
            "licenses",
            "migrations",
            "prompts",
            "scripts",
            "ops/platform-proof.sh"
        )
        excluded = @(
            ".git",
            ".runtime",
            ".runtime-cache",
            "user database files",
            "ChromaDB persist data"
        )
        limitations = $limitations
    }
    $manifest | ConvertTo-Json -Depth 8 | Set-Content -LiteralPath (Join-Path $targetRoot "PLATFORM_PACKAGE_MANIFEST.json") -Encoding UTF8
    Write-PackageReleaseStatus $targetRoot $packageVersionLabel $releaseReady
    Write-PackageMigrationUpdateManifest $targetRoot $packageVersionLabel
    $requiredManagedEntries = @(
        "bin/archive-center-go",
        "bin/archive-center-updater",
        "bin/mariadb-schema",
        "PACKAGE_MIGRATION_UPDATE.json",
        "PACKAGE_RELEASE_STATUS.json",
        "Archive Center.js",
        "scripts/start-full-posix.sh",
        "scripts/process-lifetime.py",
        "scripts/$($target.Script)",
        $target.Launcher
    )
    foreach ($requiredEntry in $requiredManagedEntries) {
        $requiredPath = Join-Path $targetRoot ($requiredEntry.Replace('/', '\'))
        if (-not (Test-Path -LiteralPath $requiredPath -PathType Leaf) -or (Get-Item -LiteralPath $requiredPath).Length -le 0) {
            throw "POSIX managed package is missing required $($target.Target) entry: $requiredEntry"
        }
    }
    $managedLauncherText = Get-Content -LiteralPath (Join-Path $targetRoot "scripts\start-full-posix.sh") -Raw
    foreach ($requiredMarker in @(
        'AC_UPDATE_STAGING_DIR="$PACKAGE_ROOT/.updates"',
        'AC_UPDATE_APPLY_MODE=managed_launcher_exit_75',
        'prepare_update_launcher_session',
        'launcher-session.json',
        'AC_UPDATE_LAUNCHER_TOKEN',
        'if [ "$backend_exit" -ne 75 ]',
        'Backend requested immediate pending-update apply (exit 75).',
        '-schema "$PACKAGE_ROOT/migrations"',
        'rollback_pending_preparation_failure',
        'rolled-back backend failed /ready or exact /version verification',
        'trap ''shutdown_from_signal 129'' HUP',
        'start_managed_process "$ARCHIVE_CENTER_GO_RUN"',
        'LIFETIME_HELPER="$SCRIPT_DIR/process-lifetime.py"'
    )) {
        if (-not $managedLauncherText.Contains($requiredMarker)) {
            throw "POSIX managed launcher is missing immediate-update marker: $requiredMarker"
        }
    }
    if ($managedLauncherText.Contains('--keep-services')) {
        throw "POSIX managed launcher must stop every managed server when its launcher exits"
    }
    Write-ManagedPackageManifest $targetRoot $packageVersionLabel

    if ($Zip) {
        $zipPath = Join-Path $outputRootFull ($target.PackageName + ".zip")
        Compress-DirectoryPortable $targetRoot $zipPath
    }

    Write-Host "Created $($target.PackageName)"
}
