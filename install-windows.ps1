$ErrorActionPreference = "Stop"

$releaseHelperUrl = "https://raw.githubusercontent.com/Flazer31/archive-center/main/scripts/install-github-release.ps1"
$externalOperationTimeoutSeconds = 1800

if ([string]::IsNullOrWhiteSpace($env:LOCALAPPDATA)) {
    throw "LOCALAPPDATA is required for this fresh install."
}

$installDir = Join-Path $env:LOCALAPPDATA "ArchiveCenter"
if (Test-Path -LiteralPath $installDir) {
    throw "Fresh install only: $installDir already exists. Updates use a separate path; nothing was changed."
}

$helperPath = Join-Path ([System.IO.Path]::GetTempPath()) ("archive-center-release-helper-" + [Guid]::NewGuid().ToString("N") + ".ps1")
$reservationActive = $false
try {
    Invoke-WebRequest -Uri $releaseHelperUrl -OutFile $helperPath -TimeoutSec $externalOperationTimeoutSeconds

    try {
        New-Item -ItemType Directory -Path $installDir -ErrorAction Stop | Out-Null
        $reservationActive = $true
        Set-Content -LiteralPath (Join-Path $installDir ".archive-center-fresh-install-reservation") -Value "fresh-install" -Encoding ASCII
    } catch {
        throw "Fresh install only: $installDir appeared during setup. Updates use a separate path; nothing was overwritten."
    }

    $windowsPowerShell = Join-Path $env:SystemRoot "System32\WindowsPowerShell\v1.0\powershell.exe"
    if (-not (Test-Path -LiteralPath $windowsPowerShell -PathType Leaf)) {
        throw "Windows PowerShell was not found: $windowsPowerShell"
    }
    & $windowsPowerShell `
        -NoProfile `
        -ExecutionPolicy Bypass `
        -File $helperPath `
        -InstallDir $installDir `
        -ExternalOperationTimeoutSeconds $externalOperationTimeoutSeconds `
        -Start
    if ($LASTEXITCODE -ne 0) {
        throw "Archive Center release installer failed with exit code $LASTEXITCODE."
    }
    $reservationActive = $false
    Remove-Item -LiteralPath (Join-Path $installDir ".archive-center-fresh-install-reservation") -Force -ErrorAction SilentlyContinue
} finally {
    Remove-Item -LiteralPath $helperPath -Force -ErrorAction SilentlyContinue
    if ($reservationActive -and (Test-Path -LiteralPath $installDir -PathType Container)) {
        Remove-Item -LiteralPath $installDir -Recurse -Force
    }
}
