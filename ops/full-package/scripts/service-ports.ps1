# Shared by the package launcher and connection check. Settings stay in the
# existing data root, outside release files replaced by the managed updater.
function Get-ArchiveDataRoot {
    if (-not [string]::IsNullOrWhiteSpace($env:ARCHIVE_CENTER_DATA_DIR)) {
        return [System.IO.Path]::GetFullPath($env:ARCHIVE_CENTER_DATA_DIR)
    }
    $localData = [Environment]::GetFolderPath("LocalApplicationData")
    if ([string]::IsNullOrWhiteSpace($localData)) {
        $localData = Join-Path $env:USERPROFILE "AppData\Local"
    }
    return Join-Path $localData "ArchiveCenter\data"
}

function Get-ArchiveDefaultPort([string]$Service) {
    switch ($Service) {
        "chroma" { return 8000 }
        "mariadb" { return 3307 }
        "backend" { return 28080 }
        default { throw "Choose chroma, mariadb, or backend." }
    }
}

function Get-ArchiveSavedPort([string]$DataRoot, [string]$Service) {
    $portFile = Join-Path $DataRoot "$Service-port.txt"
    if (-not (Test-Path -LiteralPath $portFile)) { return $null }
    $saved = ([string](Get-Content -LiteralPath $portFile -Raw)).Trim()
    $number = 0
    if ($saved -match '^[0-9]{1,5}$' -and [int]::TryParse($saved, [ref]$number) -and $number -ge 1 -and $number -le 65535) {
        return $number
    }
    Write-Warning "Ignoring invalid saved $Service port: $portFile"
    return $null
}

function Set-ArchiveServicePort([string]$DataRoot, [string]$Service, [string]$Port, [switch]$Prompt) {
    $defaultPort = Get-ArchiveDefaultPort $Service
    if ($Prompt -and [string]::IsNullOrWhiteSpace($Port)) {
        $saved = Get-ArchiveSavedPort $DataRoot $Service
        if ($null -eq $saved) { $saved = "not set" }
        $Port = Read-Host "$Service port (saved: $saved). Enter = default $defaultPort"
    }
    if ([string]::IsNullOrWhiteSpace($Port)) { $Port = [string]$defaultPort }
    $number = 0
    if ($Port -notmatch '^[0-9]{1,5}$' -or -not [int]::TryParse($Port, [ref]$number) -or $number -lt 1 -or $number -gt 65535) {
        throw "$Service port must be a number from 1 to 65535. Settings unchanged."
    }
    $portFile = Join-Path $DataRoot "$Service-port.txt"
    New-Item -ItemType Directory -Force -Path $DataRoot | Out-Null
    [System.IO.File]::WriteAllText("$portFile.tmp", "$number`n", (New-Object System.Text.UTF8Encoding($false)))
    Move-Item -LiteralPath "$portFile.tmp" -Destination $portFile -Force
    Write-Host "Saved $Service port: $number. Applies on next start."
    if ($Service -eq "backend") {
        Write-Host "Set the port in the RisuAI backend URL to $number after restarting Archive Center."
    }
}

function Set-ArchiveChromaEndpoint([string]$DataRoot) {
    if ($env:AC_VECTOR_MODE -notin @("local_native", "local_proot", "bundled")) { return }
    $saved = Get-ArchiveSavedPort $DataRoot "chroma"
    if ($null -ne $saved) { $env:AC_CHROMA_ENDPOINT = "http://127.0.0.1:$saved" }
}

function Set-ArchiveBackendEndpoint([string]$DataRoot) {
    if ([string]::IsNullOrWhiteSpace($env:AC_BIND_ADDR)) { $env:AC_BIND_ADDR = "0.0.0.0:28080" }
    $saved = Get-ArchiveSavedPort $DataRoot "backend"
    if ($null -ne $saved) {
        # Replace only the port; preserve loopback, LAN, wildcard and IPv6 hosts.
        $env:AC_BIND_ADDR = $env:AC_BIND_ADDR -replace ':\d+$', ":$saved"
    }
}

function Set-ArchiveServicePorts([string]$DataRoot, [string]$ChromaPort, [string]$MariaDBPort, [string]$BackendPort, [string]$BindAddr) {
    foreach ($entry in @(@("chroma", $ChromaPort), @("mariadb", $MariaDBPort), @("backend", $BackendPort))) {
        if (-not [string]::IsNullOrWhiteSpace($entry[1])) {
            Set-ArchiveServicePort $DataRoot $entry[0] $entry[1]
        }
    }
    Set-ArchiveChromaEndpoint $DataRoot
    Set-ArchiveBackendEndpoint $DataRoot
    # A complete explicit bind address is a one-run override. A requested port
    # wins its port component while retaining the explicit host.
    if (-not [string]::IsNullOrWhiteSpace($BindAddr)) {
        $env:AC_BIND_ADDR = $BindAddr
        if (-not [string]::IsNullOrWhiteSpace($BackendPort)) { Set-ArchiveBackendEndpoint $DataRoot }
    }
    $mariaPort = Get-ArchiveSavedPort $DataRoot "mariadb"
    if ($null -eq $mariaPort) {
        $mariaPort = if ([string]::IsNullOrWhiteSpace($env:AC_MARIADB_PORT)) { 3307 } else { [int]$env:AC_MARIADB_PORT }
    }
    $env:AC_MARIADB_PORT = [string]$mariaPort
    if ([string]::IsNullOrWhiteSpace($env:AC_MARIADB_DSN)) {
        $env:AC_MARIADB_DSN = "archive_center:archive-center-local-pass@tcp(127.0.0.1:$mariaPort)/archive_center?parseTime=true"
    } elseif ($null -ne (Get-ArchiveSavedPort $DataRoot "mariadb")) {
        # The saved setting controls the managed local server, not a remote DB.
        # Do not reconstruct credentials, database names or query parameters.
        $endpoint = [regex]::Match($env:AC_MARIADB_DSN, '@tcp\(([^()]*)\)/[^/?]*(?:\?.*)?$', [Text.RegularExpressions.RegexOptions]::RightToLeft)
        if ($endpoint.Success -and $endpoint.Groups[1].Value -match '^(127\.0\.0\.1|localhost|\[::1\])(?::\d+)?$') {
            $address = $endpoint.Groups[1]
            $replacement = "$($Matches[1]):$mariaPort"
            $env:AC_MARIADB_DSN = $env:AC_MARIADB_DSN.Substring(0, $address.Index) + $replacement + $env:AC_MARIADB_DSN.Substring($address.Index + $address.Length)
        }
    }
    return [int]$mariaPort
}

function Show-ArchivePortMenu([string]$DataRoot, [string]$Service, [string]$Port) {
    if ([string]::IsNullOrWhiteSpace($Service)) {
        Write-Host "Archive Center - service ports"
        Write-Host "  1. ChromaDB      (default 8000)"
        Write-Host "  2. MariaDB       (default 3307)"
        Write-Host "  3. Go backend    (default 28080)"
        Write-Host "  0. Exit"
        switch (Read-Host "Select a service") {
            "1" { $Service = "chroma" }
            "2" { $Service = "mariadb" }
            "3" { $Service = "backend" }
            "0" { return }
            "" { return }
            default { throw "Choose 1, 2, 3, or 0." }
        }
    }
    Set-ArchiveServicePort $DataRoot $Service $Port -Prompt
    Write-Host "Database location unchanged. Start Archive Center with your usual launcher."
}
