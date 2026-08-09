param(
    [string]$EnvFile = ".\.env.full.local",
    [string]$BindAddr = "",
    [string]$RuntimeProfile = "",
    [string]$VectorMode = "",
    [int]$MariaDBPort = 3307
)

$ErrorActionPreference = "Stop"
$packagedBuildVersion = "__ARCHIVE_CENTER_PACKAGE_VERSION__"
$managedChromaDBVersion = "1.5.9"

if (-not ("ArchiveCenter.ManagedProcessJob" -as [type])) {
    Add-Type -TypeDefinition @"
using System;
using System.ComponentModel;
using System.Runtime.InteropServices;
using System.Threading;

namespace ArchiveCenter {
    public sealed class ManagedProcessJob : IDisposable {
        private const uint JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE = 0x00002000;
        private const uint SYNCHRONIZE = 0x00100000;
        private const uint WAIT_OBJECT_0 = 0x00000000;
        private const uint INFINITE = 0xFFFFFFFF;

        private enum JobObjectInfoType {
            ExtendedLimitInformation = 9
        }

        [StructLayout(LayoutKind.Sequential)]
        private struct JOBOBJECT_BASIC_LIMIT_INFORMATION {
            public long PerProcessUserTimeLimit;
            public long PerJobUserTimeLimit;
            public uint LimitFlags;
            public UIntPtr MinimumWorkingSetSize;
            public UIntPtr MaximumWorkingSetSize;
            public uint ActiveProcessLimit;
            public UIntPtr Affinity;
            public uint PriorityClass;
            public uint SchedulingClass;
        }

        [StructLayout(LayoutKind.Sequential)]
        private struct IO_COUNTERS {
            public ulong ReadOperationCount;
            public ulong WriteOperationCount;
            public ulong OtherOperationCount;
            public ulong ReadTransferCount;
            public ulong WriteTransferCount;
            public ulong OtherTransferCount;
        }

        [StructLayout(LayoutKind.Sequential)]
        private struct JOBOBJECT_EXTENDED_LIMIT_INFORMATION {
            public JOBOBJECT_BASIC_LIMIT_INFORMATION BasicLimitInformation;
            public IO_COUNTERS IoInfo;
            public UIntPtr ProcessMemoryLimit;
            public UIntPtr JobMemoryLimit;
            public UIntPtr PeakProcessMemoryUsed;
            public UIntPtr PeakJobMemoryUsed;
        }

        [StructLayout(LayoutKind.Sequential)]
        private struct PROCESS_BASIC_INFORMATION {
            public IntPtr Reserved1;
            public IntPtr PebBaseAddress;
            public IntPtr Reserved2_0;
            public IntPtr Reserved2_1;
            public IntPtr UniqueProcessId;
            public IntPtr InheritedFromUniqueProcessId;
        }

        [DllImport("kernel32.dll", CharSet = CharSet.Unicode, SetLastError = true)]
        private static extern IntPtr CreateJobObject(IntPtr jobAttributes, string name);

        [DllImport("kernel32.dll", SetLastError = true)]
        private static extern bool SetInformationJobObject(
            IntPtr job,
            JobObjectInfoType infoType,
            IntPtr info,
            uint infoLength);

        [DllImport("kernel32.dll", SetLastError = true)]
        private static extern bool AssignProcessToJobObject(IntPtr job, IntPtr process);

        [DllImport("kernel32.dll", SetLastError = true)]
        private static extern bool CloseHandle(IntPtr handle);

        [DllImport("kernel32.dll")]
        private static extern IntPtr GetCurrentProcess();

        [DllImport("kernel32.dll", SetLastError = true)]
        private static extern IntPtr OpenProcess(uint desiredAccess, bool inheritHandle, int processId);

        [DllImport("kernel32.dll", SetLastError = true)]
        private static extern uint WaitForSingleObject(IntPtr handle, uint milliseconds);

        [DllImport("ntdll.dll")]
        private static extern int NtQueryInformationProcess(
            IntPtr processHandle,
            int processInformationClass,
            ref PROCESS_BASIC_INFORMATION processInformation,
            uint processInformationLength,
            out uint returnLength);

        private IntPtr handle;

        public ManagedProcessJob() {
            handle = CreateJobObject(IntPtr.Zero, null);
            if (handle == IntPtr.Zero) {
                throw new Win32Exception(Marshal.GetLastWin32Error());
            }

            JOBOBJECT_EXTENDED_LIMIT_INFORMATION info = new JOBOBJECT_EXTENDED_LIMIT_INFORMATION();
            info.BasicLimitInformation.LimitFlags = JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE;
            int size = Marshal.SizeOf(typeof(JOBOBJECT_EXTENDED_LIMIT_INFORMATION));
            IntPtr pointer = Marshal.AllocHGlobal(size);
            try {
                Marshal.StructureToPtr(info, pointer, false);
                if (!SetInformationJobObject(handle, JobObjectInfoType.ExtendedLimitInformation, pointer, (uint)size)) {
                    throw new Win32Exception(Marshal.GetLastWin32Error());
                }
            } catch {
                CloseHandle(handle);
                handle = IntPtr.Zero;
                throw;
            } finally {
                Marshal.FreeHGlobal(pointer);
            }
        }

        public void AddProcess(IntPtr processHandle) {
            if (handle == IntPtr.Zero) {
                throw new ObjectDisposedException("ManagedProcessJob");
            }
            if (!AssignProcessToJobObject(handle, processHandle)) {
                throw new Win32Exception(Marshal.GetLastWin32Error());
            }
        }

        public static int GetCurrentParentProcessId() {
            PROCESS_BASIC_INFORMATION information = new PROCESS_BASIC_INFORMATION();
            uint returnLength;
            int status = NtQueryInformationProcess(
                GetCurrentProcess(),
                0,
                ref information,
                (uint)Marshal.SizeOf(typeof(PROCESS_BASIC_INFORMATION)),
                out returnLength);
            if (status != 0) {
                throw new InvalidOperationException("NtQueryInformationProcess failed with status " + status + ".");
            }
            return information.InheritedFromUniqueProcessId.ToInt32();
        }

        public void ExitWhenProcessEnds(int processId) {
            IntPtr processHandle = OpenProcess(SYNCHRONIZE, false, processId);
            if (processHandle == IntPtr.Zero) {
                throw new Win32Exception(Marshal.GetLastWin32Error());
            }
            Thread watcher = new Thread(delegate() {
                uint result = WaitForSingleObject(processHandle, INFINITE);
                CloseHandle(processHandle);
                if (result == WAIT_OBJECT_0) {
                    Dispose();
                    Environment.Exit(0);
                }
            });
            watcher.IsBackground = true;
            watcher.Start();
        }

        public void Dispose() {
            if (handle != IntPtr.Zero) {
                CloseHandle(handle);
                handle = IntPtr.Zero;
            }
            GC.SuppressFinalize(this);
        }

        ~ManagedProcessJob() {
            Dispose();
        }
    }
}
"@
}

$launcherParameters = @{}
foreach ($entry in $PSBoundParameters.GetEnumerator()) {
    $launcherParameters[$entry.Key] = $entry.Value
}

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

function Read-DotEnvContent([string]$Path) {
    $protectedPath = "$Path.protected"
    if (Test-Path -LiteralPath $protectedPath -PathType Leaf) {
        return ConvertFrom-ProtectedEnvText $protectedPath
    } elseif (Test-Path -LiteralPath $Path -PathType Leaf) {
        return Get-Content -LiteralPath $Path -Raw
    }
    throw "Env file not found: $Path or $protectedPath. Copy .env.full.example to .env.full.local first."
}

function Get-DotEnvValue([string]$Path, [string]$Name) {
    $content = Read-DotEnvContent $Path
    foreach ($rawLine in ($content -split "\r?\n")) {
        $line = $rawLine.Trim()
        if ($line -eq "" -or $line.StartsWith("#")) { continue }
        $idx = $line.IndexOf("=")
        if ($idx -lt 1) { continue }
        if ($line.Substring(0, $idx).Trim() -eq $Name) {
            return $line.Substring($idx + 1).Trim()
        }
    }
    return ""
}

function Import-DotEnv([string]$Path) {
    $protectedPath = "$Path.protected"
    if (Test-Path -LiteralPath $protectedPath -PathType Leaf) {
        Write-Host "Using protected env: $protectedPath"
    }
    $content = Read-DotEnvContent $Path
    $content -split "\r?\n" | ForEach-Object {
        $line = $_.Trim()
        if ($line -eq "" -or $line.StartsWith("#")) { return }
        $idx = $line.IndexOf("=")
        if ($idx -lt 1) { return }
        [Environment]::SetEnvironmentVariable($line.Substring(0, $idx).Trim(), $line.Substring($idx + 1).Trim(), "Process")
    }
}

