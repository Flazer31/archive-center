param(
    [string]$OutputRoot,
    [string]$PackageName = "",
    [string]$PackageKind = "full",
    [string]$PackageVersion = "2.3",
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

function Set-RuntimeDefaultsInEnvExample([string]$Path, [string]$RuntimeProfile, [string]$VectorMode) {
    if (-not (Test-Path -LiteralPath $Path -PathType Leaf)) {
        throw "Missing env example file: $Path"
    }
    $text = [System.IO.File]::ReadAllText($Path, [System.Text.Encoding]::UTF8)
    $text = [regex]::Replace($text, '(?m)^AC_RUNTIME_PROFILE=.*$', "AC_RUNTIME_PROFILE=$RuntimeProfile")
    $text = [regex]::Replace($text, '(?m)^AC_VECTOR_MODE=.*$', "AC_VECTOR_MODE=$VectorMode")
    $utf8NoBom = [System.Text.UTF8Encoding]::new($false)
    [System.IO.File]::WriteAllText($Path, $text, $utf8NoBom)
}

function Set-CopiedPackageKindText([string]$Path, [string]$PackageKind, [string]$PackageVersion) {
    if (-not (Test-Path -LiteralPath $Path -PathType Leaf)) {
        return
    }
    $version = if ([string]::IsNullOrWhiteSpace($PackageVersion)) { "2.3" } else { $PackageVersion.Trim() }
    $text = [System.IO.File]::ReadAllText($Path, [System.Text.Encoding]::UTF8)
    $text = $text.Replace("Archive Center 2.1 Windows Full Package", "Archive Center $version Windows Package")
    $text = $text.Replace("Starting Archive Center 2.1 full package", "Starting Archive Center $version package")
    $utf8NoBom = [System.Text.UTF8Encoding]::new($false)
    [System.IO.File]::WriteAllText($Path, $text, $utf8NoBom)
}

function Set-CopiedPackageVersionText([string]$Root, [string]$PackageVersion) {
    $version = if ([string]::IsNullOrWhiteSpace($PackageVersion)) { "2.3" } else { $PackageVersion.Trim() }
    $utf8NoBom = [System.Text.UTF8Encoding]::new($false)
    $patterns = @("*.md", "*.txt", "*.bat", "*.cmd", "*.ps1", "*.sh", "*.command")
    foreach ($pattern in $patterns) {
        Get-ChildItem -LiteralPath $Root -Filter $pattern -File -Recurse -ErrorAction SilentlyContinue | ForEach-Object {
            $text = [System.IO.File]::ReadAllText($_.FullName, [System.Text.Encoding]::UTF8)
            $next = $text.Replace("Archive Center 2.1", "Archive Center $version")
            $next = $next.Replace("archivecenter2.1", ("archivecenter" + ($version -replace '\s+', '').ToLowerInvariant()))
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
    $files = Get-ChildItem -LiteralPath $Root -Recurse -File -ErrorAction SilentlyContinue |
        Where-Object {
            $rel = Get-RelativePackagePath $_.FullName $Root
            ($payloadExts -contains $_.Extension.ToLowerInvariant()) -and ($selfFiles -notcontains $rel)
        } |
        Sort-Object FullName

    $items = @()
    $sumLines = @()
    foreach ($file in $files) {
        $rel = Get-RelativePackagePath $file.FullName $Root
        $ext = $file.Extension.ToLowerInvariant()
        $hash = Get-FileHash -LiteralPath $file.FullName -Algorithm SHA256
        $sha = $hash.Hash.ToUpperInvariant()
        $signature = Get-PackageSignatureSummary $file.FullName $ext
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
        scope = "executable_and_script_payloads"
        package_root = $Root
        automatic_defender_exclusions = $false
        checked_files = $items.Count
        unsigned_or_untrusted_signature_count = $unsigned.Count
        files = $items
    }
    $manifest | ConvertTo-Json -Depth 8 | Set-Content -LiteralPath (Join-Path $Root "PACKAGE_FILE_MANIFEST.json") -Encoding UTF8
    $sumLines | Set-Content -LiteralPath (Join-Path $Root "SHA256SUMS.txt") -Encoding ASCII
    return [ordered]@{
        file_manifest = "PACKAGE_FILE_MANIFEST.json"
        sha256sums = "SHA256SUMS.txt"
        scope = "executable_and_script_payloads"
        checked_files = $items.Count
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
Push-Location $goServiceRoot
try {
    & go build -buildvcs=false -trimpath -ldflags "-s -w" -o (Join-Path $targetFull "bin\archive-center-go.exe") ./cmd/archive-center-go
    if ($LASTEXITCODE -ne 0) {
        throw "go build archive-center-go failed."
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
Set-RuntimeDefaultsInEnvExample (Join-Path $targetFull ".env.full.example") $runtimeProfileDefault $vectorModeDefault
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
    milvus_included = $false
    includes_runtime_binaries = $releaseReady
    runtime_profile_default = $runtimeProfileDefault
    vector_mode_default = $vectorModeDefault
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
        "Milvus runtime/data",
        "backup/release/deploy outputs"
    )
    missing = $missing
}
$manifest | ConvertTo-Json -Depth 8 | Set-Content -LiteralPath (Join-Path $targetFull "FULL_PACKAGE_MANIFEST.json") -Encoding UTF8

$zipPath = ""
if ($Zip) {
    if (-not $releaseReady -and -not $AllowMissingRuntimePayloads) {
        throw "Refusing to zip a non-ready package."
    }
    $zipPath = Join-Path $outputRootFull ($PackageName + ".zip")
    if (Test-Path -LiteralPath $zipPath) {
        Remove-Item -LiteralPath $zipPath -Force
    }
    Compress-Archive -LiteralPath $targetFull -DestinationPath $zipPath -Force
}

Write-Host "Windows package staging created:"
Write-Host "  $targetFull"
Write-Host "Kind: $PackageKind"
Write-Host "Status: $($manifest.status)"
if ($zipPath -ne "") {
    Write-Host "Zip:"
    Write-Host "  $zipPath"
}
if ($missing.Count -gt 0) {
    Write-Host ""
    Write-Host "Missing runtime payloads:"
    foreach ($item in $missing) {
        Write-Host "  - $item"
    }
}
