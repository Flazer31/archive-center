param([Parameter(Mandatory=$true)][string]$BackendPath, [Parameter(Mandatory=$true)][string]$EvidenceRoot)
$ErrorActionPreference = 'Stop'
$repo = Split-Path $PSScriptRoot
$launcherPath = Join-Path $repo 'ops/full-package/scripts/start-full-windows.ps1'
$text = [IO.File]::ReadAllText($launcherPath)
$tokens=$null; $errors=$null
$ast=[Management.Automation.Language.Parser]::ParseInput($text,[ref]$tokens,[ref]$errors)
if ($errors) { throw $errors[0] }
$names=@('Join-Args','Unblock-PackageFile','Start-ArchiveChildProcess','Stop-ArchiveChildProcess','Wait-ArchiveBackendLifetime')
$functions=@($ast.FindAll({param($n) $n -is [Management.Automation.Language.FunctionDefinitionAst]},$true) | Where-Object {$names -contains $_.Name} | ForEach-Object {$_.Extent.Text}) -join "`n"
$jobStart=$text.IndexOf('if (-not ("ArchiveCenter.ManagedProcessJob" -as [type]))')
$jobEnd=$text.IndexOf('$consoleControlScript =')
$job=$text.Substring($jobStart,$jobEnd-$jobStart)
$caseRoot=Join-Path ([IO.Path]::GetFullPath($EvidenceRoot)) (([string][char]0xc5b4)+[char]0xb4dc+[char]0xbbfc+' package')
New-Item -ItemType Directory -Force -Path (Join-Path $caseRoot 'bin'),(Join-Path $caseRoot 'scripts') | Out-Null
Copy-Item -LiteralPath $BackendPath -Destination (Join-Path $caseRoot 'bin/archive-center-go.exe') -Force
foreach($name in @('diagnostics.ps1','windows-console-control.ps1','export-diagnostics.ps1','service-ports.ps1')) {Copy-Item -LiteralPath (Join-Path $repo ('ops/full-package/scripts/'+$name)) -Destination (Join-Path $caseRoot 'scripts') -Force}
$fixture=@'
$ErrorActionPreference='Stop'
. (Join-Path $PSScriptRoot 'diagnostics.ps1')
. (Join-Path $PSScriptRoot 'windows-console-control.ps1')
$env:ARCHIVE_CENTER_DATA_DIR=Join-Path (Split-Path $PSScriptRoot) 'data'
$env:AC_LOG_DIR=Join-Path $env:ARCHIVE_CENTER_DATA_DIR 'logs'
Initialize-ArchiveDiagnostics
'@ + "`n" + $job + "`n" + $functions + "`n" + @'
$script:archiveProcessJob=New-Object ArchiveCenter.ManagedProcessJob
$env:AC_MODE='shadow';$env:AC_STORE_MODE='diagnostic_invalid_store';$env:AC_MARIADB_DSN=''
$env:AC_RUNTIME_PROFILE='core_lite';$env:AC_VECTOR_MODE='off';$env:AC_CHROMA_ENDPOINT=''
$process=$null
try {
 $process=Start-ArchiveChildProcess -FilePath (Join-Path (Split-Path $PSScriptRoot) 'bin/archive-center-go.exe')
 $code=Wait-ArchiveBackendLifetime -Process $process
 if($code -ne 1){throw "Expected Go exit 1, got $code"}
} finally {
 Stop-ArchiveChildProcess -Process $process
 $archiveProcessJob.Dispose()
 Complete-ArchiveProcessLog -Process $process
 if($script:archiveTranscriptStarted){Stop-Transcript | Out-Null}
}
& (Join-Path $PSScriptRoot 'export-diagnostics.ps1') -OutputPath (Join-Path (Split-Path $PSScriptRoot) 'report.json')
'@
$fixturePath=Join-Path $caseRoot 'scripts/fixture.ps1'
[IO.File]::WriteAllText($fixturePath,$fixture,[Text.UTF8Encoding]::new($true))
& (Join-Path $env:SystemRoot 'System32/WindowsPowerShell/v1.0/powershell.exe') -NoProfile -ExecutionPolicy Bypass -File $fixturePath
if($LASTEXITCODE -ne 0){throw "Windows diagnostic smoke failed: $LASTEXITCODE"}
$reportText=[IO.File]::ReadAllText((Join-Path $caseRoot 'report.json'))
$report=$reportText | ConvertFrom-Json
if($report.contract_version -ne 'archive-center.diagnostics.v1' -or $reportText -notmatch 'invalid config' -or $reportText -notmatch 'diagnostic_invalid_store'){throw 'Actual Go startup cause absent from offline report'}
if(-not (@($report.files | Where-Object {$_.name -eq 'launcher.err.log'}).Count)){throw 'Managed child stderr missing'}
if($report.log_directory -cne (Join-Path $caseRoot 'data\logs')){throw 'Unicode diagnostic path changed in JSON'}
$noBinaryRoot=Join-Path $caseRoot 'offline-no-backend'
New-Item -ItemType Directory -Force -Path (Join-Path $noBinaryRoot 'scripts') | Out-Null
Copy-Item -LiteralPath (Join-Path $caseRoot 'scripts/export-diagnostics.ps1') -Destination (Join-Path $noBinaryRoot 'scripts') -Force
$savedLogDir=$env:AC_LOG_DIR
try {
 $env:AC_LOG_DIR=Join-Path $caseRoot 'data/logs'
 & (Join-Path $env:SystemRoot 'System32/WindowsPowerShell/v1.0/powershell.exe') -NoProfile -ExecutionPolicy Bypass -File (Join-Path $noBinaryRoot 'scripts/export-diagnostics.ps1') -OutputPath (Join-Path $noBinaryRoot 'report.json')
 if($LASTEXITCODE -ne 0){throw 'Offline export required a backend executable'}
 if([IO.File]::ReadAllText((Join-Path $noBinaryRoot 'report.json')) -notmatch 'diagnostic_invalid_store'){throw 'Offline export lost startup failure'}
} finally {$env:AC_LOG_DIR=$savedLogDir}
Write-Output 'PASS: actual Go failure, production Windows process launch/drain/wait, Unicode path, offline report.'
