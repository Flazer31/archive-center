param(
    [string]$Repo = "Flazer31/archive-center",
    [string]$InstallDir = "",
    [switch]$Start
)

$ErrorActionPreference = "Stop"

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
        "windows-x64" { return "windows package" }
        "linux-x64" { return "linux x64" }
        "linux-arm64" { return "linux arm64" }
        "macos-intel" { return "macos intel" }
        "macos-apple-silicon" { return "macos apple silicon" }
        default { throw "Unsupported platform: $Platform" }
    }
}

function Find-ReleaseAsset($Release, [string]$Needle) {
    $needleLower = $Needle.ToLowerInvariant()
    foreach ($asset in $Release.assets) {
        $name = [string]$asset.name
        $lower = $name.ToLowerInvariant()
        if ($lower.EndsWith(".zip") -and $lower.Contains($needleLower) -and $lower.Contains("archive center")) {
            return $asset
        }
    }
    return $null
}

function Find-SHA256Asset($Release) {
    foreach ($asset in $Release.assets) {
        $name = [string]$asset.name
        $lower = $name.ToLowerInvariant()
        if ($lower.StartsWith("sha256sums") -and $lower.EndsWith(".txt")) {
            return $asset
        }
    }
    return $null
}

function Get-ExpectedSHA256([string]$SumsPath, [string]$AssetName) {
    foreach ($line in Get-Content -LiteralPath $SumsPath) {
        $parts = $line.Trim() -split "\s+"
        if ($parts.Count -ge 2 -and $parts[1].TrimStart("*") -eq $AssetName) {
            return $parts[0].ToLowerInvariant()
        }
    }
    return ""
}

if ($Repo -notmatch '^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$') {
    throw "Repo must be OWNER/REPO."
}

$platform = Get-ArchiveCenterPlatform
$needle = Get-AssetNeedle $platform
$apiUrl = "https://api.github.com/repos/$Repo/releases/latest"
$headers = @{ "Accept" = "application/vnd.github+json"; "User-Agent" = "Archive-Center-Installer" }
$release = Invoke-RestMethod -Method Get -Uri $apiUrl -Headers $headers
$asset = Find-ReleaseAsset $release $needle
if ($null -eq $asset) {
    throw "No release package asset matched platform $platform."
}
$sumsAsset = Find-SHA256Asset $release
if ($null -eq $sumsAsset) {
    throw "Release has no SHA256SUMS asset."
}

$workDir = Join-Path ([System.IO.Path]::GetTempPath()) ("archive-center-update-" + [guid]::NewGuid().ToString("N"))
New-Item -ItemType Directory -Force -Path $workDir | Out-Null
try {
    $zipPath = Join-Path $workDir ([string]$asset.name)
    $sumsPath = Join-Path $workDir ([string]$sumsAsset.name)
    Invoke-WebRequest -Uri ([string]$sumsAsset.browser_download_url) -Headers $headers -OutFile $sumsPath
    Invoke-WebRequest -Uri ([string]$asset.browser_download_url) -Headers $headers -OutFile $zipPath

    $expected = Get-ExpectedSHA256 $sumsPath ([string]$asset.name)
    if ([string]::IsNullOrWhiteSpace($expected)) {
        throw "SHA256SUMS did not contain $($asset.name)."
    }
    $actual = (Get-FileHash -Algorithm SHA256 -LiteralPath $zipPath).Hash.ToLowerInvariant()
    if ($actual -ne $expected) {
        throw "SHA256 mismatch for $($asset.name)."
    }

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
    Set-Content -LiteralPath (Join-Path $InstallDir "current-package.txt") -Value $packageRoot -Encoding UTF8
    Set-Content -LiteralPath (Join-Path $InstallDir "current-version.txt") -Value ([string]$release.tag_name) -Encoding UTF8

    Write-Host "Installed Archive Center $($release.tag_name)"
    Write-Host "  Platform: $platform"
    Write-Host "  Package:  $packageRoot"
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
