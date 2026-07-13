param(
    [string]$BindAddr = "127.0.0.1:28080",
    [int]$MariaDBPort = 3307,
    [string]$RuntimeDir = "",
    [string]$ProviderBin = "",
    [string]$GoBinary = "",
    [string]$GoCacheDir = "",
    [string]$Out = "",
    [switch]$UseExistingBinary,
    [switch]$WriteSmoke,
    [switch]$SmokeOnly
)

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

function Resolve-FullPath {
    param([string]$Path)
    if ([string]::IsNullOrWhiteSpace($Path)) {
        return ""
    }
    return [System.IO.Path]::GetFullPath($Path)
}

function Join-CommandArgs {
    param([string[]]$ArgList)
    $parts = @()
    foreach ($arg in $ArgList) {
        if ($arg -match '[\s"]') {
            $parts += '"' + ($arg -replace '"', '\"') + '"'
        } else {
            $parts += $arg
        }
    }
    return ($parts -join " ")
}

function Test-PortOpen {
    param([int]$Port)
    $client = New-Object System.Net.Sockets.TcpClient
    try {
        $iar = $client.BeginConnect("127.0.0.1", $Port, $null, $null)
        if (-not $iar.AsyncWaitHandle.WaitOne(400)) {
            return $false
        }
        $client.EndConnect($iar)
        return $true
    } catch {
        return $false
    } finally {
        $client.Close()
    }
}

function Get-FreeTcpPort {
    $listener = [System.Net.Sockets.TcpListener]::new([System.Net.IPAddress]::Parse("127.0.0.1"), 0)
    try {
        $listener.Start()
        return [int]$listener.LocalEndpoint.Port
    } finally {
        $listener.Stop()
    }
}

function Wait-TcpPort {
    param([int]$Port, [int]$TimeoutSeconds = 60)
    for ($i = 0; $i -lt $TimeoutSeconds; $i++) {
        if (Test-PortOpen $Port) {
            return
        }
        Start-Sleep -Seconds 1
    }
    throw "TCP port did not become ready on 127.0.0.1:$Port"
}

function Find-MariaDBProvider {
    param([string]$RepoRoot, [string]$WorkspaceRoot, [string]$ExplicitProvider)

    if (-not [string]::IsNullOrWhiteSpace($ExplicitProvider)) {
        $resolved = Resolve-FullPath $ExplicitProvider
        if (Test-Path -LiteralPath $resolved -PathType Leaf) {
            return $resolved
        }
        throw "MariaDB provider not found: $ExplicitProvider"
    }

    if (-not [string]::IsNullOrWhiteSpace($env:AC_MARIADB_PROVIDER_BIN)) {
        $resolved = Resolve-FullPath $env:AC_MARIADB_PROVIDER_BIN
        if (Test-Path -LiteralPath $resolved -PathType Leaf) {
            return $resolved
        }
    }

    $roots = @(
        (Join-Path $RepoRoot "runtime\MariaDB"),
        (Join-Path $RepoRoot "runtime\mariadb"),
        (Join-Path $WorkspaceRoot ".runtime-cache\archive-center-2.0-install\runtime\MariaDB")
    )
    foreach ($root in $roots) {
        if (-not (Test-Path -LiteralPath $root -PathType Container)) {
            continue
        }
        $hit = Get-ChildItem -LiteralPath $root -Recurse -File -ErrorAction SilentlyContinue |
            Where-Object { $_.Name -in @("mariadbd.exe", "mysqld.exe") } |
            Select-Object -First 1
        if ($hit) {
            return $hit.FullName
        }
    }

    throw "Bundled MariaDB provider was not found. Stage the managed runtime first; normal users must not install MariaDB manually."
}

function Start-ManagedProcess {
    param(
        [string]$FileName,
        [string[]]$ArgList,
        [string]$WorkingDirectory = "",
        [hashtable]$Environment = @{},
        [string]$Stdout = "",
        [string]$Stderr = ""
    )
    $psi = New-Object System.Diagnostics.ProcessStartInfo
    $psi.FileName = $FileName
    $argumentListProperty = $psi.PSObject.Properties["ArgumentList"]
    if ($null -ne $argumentListProperty) {
        foreach ($arg in $ArgList) {
            [void]$psi.ArgumentList.Add($arg)
        }
    } else {
        $psi.Arguments = Join-CommandArgs -ArgList $ArgList
    }
    if (-not [string]::IsNullOrWhiteSpace($WorkingDirectory)) {
        $psi.WorkingDirectory = $WorkingDirectory
    }
    $psi.UseShellExecute = $false
    $psi.CreateNoWindow = $true
    if (-not [string]::IsNullOrWhiteSpace($Stdout)) {
        $psi.RedirectStandardOutput = $true
    }
    if (-not [string]::IsNullOrWhiteSpace($Stderr)) {
        $psi.RedirectStandardError = $true
    }
    foreach ($key in $Environment.Keys) {
        $value = $Environment[$key]
        if ($null -ne $value) {
            if ($null -ne $psi.Environment) {
                $psi.Environment[$key] = [string]$value
            } else {
                $psi.EnvironmentVariables[$key] = [string]$value
            }
        }
    }
    $p = [System.Diagnostics.Process]::Start($psi)
    return $p
}

