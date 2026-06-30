param(
    [string]$ProtectedEnvFile = ".\.env.full.local.protected",
    [string]$OutputEnvFile = ".\.env.full.local",
    [switch]$Force
)

$ErrorActionPreference = "Stop"

if (-not (Test-Path -LiteralPath $ProtectedEnvFile -PathType Leaf)) {
    throw "Protected env file not found: $ProtectedEnvFile"
}
if ((Test-Path -LiteralPath $OutputEnvFile -PathType Leaf) -and -not $Force) {
    throw "Output env file already exists: $OutputEnvFile. Re-run with -Force to overwrite."
}

$cipherText = (Get-Content -LiteralPath $ProtectedEnvFile -Raw).Trim()
$secureText = ConvertTo-SecureString $cipherText
$bstr = [Runtime.InteropServices.Marshal]::SecureStringToBSTR($secureText)
try {
    $plainText = [Runtime.InteropServices.Marshal]::PtrToStringBSTR($bstr)
} finally {
    if ($bstr -ne [IntPtr]::Zero) {
        [Runtime.InteropServices.Marshal]::ZeroFreeBSTR($bstr)
    }
}

$outputPath = [System.IO.Path]::GetFullPath($OutputEnvFile)
[System.IO.File]::WriteAllText($outputPath, $plainText, [System.Text.UTF8Encoding]::new($false))
Write-Host "Plaintext env restored: $OutputEnvFile"
Write-Host "Run 04_protect_env_windows.bat again after editing if you want to remove plaintext."
