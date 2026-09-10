$ErrorActionPreference = 'Stop'
$repo = Split-Path $PSScriptRoot -Parent
$launcher = Join-Path $repo 'ops/full-package/scripts/start-full-windows.ps1'
$tempRoot = Join-Path ([IO.Path]::GetTempPath()) ('ac-chroma-port-' + [guid]::NewGuid().ToString('N'))
$savedEnv = @{}
foreach ($name in @('ARCHIVE_CENTER_DATA_DIR', 'AC_CHROMA_ENDPOINT', 'AC_VECTOR_MODE', 'AC_BIND_ADDR', 'AC_MARIADB_PORT', 'AC_MARIADB_DSN')) {
    $savedEnv[$name] = [Environment]::GetEnvironmentVariable($name, 'Process')
}
function Assert($condition, $message) { if (-not $condition) { throw $message } }
try {
    $scripts = Join-Path $tempRoot 'package/scripts'
    New-Item -ItemType Directory -Force $scripts | Out-Null
    Copy-Item -LiteralPath $launcher -Destination $scripts
    Copy-Item -LiteralPath (Join-Path $repo 'ops/full-package/scripts/service-ports.ps1') -Destination $scripts
    Copy-Item -LiteralPath (Join-Path $repo 'ops/full-package/scripts/windows-console-control.ps1') -Destination $scripts
    $env:ARCHIVE_CENTER_DATA_DIR = Join-Path $tempRoot 'existing data'
    $envFile = Join-Path $tempRoot '.env.fixture'
    [IO.File]::WriteAllText($envFile, "AC_VECTOR_MODE=bundled`nAC_CHROMA_ENDPOINT=http://127.0.0.1:8000`n")
    # Pending-update poison: configuration must exit before update application/parsing.
    $updates = Join-Path $tempRoot 'package/.updates'
    New-Item -ItemType Directory -Force $updates | Out-Null
    [IO.File]::WriteAllText((Join-Path $updates 'pending-update.json'), 'not a pending manifest')
    $copy = Join-Path $scripts 'start-full-windows.ps1'
    & powershell.exe -NoProfile -ExecutionPolicy Bypass -File $copy -EnvFile $envFile -ConfigureChromaPort -ChromaPort 8001
    Assert ($LASTEXITCODE -eq 0) 'Production configuration entrypoint failed'
    $portFile = Join-Path $env:ARCHIVE_CENTER_DATA_DIR 'chroma-port.txt'
    Assert ((Get-Content $portFile -Raw).Trim() -eq '8001') 'Menu did not persist port'
    Assert (-not (Test-Path (Join-Path $env:ARCHIVE_CENTER_DATA_DIR 'mariadb'))) 'Configuration started database setup'

    # Execute the real launcher functions via the PowerShell parser, not copies of their logic.
    $parseErrors = $null
    $tokens = $null
    $ast = [Management.Automation.Language.Parser]::ParseFile($launcher, [ref]$tokens, [ref]$parseErrors)
    Assert ($parseErrors.Count -eq 0) 'Launcher syntax error'
    . (Join-Path $scripts 'service-ports.ps1')
    foreach ($name in @('Start-ManagedChromaDB')) {
        $fn = $ast.Find({ param($node) $node -is [Management.Automation.Language.FunctionDefinitionAst] -and $node.Name -eq $name }, $true)
        Assert ($null -ne $fn) "Missing production function: $name"
        . ([scriptblock]::Create($fn.Extent.Text))
    }
    $dataRoot = Get-ArchiveDataRoot
    Assert ($dataRoot -eq $env:ARCHIVE_CENTER_DATA_DIR) 'Data directory changed'
    $env:AC_VECTOR_MODE = 'bundled'
    $env:AC_CHROMA_ENDPOINT = 'http://127.0.0.1:8000'
    Set-ArchiveChromaEndpoint $dataRoot
    Assert ($env:AC_CHROMA_ENDPOINT -eq 'http://127.0.0.1:8001') 'Normal launcher ignored saved port'
    foreach ($invalid in @('0', '65536', 'abc', '-1', '1.5', '8001;echo bad')) {
        $rejected = $false
        try { Set-ArchiveServicePort $dataRoot chroma $invalid } catch { $rejected = $true }
        Assert $rejected 'Invalid port accepted'
        Assert ((Get-Content $portFile -Raw).Trim() -eq '8001') 'Invalid input overwrote saved port'
    }
    function Read-Host { param($Prompt) return '' }
    Set-ArchiveServicePort -DataRoot $dataRoot -Service chroma -Prompt
    Assert ((Get-Content $portFile -Raw).Trim() -eq '8000') 'Empty input did not restore default'
    Set-ArchiveServicePort $dataRoot chroma '08002'
    Set-ArchiveChromaEndpoint $dataRoot
    Assert ($env:AC_CHROMA_ENDPOINT -eq 'http://127.0.0.1:8002') 'Explicit port failed'
    $env:AC_VECTOR_MODE = 'external'
    $env:AC_CHROMA_ENDPOINT = 'https://vector.example:9443'
    Set-ArchiveChromaEndpoint $dataRoot
    Assert ($env:AC_CHROMA_ENDPOINT -eq 'https://vector.example:9443') 'External endpoint was changed'
    $env:AC_VECTOR_MODE = 'off'
    $env:AC_CHROMA_ENDPOINT = ''
    Set-ArchiveChromaEndpoint $dataRoot
    Assert ([string]$env:AC_CHROMA_ENDPOINT -eq '') 'Off profile activated ChromaDB'
    $env:AC_VECTOR_MODE = 'bundled'
    Set-ArchiveServicePort $dataRoot chroma '8000'
    Set-ArchiveChromaEndpoint $dataRoot
    Assert ($env:AC_CHROMA_ENDPOINT -eq 'http://127.0.0.1:8000') 'Reset failed'

    # Process and runtime-validation boundaries only; actual ChromaDB command assembly remains production code.
    $managedChromaDBVersion = '1.5.9'
    function Find-ChromaRuntimePython($Root) { Assert ($Root -eq 'fixture-runtime') 'Unexpected runtime lookup'; return 'fixture-python' }
    function Unblock-PackageFile($Path) { Assert ($Path -eq 'fixture-python') 'Unexpected file unblock' }
    function Test-ChromaRuntimeVersion($PythonPath, $RequiredVersion) {
        Assert ($PythonPath -eq 'fixture-python' -and $RequiredVersion -eq '1.5.9') 'Unexpected runtime check'
        return $true
    }
    function Start-ArchiveChildProcess($FilePath, $ArgumentList, $WorkingDirectory) {
        Assert ($FilePath -eq 'fixture-python') 'Unexpected process'
        $script:chromaArguments = $ArgumentList
        $script:inheritedEndpoint = $env:AC_CHROMA_ENDPOINT
    }
    Set-ArchiveServicePort $dataRoot chroma '8001'
    Set-ArchiveChromaEndpoint $dataRoot
    Start-ManagedChromaDB -PackageRoot (Join-Path $tempRoot 'package') -RuntimeRoot 'fixture-runtime' -Endpoint ([uri]$env:AC_CHROMA_ENDPOINT)
    Assert ($chromaArguments[3] -eq '8001') 'ChromaDB process uses different port'
    Assert ($chromaArguments[4] -eq (Join-Path $dataRoot 'chromadb')) 'Existing database directory changed'
    Assert ($inheritedEndpoint -eq 'http://127.0.0.1:8001') 'Go inherited endpoint differs'
    foreach ($service in @('mariadb', 'backend')) {
        $switchName = if ($service -eq 'mariadb') { '-MariaDBPort' } else { '-BackendPort' }
        $value = if ($service -eq 'mariadb') { '3311' } else { '28111' }
        & powershell.exe -NoProfile -ExecutionPolicy Bypass -File $copy -EnvFile $envFile -ConfigurePorts -PortService $service $switchName $value
        Assert ($LASTEXITCODE -eq 0) "Configuration failed: $service"
        Assert ((Get-ArchiveSavedPort $dataRoot $service) -eq $value) "Wrong saved $service port"
        foreach ($invalid in @('0', '65536', 'bad', '-1')) {
            $rejected = $false
            try { Set-ArchiveServicePort $dataRoot $service $invalid } catch { $rejected = $true }
            Assert $rejected "Invalid $service port accepted"
            Assert ((Get-ArchiveSavedPort $dataRoot $service) -eq $value) 'Invalid input overwrote setting'
        }
        # A blank port at the actual configuration entrypoint must reset, including EOF.
        '' | & powershell.exe -NoProfile -ExecutionPolicy Bypass -File $copy -EnvFile $envFile -ConfigurePorts -PortService $service
        Assert ($LASTEXITCODE -eq 0) 'Blank menu input failed'
        Assert ((Get-ArchiveSavedPort $dataRoot $service) -eq (Get-ArchiveDefaultPort $service)) 'Blank menu input did not reset'
        Set-ArchiveServicePort $dataRoot $service $value
    }
    # Empty startup arguments retain saved values. Only configuration input resets.
    $env:AC_BIND_ADDR = '0.0.0.0:28080'
    $env:AC_MARIADB_DSN = 'fixture:pass@tcp(fake)/word@tcp(127.0.0.1:3307)/fixture_db?parseTime=true&charset=utf8mb4'
    $resolvedMaria = Set-ArchiveServicePorts -DataRoot $dataRoot
    Assert ($resolvedMaria -eq 3311 -and $env:AC_MARIADB_PORT -eq '3311') 'MariaDB process port differs'
    Assert ($env:AC_MARIADB_DSN -ceq 'fixture:pass@tcp(fake)/word@tcp(127.0.0.1:3311)/fixture_db?parseTime=true&charset=utf8mb4') 'Local DSN port or credentials changed incorrectly'
    Assert ($env:AC_BIND_ADDR -eq '0.0.0.0:28111') 'Backend ignored saved port'
    foreach ($bindHost in @('127.0.0.1', '100.96.60.55', '[::]', '[::1]')) {
        $env:AC_BIND_ADDR = "${bindHost}:28080"
        Set-ArchiveBackendEndpoint $dataRoot
        Assert ($env:AC_BIND_ADDR -eq "${bindHost}:28111") 'Saved backend port changed bind host'
    }
    $env:AC_MARIADB_DSN = 'fixture:pass@tcp(db.example:3306)/remote?parseTime=true'
    $null = Set-ArchiveServicePorts -DataRoot $dataRoot -BindAddr '127.0.0.1:28112'
    Assert ($env:AC_MARIADB_DSN -ceq 'fixture:pass@tcp(db.example:3306)/remote?parseTime=true') 'Remote DSN changed'
    Assert ($env:AC_BIND_ADDR -eq '127.0.0.1:28112') 'Explicit bind override ignored'
    $env:AC_MARIADB_DSN = ''
    $env:AC_VECTOR_MODE = 'off'
    $env:AC_BIND_ADDR = '0.0.0.0:28080'
    $MariaDBPort = Set-ArchiveServicePorts -DataRoot $dataRoot
    Assert ($env:AC_MARIADB_DSN -match '@tcp\(127\.0\.0\.1:3311\)/') 'Default DSN uses wrong port'

    # Execute the startup statements through schema invocation, substituting only process/socket boundaries.
    $source = Get-Content -LiteralPath $launcher -Raw
    $start = $source.IndexOf('    if (-not (Test-PortOpen $MariaDBPort)) {')
    $end = $source.IndexOf('Write-Host "Starting Archive Center', $start)
    $startup = $source.Substring($start, $end - $start)
    $dataDir = Join-Path $dataRoot 'mariadb'
    $packRoot = Join-Path $tempRoot 'package'
    $mariadbd = 'fixture-mariadb'
    function Test-PortOpen($Port) { Assert ($Port -eq 3311) 'MariaDB probe port'; return $false }
    function Wait-Port($Port, $Seconds) { Assert ($Port -eq 3311) 'MariaDB readiness port' }
    function Test-VectorRequiresChroma($Mode) { return $false }
    function Start-ArchiveChildProcess($FilePath, $ArgumentList, $WorkingDirectory) {
        Assert ($FilePath -eq 'fixture-mariadb') 'Unexpected startup process'
        Assert ($ArgumentList -contains '--port=3311') 'MariaDB command port'
        Assert ($ArgumentList -contains "--datadir=$dataDir") 'MariaDB directory moved'
    }
    $schemaCommand = Join-Path $packRoot 'bin\mariadb-schema.exe'
    Set-Item -LiteralPath "Function:$schemaCommand" -Value {
        Assert ($args[$args.IndexOf('-managed-port') + 1] -eq 3311) 'Schema bootstrap port'
        Assert ($args[$args.IndexOf('-dsn') + 1] -eq $env:AC_MARIADB_DSN) 'Schema DSN differs from backend'
        Assert ($args[$args.IndexOf('-expected-datadir') + 1] -eq $dataDir) 'Schema directory changed'
        $global:LASTEXITCODE = 0
    }
    . ([scriptblock]::Create($startup))
    Assert ($backendPort -eq 28111) 'Backend readiness port differs from listener'
    Remove-Item -LiteralPath "Function:$schemaCommand"

    # Execute production smoke address resolution and stop at its first HTTP boundary.
    $smokeCopy = Join-Path $scripts 'smoke-live.ps1'
    Copy-Item -LiteralPath (Join-Path $repo 'ops/full-package/scripts/smoke-live.ps1') -Destination $smokeCopy
    function Invoke-RestMethod { param($Method, $Uri) $global:portTestObservedSmokeUri = $Uri; throw 'fixture-http-stop' }
    $env:AC_BIND_ADDR = '0.0.0.0:28080'
    try { & $smokeCopy -EnvFile $envFile } catch { Assert ($_.Exception.Message -eq 'fixture-http-stop') 'Unexpected smoke error' }
    Assert ($global:portTestObservedSmokeUri -eq 'http://127.0.0.1:28111/health') '02 smoke check ignored saved backend port'
    try { & $smokeCopy -EnvFile $envFile -BaseUrl 'http://fixture.example:28112' } catch { Assert ($_.Exception.Message -eq 'fixture-http-stop') 'Unexpected explicit smoke error' }
    Assert ($global:portTestObservedSmokeUri -eq 'http://fixture.example:28112/health') 'Explicit smoke URL overwritten'
    Remove-Variable -Name portTestObservedSmokeUri -Scope Global
    Assert (-not (Test-Path (Join-Path $dataRoot 'mariadb'))) 'Configuration created a database'
    Write-Host 'Service ports: Windows configuration, startup/schema and connection-check boundaries passed.'
} finally {
    foreach ($name in $savedEnv.Keys) { [Environment]::SetEnvironmentVariable($name, $savedEnv[$name], 'Process') }
    $resolvedTemp = [IO.Path]::GetFullPath($tempRoot)
    $allowedParent = [IO.Path]::GetFullPath([IO.Path]::GetTempPath())
    if ($resolvedTemp.StartsWith($allowedParent, [StringComparison]::OrdinalIgnoreCase) -and (Split-Path $resolvedTemp -Leaf) -like 'ac-chroma-port-*') {
        Remove-Item -LiteralPath $resolvedTemp -Recurse -Force -ErrorAction SilentlyContinue
    }
}
