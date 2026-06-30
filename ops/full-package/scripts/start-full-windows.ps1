param(
    [string]$EnvFile = ".\.env.full.local",
    [string]$BindAddr = "",
    [string]$RuntimeProfile = "",
    [string]$VectorMode = "",
    [int]$MariaDBPort = 3307,
    [switch]$KeepServices
)

$ErrorActionPreference = "Stop"

function ConvertFrom-ProtectedEnvText([string]$ProtectedPath) {
    $cipherText = (Get-Content -LiteralPath $ProtectedPath -Raw).Trim()
    $secureText = ConvertTo-SecureString $cipherText
    $bstr = [Runtime.InteropServices.Marshal]::SecureStringToBSTR($secureText)
    try {
        [Runtime.InteropServices.Marshal]::PtrToStringBSTR($bstr)
    } finally {
        if ($bstr -ne [IntPtr]::Zero) {
            [Runtime.InteropServices.Marshal]::ZeroFreeBSTR($bstr)
        }
    }
}

function Import-DotEnv([string]$Path) {
    $protectedPath = "$Path.protected"
    $content = ""
    if (Test-Path -LiteralPath $protectedPath -PathType Leaf) {
        Write-Host "Using protected env: $protectedPath"
        $content = ConvertFrom-ProtectedEnvText $protectedPath
    } elseif (Test-Path -LiteralPath $Path -PathType Leaf) {
        $content = Get-Content -LiteralPath $Path -Raw
    } else {
        throw "Env file not found: $Path or $protectedPath. Copy .env.full.example to .env.full.local first."
    }
    $content -split "\r?\n" | ForEach-Object {
        $line = $_.Trim()
        if ($line -eq "" -or $line.StartsWith("#")) { return }
        $idx = $line.IndexOf("=")
        if ($idx -lt 1) { return }
        [Environment]::SetEnvironmentVariable($line.Substring(0, $idx).Trim(), $line.Substring($idx + 1).Trim(), "Process")
    }
}

function Test-PortOpen([int]$Port) {
    $client = New-Object Net.Sockets.TcpClient
    try {
        $iar = $client.BeginConnect("127.0.0.1", $Port, $null, $null)
        $ok = $iar.AsyncWaitHandle.WaitOne(500, $false)
        if ($ok) { $client.EndConnect($iar) }
        return $ok
    } catch {
        return $false
    } finally {
        $client.Close()
    }
}

function Wait-Port([int]$Port, [int]$TimeoutSeconds = 60) {
    for ($i = 0; $i -lt $TimeoutSeconds; $i++) {
        if (Test-PortOpen $Port) { return }
        Start-Sleep -Seconds 1
    }
    throw "Port did not become ready on 127.0.0.1:$Port"
}

function Join-Args([string[]]$ArgList) {
    ($ArgList | ForEach-Object {
        if ($_ -match '[\s"]') { '"' + ($_ -replace '"', '\"') + '"' } else { $_ }
    }) -join " "
}

function Unblock-PackageFile([string]$Path) {
    if ([string]::IsNullOrWhiteSpace($Path) -or -not (Test-Path -LiteralPath $Path -PathType Leaf)) {
        return
    }
    try {
        Unblock-File -LiteralPath $Path -ErrorAction SilentlyContinue
    } catch {
        # Best effort only. The process start path below still reports a clear error.
    }
}