function Wait-MariaDB {
    param([string]$AdminExe, [int]$Port)
    for ($i = 0; $i -lt 60; $i++) {
        $oldPreference = $ErrorActionPreference
        $ErrorActionPreference = "Continue"
        & $AdminExe --protocol=tcp --ssl=0 -h 127.0.0.1 -P $Port -u root ping *> $null
        $ErrorActionPreference = $oldPreference
        if ($LASTEXITCODE -eq 0) {
            return
        }
        Start-Sleep -Seconds 1
    }
    throw "MariaDB did not become ready on 127.0.0.1:$Port"
}

function Wait-GoReady {
    param([string]$BaseUrl)
    for ($i = 0; $i -lt 60; $i++) {
        try {
            $ready = Invoke-RestMethod -Uri "$BaseUrl/ready" -TimeoutSec 2
            if ($ready.ready -eq $true) {
                return $ready
            }
        } catch {
        }
        Start-Sleep -Seconds 1
    }
    throw "Go backend did not become ready at $BaseUrl"
}

function Get-StatNumber {
    param([object]$Stats, [string]$Name)
    if ($null -eq $Stats) {
        return 0
    }
    $value = $Stats.$Name
    if ($null -eq $value) {
        return 0
    }
    return [int]$value
}

function Resolve-GoCommand {
    param([string]$RepoRoot, [string]$WorkspaceRoot)

    $candidates = @()
    if (-not [string]::IsNullOrWhiteSpace($env:AC_GO_BIN)) {
        $candidates += (Resolve-FullPath $env:AC_GO_BIN)
    }

    $cmd = Get-Command go -ErrorAction SilentlyContinue
    if ($cmd -and -not [string]::IsNullOrWhiteSpace($cmd.Source)) {
        $candidates += $cmd.Source
    }

    $candidates += "C:\Program Files\Go\bin\go.exe"
    $candidates += "C:\Program Files (x86)\Go\bin\go.exe"
    $candidates += (Join-Path $RepoRoot "runtime\Go\bin\go.exe")
    $candidates += (Join-Path $RepoRoot "runtime\go\bin\go.exe")
    $candidates += (Join-Path $WorkspaceRoot ".runtime-cache\archive-center-2.0-install\runtime\Go\bin\go.exe")
    $candidates += (Join-Path $WorkspaceRoot ".runtime-cache\archive-center-2.0-install\runtime\go\bin\go.exe")

    foreach ($candidate in $candidates) {
        if ([string]::IsNullOrWhiteSpace($candidate)) {
            continue
        }
        if (Test-Path -LiteralPath $candidate -PathType Leaf) {
            return (Resolve-FullPath $candidate)
        }
    }

    throw "Go executable was not found. Install Go or set AC_GO_BIN to the full path of go.exe."
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

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$repoRoot = Split-Path -Parent $scriptDir
$workspaceRoot = Split-Path -Parent $repoRoot
$goServiceRoot = Join-Path $repoRoot "go-service"
$goCmd = Resolve-GoCommand $repoRoot $workspaceRoot
Normalize-ProcessPathForStartProcess

if ([string]::IsNullOrWhiteSpace($RuntimeDir)) {
    $RuntimeDir = Join-Path $workspaceRoot ".runtime-cache\archive-center-2.0-live"
}
$RuntimeDir = Resolve-FullPath $RuntimeDir
$dataDir = Join-Path $RuntimeDir "mariadb-data"
$logDir = Join-Path $RuntimeDir "logs"
New-Item -ItemType Directory -Force -Path $dataDir, $logDir | Out-Null

$provider = Find-MariaDBProvider $repoRoot $workspaceRoot $ProviderBin
$providerBinDir = Split-Path -Parent $provider
$installDb = Join-Path $providerBinDir "mariadb-install-db.exe"
$client = Join-Path $providerBinDir "mariadb.exe"
$admin = Join-Path $providerBinDir "mariadb-admin.exe"
if (-not (Test-Path -LiteralPath $installDb -PathType Leaf)) {
    $installDb = Join-Path $providerBinDir "mysql_install_db.exe"
}
foreach ($tool in @($installDb, $client, $admin)) {
    if (-not (Test-Path -LiteralPath $tool -PathType Leaf)) {
        throw "Required MariaDB tool not found: $tool"
    }
}

if (-not (Test-Path -LiteralPath (Join-Path $dataDir "mysql") -PathType Container)) {
    & $installDb "--datadir=$dataDir" "--password="
    if ($LASTEXITCODE -ne 0) {
        throw "MariaDB data directory initialization failed."
    }
}

$startedMariaDB = $null
if (-not (Test-PortOpen $MariaDBPort)) {
    $mariaArgs = @("--no-defaults", "--datadir=$dataDir", "--port=$MariaDBPort", "--socket=$(Join-Path $dataDir "mysql.sock")", "--skip-networking=0", "--bind-address=127.0.0.1", "--pid-file=$(Join-Path $dataDir "mysqld.pid")", "--console")
    $startedMariaDB = Start-Process -FilePath $provider -ArgumentList (Join-CommandArgs -ArgList $mariaArgs) -WorkingDirectory $dataDir -WindowStyle Hidden -PassThru
    Start-Sleep -Seconds 2
    if ($startedMariaDB.HasExited) {
        throw "MariaDB exited early with code $($startedMariaDB.ExitCode). See $dataDir\mysqld.err for details."
    }
}
Wait-MariaDB $admin $MariaDBPort

$dbName = "archive_center_temp"
$dbUser = "ac_root"
$dbPassword = "archive-center-live-pass"
$dsn = "${dbUser}:${dbPassword}@tcp(127.0.0.1:${MariaDBPort})/${dbName}?parseTime=true"
$sqlPassword = $dbPassword.Replace("'", "''")
$sql = "CREATE DATABASE IF NOT EXISTS $dbName CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci; CREATE USER IF NOT EXISTS '$dbUser'@'127.0.0.1' IDENTIFIED BY '$sqlPassword'; GRANT ALL PRIVILEGES ON $dbName.* TO '$dbUser'@'127.0.0.1'; CREATE USER IF NOT EXISTS '$dbUser'@'localhost' IDENTIFIED BY '$sqlPassword'; GRANT ALL PRIVILEGES ON $dbName.* TO '$dbUser'@'localhost'; FLUSH PRIVILEGES;"
& $client --protocol=tcp --ssl=0 -h 127.0.0.1 -P $MariaDBPort -u root -e $sql
if ($LASTEXITCODE -ne 0) {
    throw "MariaDB bootstrap SQL failed."
}

$defaultGoCache = Join-Path $workspaceRoot ".runtime-cache\temp\go-cache-active"
if ([string]::IsNullOrWhiteSpace($GoCacheDir)) {
    $GoCacheDir = $defaultGoCache
}
$env:GOCACHE = Resolve-FullPath $GoCacheDir
if ([string]::IsNullOrWhiteSpace($env:GOMODCACHE)) {
    $env:GOMODCACHE = Join-Path $workspaceRoot ".runtime-cache\go-mod"
}
New-Item -ItemType Directory -Force -Path $env:GOCACHE, $env:GOMODCACHE | Out-Null

Push-Location $goServiceRoot
try {
    & $goCmd run -buildvcs=false ./cmd/mariadb-schema -dsn $dsn -execute
    if ($LASTEXITCODE -ne 0) {
        throw "mariadb-schema failed."
    }
} finally {
    Pop-Location
}

$goEnv = @{
    "AC_BIND_ADDR" = $BindAddr
    "AC_MODE" = "shadow"
    "AC_STORE_MODE" = "mariadb_authority"
    "AC_MARIADB_DSN" = $dsn
    "AC_PROMPT_DIR" = (Join-Path $repoRoot "prompts")
    "GOCACHE" = $env:GOCACHE
    "GOMODCACHE" = $env:GOMODCACHE
}

if ([string]::IsNullOrWhiteSpace($GoBinary)) {
    if (-not [string]::IsNullOrWhiteSpace($env:ARCHIVE_CENTER_GO_BINARY)) {
        $GoBinary = $env:ARCHIVE_CENTER_GO_BINARY
    } elseif ($UseExistingBinary -and (Test-Path -LiteralPath (Join-Path $goServiceRoot "archive-center-go.exe") -PathType Leaf)) {
        $GoBinary = Join-Path $goServiceRoot "archive-center-go.exe"
    }
}

$bindParts = $BindAddr.Split(":")
$baseUrl = "http://$BindAddr"
if ($bindParts.Count -eq 2 -and $bindParts[0] -eq "0.0.0.0") {
    $baseUrl = "http://127.0.0.1:$($bindParts[1])"
}

Write-Host "Archive Center 2.0 live source runtime"
Write-Host "  Go:      $BindAddr"
Write-Host "  MariaDB: 127.0.0.1:$MariaDBPort ($dbName)"
Write-Host "  Store:   mariadb_authority"
if ($SmokeOnly) {
    if (-not [string]::IsNullOrWhiteSpace($GoBinary)) {
        $goProcess = Start-ManagedProcess -FileName (Resolve-FullPath $GoBinary) -ArgList @() -WorkingDirectory $goServiceRoot -Environment $goEnv
    } else {
        $goProcess = Start-ManagedProcess -FileName $goCmd -ArgList @("run", "-buildvcs=false", "./cmd/archive-center-go") -WorkingDirectory $goServiceRoot -Environment $goEnv
    }
    try {
        $ready = Wait-GoReady $baseUrl
        $beforeStats = Invoke-RestMethod -Uri "$baseUrl/stats" -TimeoutSec 3
        $afterStats = $beforeStats
        $writeSmokeReport = $null

        if ($WriteSmoke) {
            $sessionId = "windows-live-smoke-$([DateTimeOffset]::UtcNow.ToUnixTimeMilliseconds())"
            $payload = @{
                chat_session_id = $sessionId
                turn_index = 1
                user_input = "windows live smoke user"
                assistant_content = "windows live smoke assistant"
                improvement_trace = @{
                    score = 1
                    source = "windows_live_smoke"
                }
            } | ConvertTo-Json -Depth 8
            $complete = Invoke-RestMethod -Method Post -Uri "$baseUrl/complete-turn" -ContentType "application/json; charset=utf-8" -Body $payload -TimeoutSec 5
            $afterStats = Invoke-RestMethod -Uri "$baseUrl/stats" -TimeoutSec 3
            $chatLogs = Invoke-RestMethod -Uri "$baseUrl/canonical/$sessionId/chat-logs" -TimeoutSec 3
            $memories = Invoke-RestMethod -Uri "$baseUrl/canonical/$sessionId/memories" -TimeoutSec 3
            $evidence = Invoke-RestMethod -Uri "$baseUrl/canonical/$sessionId/evidence" -TimeoutSec 3
            $kgTriples = Invoke-RestMethod -Uri "$baseUrl/canonical/$sessionId/kg-triples" -TimeoutSec 3

            $delta = [pscustomobject]@{
                chat_logs = (Get-StatNumber $afterStats "chat_logs") - (Get-StatNumber $beforeStats "chat_logs")
                memories = (Get-StatNumber $afterStats "memories") - (Get-StatNumber $beforeStats "memories")
                kg_triples = (Get-StatNumber $afterStats "kg_triples") - (Get-StatNumber $beforeStats "kg_triples")
            }
            $writeSmokeOk = (
                $complete.save_ok -eq $true -and
                $delta.chat_logs -ge 2 -and
                $delta.memories -ge 1 -and
                $delta.kg_triples -ge 1 -and
                [int]$chatLogs.count -ge 2 -and
                [int]$memories.count -ge 1 -and
                [int]$evidence.count -ge 1 -and
                [int]$kgTriples.count -ge 1
            )
            $writeSmokeReport = [pscustomobject]@{
                status = $(if ($writeSmokeOk) { "ok" } else { "failed" })
                session_id = $sessionId
                complete_turn = $complete
                stats_delta = $delta
                canonical_counts = [pscustomobject]@{
                    chat_logs = [int]$chatLogs.count
                    memories = [int]$memories.count
                    direct_evidence = [int]$evidence.count
                    kg_triples = [int]$kgTriples.count
                }
            }
        }

        $report = [pscustomobject]@{
            status = "ok"
            base_url = $baseUrl
            ready = $ready
            stats_before = $beforeStats
            stats_after = $afterStats
            write_smoke = $writeSmokeReport
            runtime_dir = $RuntimeDir
        }
        $json = $report | ConvertTo-Json -Depth 16
        if (-not [string]::IsNullOrWhiteSpace($Out)) {
            $outPath = Resolve-FullPath $Out
            New-Item -ItemType Directory -Force -Path (Split-Path -Parent $outPath) | Out-Null
            Set-Content -LiteralPath $outPath -Value $json -Encoding UTF8
        }
        $json
    } finally {
        if ($goProcess -and -not $goProcess.HasExited) {
            $goProcess.Kill()
        }
        if ($startedMariaDB -and -not $startedMariaDB.HasExited) {
            $startedMariaDB.Kill()
        }
    }
    exit 0
}

try {
    if (-not [string]::IsNullOrWhiteSpace($GoBinary)) {
        foreach ($key in $goEnv.Keys) {
            Set-Item -Path "Env:$key" -Value $goEnv[$key]
        }
        & (Resolve-FullPath $GoBinary)
    } else {
        Push-Location $goServiceRoot
        try {
            foreach ($key in $goEnv.Keys) {
                Set-Item -Path "Env:$key" -Value $goEnv[$key]
            }
            & $goCmd run -buildvcs=false ./cmd/archive-center-go
        } finally {
            Pop-Location
        }
    }
} finally {
    if ($startedMariaDB -and -not $startedMariaDB.HasExited) {
        $startedMariaDB.Kill()
    }
}
