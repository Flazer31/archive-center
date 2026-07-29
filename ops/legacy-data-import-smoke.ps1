param(
    [string]$LauncherPath = "",
    [string]$InstallerPath = ""
)

$ErrorActionPreference = "Stop"
Set-StrictMode -Version 2.0

if ([string]::IsNullOrWhiteSpace($LauncherPath)) {
    $LauncherPath = Join-Path $PSScriptRoot "full-package\scripts\start-full-windows.ps1"
}
if ([string]::IsNullOrWhiteSpace($InstallerPath)) {
    $InstallerPath = Join-Path $PSScriptRoot "..\scripts\install-github-release.ps1"
}
$LauncherPath = (Resolve-Path -LiteralPath $LauncherPath).Path
$InstallerPath = (Resolve-Path -LiteralPath $InstallerPath).Path

function Assert-True([bool]$Condition, [string]$Message) {
    if (-not $Condition) {
        throw $Message
    }
}

function Get-ClosedLoopbackPort {
    $probe = [System.Net.Sockets.TcpListener]::new([System.Net.IPAddress]::Loopback, 0)
    try {
        $probe.Start()
        return ([System.Net.IPEndPoint]$probe.LocalEndpoint).Port
    } finally {
        $probe.Stop()
    }
}

function Get-FunctionDefinitions([string]$Path, [string[]]$Names) {
    $tokens = $null
    $errors = $null
    $ast = [System.Management.Automation.Language.Parser]::ParseFile(
        $Path,
        [ref]$tokens,
        [ref]$errors
    )
    if ($errors.Count -gt 0) {
        throw "Unable to parse $Path`: $($errors[0].Message)"
    }
    $definitions = @{}
    foreach ($node in $ast.FindAll({
        param($candidate)
        $candidate -is [System.Management.Automation.Language.FunctionDefinitionAst]
    }, $true)) {
        if ($Names -contains $node.Name) {
            $definitions[$node.Name] = $node.Extent.Text
        }
    }
    foreach ($name in $Names) {
        if (-not $definitions.ContainsKey($name)) {
            throw "Production function $name was not found in $Path"
        }
        $definitions[$name]
    }
}

function Invoke-ImportScenario(
    [string]$Scenario,
    [scriptblock]$Import,
    [string]$TempRoot
) {
    $scenarioRoot = Join-Path $TempRoot $Scenario
    $packageRoot = Join-Path $scenarioRoot "package"
    $source = Join-Path $packageRoot ".runtime\chromadb"
    $dataRoot = Join-Path $scenarioRoot "data"
    New-Item -ItemType Directory -Force -Path $source | Out-Null
    [System.IO.File]::WriteAllBytes(
        (Join-Path $source "chroma.sqlite3"),
        [byte[]](1, 2, 3, 4, 5)
    )
    [System.IO.File]::WriteAllBytes(
        (Join-Path $source "chroma.sqlite3-wal"),
        [byte[]](6, 7, 8)
    )

    $previousEndpoint = $env:AC_CHROMA_ENDPOINT
    $port = 0
    try {
        foreach ($case in @(
            [pscustomobject]@{
                Address = [System.Net.IPAddress]::Loopback
                URIHost = "127.0.0.1"
                Label = "default IPv4"
                ConfiguredEndpoint = $false
            },
            [pscustomobject]@{
                Address = [System.Net.IPAddress]::IPv6Loopback
                URIHost = "[::1]"
                Label = "default IPv6"
                ConfiguredEndpoint = $false
            },
            [pscustomobject]@{
                Address = [System.Net.IPAddress]::Loopback
                URIHost = "127.0.0.1"
                Label = "configured IPv4"
                ConfiguredEndpoint = $true
            },
            [pscustomobject]@{
                Address = [System.Net.IPAddress]::IPv6Loopback
                URIHost = "[::1]"
                Label = "configured IPv6"
                ConfiguredEndpoint = $true
            }
        )) {
            $listener = [System.Net.Sockets.TcpListener]::new($case.Address, 0)
            if ($case.Address.AddressFamily -eq [System.Net.Sockets.AddressFamily]::InterNetworkV6) {
                $listener.Server.DualMode = $false
            }
            $listener.Start()
            $port = ([System.Net.IPEndPoint]$listener.LocalEndpoint).Port
            $defaultPort = if ($case.ConfiguredEndpoint) {
                Get-ClosedLoopbackPort
            } else {
                $port
            }
            $env:AC_CHROMA_ENDPOINT = if ($case.ConfiguredEndpoint) {
                "http://$($case.URIHost):$port"
            } else {
                $null
            }
            try {
                $blocked = $false
                try {
                    & $Import $packageRoot $dataRoot $defaultPort
                } catch {
                    $blocked = $_.Exception.Message -like "*Legacy ChromaDB data may be active*"
                }
                Assert-True $blocked "$Scenario did not reject a live $($case.Label) Chroma endpoint"
                Assert-True (-not (Test-Path -LiteralPath (Join-Path $dataRoot "chromadb"))) "$Scenario copied Chroma data while the $($case.Label) endpoint was live"
                Assert-True (-not (Test-Path -LiteralPath (Join-Path $dataRoot ".archive-center-legacy-runtime-import-v1.json"))) "$Scenario wrote a success marker while the $($case.Label) endpoint was live"
            } finally {
                $listener.Stop()
            }
        }

        $env:AC_CHROMA_ENDPOINT = "http://127.0.0.1:$port"
        & $Import $packageRoot $dataRoot $port
    } finally {
        $env:AC_CHROMA_ENDPOINT = $previousEndpoint
    }
    Assert-True (Test-Path -LiteralPath (Join-Path $dataRoot "chromadb\chroma.sqlite3") -PathType Leaf) "$Scenario did not promote the verified Chroma snapshot"
    Assert-True (Test-Path -LiteralPath (Join-Path $dataRoot ".archive-center-legacy-runtime-import-v1.json") -PathType Leaf) "$Scenario did not write the verified import marker"
    Assert-True (Test-Path -LiteralPath (Join-Path $source "chroma.sqlite3") -PathType Leaf) "$Scenario removed the legacy source"
    $staging = @(Get-ChildItem -LiteralPath $dataRoot -Directory -Force -ErrorAction SilentlyContinue |
        Where-Object { $_.Name -like ".chromadb.import-*" })
    Assert-True ($staging.Count -eq 0) "$Scenario left an import staging directory"
}