function Start-ArchiveChildProcess {
    param(
        [Parameter(Mandatory = $true)][string]$FilePath,
        [string[]]$ArgumentList = @(),
        [string]$WorkingDirectory = ""
    )

    if ([string]::IsNullOrWhiteSpace($FilePath) -or -not (Test-Path -LiteralPath $FilePath -PathType Leaf)) {
        throw "Executable not found: $FilePath"
    }

    Unblock-PackageFile $FilePath

    $psi = New-Object System.Diagnostics.ProcessStartInfo
    $psi.FileName = $FilePath
    $psi.Arguments = Join-Args $ArgumentList
    if ([string]::IsNullOrWhiteSpace($WorkingDirectory)) {
        $psi.WorkingDirectory = Split-Path -Parent $FilePath
    } else {
        $psi.WorkingDirectory = $WorkingDirectory
    }
    $psi.UseShellExecute = $false
    $psi.CreateNoWindow = $true

    try {
        $proc = [System.Diagnostics.Process]::Start($psi)
        if ($null -eq $proc) {
            throw "Process.Start returned null."
        }
        return $proc
    } catch {
        $nativeCode = $null
        if ($_.Exception -is [System.ComponentModel.Win32Exception]) {
            $nativeCode = $_.Exception.NativeErrorCode
        } elseif ($_.Exception.InnerException -is [System.ComponentModel.Win32Exception]) {
            $nativeCode = $_.Exception.InnerException.NativeErrorCode
        }
        $message = $_.Exception.Message
        $hint = "Failed to start bundled runtime executable."
        if ($nativeCode -eq 1223 -or $message -match "(?i)cancel|cancell|operation.*canceled|user.*cancel") {
            $hint = "Windows cancelled the bundled runtime executable launch. This is commonly caused by Mark-of-the-Web, SmartScreen, Defender quarantine, or a cancelled security prompt. From the package root, run: Get-ChildItem -Recurse -File | Unblock-File"
        }
        throw "$hint`nFile: $FilePath`nOriginal error: $message"
    }
}

function Find-BundledPython([string]$Root) {
    $candidates = @(
        "runtime\Python\python.exe",
        "runtime\python\python.exe",
        "runtime\Python\Scripts\python.exe",
        "runtime\python\Scripts\python.exe",
        "runtime\ChromaDB\python.exe",
        "runtime\chromadb\python.exe",
        "runtime\ChromaDB\Scripts\python.exe",
        "runtime\chromadb\Scripts\python.exe"
    )
    foreach ($rel in $candidates) {
        $path = Join-Path $Root $rel
        if (Test-Path -LiteralPath $path -PathType Leaf) {
            return (Resolve-Path -LiteralPath $path).Path
        }
    }
    $hit = Get-ChildItem -LiteralPath (Join-Path $Root "runtime") -Recurse -File -ErrorAction SilentlyContinue |
        Where-Object { $_.Name -ieq "python.exe" } |
        Sort-Object FullName |
        Select-Object -First 1
    if ($null -eq $hit) {
        return ""
    }
    return $hit.FullName
}

function Start-BundledChromaDB {
    param(
        [Parameter(Mandatory = $true)][string]$PackageRoot,
        [Parameter(Mandatory = $true)][Uri]$Endpoint
    )

    $hostName = if ([string]::IsNullOrWhiteSpace($Endpoint.Host)) { "127.0.0.1" } else { $Endpoint.Host }
    $port = if ($Endpoint.Port -gt 0) { $Endpoint.Port } else { 8000 }
    $dataDir = Join-Path $PackageRoot ".runtime\chromadb"
    New-Item -ItemType Directory -Force -Path $dataDir | Out-Null

    $python = Find-BundledPython $PackageRoot
    if ([string]::IsNullOrWhiteSpace($python)) {
        throw "Bundled ChromaDB Python runtime not found under runtime/. Package is incomplete."
    }
    Unblock-PackageFile $python

    & $python -c "import importlib.util, sys; sys.exit(0 if importlib.util.find_spec('chromadb') else 1)"
    if ($LASTEXITCODE -ne 0) {
        throw "Bundled Python exists but does not include chromadb."
    }

    Write-Host "Starting bundled ChromaDB"
    Write-Host "  Endpoint: http://$hostName`:$port"
    Write-Host "  Data:     $dataDir"

    $chromaCode = "import sys; from chromadb.cli.cli import app; sys.argv = ['chroma', 'run', '--host', sys.argv[1], '--port', sys.argv[2], '--path', sys.argv[3]]; app()"
    Start-ArchiveChildProcess -FilePath $python -ArgumentList @("-c", $chromaCode, $hostName, ([string]$port), $dataDir) -WorkingDirectory $PackageRoot
}

