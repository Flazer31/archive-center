param(
    [string]$BindAddr = "127.0.0.1:28080",
    [int]$MariaDBPort = 3307,
    [string]$RuntimeDir = "",
    [string]$ProviderBin = "",
    [string]$GoBinary = "",
    [string]$MilvusEndpoint = "",
    [string]$MilvusLitePython = "",
    [string]$MilvusDataDir = "",
    [int]$MilvusPort = 0,
    [string]$MilvusQuerySet = "",
    [string]$GoCacheDir = "",
    [string]$Out = "",
    [switch]$UseExistingBinary,
    [switch]$StartManagedMilvus,
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

function Find-MilvusLitePython {
    param([string]$RepoRoot, [string]$WorkspaceRoot, [string]$ExplicitPython)

    $candidates = @()
    if (-not [string]::IsNullOrWhiteSpace($ExplicitPython)) {
        $candidates += (Resolve-FullPath $ExplicitPython)
    }
    if (-not [string]::IsNullOrWhiteSpace($env:AC_MILVUS_LITE_PYTHON)) {
        $candidates += (Resolve-FullPath $env:AC_MILVUS_LITE_PYTHON)
    }
    $candidates += (Join-Path $WorkspaceRoot ".runtime-cache\temp\archive-center-2.0\milvus-lite-runtime\Scripts\python.exe")
    $candidates += (Join-Path $WorkspaceRoot ".runtime-cache\archive-center-2.0\milvus-lite-runtime\Scripts\python.exe")
    $candidates += (Join-Path $RepoRoot "runtime\Python\Scripts\python.exe")
    $candidates += (Join-Path $RepoRoot "runtime\python\Scripts\python.exe")

    foreach ($candidate in $candidates) {
        if (-not (Test-Path -LiteralPath $candidate -PathType Leaf)) {
            continue
        }
        & $candidate -c "import importlib.util, sys; sys.exit(0 if importlib.util.find_spec('milvus_lite') else 1)" *> $null
        if ($LASTEXITCODE -eq 0) {
            return $candidate
        }
    }

    throw "Bundled Milvus Lite Python runtime was not found. Stage the managed runtime first; normal users must not install Milvus manually."
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

function Read-FirstMilvusQuery {
    param([string]$QuerySetPath)
    if ([string]::IsNullOrWhiteSpace($QuerySetPath) -or -not (Test-Path -LiteralPath $QuerySetPath -PathType Leaf)) {
        return $null
    }
    $payload = Get-Content -LiteralPath $QuerySetPath -Raw -Encoding UTF8 | ConvertFrom-Json
    if ($null -eq $payload.queries -or $payload.queries.Count -lt 1) {
        return $null
    }
    return $payload.queries[0]
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

if ([string]::IsNullOrWhiteSpace($MilvusQuerySet)) {
    $MilvusQuerySet = Join-Path $repoRoot "benchmarks\chroma-milvus-query-set-2026-05-25-real.json"
}

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

$startedMilvus = $null
$milvusPrepareReport = $null
if ($StartManagedMilvus) {
    if (-not [string]::IsNullOrWhiteSpace($MilvusEndpoint)) {
        throw "Use either -StartManagedMilvus or -MilvusEndpoint, not both."
    }
    $milvusPython = Find-MilvusLitePython $repoRoot $workspaceRoot $MilvusLitePython
    if ($MilvusPort -le 0) {
        $MilvusPort = Get-FreeTcpPort
    }
    if ([string]::IsNullOrWhiteSpace($MilvusDataDir)) {
        $MilvusDataDir = Join-Path $workspaceRoot ".runtime-cache\temp\archive-center-2.0-live\milvus-lite-$MilvusPort.db"
    }
    $MilvusDataDir = Resolve-FullPath $MilvusDataDir
    New-Item -ItemType Directory -Force -Path (Split-Path -Parent $MilvusDataDir) | Out-Null
    $milvusArgs = @("-m", "milvus_lite", "server", "--data-dir", $MilvusDataDir, "--host", "127.0.0.1", "--port", [string]$MilvusPort)
    $startedMilvus = Start-ManagedProcess -FileName $milvusPython -ArgList $milvusArgs -WorkingDirectory $RuntimeDir -Stdout "capture" -Stderr "capture"
    Start-Sleep -Seconds 2
    if ($startedMilvus.HasExited) {
        $milvusOut = $startedMilvus.StandardOutput.ReadToEnd()
        $milvusErr = $startedMilvus.StandardError.ReadToEnd()
        throw "Milvus Lite exited early with code $($startedMilvus.ExitCode). stdout=$milvusOut stderr=$milvusErr"
    }
    Wait-TcpPort -Port $MilvusPort -TimeoutSeconds 60
    $MilvusEndpoint = "http://127.0.0.1:$MilvusPort"
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
    if (-not [string]::IsNullOrWhiteSpace($MilvusEndpoint) -and (Test-Path -LiteralPath $MilvusQuerySet -PathType Leaf)) {
        $milvusPrepareOut = Join-Path $workspaceRoot ".runtime-cache\temp\archive-center-2.0-live\milvus-collection-prepare-$MilvusPort.json"
        New-Item -ItemType Directory -Force -Path (Split-Path -Parent $milvusPrepareOut) | Out-Null
        & $goCmd run -buildvcs=false ./cmd/milvus-sdk-smoke -execute -ensure-collection -endpoint $MilvusEndpoint -query-set $MilvusQuerySet -out $milvusPrepareOut
        if ($LASTEXITCODE -ne 0) {
            throw "milvus-sdk-smoke collection prepare failed."
        }
        if (Test-Path -LiteralPath $milvusPrepareOut -PathType Leaf) {
            $milvusPrepareReport = Get-Content -LiteralPath $milvusPrepareOut -Raw -Encoding UTF8 | ConvertFrom-Json
        }
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

if (-not [string]::IsNullOrWhiteSpace($MilvusEndpoint)) {
    $goEnv["AC_MILVUS_ENDPOINT"] = $MilvusEndpoint
    $goEnv["AC_MILVUS_SDK_ENABLED"] = "true"
    $goEnv["AC_MILVUS_RECALL_READ_ENABLED"] = "true"
    $goEnv["AC_MILVUS_PRODUCT_READ_ENABLED"] = "true"
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
if ([string]::IsNullOrWhiteSpace($MilvusEndpoint)) {
    Write-Host "  Milvus:  not enabled (pass -MilvusEndpoint to enable SDK/product-read path)"
} else {
    Write-Host "  Milvus:  $MilvusEndpoint"
}

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

        $milvusSmokeReport = $null
        if (-not [string]::IsNullOrWhiteSpace($MilvusEndpoint)) {
            $milvusQuery = Read-FirstMilvusQuery -QuerySetPath $MilvusQuerySet
            if ($null -ne $milvusQuery) {
                $milvusSid = [string]$milvusQuery.chat_session_id
                if ([string]::IsNullOrWhiteSpace($milvusSid) -and $null -ne $milvusPrepareReport) {
                    $milvusSid = [string]$milvusPrepareReport.session_id
                }
                $sourceId = [string]$milvusQuery.source_id
                $vector = @($milvusQuery.embedding)
                $limit = 5
                $filter = ""
                if (-not [string]::IsNullOrWhiteSpace($milvusSid)) {
                    $filter = "chat_session_id == `"$milvusSid`""
                }
                $preparePayload = @{
                    chat_session_id = $milvusSid
                    turn_index = 1
                    raw_user_input = "Milvus product-read smoke"
                    client_meta = @{
                        milvus_query_vector = $vector
                        milvus_filter = $filter
                    }
                    settings = @{
                        top_k = $limit
                        injection_enabled = $false
                        input_context_enabled = $false
                    }
                } | ConvertTo-Json -Depth 32
                $prepareTurn = Invoke-RestMethod -Method Post -Uri "$baseUrl/prepare-turn" -ContentType "application/json; charset=utf-8" -Body $preparePayload -TimeoutSec 20
                $recall = $prepareTurn.recall_result
                $vectorShadow = $recall.vector_shadow
                $topId = $null
                if ($null -ne $vectorShadow.search_results -and $vectorShadow.search_results.Count -gt 0) {
                    $topId = [string]$vectorShadow.search_results[0].id
                }
                $milvusOk = (
                    $vectorShadow.milvus_live_enabled -eq $true -and
                    $vectorShadow.live_retrieval_enabled -eq $true -and
                    $vectorShadow.product_read_enabled -eq $true -and
                    $vectorShadow.search_result -eq "ok" -and
                    $vectorShadow.search_result_count -ge 1
                )
                $milvusSmokeReport = [pscustomobject]@{
                    status = $(if ($milvusOk) { "ok" } else { "failed" })
                    endpoint = $MilvusEndpoint
                    query_set = $MilvusQuerySet
                    session_id = $milvusSid
                    source_id = $sourceId
                    top_id = $topId
                    collection_prepare = $milvusPrepareReport
                    prepare_turn = $prepareTurn
                    summary = [pscustomobject]@{
                        persisted_milvus_live_enabled = $true
                        persisted_live_retrieval_enabled = $true
                        bounded_shadow_route_only = $false
                        search_result = [string]$vectorShadow.search_result
                        search_result_count = [int]$vectorShadow.search_result_count
                        product_read_enabled = [bool]$vectorShadow.product_read_enabled
                        milvus_live_enabled = [bool]$vectorShadow.milvus_live_enabled
                        live_retrieval_enabled = [bool]$vectorShadow.live_retrieval_enabled
                    }
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
            milvus_smoke = $milvusSmokeReport
            milvus_endpoint = $MilvusEndpoint
            managed_milvus_started = [bool]$StartManagedMilvus
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
        if ($startedMilvus -and -not $startedMilvus.HasExited) {
            $startedMilvus.Kill()
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
    if ($startedMilvus -and -not $startedMilvus.HasExited) {
        $startedMilvus.Kill()
    }
}
