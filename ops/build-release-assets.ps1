param(
    [string]$PackageVersion = "4.3.0",
    [string]$OutputRoot = "",
    [switch]$ForceRefresh
)

$ErrorActionPreference = "Stop"
Set-StrictMode -Version 2.0

$repoRoot = Split-Path -Parent $PSScriptRoot
if ([string]::IsNullOrWhiteSpace($OutputRoot)) {
    $OutputRoot = Join-Path $repoRoot ("_release-builds\{0}" -f $PackageVersion.Trim())
}
$outputRootFull = $ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath($OutputRoot)
New-Item -ItemType Directory -Path $outputRootFull -Force | Out-Null

$windowsBuilder = Join-Path $PSScriptRoot "build-full-package.ps1"
$posixBuilder = Join-Path $PSScriptRoot "build-posix-managed-packages.ps1"
$refreshArgs = if ($ForceRefresh) { @{ ForceRefresh = $true } } else { @{} }

& $windowsBuilder -OutputRoot $outputRootFull -PackageVersion $PackageVersion -PackageKind managed -Zip @refreshArgs
& $windowsBuilder -OutputRoot $outputRootFull -PackageVersion $PackageVersion -PackageKind managed -UpdateZip -ForceRefresh
& $posixBuilder -OutputRoot $outputRootFull -PackageVersion $PackageVersion -Zip @refreshArgs

$generatedNames = @(
    "Archive Center $PackageVersion Windows Auto Install Package.zip",
    "Archive Center $PackageVersion Windows Update Package.zip",
    "Archive Center $PackageVersion Linux x64 Auto Install Package.zip",
    "Archive Center $PackageVersion Linux arm64 Auto Install Package.zip",
    "Archive Center $PackageVersion macOS Intel Auto Install Package.zip",
    "Archive Center $PackageVersion macOS Apple Silicon Auto Install Package.zip",
    "Archive Center $PackageVersion Termux arm64 Auto Install Package.zip"
)
$expectedNames = @($generatedNames | ForEach-Object { $_.Replace(' ', '.') })

for ($index = 0; $index -lt $generatedNames.Count; $index++) {
    $generatedPath = Join-Path $outputRootFull $generatedNames[$index]
    $releasePath = Join-Path $outputRootFull $expectedNames[$index]
    if (-not (Test-Path -LiteralPath $generatedPath -PathType Leaf)) {
        throw "Release asset was not generated: $generatedPath"
    }
    if (Test-Path -LiteralPath $releasePath -PathType Leaf) {
        Remove-Item -LiteralPath $releasePath -Force
    }
    Move-Item -LiteralPath $generatedPath -Destination $releasePath
}

$checksumLines = foreach ($name in $expectedNames) {
    $path = Join-Path $outputRootFull $name
    if (-not (Test-Path -LiteralPath $path -PathType Leaf)) {
        throw "Release asset was not generated: $path"
    }
    $sha256 = (Get-FileHash -LiteralPath $path -Algorithm SHA256).Hash.ToLowerInvariant()
    "$sha256  $name"
}

$checksumPath = Join-Path $outputRootFull "SHA256SUMS-$PackageVersion.txt"
[System.IO.File]::WriteAllText(
    $checksumPath,
    (($checksumLines -join "`n") + "`n"),
    [System.Text.Encoding]::ASCII
)

Write-Host "Archive Center $PackageVersion release assets created:"
foreach ($name in $expectedNames) {
    Write-Host "  $(Join-Path $outputRootFull $name)"
}
Write-Host "  $checksumPath"
