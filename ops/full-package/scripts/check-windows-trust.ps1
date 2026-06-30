param(
    [string]$PackageRoot = "",
    [string]$OutFile = ""
)

$ErrorActionPreference = "Stop"

function Resolve-FullPath([string]$Path) {
    $executionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath($Path)
}

function Get-RelativePath([string]$Path, [string]$Root) {
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

function Get-SignatureSummary([string]$Path, [string]$Extension) {
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

if ([string]::IsNullOrWhiteSpace($PackageRoot)) {
    $PackageRoot = Join-Path $PSScriptRoot ".."
}
$root = Resolve-FullPath $PackageRoot
if (-not (Test-Path -LiteralPath $root -PathType Container)) {
    throw "Package root not found: $root"
}
if ([string]::IsNullOrWhiteSpace($OutFile)) {
    $outDir = Join-Path $root ".runtime\reports"
    New-Item -ItemType Directory -Force -Path $outDir | Out-Null
    $OutFile = Join-Path $outDir "windows-trust-report.json"
}

$interestingExts = @(".exe", ".dll", ".ps1", ".psm1", ".bat", ".cmd", ".msi")
$files = Get-ChildItem -LiteralPath $root -Recurse -File -ErrorAction SilentlyContinue |
    Where-Object { $interestingExts -contains $_.Extension.ToLowerInvariant() } |
    Sort-Object FullName

$items = @()
foreach ($file in $files) {
    $ext = $file.Extension.ToLowerInvariant()
    $hash = Get-FileHash -LiteralPath $file.FullName -Algorithm SHA256
    $signature = Get-SignatureSummary $file.FullName $ext
    $items += [ordered]@{
        path = Get-RelativePath $file.FullName $root
        extension = $ext
        size_bytes = [int64]$file.Length
        sha256 = $hash.Hash.ToUpperInvariant()
        has_mark_of_the_web = Test-ZoneIdentifier $file.FullName
        signature = $signature
    }
}

$unsigned = @($items | Where-Object { $_.signature.checked -and $_.signature.status -ne "Valid" })
$motw = @($items | Where-Object { $_.has_mark_of_the_web })
$report = [ordered]@{
    schema_version = "archive-center.windows.trust-report.v1"
    generated_at = [DateTimeOffset]::UtcNow.ToString("o")
    package_root = $root
    note = "Read-only diagnostic report. This script does not disable Defender, add exclusions, or change package files."
    local_services = [ordered]@{
        backend = "archive-center-go.exe on 0.0.0.0:28080 by default"
        mariadb = "bundled MariaDB on 127.0.0.1:3307 by default"
        chromadb = "bundled ChromaDB on 127.0.0.1:8000 only for full_local/bundled vector mode"
    }
    summary = [ordered]@{
        checked_files = $items.Count
        unsigned_or_untrusted_signature_count = $unsigned.Count
        mark_of_the_web_count = $motw.Count
    }
    recommended_false_positive_fields = [ordered]@{
        submitter_category = "Software developer"
        detection_type = "Incorrectly detected as malware/malicious"
        include = @(
            "Exact detected file path",
            "SHA256 from this report",
            "Microsoft Defender detection name",
            "Archive Center package version",
            "Confirm the file came from the official Archive Center package"
        )
    }
    files = $items
}

New-Item -ItemType Directory -Force -Path (Split-Path -Parent $OutFile) | Out-Null
$report | ConvertTo-Json -Depth 8 | Set-Content -LiteralPath $OutFile -Encoding UTF8
$report | ConvertTo-Json -Depth 8
