param(
    [string]$EnvFile = ".\.env.full.local",
    [switch]$RemovePlaintext,
    [switch]$Force
)

$ErrorActionPreference = "Stop"

if (-not (Test-Path -LiteralPath $EnvFile -PathType Leaf)) {
    throw "Env file not found: $EnvFile"
}

$protectedPath = "$EnvFile.protected"
if ((Test-Path -LiteralPath $protectedPath -PathType Leaf) -and -not $Force) {
    throw "Protected env already exists: $protectedPath. Re-run with -Force to overwrite."
}

$plainText = [System.IO.File]::ReadAllText((Resolve-Path -LiteralPath $EnvFile).Path, [System.Text.Encoding]::UTF8)
if ([string]::IsNullOrWhiteSpace($plainText)) {
    throw "Refusing to protect an empty env file."
}

$secureText = ConvertTo-SecureString $plainText -AsPlainText -Force
$cipherText = ConvertFrom-SecureString $secureText
Set-Content -LiteralPath $protectedPath -Value $cipherText -Encoding ASCII

if ($RemovePlaintext) {
    Remove-Item -LiteralPath $EnvFile -Force
}

Write-Host "Protected env written: $protectedPath"
if ($RemovePlaintext) {
    Write-Host "Plaintext env removed: $EnvFile"
} else {
    Write-Host "Plaintext env kept: $EnvFile"
}
