param(
    [int]$FirstSignalDelayMilliseconds = 1500,
    [int]$SecondSignalDelayMilliseconds = 5000,
    [string]$ConsoleControlScript = ""
)

$ErrorActionPreference = "Stop"

if ($FirstSignalDelayMilliseconds -lt 250) {
    throw "FirstSignalDelayMilliseconds must be at least 250."
}
if ($SecondSignalDelayMilliseconds -le $FirstSignalDelayMilliseconds) {
    throw "SecondSignalDelayMilliseconds must be greater than FirstSignalDelayMilliseconds."
}

$repoRoot = Split-Path -Parent $PSScriptRoot
if ([string]::IsNullOrWhiteSpace($ConsoleControlScript)) {
    $consoleControlScript = Join-Path $repoRoot "ops\full-package\scripts\windows-console-control.ps1"
} else {
    $consoleControlScript = [System.IO.Path]::GetFullPath($ConsoleControlScript)
}
if (-not (Test-Path -LiteralPath $consoleControlScript -PathType Leaf)) {
    throw "Console control script not found: $consoleControlScript"
}
. $consoleControlScript

if (-not ("ArchiveCenter.CtrlCRegressionSignalSchedule" -as [type])) {
    Add-Type -TypeDefinition @"
using System;
using System.ComponentModel;
using System.Runtime.InteropServices;
using System.Threading;

namespace ArchiveCenter {
    public sealed class CtrlCRegressionSignalSchedule : IDisposable {
        private const uint CTRL_C_EVENT = 0;

        [DllImport("kernel32.dll", SetLastError = true)]
        private static extern bool GenerateConsoleCtrlEvent(uint controlType, uint processGroupId);

        private readonly Timer firstTimer;
        private readonly Timer secondTimer;
        private int signalCount;

        public CtrlCRegressionSignalSchedule(int firstDelayMilliseconds, int secondDelayMilliseconds) {
            firstTimer = new Timer(Generate, null, firstDelayMilliseconds, Timeout.Infinite);
            secondTimer = new Timer(Generate, null, secondDelayMilliseconds, Timeout.Infinite);
        }

        private void Generate(object state) {
            if (!GenerateConsoleCtrlEvent(CTRL_C_EVENT, 0)) {
                return;
            }
            Interlocked.Increment(ref signalCount);
        }

        public int SignalCount {
            get { return Interlocked.CompareExchange(ref signalCount, 0, 0); }
        }

        public void Dispose() {
            firstTimer.Dispose();
            secondTimer.Dispose();
        }
    }
}
"@
}

$powershellPath = Join-Path $env:SystemRoot "System32\WindowsPowerShell\v1.0\powershell.exe"
$startInfo = New-Object System.Diagnostics.ProcessStartInfo
$startInfo.FileName = $powershellPath
$startInfo.Arguments = '-NoProfile -Command "while ($true) { [System.Threading.Thread]::Sleep(100) }"'
$startInfo.WorkingDirectory = $repoRoot
$startInfo.UseShellExecute = $false
$startInfo.CreateNoWindow = $true

$child = $null
$schedule = $null
$passed = $false
try {
    $child = Start-ArchiveCtrlCIsolatedProcess -StartInfo $startInfo
    if ($null -eq $child -or $child.HasExited) {
        throw "The isolated regression child did not start."
    }

    Write-Host "Ctrl+C regression child PID: $($child.Id)"
    Write-Host "At the first prompt choose N. At the second prompt choose Y."
    $schedule = [ArchiveCenter.CtrlCRegressionSignalSchedule]::new(
        $FirstSignalDelayMilliseconds,
        $SecondSignalDelayMilliseconds
    )

    $shutdownConfirmed = Wait-ArchiveProcessWithCtrlCConfirmation -Process $child
    if (-not $shutdownConfirmed) {
        throw "The child exited before a confirmed shutdown. Ctrl+C isolation failed."
    }
    if ($schedule.SignalCount -ne 2) {
        throw "Expected two Ctrl+C requests, observed $($schedule.SignalCount)."
    }
    if ($child.HasExited) {
        throw "The managed child exited before the confirmed shutdown cleanup."
    }
    $passed = $true
} finally {
    if ($null -ne $schedule) {
        $schedule.Dispose()
    }
    if ($null -ne $child) {
        try {
            if (-not $child.HasExited) {
                $child.Kill()
                $child.WaitForExit()
            }
        } catch {
        }
    }
}

if ($passed) {
    Write-Host "PASS: N preserved the managed child and Y reached confirmed cleanup."
}
