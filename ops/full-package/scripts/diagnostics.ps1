# Logging only: managed process ownership and Ctrl+C remain in the launcher.
function Initialize-ArchiveDiagnostics {
    if ([string]::IsNullOrWhiteSpace($env:AC_LOG_DIR)) {
        $root = $env:ARCHIVE_CENTER_DATA_DIR
        if ([string]::IsNullOrWhiteSpace($root)) { $root = Join-Path ([Environment]::GetFolderPath('LocalApplicationData')) 'ArchiveCenter\data' }
        $env:AC_LOG_DIR = Join-Path $root 'logs'
    }
    $script:archiveLogPumps = @{}
    try {
        New-Item -ItemType Directory -Path $env:AC_LOG_DIR -Force | Out-Null
        $path = Join-Path $env:AC_LOG_DIR 'launcher.log'
        if (Test-Path -LiteralPath $path) { Move-Item -LiteralPath $path -Destination ($path + '.1') -Force }
        Start-Transcript -LiteralPath $path -Force | Out-Null
        $script:archiveTranscriptStarted = $true
        Write-Host "Diagnostic logs: $env:AC_LOG_DIR"
    } catch { Write-Warning "Diagnostic log initialization failed: $($_.Exception.Message)" }
    if (-not ('ArchiveCenter.DiagnosticPump' -as [type])) {
        Add-Type -TypeDefinition @'
using System;
using System.IO;
using System.Threading.Tasks;
namespace ArchiveCenter {
    public static class DiagnosticPump {
        public static async Task Copy(Stream input, string path) {
            byte[] buffer = new byte[8192];
            bool warned = false;
            int count;
            while ((count = await input.ReadAsync(buffer, 0, buffer.Length).ConfigureAwait(false)) > 0) {
                try {
                    if (File.Exists(path) && new FileInfo(path).Length + count > 4194304) {
                        if (File.Exists(path + ".3")) File.Delete(path + ".3");
                        for (int i = 2; i >= 0; i--) {
                            string from = i == 0 ? path : path + "." + i;
                            if (File.Exists(from)) File.Move(from, path + "." + (i + 1));
                        }
                    }
                    using (var output = new FileStream(path, FileMode.Append, FileAccess.Write, FileShare.ReadWrite)) {
                        await output.WriteAsync(buffer, 0, count).ConfigureAwait(false);
                    }
                } catch (Exception e) {
                    if (!warned) { Console.Error.WriteLine("Diagnostic log write failed: " + e.Message); warned = true; }
                    // Keep draining the child even when the disk is unavailable.
                    Console.Error.Write(System.Text.Encoding.UTF8.GetString(buffer, 0, count));
                }
            }
        }
    }
}
'@
    }
}

function Start-ArchiveProcessLog($Process, [string]$FilePath) {
    $name = [IO.Path]::GetFileNameWithoutExtension($FilePath)
    $name = if ($name -match 'python|chroma') { 'chromadb' } elseif ($name -match 'maria|mysqld') { 'mariadb' } else { 'launcher' }
    $out = Join-Path $env:AC_LOG_DIR ($name + '.out.log')
    $err = Join-Path $env:AC_LOG_DIR ($name + '.err.log')
    $script:archiveLogPumps[$Process.Id] = @{
        Tasks = @([ArchiveCenter.DiagnosticPump]::Copy($Process.StandardOutput.BaseStream, $out), [ArchiveCenter.DiagnosticPump]::Copy($Process.StandardError.BaseStream, $err))
        ErrorPath = $err
    }
}

function Complete-ArchiveProcessLog($Process, [switch]$ShowError) {
    if ($null -eq $Process) { return }
    $entry = $script:archiveLogPumps[$Process.Id]
    if ($null -eq $entry) { return }
    foreach ($task in $entry.Tasks) {
        try { if (-not $task.Wait(5000)) { Write-Warning 'Diagnostic stream is still draining.' } }
        catch { Write-Warning "Diagnostic stream failed: $($_.Exception.Message)" }
    }
    if ($ShowError -and (Test-Path -LiteralPath $entry.ErrorPath)) {
        Write-Host 'Backend error output:'
        Get-Content -LiteralPath $entry.ErrorPath -Tail 25 -Encoding UTF8 | Out-Host
        Write-Host "Full diagnostic logs: $env:AC_LOG_DIR"
        Write-Host 'Run 07_export_diagnostics_windows.bat to save an error report.'
    }
    $script:archiveLogPumps.Remove($Process.Id)
}

function Write-ArchiveLauncherFailure($Failure) {
    $message = "$(Get-Date -Format o) launcher failed: $($Failure.Exception.Message)`n$($Failure.ScriptStackTrace)"
    try { Add-Content -LiteralPath (Join-Path $env:AC_LOG_DIR 'launcher.err.log') -Value $message -Encoding UTF8 }
    catch { Write-Warning "Could not save launcher error: $($_.Exception.Message)" }
    Write-Host $message -ForegroundColor Red
    Write-Host "Diagnostic logs: $env:AC_LOG_DIR"
}
