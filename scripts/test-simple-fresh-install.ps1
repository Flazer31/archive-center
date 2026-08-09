$ErrorActionPreference = "Stop"

function Assert-True([bool]$Condition, [string]$Message) {
    if (-not $Condition) {
        throw "simple fresh-install test failed: $Message"
    }
}

function Get-TreeSnapshot([string]$Root) {
    $rootFull = [System.IO.Path]::GetFullPath($Root)
    $items = @((Get-Item -LiteralPath $rootFull)) + @(Get-ChildItem -LiteralPath $rootFull -Force -Recurse)
    @($items | Sort-Object FullName | ForEach-Object {
        $relative = $_.FullName.Substring($rootFull.Length).TrimStart('\', '/')
        if ($_.PSIsContainer) {
            "path=$relative|dir=True|attributes=$($_.Attributes)"
        } else {
            $hash = (Get-FileHash -LiteralPath $_.FullName -Algorithm SHA256).Hash
            "path=$relative|dir=False|length=$($_.Length)|mtime=$($_.LastWriteTimeUtc.Ticks)|attributes=$($_.Attributes)|sha256=$hash"
        }
    }) -join "`n"
}

$repoRoot = Split-Path -Parent $PSScriptRoot
$installerPath = Join-Path $repoRoot "install-windows.ps1"
$installerSource = Get-Content -LiteralPath $installerPath -Raw -Encoding UTF8
$testRoot = Join-Path ([System.IO.Path]::GetTempPath()) ("archive-center-simple-install-test-" + [guid]::NewGuid().ToString("N"))
$packageTestRoot = Join-Path $repoRoot (".tmp-simple-fresh-install-package-" + [guid]::NewGuid().ToString("N"))
$originalLocalAppData = $env:LOCALAPPDATA
$originalExecutionPolicy = Get-ExecutionPolicy -Scope Process
$global:ACSimpleInstallDownloadCalls = 0

function Invoke-WebRequest {
    param(
        [string]$Uri,
        [string]$OutFile,
        [int]$TimeoutSec
    )
    $global:ACSimpleInstallDownloadCalls++
    @'
param(
    [string]$InstallDir,
    [Nullable[int]]$ExternalOperationTimeoutSeconds,
    [switch]$Start
)
if ($env:AC_TEST_HELPER_FAIL -eq "1") {
    exit 42
}
@(
    "install_dir=$InstallDir",
    "timeout=$ExternalOperationTimeoutSeconds",
    "start=$($Start.IsPresent)"
) | Set-Content -LiteralPath $env:AC_TEST_HELPER_LOG -Encoding UTF8
'@ | Set-Content -LiteralPath $OutFile -Encoding UTF8
}

New-Item -ItemType Directory -Path $testRoot | Out-Null
try {
    Set-ExecutionPolicy -Scope Process -ExecutionPolicy Restricted -Force
    $existingLocal = Join-Path $testRoot "existing-local"
    $existingInstall = Join-Path $existingLocal "ArchiveCenter"
    New-Item -ItemType Directory -Path $existingInstall -Force | Out-Null
    $sentinelPath = Join-Path $existingInstall "sentinel.txt"
    Set-Content -LiteralPath $sentinelPath -Value "preserve-me" -Encoding UTF8
    $nestedDir = Join-Path $existingInstall "nested"
    New-Item -ItemType Directory -Path $nestedDir | Out-Null
    Set-Content -LiteralPath (Join-Path $nestedDir "value.txt") -Value "nested-preserve" -Encoding UTF8
    $existingSnapshotBefore = Get-TreeSnapshot $existingInstall
    $env:LOCALAPPDATA = $existingLocal
    $global:ACSimpleInstallDownloadCalls = 0
    $existingFailed = $false
    try {
        Invoke-Expression $installerSource
    } catch {
        $existingFailed = $_.Exception.Message.Contains("Fresh install only")
    }
    Assert-True $existingFailed "existing Windows install was not rejected explicitly"
    Assert-True ($global:ACSimpleInstallDownloadCalls -eq 0) "existing Windows install reached the network boundary"
    Assert-True ((Get-TreeSnapshot $existingInstall) -ceq $existingSnapshotBefore) "existing Windows install tree changed"

    $cleanLocal = Join-Path $testRoot "clean-local"
    New-Item -ItemType Directory -Path $cleanLocal -Force | Out-Null
    $env:LOCALAPPDATA = $cleanLocal
    $env:AC_TEST_HELPER_LOG = Join-Path $testRoot "windows-helper.log"
    $global:ACSimpleInstallDownloadCalls = 0
    Invoke-Expression $installerSource
    Assert-True ($global:ACSimpleInstallDownloadCalls -eq 1) "clean Windows install did not download exactly one helper"
    $helperLines = @(Get-Content -LiteralPath $env:AC_TEST_HELPER_LOG -Encoding UTF8)
    Assert-True ($helperLines -contains "install_dir=$(Join-Path $cleanLocal 'ArchiveCenter')") "Windows install directory drifted"
    Assert-True ($helperLines -contains "timeout=1800") "Windows default timeout drifted"
    Assert-True ($helperLines -contains "start=True") "Windows package was not started"

    $posixOutputRoot = $packageTestRoot
    $posixBuilder = Join-Path $repoRoot "ops\build-posix-managed-packages.ps1"
    & powershell.exe `
        -NoProfile `
        -ExecutionPolicy Bypass `
        -File $posixBuilder `
        -OutputRoot $posixOutputRoot `
        -TargetFilter macos-arm64 `
        -PackageVersion 3.9.0-timeout-contract
    Assert-True ($LASTEXITCODE -eq 0) "macOS launcher contract package build failed"

    $macLauncher = Get-ChildItem -LiteralPath $posixOutputRoot -Recurse -File -Filter "Start Archive Center macOS.command" |
        Select-Object -First 1
    Assert-True ($null -ne $macLauncher) "generated macOS public launcher was not found"
    $bashCommand = Get-Command bash.exe -ErrorAction SilentlyContinue
    $bashPath = if ($null -ne $bashCommand) { $bashCommand.Source } else { Join-Path $env:ProgramFiles "Git\bin\bash.exe" }
    Assert-True (Test-Path -LiteralPath $bashPath -PathType Leaf) "Git Bash was not found for generated macOS launcher verification"
    Assert-True (-not $macLauncher.FullName.Contains("'")) "generated macOS launcher path contains an unsupported apostrophe"
    $posixLauncher = (& $bashPath -lc "cygpath -u '$($macLauncher.FullName)'").Trim()
    Assert-True (-not [string]::IsNullOrWhiteSpace($posixLauncher)) "generated macOS launcher path was not converted for POSIX execution"

    $defaultTrace = @(& $bashPath -lc "unset AC_EXTERNAL_OPERATION_TIMEOUT_SECONDS AC_REQUEST_TIMEOUT_SECONDS AC_READINESS_TIMEOUT_SECONDS AC_READINESS_POLL_INTERVAL_SECONDS; sh -x '$posixLauncher' --preflight 2>&1")
    Assert-True ($LASTEXITCODE -eq 0) "generated macOS launcher default preflight failed"
    $defaultTraceText = $defaultTrace -join "`n"
    foreach ($expected in @(
        "AC_EXTERNAL_OPERATION_TIMEOUT_SECONDS=1800",
        "AC_REQUEST_TIMEOUT_SECONDS=30",
        "AC_READINESS_TIMEOUT_SECONDS=180",
        "AC_READINESS_POLL_INTERVAL_SECONDS=1"
    )) {
        Assert-True $defaultTraceText.Contains($expected) "generated macOS launcher did not pass $expected"
    }

    $overrideTrace = @(& $bashPath -lc "AC_EXTERNAL_OPERATION_TIMEOUT_SECONDS=17 AC_REQUEST_TIMEOUT_SECONDS=18 AC_READINESS_TIMEOUT_SECONDS=19 AC_READINESS_POLL_INTERVAL_SECONDS=20 sh -x '$posixLauncher' --preflight 2>&1")
    Assert-True ($LASTEXITCODE -eq 0) "generated macOS launcher override preflight failed"
    $overrideTraceText = $overrideTrace -join "`n"
    foreach ($expected in @(
        "AC_EXTERNAL_OPERATION_TIMEOUT_SECONDS=17",
        "AC_REQUEST_TIMEOUT_SECONDS=18",
        "AC_READINESS_TIMEOUT_SECONDS=19",
        "AC_READINESS_POLL_INTERVAL_SECONDS=20"
    )) {
        Assert-True $overrideTraceText.Contains($expected) "generated macOS launcher did not preserve $expected"
    }

    $failedLocal = Join-Path $testRoot "failed-local"
    New-Item -ItemType Directory -Path $failedLocal -Force | Out-Null
    $env:LOCALAPPDATA = $failedLocal
    $env:AC_TEST_HELPER_FAIL = "1"
    $failed = $false
    try {
        Invoke-Expression $installerSource
    } catch {
        $failed = $true
    }
    Assert-True $failed "failed Windows helper was reported as success"
    Assert-True (-not (Test-Path -LiteralPath (Join-Path $failedLocal "ArchiveCenter"))) "failed Windows helper left the reserved install root behind"
    Remove-Item Env:AC_TEST_HELPER_FAIL -ErrorAction SilentlyContinue
    $global:LASTEXITCODE = 0

    $readme = Get-Content -LiteralPath (Join-Path $repoRoot "README.md") -Raw -Encoding UTF8
    Assert-True $readme.Contains("irm https://raw.githubusercontent.com/Flazer31/archive-center/main/install-windows.ps1 | iex") "README Windows command drifted"
    Write-Host "simple Windows and generated macOS launcher contracts: ok"
} finally {
    $env:LOCALAPPDATA = $originalLocalAppData
    Set-ExecutionPolicy -Scope Process -ExecutionPolicy $originalExecutionPolicy -Force
    Remove-Item Env:AC_TEST_HELPER_LOG -ErrorAction SilentlyContinue
    Remove-Item Env:AC_TEST_HELPER_FAIL -ErrorAction SilentlyContinue
    Remove-Variable ACSimpleInstallDownloadCalls -Scope Global -ErrorAction SilentlyContinue
    Remove-Item -LiteralPath $testRoot -Recurse -Force -ErrorAction SilentlyContinue
    if ($packageTestRoot.StartsWith($repoRoot + [System.IO.Path]::DirectorySeparatorChar, [System.StringComparison]::OrdinalIgnoreCase) -and
        [System.IO.Path]::GetFileName($packageTestRoot).StartsWith(".tmp-simple-fresh-install-package-", [System.StringComparison]::Ordinal)) {
        Remove-Item -LiteralPath $packageTestRoot -Recurse -Force -ErrorAction SilentlyContinue
    }
}