function Test-PortOpen([int]$Port, [string]$ConnectHost = "127.0.0.1") {
    try {
        $addresses = [System.Net.Dns]::GetHostAddresses($ConnectHost)
    } catch {
        return $false
    }
    foreach ($address in $addresses) {
        $client = [System.Net.Sockets.TcpClient]::new($address.AddressFamily)
        try {
            $client.Connect($address, $Port)
            if ($client.Connected) {
                return $true
            }
        } catch {
            continue
        } finally {
            $client.Close()
        }
    }
    return $false
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
        try {
            $script:archiveProcessJob.AddProcess($proc.Handle)
        } catch {
            try {
                if (-not $proc.HasExited) {
                    $proc.Kill()
                    $proc.WaitForExit()
                }
            } catch {
            }
            throw "Failed to bind the managed server process to the launcher lifetime.`nFile: $FilePath`nOriginal error: $($_.Exception.Message)"
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
        $hint = "Failed to start managed runtime executable."
        if ($nativeCode -eq 1223 -or $message -match "(?i)cancel|cancell|operation.*canceled|user.*cancel") {
            $hint = "Windows cancelled the bundled runtime executable launch. This is commonly caused by Mark-of-the-Web, SmartScreen, Defender quarantine, or a cancelled security prompt. From the package root, run: Get-ChildItem -Recurse -File | Unblock-File"
        }
        throw "$hint`nFile: $FilePath`nOriginal error: $message"
    }
}

