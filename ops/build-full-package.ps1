param(
    [string]$OutputRoot,
    [string]$PackageName = "",
    [string]$PackageKind = "full",
    [string]$PackageVersion = "3.0.0",
    [string]$MariaDBRuntime = "",
    [string]$ChromaRuntime = "",
    [string]$CodeSigningCertThumbprint = "",
    [string]$TimestampServer = "http://timestamp.digicert.com",
    [switch]$AllowMissingRuntimePayloads,
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

function Copy-File([string]$Source, [string]$DestRelative) {
    $src = Join-Path $repoRoot $Source
    if (-not (Test-Path -LiteralPath $src -PathType Leaf)) {
        throw "Missing source file: $src"
    }
    $dest = Join-Path $targetFull $DestRelative
    New-Item -ItemType Directory -Force -Path (Split-Path -Parent $dest) | Out-Null
    Copy-Item -LiteralPath $src -Destination $dest -Force
}

function Copy-Directory([string]$Source, [string]$DestRelative) {
    $src = Join-Path $repoRoot $Source
    if (-not (Test-Path -LiteralPath $src -PathType Container)) {
        throw "Missing source directory: $src"
    }
    $dest = Join-Path $targetFull $DestRelative
    New-Item -ItemType Directory -Force -Path $dest | Out-Null
    Get-ChildItem -LiteralPath $src -Force | ForEach-Object {
        Copy-Item -LiteralPath $_.FullName -Destination $dest -Recurse -Force
    }
}

function Copy-RuntimePayload([string]$Source, [string]$DestRelative) {
    if ([string]::IsNullOrWhiteSpace($Source)) {
        return $false
    }
    $resolved = Resolve-FullPath $Source
    if (-not (Test-Path -LiteralPath $resolved)) {
        throw "Runtime payload not found: $Source"
    }
    $dest = Join-Path $targetFull $DestRelative
    if (Test-Path -LiteralPath $resolved -PathType Leaf) {
        New-Item -ItemType Directory -Force -Path $dest | Out-Null
        Expand-Archive -LiteralPath $resolved -DestinationPath $dest -Force
        return $true
    }
    New-Item -ItemType Directory -Force -Path $dest | Out-Null
    Get-ChildItem -LiteralPath $resolved -Force | ForEach-Object {
        Copy-Item -LiteralPath $_.FullName -Destination $dest -Recurse -Force
    }
    return $true
}

function Find-MariaDBProvider([string]$Root) {
    $hits = Get-ChildItem -LiteralPath $Root -Recurse -File -ErrorAction SilentlyContinue |
        Where-Object { $_.Name -ieq "mariadbd.exe" -or $_.Name -ieq "mysqld.exe" } |
        Sort-Object FullName |
        Select-Object -First 1
    if ($null -eq $hits) {
        return ""
    }
    return $hits.FullName
}

function Find-ChromaRuntime([string]$Root) {
    $python = Get-ChildItem -LiteralPath $Root -Recurse -File -ErrorAction SilentlyContinue |
        Where-Object { $_.Name -ieq "python.exe" } |
        Sort-Object FullName |
        Select-Object -First 1
    if ($null -ne $python) {
        return $python.FullName
    }
    $chroma = Get-ChildItem -LiteralPath $Root -Recurse -Directory -ErrorAction SilentlyContinue |
        Where-Object { $_.Name -ieq "chromadb" -or $_.Name -ieq "ChromaDB" } |
        Sort-Object FullName |
        Select-Object -First 1
    if ($null -ne $chroma) {
        return $chroma.FullName
    }
    return ""
}

function Set-RuntimeDefaultsInEnvExample([string]$Path, [string]$RuntimeProfile, [string]$VectorMode, [string]$PackageVersion) {
    if (-not (Test-Path -LiteralPath $Path -PathType Leaf)) {
        throw "Missing env example file: $Path"
    }
    $text = [System.IO.File]::ReadAllText($Path, [System.Text.Encoding]::UTF8)
    $text = [regex]::Replace($text, '(?m)^AC_RUNTIME_PROFILE=.*$', "AC_RUNTIME_PROFILE=$RuntimeProfile")
    $text = [regex]::Replace($text, '(?m)^AC_VECTOR_MODE=.*$', "AC_VECTOR_MODE=$VectorMode")
    $text = [regex]::Replace($text, '(?m)^AC_BUILD_VERSION=.*$', "AC_BUILD_VERSION=$PackageVersion")
    $utf8NoBom = [System.Text.UTF8Encoding]::new($false)
    [System.IO.File]::WriteAllText($Path, $text, $utf8NoBom)
}

function Set-CopiedPackageKindText([string]$Path, [string]$PackageKind, [string]$PackageVersion) {
    if (-not (Test-Path -LiteralPath $Path -PathType Leaf)) {
        return
    }
    $version = if ([string]::IsNullOrWhiteSpace($PackageVersion)) { "3.0.0" } else { $PackageVersion.Trim() }
    $text = [System.IO.File]::ReadAllText($Path, [System.Text.Encoding]::UTF8)
    $text = $text.Replace("Archive Center 2.1 Windows Full Package", "Archive Center $version Windows Package")
    $text = $text.Replace("Starting Archive Center 2.1 full package", "Starting Archive Center $version package")
    $utf8NoBom = [System.Text.UTF8Encoding]::new($false)
    [System.IO.File]::WriteAllText($Path, $text, $utf8NoBom)
}

function Set-CopiedPackageVersionText([string]$Root, [string]$PackageVersion) {
    $version = if ([string]::IsNullOrWhiteSpace($PackageVersion)) { "3.0.0" } else { $PackageVersion.Trim() }
    $utf8NoBom = [System.Text.UTF8Encoding]::new($false)
    $patterns = @("*.md", "*.txt", "*.bat", "*.cmd", "*.ps1", "*.sh", "*.command")
    foreach ($pattern in $patterns) {
        Get-ChildItem -LiteralPath $Root -Filter $pattern -File -Recurse -ErrorAction SilentlyContinue | ForEach-Object {
            $text = [System.IO.File]::ReadAllText($_.FullName, [System.Text.Encoding]::UTF8)
            $next = $text.Replace("Archive Center 2.1", "Archive Center $version")
            $next = $next.Replace("archivecenter2.1", ("archivecenter" + ($version -replace '\s+', '').ToLowerInvariant()))
            $next = $next.Replace("__ARCHIVE_CENTER_PACKAGE_VERSION__", $version)
            if ($next -ne $text) {
                [System.IO.File]::WriteAllText($_.FullName, $next, $utf8NoBom)
            }
        }
    }
}

function Get-RelativePackagePath([string]$Path, [string]$Root) {
    $rootFull = [System.IO.Path]::GetFullPath($Root).TrimEnd('\') + '\'
    $pathFull = [System.IO.Path]::GetFullPath($Path)
    if ($pathFull.StartsWith($rootFull, [System.StringComparison]::OrdinalIgnoreCase)) {
        return $pathFull.Substring($rootFull.Length).Replace('\', '/')
    }
    return $pathFull
}

function Test-ZoneIdentifier([string]$Path) {
    try {
        $stream = Get-Item -LiteralPath $Path -Stream Zone.Identifier -ErrorAction SilentlyContinue
        return $null -ne $stream
    } catch {
        return $false
    }
}

function Get-PackageSignatureSummary([string]$Path, [string]$Extension) {
    if (@(".exe", ".dll", ".ps1", ".psm1", ".msi") -notcontains $Extension) {
        return [ordered]@{
            checked = $false
            status = "not_applicable"
            signer = ""
        }
    }
    try {
        $sig = Get-AuthenticodeSignature -LiteralPath $Path
        $signer = ""
        if ($sig.SignerCertificate) {
            $signer = $sig.SignerCertificate.Subject
        }
        return [ordered]@{
            checked = $true
            status = [string]$sig.Status
            signer = $signer
        }
    } catch {
        return [ordered]@{
            checked = $true
            status = "signature_check_failed"
            signer = ""
        }
    }
}

function Get-CodeSigningCertificate([string]$Thumbprint) {
    if ([string]::IsNullOrWhiteSpace($Thumbprint)) {
        return $null
    }
    $normalized = ($Thumbprint -replace '\s', '').ToUpperInvariant()
    foreach ($store in @("Cert:\CurrentUser\My", "Cert:\LocalMachine\My")) {
        $cert = Get-ChildItem -Path $store -CodeSigningCert -ErrorAction SilentlyContinue |
            Where-Object { ($_.Thumbprint -replace '\s', '').ToUpperInvariant() -eq $normalized } |
            Select-Object -First 1
        if ($null -ne $cert) {
            return $cert
        }
    }
    throw "Code signing certificate not found: $Thumbprint"
}

function Set-OwnPayloadSignatures([string]$Root, [string]$Thumbprint, [string]$TimestampUrl) {
    $result = [ordered]@{
        requested = -not [string]::IsNullOrWhiteSpace($Thumbprint)
        status = "not_requested"
        signed_files = @()
    }
    if (-not $result.requested) {
        return $result
    }

    $cert = Get-CodeSigningCertificate $Thumbprint
    $signTargets = @()
    $signTargets += Get-ChildItem -LiteralPath (Join-Path $Root "bin") -Filter "*.exe" -File -ErrorAction SilentlyContinue
    $signTargets += Get-ChildItem -LiteralPath (Join-Path $Root "scripts") -Filter "*.ps1" -File -ErrorAction SilentlyContinue
    $toolsRoot = Join-Path $Root "tools"
    if (Test-Path -LiteralPath $toolsRoot -PathType Container) {
        $signTargets += Get-ChildItem -LiteralPath $toolsRoot -Filter "*.ps1" -File -Recurse -ErrorAction SilentlyContinue
    }

    $signed = @()
    foreach ($target in ($signTargets | Sort-Object FullName -Unique)) {
        $sig = Set-AuthenticodeSignature -FilePath $target.FullName -Certificate $cert -TimestampServer $TimestampUrl
        if ($sig.Status -ne "Valid") {
            throw "Code signing failed for $($target.FullName): $($sig.StatusMessage)"
        }
        $signed += Get-RelativePackagePath $target.FullName $Root
    }
    $result.status = "signed"
    $result.signed_files = $signed
    return $result
}

function Write-PackageTrustEvidence([string]$Root) {
    $payloadExts = @(".exe", ".dll", ".ps1", ".psm1", ".bat", ".cmd", ".msi")
    $selfFiles = @(
        "FULL_PACKAGE_MANIFEST.json",
        "PACKAGE_FILE_MANIFEST.json",
        "SHA256SUMS.txt"
    )
    $excludedRoots = @(".runtime", ".runtime-cache", ".updates")
    $excludedLocalFiles = @(".env.full.local", ".env.full.local.protected")
    $files = Get-ChildItem -LiteralPath $Root -Recurse -File -ErrorAction SilentlyContinue |
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
        $ext = $file.Extension.ToLowerInvariant()
        $hash = Get-FileHash -LiteralPath $file.FullName -Algorithm SHA256
        $sha = $hash.Hash.ToUpperInvariant()
        $signature = if ($payloadExts -contains $ext) {
            Get-PackageSignatureSummary $file.FullName $ext
        } else {
            [ordered]@{ checked = $false; status = "not_applicable"; signer = ""; thumbprint = "" }
        }
        $items += [ordered]@{
            path = $rel
            extension = $ext
            size_bytes = [int64]$file.Length
            sha256 = $sha
            has_mark_of_the_web = Test-ZoneIdentifier $file.FullName
            signature = $signature
        }
        $sumLines += "$sha  $rel"
    }

    $unsigned = @($items | Where-Object { $_.signature.checked -and $_.signature.status -ne "Valid" })
    $manifest = [ordered]@{
        schema_version = "archive-center.package-file-manifest.v1"
        generated_at = [DateTimeOffset]::UtcNow.ToString("o")
        scope = "managed_package_payloads"
        signature_scope = "executable_and_script_payloads"
        package_root = $Root
        automatic_defender_exclusions = $false
        checked_files = $items.Count
        managed_file_count = $items.Count
        signature_checked_file_count = @($items | Where-Object { $_.signature.checked }).Count
        unsigned_or_untrusted_signature_count = $unsigned.Count
        excluded_runtime_roots = $excludedRoots
        excluded_local_files = $excludedLocalFiles
        self_excluded_files = $selfFiles
        files = $items
    }
    $manifestJson = $manifest | ConvertTo-Json -Depth 8
    [System.IO.File]::WriteAllText(
        (Join-Path $Root "PACKAGE_FILE_MANIFEST.json"),
        $manifestJson + [Environment]::NewLine,
        (New-Object System.Text.UTF8Encoding($false))
    )
    $sumLines | Set-Content -LiteralPath (Join-Path $Root "SHA256SUMS.txt") -Encoding ASCII
    return [ordered]@{
        file_manifest = "PACKAGE_FILE_MANIFEST.json"
        sha256sums = "SHA256SUMS.txt"
        scope = "managed_package_payloads"
        signature_scope = "executable_and_script_payloads"
        checked_files = $items.Count
        managed_file_count = $items.Count
        signature_checked_file_count = @($items | Where-Object { $_.signature.checked }).Count
        unsigned_or_untrusted_signature_count = $unsigned.Count
    }
}

$repoRoot = Resolve-FullPath (Join-Path $PSScriptRoot "..")
if ([string]::IsNullOrWhiteSpace($OutputRoot)) {
    $OutputRoot = Join-Path $repoRoot "_dist"
}

$PackageKind = $PackageKind.Trim().ToLowerInvariant()
if ($PackageKind -ne "full") {
    throw "Unsupported PackageKind: $PackageKind. Archive Center builds the standard package only."
}

if ([string]::IsNullOrWhiteSpace($PackageName)) {
    $PackageName = "Archive Center $PackageVersion Windows Package"
}

$runtimeProfileDefault = "full_local"
$vectorModeDefault = "bundled"
$packageProfile = "windows_full_local"
$vectorEngine = "chromadb"
$requiredRuntimePayloads = @("mariadb", "chromadb")

$outputRootFull = Resolve-FullPath $OutputRoot
$targetFull = Resolve-FullPath (Join-Path $outputRootFull $PackageName)

if (-not (Test-PathInside $targetFull $repoRoot)) {
    throw "Refusing to write full package outside Archive Center 2.0: $targetFull"
}

if ((Test-Path -LiteralPath $targetFull) -and -not $ForceRefresh) {
    throw "Target already exists: $targetFull. Re-run with -ForceRefresh to rebuild it."
}

if (Test-Path -LiteralPath $targetFull) {
    $resolved = Resolve-FullPath $targetFull
    if (-not (Test-PathInside $resolved $outputRootFull)) {
        throw "Refusing recursive delete outside output root: $resolved"
    }
    Remove-Item -LiteralPath $resolved -Recurse -Force
}

New-Item -ItemType Directory -Force -Path $targetFull | Out-Null
New-Item -ItemType Directory -Force -Path (Join-Path $targetFull "bin") | Out-Null

$goServiceRoot = Join-Path $repoRoot "go-service"
$goVersionText = (& go version 2>&1 | Out-String).Trim()
if ($LASTEXITCODE -ne 0 -or $goVersionText -notmatch '\bgo(\d+)\.(\d+)\.(\d+)\b') {
    throw "Archive Center release packaging requires Go 1.26.5 or newer. Detected: $goVersionText"
}
$detectedGoVersion = [Version]("{0}.{1}.{2}" -f $Matches[1], $Matches[2], $Matches[3])
if ($detectedGoVersion -lt [Version]"1.26.5") {
    throw "Archive Center release packaging requires Go 1.26.5 or newer. Detected: $goVersionText"
}
Push-Location $goServiceRoot
try {
    & go build -buildvcs=false -trimpath -ldflags "-s -w" -o (Join-Path $targetFull "bin\archive-center-go.exe") ./cmd/archive-center-go
    if ($LASTEXITCODE -ne 0) {
        throw "go build archive-center-go failed."
    }
    & go build -buildvcs=false -trimpath -ldflags "-s -w" -o (Join-Path $targetFull "bin\archive-center-updater.exe") ./cmd/archive-center-updater
    if ($LASTEXITCODE -ne 0) {
        throw "go build archive-center-updater failed."
    }
    & go build -buildvcs=false -trimpath -ldflags "-s -w" -o (Join-Path $targetFull "bin\mariadb-schema.exe") ./cmd/mariadb-schema
    if ($LASTEXITCODE -ne 0) {
        throw "go build mariadb-schema failed."
    }
    $migrationTools = @(
        "sqlite-export",
        "dry-run-validator",
        "compare-dry-run",
        "mariadb-dry-run-import",
        "mariadb-import",
        "legacy10-migrate"
    )
    foreach ($tool in $migrationTools) {
        & go build -buildvcs=false -trimpath -ldflags "-s -w" -o (Join-Path $targetFull "bin\$tool.exe") "./cmd/$tool"
        if ($LASTEXITCODE -ne 0) {
            throw "go build $tool failed."
        }
    }
} finally {
    Pop-Location
}

Copy-File "Archive Center.js" "Archive Center.js"
Copy-File ".env.example" ".env.source.example"
Copy-Directory "migrations" "migrations"
Copy-Directory "prompts" "prompts"

Copy-File "ops/full-package/README_FULL_PACKAGE.md" "README.md"
Copy-File "ops/full-package/README_FULL_PACKAGE.md" "README_FULL_PACKAGE.md"
Copy-File "ops/full-package/WINDOWS_TRUST_AND_DEFENDER.md" "WINDOWS_TRUST_AND_DEFENDER.md"
Copy-File "ops/full-package/00_README_FIRST_WINDOWS.md" "00_README_FIRST_WINDOWS.md"
Copy-File "ops/full-package/01_start_archive_center_windows.bat" "01_start_archive_center_windows.bat"
Copy-File "ops/full-package/02_smoke_test_windows.bat" "02_smoke_test_windows.bat"
Copy-File "ops/full-package/03_run_backend.bat" "03_run_backend.bat"
Copy-File "ops/full-package/04_protect_env_windows.bat" "04_protect_env_windows.bat"
Copy-File "ops/full-package/05_unprotect_env_windows.bat" "05_unprotect_env_windows.bat"
Copy-File "ops/full-package/06_migrate_1_0_to_2_0_windows.bat" "06_migrate_1_0_to_2_0_windows.bat"
Copy-File "ops/full-package/.env.full.example" ".env.full.example"
Copy-Directory "ops/full-package/scripts" "scripts"
Copy-File "ops/install-windows.ps1" "tools/install-windows.ps1"
Set-RuntimeDefaultsInEnvExample (Join-Path $targetFull ".env.full.example") $runtimeProfileDefault $vectorModeDefault $PackageVersion
Set-CopiedPackageKindText (Join-Path $targetFull "01_start_archive_center_windows.bat") $PackageKind $PackageVersion
Set-CopiedPackageKindText (Join-Path $targetFull "scripts\start-full-windows.ps1") $PackageKind $PackageVersion
Set-CopiedPackageVersionText $targetFull $PackageVersion

$mariadbCopied = Copy-RuntimePayload $MariaDBRuntime "runtime\MariaDB"
$chromaCopied = Copy-RuntimePayload $ChromaRuntime "runtime\ChromaDB"

$runtimeRoot = Join-Path $targetFull "runtime"
$mariadbProvider = Find-MariaDBProvider $runtimeRoot
$chromaRuntimeFound = Find-ChromaRuntime $runtimeRoot
$codeSigning = Set-OwnPayloadSignatures $targetFull $CodeSigningCertThumbprint $TimestampServer
$trustEvidence = Write-PackageTrustEvidence $targetFull
$missing = @()
if ([string]::IsNullOrWhiteSpace($mariadbProvider)) {
    $missing += "mariadb_runtime"
}
if ([string]::IsNullOrWhiteSpace($chromaRuntimeFound)) {
    $missing += "chromadb_runtime"
}

$releaseReady = $missing.Count -eq 0
if (-not $releaseReady -and -not $AllowMissingRuntimePayloads) {
    $manifest = [ordered]@{
        package_name = $PackageName
        package_kind = $PackageKind
        package_profile = $packageProfile
        status = "blocked_missing_runtime_payloads"
        generated_at = [DateTimeOffset]::UtcNow.ToString("o")
        target_root = $targetFull
        runtime_profile_default = $runtimeProfileDefault
        vector_mode_default = $vectorModeDefault
        required_runtime_payloads = $requiredRuntimePayloads
        missing = $missing
        windows_trust = [ordered]@{
            automatic_defender_exclusions = $false
            code_signing = $codeSigning
            evidence = $trustEvidence
            false_positive_submission = "Submit the exact detected file and SHA256 from PACKAGE_FILE_MANIFEST.json through Microsoft Security Intelligence if a known-good build is incorrectly detected."
        }
        note = "Pass the required runtime payloads, or use -AllowMissingRuntimePayloads for a non-release staging folder."
    }
    $manifest | ConvertTo-Json -Depth 6 | Set-Content -LiteralPath (Join-Path $targetFull "FULL_PACKAGE_MANIFEST.json") -Encoding UTF8
    throw "Windows $PackageKind package is missing required runtime payloads: $($missing -join ', ')"
}

$sizeBytes = (Get-ChildItem -LiteralPath $targetFull -Recurse -File -ErrorAction SilentlyContinue | Measure-Object -Property Length -Sum).Sum
$manifest = [ordered]@{
    package_name = $PackageName
    package_kind = $PackageKind
    package_profile = $packageProfile
    generated_at = [DateTimeOffset]::UtcNow.ToString("o")
    source_root = $repoRoot
    target_root = $targetFull
    release_ready = $releaseReady
    status = if ($releaseReady) { "green" } else { "red_missing_runtime_payloads" }
    size_bytes = [int64]$sizeBytes
    canonical_store = "mariadb"
    vector_engine = $vectorEngine
    includes_runtime_binaries = $releaseReady
    runtime_profile_default = $runtimeProfileDefault
    vector_mode_default = $vectorModeDefault
    go_toolchain = $goVersionText
    chromadb_version = "1.5.9"
    chromadb_api_path = "/api/v2"
    required_runtime_payloads = $requiredRuntimePayloads
    runtime_payloads = [ordered]@{
        mariadb_copied_from = $MariaDBRuntime
        mariadb_payload_copied = [bool]$mariadbCopied
        mariadb_provider_path = $mariadbProvider
        chromadb_copied_from = $ChromaRuntime
        chromadb_payload_copied = [bool]$chromaCopied
        chromadb_runtime_path = $chromaRuntimeFound
    }
    windows_trust = [ordered]@{
        automatic_defender_exclusions = $false
        code_signing = $codeSigning
        evidence = $trustEvidence
        note = "Archive Center does not disable Defender and does not add Defender exclusions. Release builds should prefer signed own binaries and scripts plus Microsoft false-positive submission for known-good detections."
        false_positive_submission = "Submit the exact detected file and SHA256 from PACKAGE_FILE_MANIFEST.json through Microsoft Security Intelligence if a known-good build is incorrectly detected."
    }
    included = @(
        "bin/archive-center-go.exe",
        "bin/archive-center-updater.exe",
        "bin/mariadb-schema.exe",
        "bin/legacy10-migrate.exe",
        "bin/sqlite-export.exe",
        "bin/dry-run-validator.exe",
        "bin/compare-dry-run.exe",
        "bin/mariadb-dry-run-import.exe",
        "bin/mariadb-import.exe",
        "Archive Center.js",
        "WINDOWS_TRUST_AND_DEFENDER.md",
        "PACKAGE_FILE_MANIFEST.json",
        "SHA256SUMS.txt",
        "migrations",
        "prompts",
        "01_start_archive_center_windows.bat",
        "02_smoke_test_windows.bat",
        "03_run_backend.bat",
        "04_protect_env_windows.bat",
        "05_unprotect_env_windows.bat",
        "06_migrate_1_0_to_2_0_windows.bat",
        "scripts",
        "tools/install-windows.ps1"
    )
    excluded = @(
        ".git",
        ".runtime",
        ".runtime-cache",
        "go-service source",
        "test binaries",
        "database files",
        "ChromaDB persist data",
        "backup/release/deploy outputs"
    )
    missing = $missing
}
$manifest | ConvertTo-Json -Depth 8 | Set-Content -LiteralPath (Join-Path $targetFull "FULL_PACKAGE_MANIFEST.json") -Encoding UTF8

$zipPath = ""
$zipChecksumPath = ""
if ($Zip) {
    if (-not $releaseReady -and -not $AllowMissingRuntimePayloads) {
        throw "Refusing to zip a non-ready package."
    }
    $zipPath = Join-Path $outputRootFull ($PackageName + ".zip")
    $zipChecksumName = "SHA256SUMS-$PackageVersion.txt"
    if ([System.IO.Path]::GetFileName($zipChecksumName) -ne $zipChecksumName) {
        throw "PackageVersion cannot be used safely in the external checksum filename: $PackageVersion"
    }
    $zipChecksumPath = Join-Path $outputRootFull $zipChecksumName
    $stagingID = [guid]::NewGuid().ToString("N")
    $zipBaseName = [System.IO.Path]::GetFileName($zipPath)
    $tempZipPath = Join-Path $outputRootFull (".{0}.{1}.tmp.zip" -f $zipBaseName, $stagingID)
    $tempChecksumPath = Join-Path $outputRootFull (".{0}.{1}.tmp" -f $zipChecksumName, $stagingID)
    $zipBackupPath = Join-Path $outputRootFull (".{0}.{1}.backup" -f $zipBaseName, $stagingID)
    $checksumBackupPath = Join-Path $outputRootFull (".{0}.{1}.backup" -f $zipChecksumName, $stagingID)
    $zipHadPrevious = Test-Path -LiteralPath $zipPath -PathType Leaf
    $checksumHadPrevious = Test-Path -LiteralPath $zipChecksumPath -PathType Leaf
    $zipInstalled = $false
    $checksumInstalled = $false
    $cleanupReplacementBackups = $false
    try {
        Compress-Archive -LiteralPath $targetFull -DestinationPath $tempZipPath

        Add-Type -AssemblyName System.IO.Compression.FileSystem
        $archive = [System.IO.Compression.ZipFile]::OpenRead($tempZipPath)
        try {
            $entryMap = @{}
            foreach ($entry in $archive.Entries) {
                $rawEntryName = ([string]$entry.FullName).Replace('\', '/')
                $normalized = $rawEntryName.TrimStart('/')
                if ([string]::IsNullOrWhiteSpace($normalized)) { continue }
                $segments = $normalized -split '/'
                if ($rawEntryName.StartsWith('/') -or $normalized -match '^[A-Za-z]:' -or $segments -contains '..') {
                    throw "Generated ZIP contains an unsafe entry name: $($entry.FullName)"
                }
                if ($entryMap.ContainsKey($normalized)) {
                    throw "Generated ZIP contains a duplicate normalized entry: $normalized"
                }
                $entryMap[$normalized] = $entry
            }

            $manifestEntries = @($entryMap.Keys | Where-Object { $_ -eq "PACKAGE_FILE_MANIFEST.json" -or $_ -match '^[^/]+/PACKAGE_FILE_MANIFEST\.json$' })
            if ($manifestEntries.Count -ne 1) {
                throw "Generated ZIP must contain exactly one package manifest at the archive root or beneath one package-root directory."
            }
            $manifestEntry = $manifestEntries[0]
            $packagePrefix = $manifestEntry.Substring(0, $manifestEntry.Length - "PACKAGE_FILE_MANIFEST.json".Length)
            foreach ($requiredEntry in @("PACKAGE_FILE_MANIFEST.json", "bin/archive-center-go.exe", "bin/archive-center-updater.exe", "Archive Center.js")) {
                $expectedEntry = $packagePrefix + $requiredEntry
                if (-not $entryMap.ContainsKey($expectedEntry) -or $entryMap[$expectedEntry].Length -le 0) {
                    throw "Generated ZIP is missing required package entry: $expectedEntry"
                }
            }
        } finally {
            $archive.Dispose()
        }

        $zipSHA256 = (Get-FileHash -LiteralPath $tempZipPath -Algorithm SHA256).Hash.ToLowerInvariant()
        $checksumRecord = "$zipSHA256  $zipBaseName"
        [System.IO.File]::WriteAllText($tempChecksumPath, $checksumRecord + "`n", [System.Text.Encoding]::ASCII)

        $checksumBytes = [System.IO.File]::ReadAllBytes($tempChecksumPath)
        if ($checksumBytes.Length -lt 1 -or $checksumBytes[$checksumBytes.Length - 1] -ne 10 -or ($checksumBytes.Length -gt 1 -and $checksumBytes[$checksumBytes.Length - 2] -eq 13)) {
            throw "External ZIP checksum must end with a single ASCII LF newline."
        }
        $records = @([System.IO.File]::ReadAllLines($tempChecksumPath, [System.Text.Encoding]::ASCII) | Where-Object { -not [string]::IsNullOrWhiteSpace($_) })
        if ($records.Count -ne 1) {
            throw "External ZIP checksum must contain exactly one nonempty record: $tempChecksumPath"
        }
        if ($records[0] -notmatch '^([0-9a-f]{64})  (.+)$') {
            throw "External ZIP checksum record has an invalid format: $($records[0])"
        }
        if ($Matches[1] -ne $zipSHA256) {
            throw "External ZIP checksum digest does not match the generated ZIP."
        }
        if ($Matches[2] -cne $zipBaseName) {
            throw "External ZIP checksum filename does not exactly match the generated ZIP basename."
        }

        if ($zipHadPrevious) {
            [System.IO.File]::Replace($tempZipPath, $zipPath, $zipBackupPath, $true)
        } else {
            Move-Item -LiteralPath $tempZipPath -Destination $zipPath
        }
        $zipInstalled = $true

        try {
            if ($checksumHadPrevious) {
                [System.IO.File]::Replace($tempChecksumPath, $zipChecksumPath, $checksumBackupPath, $true)
            } else {
                Move-Item -LiteralPath $tempChecksumPath -Destination $zipChecksumPath
            }
            $checksumInstalled = $true
            $cleanupReplacementBackups = $true
        } catch {
            $installError = $_.Exception.Message
            $rollbackErrors = [System.Collections.Generic.List[string]]::new()
            if ($checksumHadPrevious -and (Test-Path -LiteralPath $checksumBackupPath -PathType Leaf)) {
                try {
                    [System.IO.File]::Replace($checksumBackupPath, $zipChecksumPath, $null, $true)
                } catch {
                    try {
                        Copy-Item -LiteralPath $checksumBackupPath -Destination $zipChecksumPath -Force
                    } catch {
                        [void]$rollbackErrors.Add("checksum restore: $($_.Exception.Message)")
                    }
                }
            } elseif (-not $checksumHadPrevious -and (Test-Path -LiteralPath $zipChecksumPath -PathType Leaf)) {
                try {
                    Remove-Item -LiteralPath $zipChecksumPath -Force
                } catch {
                    [void]$rollbackErrors.Add("new checksum removal: $($_.Exception.Message)")
                }
            }
            if ($zipHadPrevious -and (Test-Path -LiteralPath $zipBackupPath -PathType Leaf)) {
                try {
                    [System.IO.File]::Replace($zipBackupPath, $zipPath, $null, $true)
                } catch {
                    try {
                        Copy-Item -LiteralPath $zipBackupPath -Destination $zipPath -Force
                    } catch {
                        [void]$rollbackErrors.Add("ZIP restore: $($_.Exception.Message)")
                    }
                }
            } elseif (-not $zipHadPrevious -and (Test-Path -LiteralPath $zipPath -PathType Leaf)) {
                try {
                    Remove-Item -LiteralPath $zipPath -Force
                } catch {
                    [void]$rollbackErrors.Add("new ZIP removal: $($_.Exception.Message)")
                }
            }
            $zipInstalled = $false
            if ($rollbackErrors.Count -gt 0) {
                throw "External checksum install failed ($installError), and release-pair restoration was incomplete: $($rollbackErrors -join '; '). Replacement backups were retained."
            }
            $cleanupReplacementBackups = $true
            throw "External checksum install failed; the previous release pair was restored: $installError"
        }
    } finally {
        foreach ($temporaryPath in @($tempZipPath, $tempChecksumPath)) {
            if (Test-Path -LiteralPath $temporaryPath -PathType Leaf) {
                Remove-Item -LiteralPath $temporaryPath -Force -ErrorAction SilentlyContinue
            }
        }
        if ($cleanupReplacementBackups) {
            foreach ($backupPath in @($zipBackupPath, $checksumBackupPath)) {
                if (Test-Path -LiteralPath $backupPath -PathType Leaf) {
                    Remove-Item -LiteralPath $backupPath -Force -ErrorAction SilentlyContinue
                }
            }
        }
    }
    if (-not $zipInstalled -or -not $checksumInstalled) {
        throw "ZIP and external checksum were not installed as one validated release pair."
    }
}

Write-Host "Windows package staging created:"
Write-Host "  $targetFull"
Write-Host "Kind: $PackageKind"
Write-Host "Status: $($manifest.status)"
if ($zipPath -ne "") {
    Write-Host "Zip:"
    Write-Host "  $zipPath"
    Write-Host "Checksum:"
    Write-Host "  $zipChecksumPath"
}
if ($missing.Count -gt 0) {
    Write-Host ""
    Write-Host "Missing runtime payloads:"
    foreach ($item in $missing) {
        Write-Host "  - $item"
    }
}
