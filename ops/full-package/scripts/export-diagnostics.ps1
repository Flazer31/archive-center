param([string]$OutputPath = '')
$ErrorActionPreference = 'Stop'
$originalEncoding = [Console]::OutputEncoding
try {
    [Console]::OutputEncoding = New-Object Text.UTF8Encoding $false
    if ([string]::IsNullOrWhiteSpace($env:AC_LOG_DIR)) {
        . (Join-Path $PSScriptRoot 'service-ports.ps1')
        $env:AC_LOG_DIR = Join-Path (Get-ArchiveDataRoot) 'logs'
    }
    if (-not $OutputPath) { $OutputPath = Join-Path ([Environment]::GetFolderPath('Desktop')) ('Archive-Center-diagnostics-' + (Get-Date -Format 'yyyyMMdd-HHmmss') + '.json') }
    $versionFile = Join-Path $PSScriptRoot '..\.env.full.example'
    if (Test-Path -LiteralPath $versionFile) {
        $versionLine = Get-Content -LiteralPath $versionFile | Where-Object { $_ -match '^AC_BUILD_VERSION=' } | Select-Object -First 1
        if ($versionLine) { $env:AC_BUILD_VERSION = $versionLine.Substring('AC_BUILD_VERSION='.Length).Trim() }
    }
    # Native export never executes the backend: old binaries may interpret an
    # unknown CLI argument as normal startup, and quarantined binaries cannot run.
        $files = @()
        $collectionErrors = @()
        $names = '^(backend(?:-crash)?|launcher(?:\.out|\.err)?|chromadb\.(?:out|err)|mariadb(?:\.out|\.err|-init)?|schema|install|update(?:-candidate\.out|-candidate\.err)?)\.log(?:\.[123])?$'
        $logFiles = @()
        try { $logFiles = @(Get-ChildItem -LiteralPath $env:AC_LOG_DIR -File | Where-Object { $_.Name -match $names -and -not ($_.Attributes -band [IO.FileAttributes]::ReparsePoint) }) }
        catch { $collectionErrors += $_.Exception.Message }
        $installLog = Join-Path ([Environment]::GetFolderPath('LocalApplicationData')) 'ArchiveCenter-install.log'
        if (Test-Path -LiteralPath $installLog -PathType Leaf) { $logFiles += Get-Item -LiteralPath $installLog }
        foreach ($file in $logFiles) {
            $stream = $null
            try {
                $stream = [IO.File]::Open($file.FullName,[IO.FileMode]::Open,[IO.FileAccess]::Read,[IO.FileShare]::ReadWrite)
                $count = [int][Math]::Min(65536,$stream.Length)
                [void]$stream.Seek(-$count,[IO.SeekOrigin]::End)
                $bytes = New-Object byte[] $count
                $read = $stream.Read($bytes,0,$count)
                $text = [Text.Encoding]::UTF8.GetString($bytes,0,$read)
                $text = $text -replace '(?i)(Bearer\s+)[^\s"'',;]+','$1[REDACTED]' -replace '(?i)((?:api[_-]?key|access[_-]?token|authorization|password|secret)["'']?\s*[:=]\s*["'']?)[^\s"'',;&}]+','$1[REDACTED]' -replace '(?i)(https?://)[^/\s:@]+:[^/\s@]+@','$1[REDACTED]@' -replace '[^\s"'':]+:[^\s"'']+@(?:tcp|unix)\([^)]*\)','[REDACTED]' -replace '(?:sk-[A-Za-z0-9_-]{8,}|AIza[A-Za-z0-9_-]{20,})','[REDACTED]'
                $files += @{name=$file.Name;text=$text;truncated=($file.Length -gt 65536)}
            } catch { $files += @{name=$file.Name;error=$_.Exception.Message} }
            finally { if ($stream) { $stream.Dispose() } }
        }
        $report = @{contract_version='archive-center.diagnostics.v1';collector='native_offline';generated_at=[DateTime]::UtcNow.ToString('o');os='windows';version=$env:AC_BUILD_VERSION;log_directory=$env:AC_LOG_DIR;collection_errors=$collectionErrors;files=$files} | ConvertTo-Json -Depth 8
    [IO.File]::WriteAllText([IO.Path]::GetFullPath($OutputPath), ($report -join "`n"), (New-Object Text.UTF8Encoding $false))
    Write-Host "Report saved: $OutputPath"
} catch { Write-Host $_.Exception.Message -ForegroundColor Red; exit 1 }
finally { [Console]::OutputEncoding = $originalEncoding }