function Find-MariaDBTool([string]$Root, [string[]]$Names) {
    foreach ($name in $Names) {
        $hit = Get-ChildItem -LiteralPath $Root -Recurse -File -ErrorAction SilentlyContinue |
            Where-Object { $_.Name -ieq $name } |
            Sort-Object FullName |
            Select-Object -First 1
        if ($null -ne $hit) {
            return $hit.FullName
        }
    }
    return ""
}

function Normalize-ProcessPathForStartProcess {
    $envs = [System.Environment]::GetEnvironmentVariables("Process")
    $pathValue = ""
    foreach ($key in @("Path", "PATH")) {
        if ($envs.Contains($key) -and -not [string]::IsNullOrWhiteSpace([string]$envs[$key])) {
            $pathValue = [string]$envs[$key]
            break
        }
    }
    if ([string]::IsNullOrWhiteSpace($pathValue)) {
        $machinePath = [System.Environment]::GetEnvironmentVariable("Path", "Machine")
        $userPath = [System.Environment]::GetEnvironmentVariable("Path", "User")
        $pathValue = @($machinePath, $userPath) -join ";"
    }
    [System.Environment]::SetEnvironmentVariable("PATH", $null, "Process")
    [System.Environment]::SetEnvironmentVariable("Path", $null, "Process")
    [System.Environment]::SetEnvironmentVariable("Path", $pathValue, "Process")
}

function Test-AllowedRuntimeProfile([string]$Value) {
    @("client_only", "core_lite", "vector_external", "vector_local_native", "full_local") -contains $Value
}

function Test-AllowedVectorMode([string]$Value) {
    @("off", "fallback", "external", "local_native", "local_proot", "bundled") -contains $Value
}

function Test-VectorRequiresChroma([string]$Value) {
    @("external", "local_native", "local_proot", "bundled") -contains $Value
}

function Test-LocalChromaRequested([string]$Value) {
    @("local_native", "local_proot", "bundled") -contains $Value
}

$packRoot = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
Set-Location $packRoot

$backendExe = Join-Path $packRoot "bin\archive-center-go.exe"
$pendingBackendExe = Join-Path $packRoot "bin\archive-center-go.new.exe"
if (Test-Path -LiteralPath $pendingBackendExe -PathType Leaf) {
    Copy-Item -LiteralPath $pendingBackendExe -Destination $backendExe -Force
    Remove-Item -LiteralPath $pendingBackendExe -Force
    Write-Host "Applied pending Archive Center backend update."
}

Import-DotEnv $EnvFile
if (-not [string]::IsNullOrWhiteSpace($BindAddr)) {
    $env:AC_BIND_ADDR = $BindAddr
}
$profileCandidate = if (-not [string]::IsNullOrWhiteSpace($RuntimeProfile)) { $RuntimeProfile } elseif (-not [string]::IsNullOrWhiteSpace($env:AC_RUNTIME_PROFILE)) { $env:AC_RUNTIME_PROFILE } else { "core_lite" }
$profileCandidate = $profileCandidate.Trim().ToLowerInvariant()
if (-not (Test-AllowedRuntimeProfile $profileCandidate)) {
    throw "Unsupported runtime profile: $profileCandidate"
}
$env:AC_RUNTIME_PROFILE = $profileCandidate

$vectorCandidate = if (-not [string]::IsNullOrWhiteSpace($VectorMode)) { $VectorMode } elseif (-not [string]::IsNullOrWhiteSpace($env:AC_VECTOR_MODE)) { $env:AC_VECTOR_MODE } else { "" }
if ([string]::IsNullOrWhiteSpace($vectorCandidate)) {
    switch ($env:AC_RUNTIME_PROFILE) {
        "client_only" { $vectorCandidate = "off" }
        "vector_external" { $vectorCandidate = "external" }
        "vector_local_native" { $vectorCandidate = "bundled" }
        "full_local" { $vectorCandidate = "bundled" }
        default { $vectorCandidate = "fallback" }
    }
}
$vectorCandidate = $vectorCandidate.Trim().ToLowerInvariant()
if (-not (Test-AllowedVectorMode $vectorCandidate)) {
    throw "Unsupported vector mode: $vectorCandidate"
}
if ($env:AC_RUNTIME_PROFILE -eq "client_only" -and $vectorCandidate -ne "off") {
    throw "client_only requires AC_VECTOR_MODE=off"
}
if ($env:AC_RUNTIME_PROFILE -eq "core_lite" -and $vectorCandidate -notin @("fallback", "off")) {
    throw "core_lite supports only fallback or off vector modes"
}
if ($env:AC_RUNTIME_PROFILE -eq "vector_external" -and $vectorCandidate -ne "external") {
    throw "vector_external requires AC_VECTOR_MODE=external"
}
$env:AC_VECTOR_MODE = $vectorCandidate

