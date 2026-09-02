$ErrorActionPreference = "Stop"

if (-not ("ArchiveCenter.ConsoleCtrlCIsolation" -as [type])) {
    Add-Type -TypeDefinition @"
using System;
using System.ComponentModel;
using System.Runtime.InteropServices;
using System.Threading;

namespace ArchiveCenter {
    public static class ConsoleCtrlCIsolation {
        [DllImport("kernel32.dll", EntryPoint = "SetConsoleCtrlHandler", SetLastError = true)]
        private static extern bool SetConsoleCtrlHandlerDefault(IntPtr handler, bool add);

        public static void SetIgnored(bool ignored) {
            if (!SetConsoleCtrlHandlerDefault(IntPtr.Zero, ignored)) {
                throw new Win32Exception(Marshal.GetLastWin32Error());
            }
        }
    }

    public sealed class ConsoleCancelGate : IDisposable {
        private const uint CTRL_C_EVENT = 0;

        private delegate bool HandlerRoutine(uint controlType);

        [DllImport("kernel32.dll", SetLastError = true)]
        private static extern bool SetConsoleCtrlHandler(HandlerRoutine handler, bool add);

        private readonly HandlerRoutine handler;
        private int cancelRequested;
        private int disposed;

        public ConsoleCancelGate() {
            handler = HandleControl;
            if (!SetConsoleCtrlHandler(handler, true)) {
                throw new Win32Exception(Marshal.GetLastWin32Error());
            }
            try {
                ConsoleCtrlCIsolation.SetIgnored(false);
            } catch {
                SetConsoleCtrlHandler(handler, false);
                throw;
            }
        }

        private bool HandleControl(uint controlType) {
            if (controlType != CTRL_C_EVENT) {
                return false;
            }
            Interlocked.Exchange(ref cancelRequested, 1);
            return true;
        }

        public bool ConsumeCancelRequested() {
            return Interlocked.Exchange(ref cancelRequested, 0) == 1;
        }

        public void Dispose() {
            if (Interlocked.Exchange(ref disposed, 1) != 0) {
                return;
            }
            if (!SetConsoleCtrlHandler(handler, false)) {
                throw new Win32Exception(Marshal.GetLastWin32Error());
            }
            GC.SuppressFinalize(this);
        }
    }
}
"@
}

function Start-ArchiveCtrlCIsolatedProcess {
    param(
        [Parameter(Mandatory = $true)][System.Diagnostics.ProcessStartInfo]$StartInfo
    )

    $process = $null
    try {
        [ArchiveCenter.ConsoleCtrlCIsolation]::SetIgnored($true)
        try {
            $process = [System.Diagnostics.Process]::Start($StartInfo)
        } finally {
            [ArchiveCenter.ConsoleCtrlCIsolation]::SetIgnored($false)
        }
        return $process
    } catch {
        if ($null -ne $process) {
            try {
                if (-not $process.HasExited) {
                    $process.Kill()
                    $process.WaitForExit()
                }
            } catch {
            }
        }
        throw
    }
}

function Wait-ArchiveProcessWithCtrlCConfirmation {
    param(
        [Parameter(Mandatory = $true)][System.Diagnostics.Process]$Process,
        [int]$PollMilliseconds = 200
    )

    $cancelGate = [ArchiveCenter.ConsoleCancelGate]::new()
    try {
        while (-not $Process.WaitForExit($PollMilliseconds)) {
            if (-not $cancelGate.ConsumeCancelRequested()) {
                continue
            }

            :confirmation while ($true) {
                $answer = [string](Read-Host "Stop Archive Center and all managed services? (Y/N)")
                switch -Regex ($answer.Trim()) {
                    '^(?i:y|yes)$' {
                        Write-Host "Stopping Archive Center and all managed services."
                        Write-Host "If cmd.exe asks whether to terminate the batch job, choose Y to close the launcher."
                        return $true
                    }
                    '^(?i:n|no)$' {
                        Write-Host "Shutdown canceled. Archive Center is still running."
                        break confirmation
                    }
                    default {
                        Write-Host "Please enter Y or N."
                    }
                }
            }
        }
        return $false
    } finally {
        $cancelGate.Dispose()
    }
}