function Find-ChromaRuntimePython([string]$Root) {
    if ([string]::IsNullOrWhiteSpace($Root) -or -not (Test-Path -LiteralPath $Root -PathType Container)) {
        return ""
    }
    $candidates = @(
        "runtime\ChromaDB\1.5.9\Scripts\python.exe",
        "runtime\chromadb\1.5.9\Scripts\python.exe",
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
    $searchRoot = Join-Path $Root "runtime"
    if (-not (Test-Path -LiteralPath $searchRoot -PathType Container)) {
        $searchRoot = $Root
    }
    $hit = Get-ChildItem -LiteralPath $searchRoot -Recurse -File -ErrorAction SilentlyContinue |
        Where-Object { $_.Name -ieq "python.exe" } |
        Sort-Object FullName |
        Select-Object -First 1
    if ($null -eq $hit) {
        return ""
    }
    return $hit.FullName
}

function Test-ChromaRuntimeVersion {
    param(
        [string]$PythonPath,
        [string]$RequiredVersion = "1.5.9"
    )

    if ([string]::IsNullOrWhiteSpace($PythonPath) -or -not (Test-Path -LiteralPath $PythonPath -PathType Leaf)) {
        return $false
    }
    Unblock-PackageFile $PythonPath
    $previousErrorActionPreference = $ErrorActionPreference
    $probeExitCode = -1
    try {
        $ErrorActionPreference = "Continue"
        & $PythonPath -c "import sys; from importlib.metadata import version; import chromadb; sys.exit(0 if version('chromadb') == sys.argv[1] else 1)" $RequiredVersion *> $null
        $probeExitCode = $LASTEXITCODE
    } catch {
        $probeExitCode = -1
    } finally {
        $ErrorActionPreference = $previousErrorActionPreference
    }
    return $probeExitCode -eq 0
}

function Start-ManagedChromaDB {
    param(
        [Parameter(Mandatory = $true)][string]$PackageRoot,
        [Parameter(Mandatory = $true)][string]$RuntimeRoot,
        [Parameter(Mandatory = $true)][Uri]$Endpoint
    )

    $hostName = if ([string]::IsNullOrWhiteSpace($Endpoint.Host)) { "127.0.0.1" } else { $Endpoint.Host }
    $port = if ($Endpoint.Port -gt 0) { $Endpoint.Port } else { 8000 }
    $dataRoot = if ([string]::IsNullOrWhiteSpace($env:ARCHIVE_CENTER_DATA_DIR)) {
        Join-Path $PackageRoot ".runtime"
    } else {
        [System.IO.Path]::GetFullPath($env:ARCHIVE_CENTER_DATA_DIR)
    }
    $dataDir = Join-Path $dataRoot "chromadb"
    New-Item -ItemType Directory -Force -Path $dataDir | Out-Null

    $python = Find-ChromaRuntimePython $RuntimeRoot
    if ([string]::IsNullOrWhiteSpace($python)) {
        throw "Managed ChromaDB Python runtime not found: $RuntimeRoot"
    }
    Unblock-PackageFile $python

    if (-not (Test-ChromaRuntimeVersion -PythonPath $python -RequiredVersion $managedChromaDBVersion)) {
        throw "Managed Python does not contain the required chromadb==$managedChromaDBVersion runtime."
    }

    Write-Host "Starting managed ChromaDB"
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
    @("client_only", "vector_external", "vector_local_native", "full_local") -contains $Value
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

function Get-LowerSHA256([string]$Path) {
    (Get-FileHash -LiteralPath $Path -Algorithm SHA256).Hash.ToLowerInvariant()
}

function ConvertTo-UpdaterDiagnostic([string]$Text) {
    if ([string]::IsNullOrWhiteSpace($Text)) {
        return "<empty>"
    }
    $redacted = $Text -replace '(?i)("(?:password|passwd|token|secret|api[_-]?key)"\s*:\s*")[^"]*"', '$1<redacted>"'
    $redacted = $redacted -replace '(?i)\b(password|passwd|token|secret|api[_-]?key)\b(\s*[:=]\s*)(?:"[^"]*"|''[^'']*''|[^,\s}]+)', '$1$2<redacted>'
    $redacted = $redacted -replace '(?i)(https?://)[^/@:\s]+:[^/@\s]+@', '$1<redacted>@'
    $redacted = $redacted -replace '[\x00-\x08\x0B\x0C\x0E-\x1F]', '?'
    if ($redacted.Length -gt 2048) {
        $redacted = $redacted.Substring(0, 2048) + "...<truncated>"
    }
    return $redacted
}

function Get-UpdaterAllowedStatuses([string]$Command) {
    switch ($Command) {
        "apply-pending" { return @("no_pending", "applied_pending_health") }
        "commit" { return @("committed") }
        "rollback" { return @("rolled_back", "nothing_to_rollback") }
        "status" { return @("no_state", "applying", "applied_pending_health", "committed", "rolled_back") }
        default { throw "Unsupported updater command: $Command" }
    }
}

function Invoke-ArchiveUpdater {
    param(
        [Parameter(Mandatory = $true)][string]$RunnerPath,
        [Parameter(Mandatory = $true)][string]$Command,
        [Parameter(Mandatory = $true)][string]$PackageRoot
    )

    if (-not (Test-Path -LiteralPath $RunnerPath -PathType Leaf)) {
        throw "Archive Center updater runner is missing: $RunnerPath"
    }
    $runnerSHA256 = Get-LowerSHA256 $RunnerPath
    $startInfo = New-Object System.Diagnostics.ProcessStartInfo
    $startInfo.FileName = $RunnerPath
    $startInfo.Arguments = Join-Args @($Command, "--root", $PackageRoot)
    $startInfo.WorkingDirectory = $PackageRoot
    $startInfo.UseShellExecute = $false
    $startInfo.CreateNoWindow = $true
    $startInfo.RedirectStandardOutput = $true
    $startInfo.RedirectStandardError = $true
    $process = New-Object System.Diagnostics.Process
    $process.StartInfo = $startInfo
    try {
        if (-not $process.Start()) {
            throw "Process.Start returned false."
        }
    } catch {
        throw "Archive Center updater failed to start for '$Command' (runner_sha256=$runnerSHA256): $(ConvertTo-UpdaterDiagnostic $_.Exception.Message)"
    }
    $stdoutTask = $process.StandardOutput.ReadToEndAsync()
    $stderrTask = $process.StandardError.ReadToEndAsync()
    $process.WaitForExit()
    $stdout = ([string]$stdoutTask.GetAwaiter().GetResult()).Trim()
    $stderr = ([string]$stderrTask.GetAwaiter().GetResult()).Trim()
    $exitCode = $process.ExitCode
    $diagnostic = "exit=$exitCode runner_sha256=$runnerSHA256 stdout=$(ConvertTo-UpdaterDiagnostic $stdout) stderr=$(ConvertTo-UpdaterDiagnostic $stderr)"

    if ($exitCode -eq 0) {
        if ([string]::IsNullOrWhiteSpace($stdout) -or -not [string]::IsNullOrWhiteSpace($stderr)) {
            throw "Archive Center updater violated success IPC for '$Command'. $diagnostic"
        }
        $payloadText = $stdout
    } else {
        if (-not [string]::IsNullOrWhiteSpace($stdout) -or [string]::IsNullOrWhiteSpace($stderr)) {
            throw "Archive Center updater violated failure IPC for '$Command'. $diagnostic"
        }
        $payloadText = $stderr
    }
    try {
        $result = $payloadText | ConvertFrom-Json -ErrorAction Stop
    } catch {
        throw "Archive Center updater returned invalid JSON for '$Command'. $diagnostic"
    }
    if ($null -eq $result -or $result -is [System.Array]) {
        throw "Archive Center updater returned a non-object response for '$Command'. $diagnostic"
    }
    $contractVersion = ([string]$result.contract_version).Trim()
    $resultAction = ([string]$result.action).Trim()
    $status = ([string]$result.status).Trim().ToLowerInvariant()
    if ($contractVersion -cne "archive-center.updater-result.v1" -or $resultAction -cne $Command) {
        throw "Archive Center updater contract/action mismatch for '$Command'. $diagnostic"
    }
    if ($exitCode -eq 0) {
        if ($status -notin @(Get-UpdaterAllowedStatuses $Command)) {
            throw "Archive Center updater returned unsupported success status '$status' for '$Command'. $diagnostic"
        }
    } elseif ($status -cne "error" -or [string]::IsNullOrWhiteSpace([string]$result.code)) {
        throw "Archive Center updater returned an invalid failure contract for '$Command'. $diagnostic"
    }
    return [pscustomobject]@{
        ExitCode = $exitCode
        Status = $status
        Result = $result
        RunnerSHA256 = $runnerSHA256
        Diagnostic = $diagnostic
    }
}

function Write-UpdaterRunnerIdentity {
    param(
        [Parameter(Mandatory = $true)][string]$PackageRoot,
        [Parameter(Mandatory = $true)][string]$RunnerPath,
        [Parameter(Mandatory = $true)][string]$TargetVersion
    )
    $runnerRoot = [System.IO.Path]::GetFullPath((Join-Path $PackageRoot ".updates\runner")).TrimEnd('\')
    $runnerFull = [System.IO.Path]::GetFullPath($RunnerPath)
    if (-not $runnerFull.StartsWith($runnerRoot + '\', [System.StringComparison]::OrdinalIgnoreCase)) {
        throw "Updater runner identity escaped .updates\runner."
    }
    if ([string]::IsNullOrWhiteSpace($TargetVersion)) {
        throw "Updater runner identity requires a target version."
    }
    $relative = $runnerFull.Substring([System.IO.Path]::GetFullPath($PackageRoot).TrimEnd('\').Length).TrimStart('\').Replace('\', '/')
    $identity = [ordered]@{
        contract_version = "archive-center.updater-runner-identity.v1"
        target_version = $TargetVersion.Trim()
        runner_path = $relative
        runner_sha256 = Get-LowerSHA256 $runnerFull
        written_at = [DateTimeOffset]::UtcNow.ToString("o")
    }
    $identityPath = Join-Path $PackageRoot ".updates\runner-identity.json"
    $temporaryPath = "$identityPath.tmp"
    New-Item -ItemType Directory -Force -Path (Split-Path -Parent $identityPath) | Out-Null
    [System.IO.File]::WriteAllText(
        $temporaryPath,
        ($identity | ConvertTo-Json -Depth 5) + [Environment]::NewLine,
        (New-Object System.Text.UTF8Encoding($false))
    )
    Move-Item -LiteralPath $temporaryPath -Destination $identityPath -Force
}

function New-BoundUpdaterRunner {
    param(
        [Parameter(Mandatory = $true)][string]$PackageRoot,
        [Parameter(Mandatory = $true)][string]$UpdaterPath,
        [string]$TargetVersion = ""
    )
    if (-not (Test-Path -LiteralPath $UpdaterPath -PathType Leaf)) {
        throw "Archive Center updater is missing: $UpdaterPath"
    }
    $runnerSHA256 = Get-LowerSHA256 $UpdaterPath
    $runnerRoot = Join-Path $PackageRoot ".updates\runner"
    New-Item -ItemType Directory -Force -Path $runnerRoot | Out-Null
    $runner = Join-Path $runnerRoot ("archive-center-updater-$runnerSHA256.exe")
    if (Test-Path -LiteralPath $runner -PathType Leaf) {
        if ((Get-LowerSHA256 $runner) -cne $runnerSHA256) {
            throw "Existing updater runner hash mismatched its bound filename."
        }
    } else {
        Copy-Item -LiteralPath $UpdaterPath -Destination $runner
    }
    if (-not [string]::IsNullOrWhiteSpace($TargetVersion)) {
        Write-UpdaterRunnerIdentity -PackageRoot $PackageRoot -RunnerPath $runner -TargetVersion $TargetVersion
    }
    return $runner
}

function Resolve-BoundUpdaterRunner {
    param(
        [Parameter(Mandatory = $true)][string]$PackageRoot,
        [Parameter(Mandatory = $true)]$State
    )
    $identityPath = Join-Path $PackageRoot ".updates\runner-identity.json"
    if (-not (Test-Path -LiteralPath $identityPath -PathType Leaf)) {
        throw "Active update state has no bound updater runner identity."
    }
    try {
        $identity = Get-Content -LiteralPath $identityPath -Raw -Encoding UTF8 | ConvertFrom-Json -ErrorAction Stop
    } catch {
        throw "Updater runner identity is unreadable."
    }
    if ([string]$identity.contract_version -cne "archive-center.updater-runner-identity.v1") {
        throw "Updater runner identity contract mismatch."
    }
    $stateTarget = ([string]$State.target_version).Trim()
    if ([string]::IsNullOrWhiteSpace($stateTarget) -or ([string]$identity.target_version).Trim() -cne $stateTarget) {
        throw "Updater runner identity target does not match active update state."
    }
    $relative = ([string]$identity.runner_path).Replace('/', '\')
    $runnerRoot = [System.IO.Path]::GetFullPath((Join-Path $PackageRoot ".updates\runner")).TrimEnd('\')
    $runner = [System.IO.Path]::GetFullPath((Join-Path $PackageRoot $relative))
    if (-not $runner.StartsWith($runnerRoot + '\', [System.StringComparison]::OrdinalIgnoreCase) -or
        -not (Test-Path -LiteralPath $runner -PathType Leaf)) {
        throw "Updater runner identity path is missing or unsafe."
    }
    $identitySHA256 = ([string]$identity.runner_sha256).Trim().ToLowerInvariant()
    if ($identitySHA256 -notmatch '^[0-9a-f]{64}$' -or (Get-LowerSHA256 $runner) -cne $identitySHA256) {
        throw "Updater runner identity SHA256 mismatch."
    }
    $stateRunnerPath = ([string]$State.runner_path).Replace('/', '\')
    $stateRunnerSHA256 = ([string]$State.runner_sha256).Trim().ToLowerInvariant()
    if (-not [string]::IsNullOrWhiteSpace($stateRunnerPath) -or -not [string]::IsNullOrWhiteSpace($stateRunnerSHA256)) {
        if ([string]::IsNullOrWhiteSpace($stateRunnerPath) -or [string]::IsNullOrWhiteSpace($stateRunnerSHA256)) {
            throw "Active update state has an incomplete runner identity."
        }
        $stateRunner = [System.IO.Path]::GetFullPath((Join-Path $PackageRoot $stateRunnerPath))
        if ($stateRunner -cne $runner -or $stateRunnerSHA256 -cne $identitySHA256) {
            throw "Updater runner identity does not match the durable update state."
        }
    }
    return $runner
}

function Test-UpdaterSafeBaselineStatus([string]$Status) {
    $Status -in @("no_pending", "rolled_back", "nothing_to_rollback")
}

function Wait-BackendMainReady {
    param(
        [Parameter(Mandatory = $true)][System.Diagnostics.Process]$Process,
        [Parameter(Mandatory = $true)][int]$Port,
        [Parameter(Mandatory = $true)][string]$ExpectedVersion,
        [int]$TimeoutSeconds = 60
    )

    $lastError = ""
    for ($i = 0; $i -lt $TimeoutSeconds; $i++) {
        if ($Process.HasExited) {
            return [pscustomobject]@{ Ready = $false; Detail = "backend exited with code $($Process.ExitCode)" }
        }
        try {
            $ready = Invoke-RestMethod -Method GET -Uri "http://127.0.0.1:$Port/ready" -TimeoutSec 2
            # Reference-vector degradation is intentionally not a failure here.
            if ($ready.ready -eq $true) {
                $version = Invoke-RestMethod -Method GET -Uri "http://127.0.0.1:$Port/version" -TimeoutSec 2
                if ([string]$version.version -eq $ExpectedVersion) {
                    $Process.Refresh()
                    if (-not $Process.HasExited) {
                        return [pscustomobject]@{ Ready = $true; Detail = "main ready at expected version $ExpectedVersion" }
                    }
                } else {
                    $lastError = "version mismatch: expected $ExpectedVersion, got $($version.version)"
                }
            } else {
                $lastError = "main ready=false"
            }
        } catch {
            $lastError = $_.Exception.Message
        }
        Start-Sleep -Seconds 1
    }
    return [pscustomobject]@{ Ready = $false; Detail = "ready timeout: $lastError" }
}

function Stop-ArchiveChildProcess([System.Diagnostics.Process[]]$Process) {
    $targets = @(
        $Process |
            Where-Object { $null -ne $_ } |
            Group-Object -Property Id |
            ForEach-Object { $_.Group[0] }
    )
    foreach ($target in $targets) {
        if ($target.HasExited) {
            continue
        }
        try {
            [void]$target.CloseMainWindow()
        } catch {
            # The managed child may not own a window. The bounded wait below
            # still preserves the existing graceful-exit opportunity.
        }
    }
    $shutdownDeadline = [DateTime]::UtcNow.AddSeconds(10)
    foreach ($target in $targets) {
        if ($target.HasExited) {
            continue
        }
        $remainingMilliseconds = [Math]::Max(0, [int][Math]::Ceiling(($shutdownDeadline - [DateTime]::UtcNow).TotalMilliseconds))
        if ($remainingMilliseconds -gt 0) {
            [void]$target.WaitForExit($remainingMilliseconds)
        }
    }
    foreach ($target in $targets) {
        if (-not $target.HasExited) {
            $target.Kill()
            $target.WaitForExit()
        }
    }
}

function Wait-ArchiveBackendLifetime {
    param(
        [Parameter(Mandatory = $true)][System.Diagnostics.Process]$Process,
        [int]$Port = 28080,
        [string]$ExpectedVersion = ""
    )
    if (-not [string]::IsNullOrWhiteSpace($ExpectedVersion)) {
        $health = Wait-BackendMainReady -Process $Process -Port $Port -ExpectedVersion $ExpectedVersion -TimeoutSeconds 60
        if (-not $health.Ready) {
            Stop-ArchiveChildProcess $Process
            throw "Restored backend failed /ready or exact /version verification: $($health.Detail)"
        }
        Write-Host "Restored backend passed /ready and exact /version verification."
    }
    Wait-Process -InputObject $Process
    $Process.Refresh()
    return [int]$Process.ExitCode
}

function Get-RelativeDataFileMap([string]$Root) {
    $rootFull = [System.IO.Path]::GetFullPath($Root).TrimEnd('\')
    $result = @{}
    if (-not (Test-Path -LiteralPath $rootFull -PathType Container)) {
        return $result
    }
    foreach ($file in @(Get-ChildItem -LiteralPath $rootFull -Recurse -File -Force | Sort-Object FullName)) {
        $relative = $file.FullName.Substring($rootFull.Length).TrimStart('\').Replace('\', '/')
        $result[$relative] = "$($file.Length):$(Get-LowerSHA256 $file.FullName)"
    }
    return $result
}

function Copy-LegacyDataDirectoryVerified([string]$Source, [string]$Destination) {
    $destinationFull = [System.IO.Path]::GetFullPath($Destination)
    $destinationParent = Split-Path -Parent $destinationFull
    $destinationLeaf = Split-Path -Leaf $destinationFull
    New-Item -ItemType Directory -Force -Path $destinationParent | Out-Null
    $staging = Join-Path $destinationParent (".$destinationLeaf.import-" + [Guid]::NewGuid().ToString("N"))
    New-Item -ItemType Directory -Path $staging | Out-Null
    $sourceFull = [System.IO.Path]::GetFullPath($Source).TrimEnd('\')
    try {
        foreach ($sourceFile in @(Get-ChildItem -LiteralPath $sourceFull -Recurse -File -Force | Sort-Object FullName)) {
            $relative = $sourceFile.FullName.Substring($sourceFull.Length).TrimStart('\')
            $stagedFile = Join-Path $staging $relative
            New-Item -ItemType Directory -Force -Path (Split-Path -Parent $stagedFile) | Out-Null
            Copy-Item -LiteralPath $sourceFile.FullName -Destination $stagedFile
        }
        $sourceMap = Get-RelativeDataFileMap $Source
        $stagingMap = Get-RelativeDataFileMap $staging
        if ($sourceMap.Count -ne $stagingMap.Count) {
            throw "Stable data import staging count mismatch for $Destination"
        }
        foreach ($relative in $sourceMap.Keys) {
            if (-not $stagingMap.ContainsKey($relative) -or $stagingMap[$relative] -cne $sourceMap[$relative]) {
                throw "Stable data import staging mismatch: $relative"
            }
        }
        if (Test-Path -LiteralPath $destinationFull) {
            $destinationMap = Get-RelativeDataFileMap $destinationFull
            if ($sourceMap.Count -ne $destinationMap.Count) {
                throw "Stable data import found a conflicting destination directory: $Destination"
            }
            foreach ($relative in $sourceMap.Keys) {
                if (-not $destinationMap.ContainsKey($relative) -or $destinationMap[$relative] -cne $sourceMap[$relative]) {
                    throw "Stable data import found a conflicting destination file: $relative"
                }
            }
            return
        }
        Move-Item -LiteralPath $staging -Destination $destinationFull
        $staging = ""
        $destinationMap = Get-RelativeDataFileMap $destinationFull
        if ($sourceMap.Count -ne $destinationMap.Count) {
            throw "Stable data import verification count mismatch for $Destination"
        }
        foreach ($relative in $sourceMap.Keys) {
            if (-not $destinationMap.ContainsKey($relative) -or $destinationMap[$relative] -cne $sourceMap[$relative]) {
                throw "Stable data import verification mismatch: $relative"
            }
        }
    } finally {
        if (-not [string]::IsNullOrWhiteSpace($staging) -and (Test-Path -LiteralPath $staging)) {
            Remove-Item -LiteralPath $staging -Recurse -Force
        }
    }
}

function Get-LegacyChromaImportTargets([int]$DefaultPort = 8000) {
    $targets = [ordered]@{}
    foreach ($defaultHost in @("127.0.0.1", "::1")) {
        $targets["$defaultHost`:$DefaultPort"] = [pscustomobject]@{
            Host = $defaultHost
            Port = $DefaultPort
        }
    }
    $endpointText = [string]$env:AC_CHROMA_ENDPOINT
    if (-not [string]::IsNullOrWhiteSpace($endpointText)) {
        $endpoint = $null
        if ([Uri]::TryCreate($endpointText.Trim(), [UriKind]::Absolute, [ref]$endpoint)) {
            $endpointHost = $endpoint.DnsSafeHost
            $endpointAddress = $null
            $isLoopback = $endpointHost.Equals("localhost", [System.StringComparison]::OrdinalIgnoreCase)
            if ([System.Net.IPAddress]::TryParse($endpointHost, [ref]$endpointAddress)) {
                $isLoopback = [System.Net.IPAddress]::IsLoopback($endpointAddress)
            }
        }
        if ($null -ne $endpoint -and $isLoopback) {
            $endpointPort = $(if ($endpoint.IsDefaultPort) { 8000 } else { $endpoint.Port })
            $targets["$($endpointHost.ToLowerInvariant())`:$endpointPort"] = [pscustomobject]@{
                Host = $endpointHost
                Port = $endpointPort
            }
        }
    }
    @($targets.Values | Where-Object { $_.Port -gt 0 -and $_.Port -le 65535 })
}

function Assert-LegacyChromaOffline([string]$Source, [int]$DefaultPort = 8000) {
    foreach ($target in @(Get-LegacyChromaImportTargets -DefaultPort $DefaultPort)) {
        if (Test-PortOpen -Port $target.Port -ConnectHost $target.Host) {
            $displayHost = $(if ($target.Host.Contains(":")) { "[$($target.Host)]" } else { $target.Host })
            throw "Legacy ChromaDB data may be active at $displayHost`:$($target.Port). Stop ChromaDB before importing raw data files from $Source."
        }
    }
}

function Import-LegacyRuntimeDataOnce([string]$PackageRoot, [string]$DataRoot, [int]$ChromaPort = 8000) {
    $dataRootFull = [System.IO.Path]::GetFullPath($DataRoot)
    $packageRootFull = [System.IO.Path]::GetFullPath($PackageRoot)
    $markerPath = Join-Path $dataRootFull ".archive-center-legacy-runtime-import-v1.json"
    if (Test-Path -LiteralPath $markerPath -PathType Leaf) {
        try {
            $marker = Get-Content -LiteralPath $markerPath -Raw -Encoding UTF8 | ConvertFrom-Json -ErrorAction Stop
        } catch {
            throw "Stable data import marker is unreadable: $markerPath"
        }
        if ([string]$marker.contract_version -cne "archive-center.legacy-runtime-import.v1" -or
            ([string]$marker.data_root).Trim() -cne $dataRootFull) {
            throw "Stable data import marker contract/path mismatch."
        }
        return
    }

    $legacyRoot = Join-Path $packageRootFull ".runtime"
    $mariaCandidates = @(
        (Join-Path $legacyRoot "mariadb"),
        (Join-Path $legacyRoot "mariadb-data")
    ) | Where-Object { Test-Path -LiteralPath $_ -PathType Container }
    if (@($mariaCandidates).Count -gt 1) {
        throw "Both legacy MariaDB data layouts exist; refusing an ambiguous automatic import."
    }
    $imports = @()
    if (@($mariaCandidates).Count -eq 1) {
        $imports += [pscustomobject]@{ Name = "mariadb"; Source = [string](@($mariaCandidates)[0]); Destination = (Join-Path $dataRootFull "mariadb") }
    }
    $legacyChroma = Join-Path $legacyRoot "chromadb"
    if (Test-Path -LiteralPath $legacyChroma -PathType Container) {
        $imports += [pscustomobject]@{ Name = "chromadb"; Source = $legacyChroma; Destination = (Join-Path $dataRootFull "chromadb") }
    }
    if ($imports.Count -eq 0) {
        return
    }

    New-Item -ItemType Directory -Force -Path $dataRootFull | Out-Null
    $completed = @()
    foreach ($item in $imports) {
        if ($item.Name -eq "mariadb") {
            $mariaProcesses = @(
                Get-Process -Name "mariadbd", "mysqld" -ErrorAction SilentlyContinue
            )
            if ((Test-PortOpen $MariaDBPort) -or $mariaProcesses.Count -gt 0) {
                throw "Legacy MariaDB data may be active. Stop MariaDB and ensure port $MariaDBPort is closed before importing raw data files."
            }
        } elseif ($item.Name -eq "chromadb") {
            Assert-LegacyChromaOffline -Source $item.Source -DefaultPort $ChromaPort
        }
        Copy-LegacyDataDirectoryVerified -Source $item.Source -Destination $item.Destination
        $completed += [ordered]@{
            name = $item.Name
            source = [System.IO.Path]::GetFullPath($item.Source)
            destination = [System.IO.Path]::GetFullPath($item.Destination)
            file_count = (Get-RelativeDataFileMap $item.Source).Count
        }
    }
    $markerValue = [ordered]@{
        contract_version = "archive-center.legacy-runtime-import.v1"
        data_root = $dataRootFull
        source_preserved = $true
        verified = $true
        completed_at = [DateTimeOffset]::UtcNow.ToString("o")
        imports = @($completed)
    }
    $temporaryPath = "$markerPath.tmp"
    [System.IO.File]::WriteAllText(
        $temporaryPath,
        ($markerValue | ConvertTo-Json -Depth 6) + [Environment]::NewLine,
        (New-Object System.Text.UTF8Encoding($false))
    )
    Move-Item -LiteralPath $temporaryPath -Destination $markerPath -Force
    foreach ($volatile in @("mariadb\mysql.sock", "mariadb\mariadb.pid", "mariadb\mysqld.pid")) {
        Remove-Item -LiteralPath (Join-Path $dataRootFull $volatile) -Force -ErrorAction SilentlyContinue
    }
    Write-Host "Imported and verified legacy package-local runtime data into: $dataRootFull"
    Write-Host "Legacy source data was preserved."
}

$packRoot = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
Set-Location $packRoot

$backendExe = Join-Path $packRoot "bin\archive-center-go.exe"
$profileBeforeApply = if (-not [string]::IsNullOrWhiteSpace($RuntimeProfile)) {
    $RuntimeProfile
} else {
    Get-DotEnvValue $EnvFile "AC_RUNTIME_PROFILE"
}
if ([string]::IsNullOrWhiteSpace($profileBeforeApply)) {
    $profileBeforeApply = "full_local"
}
$profileBeforeApply = $profileBeforeApply.Trim().ToLowerInvariant()

$updaterExe = Join-Path $packRoot "bin\archive-center-updater.exe"
$updaterRunnerDir = Join-Path $packRoot ".updates\runner"
$pendingMarker = Join-Path $packRoot ".updates\pending-update.json"
$stateMarker = Join-Path $packRoot ".updates\update-state.json"
$statePreviousMarker = "$stateMarker.previous"
$pendingMarkerPresent = Test-Path -LiteralPath $pendingMarker -PathType Leaf
$stateMarkerPresent = (Test-Path -LiteralPath $stateMarker -PathType Leaf) -or (Test-Path -LiteralPath $statePreviousMarker -PathType Leaf)
$stateStatusSource = if (Test-Path -LiteralPath $stateMarker -PathType Leaf) { $stateMarker } else { $statePreviousMarker }
$updateStatePresent = $pendingMarkerPresent -or $stateMarkerPresent
$updaterRunner = ""
$pendingApplyStatus = "no_pending"
$pendingTargetVersion = ""
$pendingCurrentVersion = ""
$observedStateStatus = ""
$observedState = $null
$pendingManifest = $null
$pendingMarkerTargetVersion = ""
$updaterRunnerCleanupAllowed = $true
if ($pendingMarkerPresent) {
    try {
        $pendingManifest = Get-Content -LiteralPath $pendingMarker -Raw -Encoding UTF8 | ConvertFrom-Json -ErrorAction Stop
    } catch {
        throw "Pending update manifest is unreadable. Startup stopped to avoid a mixed package."
    }
    if ([string]$pendingManifest.contract_version -cne "archive-center.pending-update.v1") {
        throw "Pending update manifest contract mismatch. Startup stopped to avoid a mixed package."
    }
    $pendingMarkerTargetVersion = ([string]$pendingManifest.target_version).Trim()
    if ([string]::IsNullOrWhiteSpace($pendingMarkerTargetVersion)) {
        throw "Pending update manifest has no target version. Startup stopped to avoid a mixed package."
    }
}
if ($stateMarkerPresent) {
    try {
        $observedState = Get-Content -LiteralPath $stateStatusSource -Raw -Encoding UTF8 | ConvertFrom-Json
        $observedStateStatus = ([string]$observedState.status).Trim().ToLowerInvariant()
    } catch {
        $observedStateStatus = "invalid"
    }
    if ([string]$observedState.contract_version -cne "archive-center.update-state.v1" -or
        $observedStateStatus -notin @("applying", "applied_pending_health", "committed", "rolled_back")) {
        $observedStateStatus = "invalid"
    }
}

if ($profileBeforeApply -eq "client_only") {
    $clientStateStatus = if ($stateMarkerPresent) { $observedStateStatus } else { "no_state" }
    if ($clientStateStatus -in @("applying", "applied_pending_health")) {
        $updaterRunner = Resolve-BoundUpdaterRunner -PackageRoot $packRoot -State $observedState
        $updaterRunnerCleanupAllowed = $false
        Write-Host "Using preserved updater recovery runner: $updaterRunner"
        Unblock-PackageFile $updaterRunner
        $clientRollback = Invoke-ArchiveUpdater -RunnerPath $updaterRunner -Command "rollback" -PackageRoot $packRoot
        if ($clientRollback.ExitCode -ne 0 -or $clientRollback.Status -notin @("rolled_back", "nothing_to_rollback")) {
            throw "client_only profile could not safely roll back active update state '$clientStateStatus'. Startup stopped to avoid a mixed package."
        }
        $updaterRunnerCleanupAllowed = $true
        Write-Host "client_only profile restored the baseline package instead of entering an unsupported backend health gate."
    } elseif ($stateMarkerPresent -and $clientStateStatus -notin @("committed", "rolled_back", "no_state")) {
        throw "client_only profile found unsupported update state '$clientStateStatus'. Startup stopped because local backend health commit is unavailable."
    } elseif ($updateStatePresent) {
        Write-Host "client_only profile: pending package apply is deferred because no local backend health gate is available."
    }
} else {
    if ($observedStateStatus -in @("applying", "applied_pending_health")) {
        $updaterRunner = Resolve-BoundUpdaterRunner -PackageRoot $packRoot -State $observedState
        $updaterRunnerCleanupAllowed = $false
        Write-Host "Using preserved updater recovery runner: $updaterRunner"
    } elseif ($updateStatePresent -and (Test-Path -LiteralPath $updaterExe -PathType Leaf)) {
        $updaterRunner = New-BoundUpdaterRunner -PackageRoot $packRoot -UpdaterPath $updaterExe -TargetVersion $pendingMarkerTargetVersion
        if ($pendingMarkerPresent) {
            $updaterRunnerCleanupAllowed = $false
        }
    } elseif ($updateStatePresent) {
        throw "Archive Center updater is missing while pending update state exists. No bound recovery runner is available; startup stopped to avoid a mixed package."
    } elseif (Test-Path -LiteralPath $updaterExe -PathType Leaf) {
        Write-Host "No pending Archive Center update exists. Normal startup will continue."
    } else {
        Write-Host "Warning: Archive Center updater is not installed. No pending state exists, so normal startup will continue."
    }

    if (-not [string]::IsNullOrWhiteSpace($updaterRunner)) {
        Unblock-PackageFile $updaterRunner
        $applyFailure = ""
        try {
            $pendingApply = Invoke-ArchiveUpdater -RunnerPath $updaterRunner -Command "apply-pending" -PackageRoot $packRoot
            $pendingApplyStatus = $pendingApply.Status
            $pendingTargetVersion = ([string]$pendingApply.Result.target_version).Trim()
            $pendingCurrentVersion = ([string]$pendingApply.Result.current_version).Trim()
            if ($pendingApply.ExitCode -ne 0 -or ($pendingApplyStatus -ne "applied_pending_health" -and -not (Test-UpdaterSafeBaselineStatus $pendingApplyStatus))) {
                $applyFailure = "status '$pendingApplyStatus' (exit $($pendingApply.ExitCode))"
            }
        } catch {
            $applyFailure = $_.Exception.Message
        }

        if (-not [string]::IsNullOrWhiteSpace($applyFailure)) {
            try {
                $safety = Invoke-ArchiveUpdater -RunnerPath $updaterRunner -Command "status" -PackageRoot $packRoot
            } catch {
                throw "Updater apply-pending failed ($applyFailure), and update state could not be verified. Startup stopped to avoid a mixed package.`n$($_.Exception.Message)"
            }
            if ($safety.ExitCode -eq 0 -and $safety.Status -in @("no_state", "rolled_back", "nothing_to_rollback")) {
                $pendingApplyStatus = "no_pending"
                $updaterRunnerCleanupAllowed = $true
                Write-Host "Warning: pending update was rejected before a live package mutation ($applyFailure). State is '$($safety.Status)'; continuing with the existing package."
            } else {
                throw "Updater apply-pending failed ($applyFailure), and state is '$($safety.Status)'. Startup stopped to avoid a mixed package."
            }
        } elseif ($pendingApplyStatus -eq "applied_pending_health") {
            $updaterRunnerCleanupAllowed = $false
            Write-Host "Applied a verified pending package. Main readiness will be checked before commit."
        } elseif (Test-UpdaterSafeBaselineStatus $pendingApplyStatus) {
            $updaterRunnerCleanupAllowed = $true
            Write-Host "Updater reported '$pendingApplyStatus'; continuing with the verified baseline package."
        }
    }
}

function Start-ArchiveBackendProcess([string]$BackendPath, [string]$PackageRoot) {
    $updates = Join-Path $PackageRoot ".updates"
    New-Item -ItemType Directory -Force -Path $updates | Out-Null
    $token = [guid]::NewGuid().ToString("N") + [guid]::NewGuid().ToString("N")
    $env:AC_UPDATE_LAUNCHER_TOKEN = $token
    $session = [ordered]@{
        contract_version = "archive-center.update-launcher-session.v1"
        token = $token
    }
    $path = Join-Path $updates "launcher-session.json"
    $temporary = "$path.tmp"
    [System.IO.File]::WriteAllText(
        $temporary,
        ($session | ConvertTo-Json -Compress) + [Environment]::NewLine,
        (New-Object System.Text.UTF8Encoding($false))
    )
    Move-Item -LiteralPath $temporary -Destination $path -Force
    return Start-ArchiveChildProcess -FilePath $BackendPath -WorkingDirectory $PackageRoot
}

Import-DotEnv $EnvFile
$env:AC_UPDATE_STAGING_DIR = Join-Path $packRoot ".updates"
$env:AC_UPDATE_APPLY_MODE = "managed_launcher_exit_75"
if ($pendingApplyStatus -eq "applied_pending_health" -and -not [string]::IsNullOrWhiteSpace($pendingTargetVersion)) {
    $env:AC_BUILD_VERSION = $pendingTargetVersion
} elseif ($packagedBuildVersion -notmatch '^__ARCHIVE_CENTER_' -and -not [string]::IsNullOrWhiteSpace($packagedBuildVersion)) {
    # Build identity belongs to the package, not to a preserved user env file.
    $env:AC_BUILD_VERSION = $packagedBuildVersion
}
if (-not [string]::IsNullOrWhiteSpace($BindAddr)) {
    $env:AC_BIND_ADDR = $BindAddr
}
$profileCandidate = if (-not [string]::IsNullOrWhiteSpace($RuntimeProfile)) { $RuntimeProfile } elseif (-not [string]::IsNullOrWhiteSpace($env:AC_RUNTIME_PROFILE)) { $env:AC_RUNTIME_PROFILE } else { "full_local" }
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
        default { $vectorCandidate = "bundled" }
    }
}
$vectorCandidate = $vectorCandidate.Trim().ToLowerInvariant()
if (-not (Test-AllowedVectorMode $vectorCandidate)) {
    throw "Unsupported vector mode: $vectorCandidate"
}
if ($env:AC_RUNTIME_PROFILE -eq "client_only" -and $vectorCandidate -ne "off") {
    throw "client_only requires AC_VECTOR_MODE=off"
}
if ($env:AC_RUNTIME_PROFILE -eq "vector_external" -and $vectorCandidate -ne "external") {
    throw "vector_external requires AC_VECTOR_MODE=external"
}
if ($env:AC_RUNTIME_PROFILE -ne "client_only" -and -not (Test-VectorRequiresChroma $vectorCandidate)) {
    throw "This Windows full package requires an active ChromaDB vector mode. Use bundled local ChromaDB or an external ChromaDB endpoint."
}
$env:AC_VECTOR_MODE = $vectorCandidate

if ($env:AC_RUNTIME_PROFILE -eq "client_only") {
    Write-Host "Archive Center client_only profile selected."
    Write-Host "No local backend, MariaDB, or ChromaDB service will be started on this device."
    Write-Host "Configure the RisuAI plugin Bridge URL to the PC/NAS Archive Center backend."
    if ($updaterRunnerCleanupAllowed) {
        Remove-Item -LiteralPath $updaterRunner -Force -ErrorAction SilentlyContinue
        Remove-Item -LiteralPath (Join-Path $packRoot ".updates\runner-identity.json") -Force -ErrorAction SilentlyContinue
    }
    exit 0
}
Normalize-ProcessPathForStartProcess

$localAppData = [Environment]::GetFolderPath("LocalApplicationData")
if ([string]::IsNullOrWhiteSpace($localAppData)) {
    $localAppData = Join-Path $env:USERPROFILE "AppData\Local"
}
$managedRuntimeInstallRoot = Join-Path $localAppData "ArchiveCenter"
$stableDataRoot = if ([string]::IsNullOrWhiteSpace($env:ARCHIVE_CENTER_DATA_DIR)) {
    Join-Path $managedRuntimeInstallRoot "data"
} else {
    [System.IO.Path]::GetFullPath($env:ARCHIVE_CENTER_DATA_DIR)
}
$env:ARCHIVE_CENTER_DATA_DIR = [System.IO.Path]::GetFullPath($stableDataRoot)
Import-LegacyRuntimeDataOnce -PackageRoot $packRoot -DataRoot $env:ARCHIVE_CENTER_DATA_DIR
$mariaInstallRoot = $managedRuntimeInstallRoot
$mariaRuntimeRoot = Join-Path $mariaInstallRoot "runtime\MariaDB"
if (-not [string]::IsNullOrWhiteSpace($env:AC_MARIADB_RUNTIME_DIR)) {
    $mariaRuntimeRoot = [System.IO.Path]::GetFullPath($env:AC_MARIADB_RUNTIME_DIR)
}

$mariadbd = Find-MariaDBTool $mariaRuntimeRoot @("mariadbd.exe", "mysqld.exe")
$installDb = Find-MariaDBTool $mariaRuntimeRoot @("mariadb-install-db.exe", "mysql_install_db.exe")
$client = Find-MariaDBTool $mariaRuntimeRoot @("mariadb.exe", "mysql.exe")
$admin = Find-MariaDBTool $mariaRuntimeRoot @("mariadb-admin.exe", "mysqladmin.exe")
if (@($mariadbd, $installDb, $client, $admin) | Where-Object { [string]::IsNullOrWhiteSpace($_) }) {
    if (-not [string]::IsNullOrWhiteSpace($env:AC_MARIADB_RUNTIME_DIR)) {
        throw "AC_MARIADB_RUNTIME_DIR does not contain a complete MariaDB runtime: $mariaRuntimeRoot"
    }
    $runtimeInstaller = Join-Path $packRoot "tools\install-windows.ps1"
    if (-not (Test-Path -LiteralPath $runtimeInstaller -PathType Leaf)) {
        throw "Separate MariaDB runtime is missing and the installer was not found: $runtimeInstaller"
    }
    Write-Host "MariaDB is not bundled with Archive Center. Installing the verified official runtime for this user."
    & powershell.exe -NoProfile -ExecutionPolicy Bypass -File $runtimeInstaller -InstallMariaDBRuntime -InstallDir $mariaInstallRoot
    if ($LASTEXITCODE -ne 0) {
        throw "Separate MariaDB runtime installation failed. Check the download connection and retry."
    }
    $mariadbd = Find-MariaDBTool $mariaRuntimeRoot @("mariadbd.exe", "mysqld.exe")
    $installDb = Find-MariaDBTool $mariaRuntimeRoot @("mariadb-install-db.exe", "mysql_install_db.exe")
    $client = Find-MariaDBTool $mariaRuntimeRoot @("mariadb.exe", "mysql.exe")
    $admin = Find-MariaDBTool $mariaRuntimeRoot @("mariadb-admin.exe", "mysqladmin.exe")
}
foreach ($tool in @($mariadbd, $installDb, $client, $admin)) {
    if ([string]::IsNullOrWhiteSpace($tool) -or -not (Test-Path -LiteralPath $tool -PathType Leaf)) {
        throw "Separate MariaDB runtime is incomplete: $mariaRuntimeRoot"
    }
    Unblock-PackageFile $tool
}

$chromaRuntimeRoot = $managedRuntimeInstallRoot
$packageChromaRuntimeSelected = $false
$customChromaRuntimeSelected = $false
$packageChromaPython = Find-ChromaRuntimePython $packRoot
if (-not [string]::IsNullOrWhiteSpace($packageChromaPython)) {
    # Internal full packages may still carry their own runtime. Standard
    # managed packages never do.
    $chromaRuntimeRoot = $packRoot
    $packageChromaRuntimeSelected = $true
} elseif (-not [string]::IsNullOrWhiteSpace($env:AC_CHROMA_RUNTIME_DIR)) {
    $chromaRuntimeRoot = [System.IO.Path]::GetFullPath($env:AC_CHROMA_RUNTIME_DIR)
    $customChromaRuntimeSelected = $true
}
if (Test-LocalChromaRequested $env:AC_VECTOR_MODE) {
    $chromaPython = Find-ChromaRuntimePython $chromaRuntimeRoot
    $chromaRuntimeReady = Test-ChromaRuntimeVersion -PythonPath $chromaPython -RequiredVersion $managedChromaDBVersion
    if (-not $chromaRuntimeReady) {
        if ($packageChromaRuntimeSelected) {
            throw "The packaged ChromaDB runtime is missing or does not contain chromadb==$managedChromaDBVersion."
        }
        if ($customChromaRuntimeSelected) {
            throw "AC_CHROMA_RUNTIME_DIR does not contain chromadb==$managedChromaDBVersion`: $chromaRuntimeRoot"
        }
        $runtimeInstaller = Join-Path $packRoot "tools\install-windows.ps1"
        if (-not (Test-Path -LiteralPath $runtimeInstaller -PathType Leaf)) {
            throw "Separate ChromaDB runtime is missing and the installer was not found: $runtimeInstaller"
        }
        Write-Host "Managed ChromaDB is missing, incomplete, or has the wrong version."
        Write-Host "Repairing the per-user runtime with verified official Python and pinned ChromaDB $managedChromaDBVersion."
        & powershell.exe -NoProfile -ExecutionPolicy Bypass -File $runtimeInstaller -InstallChromaDBRuntime -InstallDir $managedRuntimeInstallRoot
        if ($LASTEXITCODE -ne 0) {
            throw "Separate ChromaDB runtime installation failed. Check the download connection and retry."
        }
        $chromaRuntimeRoot = $managedRuntimeInstallRoot
        $chromaPython = Find-ChromaRuntimePython $chromaRuntimeRoot
        $chromaRuntimeReady = Test-ChromaRuntimeVersion -PythonPath $chromaPython -RequiredVersion $managedChromaDBVersion
    }
    if (-not $chromaRuntimeReady) {
        throw "Separate ChromaDB runtime is incomplete: $chromaRuntimeRoot"
    }
    Unblock-PackageFile $chromaPython
}

Unblock-PackageFile $backendExe
Unblock-PackageFile (Join-Path $packRoot "bin\mariadb-schema.exe")

$dataDir = Join-Path $env:ARCHIVE_CENTER_DATA_DIR "mariadb"
$logDir = Join-Path $env:ARCHIVE_CENTER_DATA_DIR "logs"
New-Item -ItemType Directory -Force -Path $dataDir, $logDir | Out-Null

if (-not (Test-Path -LiteralPath (Join-Path $dataDir "mysql") -PathType Container)) {
    & $installDb "--datadir=$dataDir" "--password="
    if ($LASTEXITCODE -ne 0) {
        throw "MariaDB data directory initialization failed."
    }
}

$startedMariaDB = $null
$startedChroma = $null
$backendProcess = $null
$candidateBackend = $null
$restoredBackend = $null
$archiveProcessJob = New-Object ArchiveCenter.ManagedProcessJob
$archiveProcessJob.ExitWhenProcessEnds([ArchiveCenter.ManagedProcessJob]::GetCurrentParentProcessId())
$backendExitCode = 0
$restartLauncherForUpdate = $false
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
    }
    Wait-Port $MariaDBPort 60

    $dbName = "archive_center"
    $dbUser = "archive_center"
    $dbPassword = "archive-center-local-pass"
    if ([string]::IsNullOrWhiteSpace($env:AC_MARIADB_DSN)) {
        $env:AC_MARIADB_DSN = "${dbUser}:${dbPassword}@tcp(127.0.0.1:${MariaDBPort})/${dbName}?parseTime=true"
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
                $startedChroma = Start-ManagedChromaDB -PackageRoot $packRoot -RuntimeRoot $chromaRuntimeRoot -Endpoint $chromaUri
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
    $backendPort = 28080
    if ($env:AC_BIND_ADDR -match ':(\d+)$') {
        $backendPort = [int]$Matches[1]
    }

    $schemaPath = Join-Path $packRoot "migrations"
    & (Join-Path $packRoot "bin\mariadb-schema.exe") `
        -dsn $env:AC_MARIADB_DSN `
        -schema $schemaPath `
        -execute `
        -managed-bootstrap `
        -managed-host 127.0.0.1 `
        -managed-port $MariaDBPort `
        -expected-datadir $dataDir
    if ($LASTEXITCODE -ne 0) {
        throw "MariaDB managed account bootstrap or schema apply failed."
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
    if ($pendingApplyStatus -eq "applied_pending_health") {
        $candidateBackend = Start-ArchiveBackendProcess -BackendPath $backendExe -PackageRoot $packRoot
        $health = Wait-BackendMainReady -Process $candidateBackend -Port $backendPort -ExpectedVersion $pendingTargetVersion -TimeoutSeconds 60
        if ($health.Ready) {
            $commitFailure = ""
            try {
                $commit = Invoke-ArchiveUpdater -RunnerPath $updaterRunner -Command "commit" -PackageRoot $packRoot
                if ($commit.ExitCode -ne 0 -or $commit.Status -ne "committed") {
                    $commitFailure = "status '$($commit.Status)' (exit $($commit.ExitCode))"
                }
            } catch {
                $commitFailure = $_.Exception.Message
            }
            if ([string]::IsNullOrWhiteSpace($commitFailure)) {
                $updaterRunnerCleanupAllowed = $true
                Write-Host "Pending Archive Center package committed after main readiness passed."
                $backendExitCode = Wait-ArchiveBackendLifetime -Process $candidateBackend -Port $backendPort
            } else {
                Stop-ArchiveChildProcess $candidateBackend
                $restartManagedMariaDB = $null -ne $startedMariaDB
                $restartManagedChroma = $null -ne $startedChroma
                Stop-ArchiveChildProcess $startedChroma
                Stop-ArchiveChildProcess $startedMariaDB
                $rollback = Invoke-ArchiveUpdater -RunnerPath $updaterRunner -Command "rollback" -PackageRoot $packRoot
                if (-not (Test-UpdaterSafeBaselineStatus $rollback.Status)) {
                    throw "Update commit failed ($commitFailure) and rollback was not reported safe (status '$($rollback.Status)'). Startup stopped to avoid a mixed package."
                }
                $updaterRunnerCleanupAllowed = $true
                if ($rollback.Status -eq "rolled_back" -and -not [string]::IsNullOrWhiteSpace($pendingCurrentVersion)) {
                    $env:AC_BUILD_VERSION = $pendingCurrentVersion
                }
                $pendingApplyStatus = "rolled_back"
                if ($restartManagedMariaDB) {
                    $startedMariaDB = Start-ArchiveChildProcess -FilePath $mariadbd -ArgumentList $mariaArgs -WorkingDirectory $dataDir
                    Wait-Port $MariaDBPort 60
                }
                if ($restartManagedChroma) {
                    $startedChroma = Start-ManagedChromaDB -PackageRoot $packRoot -RuntimeRoot $chromaRuntimeRoot -Endpoint $chromaUri
                    Wait-Port $chromaPort 60
                }
                Write-Host "Update commit did not return a clean acknowledgement ($commitFailure). Recovery is safe; starting the verified current backend."
                $restoredBackend = Start-ArchiveBackendProcess -BackendPath $backendExe -PackageRoot $packRoot
                $backendExitCode = Wait-ArchiveBackendLifetime -Process $restoredBackend -Port $backendPort -ExpectedVersion $pendingCurrentVersion
            }
        } else {
            Stop-ArchiveChildProcess $candidateBackend
            $restartManagedMariaDB = $null -ne $startedMariaDB
            $restartManagedChroma = $null -ne $startedChroma
            Stop-ArchiveChildProcess $startedChroma
            Stop-ArchiveChildProcess $startedMariaDB
            $rollback = Invoke-ArchiveUpdater -RunnerPath $updaterRunner -Command "rollback" -PackageRoot $packRoot
            if (-not (Test-UpdaterSafeBaselineStatus $rollback.Status)) {
                throw "Updated backend failed main readiness ($($health.Detail)) and rollback was not reported safe (status '$($rollback.Status)'). Startup stopped to avoid a mixed package."
            }
            $updaterRunnerCleanupAllowed = $true
            if ($rollback.Status -eq "rolled_back" -and -not [string]::IsNullOrWhiteSpace($pendingCurrentVersion)) {
                $env:AC_BUILD_VERSION = $pendingCurrentVersion
            }
            $pendingApplyStatus = "rolled_back"
            if ($restartManagedMariaDB) {
                $startedMariaDB = Start-ArchiveChildProcess -FilePath $mariadbd -ArgumentList $mariaArgs -WorkingDirectory $dataDir
                Wait-Port $MariaDBPort 60
            }
            if ($restartManagedChroma) {
                $startedChroma = Start-ManagedChromaDB -PackageRoot $packRoot -RuntimeRoot $chromaRuntimeRoot -Endpoint $chromaUri
                Wait-Port $chromaPort 60
            }
            Write-Host "Updated backend failed main readiness ($($health.Detail)). The verified baseline was restored; starting the old backend."
            $restoredBackend = Start-ArchiveBackendProcess -BackendPath $backendExe -PackageRoot $packRoot
            $backendExitCode = Wait-ArchiveBackendLifetime -Process $restoredBackend -Port $backendPort -ExpectedVersion $pendingCurrentVersion
        }
    } else {
        $backendProcess = Start-ArchiveBackendProcess -BackendPath $backendExe -PackageRoot $packRoot
        $backendExitCode = Wait-ArchiveBackendLifetime -Process $backendProcess
    }
    if ($backendExitCode -eq 75) {
        $restartLauncherForUpdate = $true
        Write-Host "Backend requested immediate pending-update apply (exit 75)."
    }
} catch {
    $startupError = $_
    if ($pendingApplyStatus -eq "applied_pending_health" -and -not [string]::IsNullOrWhiteSpace($updaterRunner)) {
        $restartManagedMariaDB = $null -ne $startedMariaDB
        $restartManagedChroma = $null -ne $startedChroma
        Stop-ArchiveChildProcess $startedChroma
        Stop-ArchiveChildProcess $startedMariaDB
        try {
            $startupRollback = Invoke-ArchiveUpdater -RunnerPath $updaterRunner -Command "rollback" -PackageRoot $packRoot
            if ($startupRollback.ExitCode -ne 0 -or $startupRollback.Status -ne "rolled_back") {
                throw "rollback status '$($startupRollback.Status)' (exit $($startupRollback.ExitCode))"
            }
            $updaterRunnerCleanupAllowed = $true
            $pendingApplyStatus = "rolled_back"
            $env:AC_BUILD_VERSION = $pendingCurrentVersion
            if ($restartManagedMariaDB) {
                $startedMariaDB = Start-ArchiveChildProcess -FilePath $mariadbd -ArgumentList $mariaArgs -WorkingDirectory $dataDir
                Wait-Port $MariaDBPort 60
            }
            if ($restartManagedChroma) {
                $startedChroma = Start-ManagedChromaDB -PackageRoot $packRoot -RuntimeRoot $chromaRuntimeRoot -Endpoint $chromaUri
                Wait-Port $chromaPort 60
            }
            & (Join-Path $packRoot "bin\mariadb-schema.exe") `
                -dsn $env:AC_MARIADB_DSN `
                -schema (Join-Path $packRoot "migrations") `
                -execute `
                -managed-bootstrap `
                -managed-host 127.0.0.1 `
                -managed-port $MariaDBPort `
                -expected-datadir $dataDir
            if ($LASTEXITCODE -ne 0) {
                throw "restored MariaDB schema apply failed"
            }
            Write-Host "Updated package preparation failed before main readiness. Managed package files were rolled back; database files were preserved."
            $restoredBackend = Start-ArchiveBackendProcess -BackendPath $backendExe -PackageRoot $packRoot
            $backendExitCode = Wait-ArchiveBackendLifetime -Process $restoredBackend -Port $backendPort -ExpectedVersion $pendingCurrentVersion
            if ($backendExitCode -eq 75) {
                $restartLauncherForUpdate = $true
                Write-Host "Restored backend requested immediate pending-update apply (exit 75)."
            }
        } catch {
            throw "Updated package preparation failed, and rollback could not be proven safe. Startup stopped to avoid a mixed package.`nOriginal: $($startupError.Exception.Message)`nRollback: $($_.Exception.Message)"
        }
    } else {
        throw $startupError
    }
} finally {
    if ($updaterRunnerCleanupAllowed -and $updaterRunner -and (Test-Path -LiteralPath $updaterRunner -PathType Leaf)) {
        Remove-Item -LiteralPath $updaterRunner -Force -ErrorAction SilentlyContinue
    }
    if ($updaterRunnerCleanupAllowed) {
        Remove-Item -LiteralPath (Join-Path $packRoot ".updates\runner-identity.json") -Force -ErrorAction SilentlyContinue
    }
    try {
        Stop-ArchiveChildProcess -Process @(
            $backendProcess,
            $candidateBackend,
            $restoredBackend,
            $startedChroma,
            $startedMariaDB
        )
    } finally {
        if ($null -ne $archiveProcessJob) {
            $archiveProcessJob.Dispose()
        }
    }
}

if ($restartLauncherForUpdate) {
    & $PSCommandPath @launcherParameters
    exit $LASTEXITCODE
}
exit $backendExitCode