if ($env:AC_RUNTIME_PROFILE -eq "client_only") {
    Write-Host "Archive Center client_only profile selected."
    Write-Host "No local backend, MariaDB, or ChromaDB service will be started on this device."
    Write-Host "Configure the RisuAI plugin Bridge URL to the PC/NAS Archive Center backend."
    exit 0
}
Normalize-ProcessPathForStartProcess

$runtimeRoot = Join-Path $packRoot "runtime"
$mariadbd = Find-MariaDBTool $runtimeRoot @("mariadbd.exe", "mysqld.exe")
$installDb = Find-MariaDBTool $runtimeRoot @("mariadb-install-db.exe", "mysql_install_db.exe")
$client = Find-MariaDBTool $runtimeRoot @("mariadb.exe", "mysql.exe")
$admin = Find-MariaDBTool $runtimeRoot @("mariadb-admin.exe", "mysqladmin.exe")
foreach ($tool in @($mariadbd, $installDb, $client, $admin)) {
    if ([string]::IsNullOrWhiteSpace($tool) -or -not (Test-Path -LiteralPath $tool -PathType Leaf)) {
        throw "Bundled MariaDB runtime is incomplete."
    }
    Unblock-PackageFile $tool
}
Unblock-PackageFile $backendExe
Unblock-PackageFile (Join-Path $packRoot "bin\mariadb-schema.exe")

$dataDir = Join-Path $packRoot ".runtime\mariadb"
$logDir = Join-Path $packRoot ".runtime\logs"
New-Item -ItemType Directory -Force -Path $dataDir, $logDir | Out-Null

if (-not (Test-Path -LiteralPath (Join-Path $dataDir "mysql") -PathType Container)) {
    & $installDb "--datadir=$dataDir" "--password="
    if ($LASTEXITCODE -ne 0) {
        throw "MariaDB data directory initialization failed."
    }
}