$tempBase = [System.IO.Path]::GetFullPath([System.IO.Path]::GetTempPath())
$tempRoot = Join-Path $tempBase ("archive-center-legacy-import-smoke-" + [Guid]::NewGuid().ToString("N"))
New-Item -ItemType Directory -Path $tempRoot | Out-Null

try {
    foreach ($definition in Get-FunctionDefinitions $LauncherPath @(
        "Test-PortOpen",
        "Get-LowerSHA256",
        "Get-RelativeDataFileMap",
        "Copy-LegacyDataDirectoryVerified",
        "Get-LegacyChromaImportTargets",
        "Assert-LegacyChromaOffline",
        "Import-LegacyRuntimeDataOnce"
    )) {
        Invoke-Expression $definition
    }
    $script:MariaDBPort = 3307
    Invoke-ImportScenario "launcher" {
        param($packageRoot, $dataRoot, $port)
        Import-LegacyRuntimeDataOnce -PackageRoot $packageRoot -DataRoot $dataRoot -ChromaPort $port
    } $tempRoot

    foreach ($definition in Get-FunctionDefinitions $InstallerPath @(
        "Test-LocalPortOpen",
        "Get-DataFileMap",
        "Copy-LegacyDataVerified",
        "Get-PreviousPackageChromaTargets",
        "Assert-PreviousPackageChromaOffline",
        "Import-PreviousPackageRuntimeOnce"
    )) {
        Invoke-Expression $definition
    }
    Invoke-ImportScenario "external-installer" {
        param($packageRoot, $dataRoot, $port)
        Import-PreviousPackageRuntimeOnce -PreviousPackageRoot $packageRoot -DataRoot $dataRoot -MariaDBPort 3307 -ChromaPort $port
    } $tempRoot

    [ordered]@{
        contract = "archive-center.legacy-data-import-smoke.v1"
        status = "ok"
        launcher_live_port_rejection = "passed"
        launcher_ipv6_live_port_rejection = "passed"
        launcher_configured_live_port_rejection = "passed"
        launcher_configured_ipv6_live_port_rejection = "passed"
        launcher_offline_atomic_promotion = "passed"
        external_installer_live_port_rejection = "passed"
        external_installer_ipv6_live_port_rejection = "passed"
        external_installer_configured_live_port_rejection = "passed"
        external_installer_configured_ipv6_live_port_rejection = "passed"
        external_installer_offline_atomic_promotion = "passed"
        source_preserved = $true
    } | ConvertTo-Json -Depth 4
} finally {
    $cleanup = [System.IO.Path]::GetFullPath($tempRoot)
    $parent = [System.IO.Path]::GetDirectoryName($cleanup)
    $leaf = [System.IO.Path]::GetFileName($cleanup)
    if (-not $parent.Equals($tempBase.TrimEnd('\'), [System.StringComparison]::OrdinalIgnoreCase) -or
        -not $leaf.StartsWith("archive-center-legacy-import-smoke-", [System.StringComparison]::Ordinal)) {
        throw "Refusing unsafe smoke cleanup path: $cleanup"
    }
    if (Test-Path -LiteralPath $cleanup) {
        Remove-Item -LiteralPath $cleanup -Recurse -Force
    }
}
