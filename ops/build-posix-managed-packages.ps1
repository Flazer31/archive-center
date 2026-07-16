param(
    [string]$OutputRoot,
    [string[]]$TargetFilter = @(),
    [string]$PackageVersion = "3.0.0",
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

function Set-CopiedPackageVersionText([string]$Root, [string]$PackageVersion) {
    $version = if ([string]::IsNullOrWhiteSpace($PackageVersion)) { "3.0.0" } else { $PackageVersion.Trim() }
    $suffix = "archivecenter" + (($version -replace '\s+', '').ToLowerInvariant())
    $utf8NoBom = New-Object System.Text.UTF8Encoding($false)
    foreach ($pattern in @("*.md", "*.txt", "*.sh", "*.command")) {
        Get-ChildItem -LiteralPath $Root -Filter $pattern -File -Recurse -ErrorAction SilentlyContinue | ForEach-Object {
            $text = [System.IO.File]::ReadAllText($_.FullName, [System.Text.Encoding]::UTF8)
            $next = $text.Replace("Archive Center 2.1", "Archive Center $version")
            $next = $next.Replace("archivecenter2.1", $suffix)
            if ($next -ne $text) {
                [System.IO.File]::WriteAllText($_.FullName, $next, $utf8NoBom)
            }
        }
    }
}

function Set-RuntimeDefaultsInEnvExample([string]$Path, [string]$RuntimeProfile, [string]$VectorMode) {
    if (-not (Test-Path -LiteralPath $Path -PathType Leaf)) {
        throw "Missing env example file: $Path"
    }
    $text = Get-Content -LiteralPath $Path -Raw
    $text = [regex]::Replace($text, '(?m)^AC_RUNTIME_PROFILE=.*$', "AC_RUNTIME_PROFILE=$RuntimeProfile")
    $text = [regex]::Replace($text, '(?m)^AC_VECTOR_MODE=.*$', "AC_VECTOR_MODE=$VectorMode")
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
    throw "Refusing to write outside Archive Center 2.0: $outputRootFull"
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

$packageVersionLabel = if ([string]::IsNullOrWhiteSpace($PackageVersion)) { "3.0.0" } else { $PackageVersion.Trim() }
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
        [System.IO.Directory]::Delete($targetRoot, $true)
    }

    New-Item -ItemType Directory -Force -Path (Join-Path $targetRoot "bin") | Out-Null
    Build-GoBinary $goServiceRoot $target.Goos $target.Goarch "./cmd/archive-center-go" (Join-Path $targetRoot "bin\archive-center-go")
    Build-GoBinary $goServiceRoot $target.Goos $target.Goarch "./cmd/mariadb-schema" (Join-Path $targetRoot "bin\mariadb-schema")
    $migrationTools = @(
        "sqlite-export",
        "dry-run-validator",
        "compare-dry-run",
        "mariadb-dry-run-import",
        "mariadb-import",
        "legacy10-migrate"
    )
    foreach ($tool in $migrationTools) {
        Build-GoBinary $goServiceRoot $target.Goos $target.Goarch "./cmd/$tool" (Join-Path $targetRoot "bin\$tool")
    }

    Copy-File (Join-Path $repoRoot "Archive Center.js") (Join-Path $targetRoot "Archive Center.js")
    $readmeSource = "ops\full-package-posix\README_POSIX_FULL_PACKAGE.md"
    $readFirstSource = "ops\full-package-posix\00_README_FIRST_POSIX_FULL.md"
    Copy-File (Join-Path $repoRoot $readmeSource) (Join-Path $targetRoot "README.md")
    Copy-File (Join-Path $repoRoot ".env.example") (Join-Path $targetRoot ".env.source.example")
    Copy-File (Join-Path $repoRoot "ops\full-package\.env.full.example") (Join-Path $targetRoot ".env.full.example")
    Set-RuntimeDefaultsInEnvExample (Join-Path $targetRoot ".env.full.example") $target.RuntimeProfileDefault $target.VectorModeDefault
    Copy-DirectoryContents (Join-Path $repoRoot "migrations") (Join-Path $targetRoot "migrations")
    Copy-DirectoryContents (Join-Path $repoRoot "prompts") (Join-Path $targetRoot "prompts")
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
    $launcherBody = "#!/usr/bin/env sh`nset -eu`nSCRIPT_DIR=`$(CDPATH= cd -- `"`$(dirname -- `"`$0`")`" && pwd -P)`nARCHIVE_CENTER_PACKAGE_ROOT=`"$SCRIPT_DIR`"`nexport ARCHIVE_CENTER_PACKAGE_ROOT`nexec sh `"`$SCRIPT_DIR/scripts/$($target.Script)`" --profile `"$($target.RuntimeProfileDefault)`" --vector-mode `"$($target.VectorModeDefault)`" `"`$@`"`n"
    Write-TextFile $launcherPath $launcherBody
    Set-CopiedPackageVersionText $targetRoot $packageVersionLabel

    $sizeBytes = (Get-ChildItem -LiteralPath $targetRoot -Recurse -File -ErrorAction SilentlyContinue | Measure-Object -Property Length -Sum).Sum
    $manifest = [ordered]@{
        package_name = $target.PackageName
        package_kind = $target.PackageKind
        target = $target.Target
        goos = $target.Goos
        goarch = $target.Goarch
        package_profile = $target.PackageProfile
        status = $target.Status
        release_ready = $false
        generated_at = [DateTimeOffset]::UtcNow.ToString("o")
        source_root = $repoRoot
        target_root = $targetRoot
        size_bytes = [int64]$sizeBytes
        canonical_store = "mariadb"
        vector_engine = "optional_chromadb"
        runtime_payloads_included = $false
        runtime_profile_default = $target.RuntimeProfileDefault
        vector_mode_default = $target.VectorModeDefault
        runtime_mode = $target.RuntimeMode
        normal_user_manual_mariadb_required = $false
        normal_user_manual_chromadb_required = $false
        real_device_proof_required = $true
        one_click_entry = $target.Launcher
        managed_scripts = @(
            "scripts/start-full-posix.sh",
            "scripts/start-full-linux.sh",
            "scripts/start-full-macos.sh",
            "scripts/install-and-start-termux.sh"
        )
        included = @(
            "bin/archive-center-go",
            "bin/mariadb-schema",
            "bin/legacy10-migrate",
            "bin/sqlite-export",
            "bin/dry-run-validator",
            "bin/compare-dry-run",
            "bin/mariadb-dry-run-import",
            "bin/mariadb-import",
            "Archive Center.js",
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
        limitations = @(
            "Built on Windows by cross-compilation.",
            "Real target OS runtime proof is still required.",
            "POSIX MariaDB is installer-managed when not bundled.",
            "This distribution has one standard package line; core_lite and vector_external remain runtime profile options, not separate package artifacts.",
            "Termux proot/local ChromaDB is full_local/local_proot by default for the standard package."
        )
    }
    $manifest | ConvertTo-Json -Depth 8 | Set-Content -LiteralPath (Join-Path $targetRoot "PLATFORM_PACKAGE_MANIFEST.json") -Encoding UTF8

    if ($Zip) {
        $zipPath = Join-Path $outputRootFull ($target.PackageName + ".zip")
        Compress-DirectoryPortable $targetRoot $zipPath
    }

    Write-Host "Created $($target.PackageName)"
}