$startedMariaDB = $null
$startedChroma = $null
try {
    if (-not (Test-PortOpen $MariaDBPort)) {
        $mariaArgs = @(
            "--no-defaults",
            "--datadir=$dataDir",
            "--port=$MariaDBPort",
            "--socket=$(Join-Path $dataDir "mysql.sock")",
            "--skip-networking=0",
            "--bind-address=127.0.0.1",
            "--pid-file=$(Join-Path $dataDir "mysqld.pid")",
            "--console"
        )
        $startedMariaDB = Start-ArchiveChildProcess -FilePath $mariadbd -ArgumentList $mariaArgs -WorkingDirectory $dataDir
        Start-Sleep -Seconds 2
        if ($startedMariaDB.HasExited) {
            throw "MariaDB exited early with code $($startedMariaDB.ExitCode). Check .runtime\mariadb and .runtime\logs for details."
        }
    }
    Wait-Port $MariaDBPort 60

    $dbName = "archive_center"
    $dbUser = "archive_center"
    $dbPassword = "archive-center-local-pass"
    if ([string]::IsNullOrWhiteSpace($env:AC_MARIADB_DSN)) {
        $env:AC_MARIADB_DSN = "${dbUser}:${dbPassword}@tcp(127.0.0.1:${MariaDBPort})/${dbName}?parseTime=true"
    }
    $sqlPassword = $dbPassword.Replace("'", "''")
    $sql = "CREATE DATABASE IF NOT EXISTS $dbName CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci; CREATE USER IF NOT EXISTS '$dbUser'@'127.0.0.1' IDENTIFIED BY '$sqlPassword'; GRANT ALL PRIVILEGES ON $dbName.* TO '$dbUser'@'127.0.0.1'; CREATE USER IF NOT EXISTS '$dbUser'@'localhost' IDENTIFIED BY '$sqlPassword'; GRANT ALL PRIVILEGES ON $dbName.* TO '$dbUser'@'localhost'; FLUSH PRIVILEGES;"
    & $client --protocol=tcp --ssl=0 -h 127.0.0.1 -P $MariaDBPort -u root -e $sql
    if ($LASTEXITCODE -ne 0) {
        throw "MariaDB bootstrap SQL failed."
    }

    if (Test-VectorRequiresChroma $env:AC_VECTOR_MODE) {
        if ($env:AC_VECTOR_MODE -eq "external") {
            if ([string]::IsNullOrWhiteSpace($env:AC_CHROMA_ENDPOINT)) {
                throw "AC_CHROMA_ENDPOINT is required for vector_external."
            }
        } else {
            if ([string]::IsNullOrWhiteSpace($env:AC_CHROMA_ENDPOINT)) {
                $env:AC_CHROMA_ENDPOINT = "http://127.0.0.1:8000"
            }
            $chromaUri = [Uri]$env:AC_CHROMA_ENDPOINT
            $chromaPort = if ($chromaUri.Port -gt 0) { $chromaUri.Port } else { 8000 }
            if (-not (Test-PortOpen $chromaPort)) {
                $startedChroma = Start-BundledChromaDB -PackageRoot $packRoot -Endpoint $chromaUri
            }
            try {
                Wait-Port $chromaPort 60
            } catch {
                Write-Host "ChromaDB failed to open port $chromaPort."
                throw
            }
        }
    } else {
        $env:AC_CHROMA_ENDPOINT = ""
    }

    $env:AC_MODE = "live"
    $env:AC_STORE_MODE = "mariadb_authority"
    if ([string]::IsNullOrWhiteSpace($env:AC_BIND_ADDR)) {
        $env:AC_BIND_ADDR = "0.0.0.0:28080"
    }
    if ([string]::IsNullOrWhiteSpace($env:AC_CHROMA_COLLECTION)) {
        $env:AC_CHROMA_COLLECTION = "archive_center_vectors"
    }
    if ([string]::IsNullOrWhiteSpace($env:AC_CHROMA_API_PATH)) {
        $env:AC_CHROMA_API_PATH = "/api/v2"
    }
    $env:AC_PROMPT_DIR = Join-Path $packRoot "prompts"

    $schemaPath = Join-Path $packRoot "migrations\001_schema.sql"
    & (Join-Path $packRoot "bin\mariadb-schema.exe") -dsn $env:AC_MARIADB_DSN -schema $schemaPath -execute
    if ($LASTEXITCODE -ne 0) {
        throw "mariadb-schema failed."
    }

Write-Host "Starting Archive Center 2.1 full package"
    Write-Host "  Go:      $($env:AC_BIND_ADDR)"
    Write-Host "  MariaDB: 127.0.0.1:$MariaDBPort"
    if (Test-VectorRequiresChroma $env:AC_VECTOR_MODE) {
        Write-Host "  Chroma:  $($env:AC_CHROMA_ENDPOINT)"
    } else {
        Write-Host "  Chroma:  disabled ($($env:AC_VECTOR_MODE))"
    }
    Write-Host "  Store:   $($env:AC_STORE_MODE)"
    Write-Host "  Profile: $($env:AC_RUNTIME_PROFILE)"
    Write-Host "  Vector:  $($env:AC_VECTOR_MODE)"
    Write-Host ""
    Write-Host "Stop with Ctrl+C."
    & $backendExe
} finally {
    if (-not $KeepServices) {
        if ($startedChroma -and -not $startedChroma.HasExited) {
            $startedChroma.Kill()
        }
        if ($startedMariaDB -and -not $startedMariaDB.HasExited) {
            $startedMariaDB.Kill()
        }
    }
}
