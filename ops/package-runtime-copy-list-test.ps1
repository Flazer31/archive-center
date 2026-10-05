param(
    [string]$RepoRoot = (Split-Path -Parent $PSScriptRoot),
    [string]$ScratchParent = [System.IO.Path]::GetTempPath()
)
$ErrorActionPreference = 'Stop'
[Console]::OutputEncoding = [System.Text.UTF8Encoding]::new($false)

# Execute only the production script-copy pipeline, never the builder body.
# Synthetic inert contents and a Copy-File boundary replace the actual payload.
function Get-CopyPipeline([string]$Builder, [string]$SourceDirectory) {
    $tokens = $null
    $parseErrors = $null
    $ast = [System.Management.Automation.Language.Parser]::ParseFile($Builder, [ref]$tokens, [ref]$parseErrors)
    if ($parseErrors.Count) { throw "Builder parse errors: $parseErrors" }
    $matches = @($ast.FindAll({
        param($node)
        $node -is [System.Management.Automation.Language.PipelineAst] -and
        $node.PipelineElements.Count -eq 3 -and
        $node.PipelineElements[0] -is [System.Management.Automation.Language.CommandAst] -and
        $node.PipelineElements[0].GetCommandName() -eq 'Get-ChildItem' -and
        $node.PipelineElements[0].Extent.Text.Replace('\', '/').Contains($SourceDirectory) -and
        $node.PipelineElements[2].Extent.Text.Contains('Copy-File')
    }, $true))
    if ($matches.Count -ne 1) { throw "Expected one production copy pipeline in $Builder, found $($matches.Count)" }
    $commands = @($matches[0].PipelineElements | ForEach-Object { $_.GetCommandName() })
    if (($commands -join ',') -ne 'Get-ChildItem,Where-Object,ForEach-Object') {
        throw "Unexpected copy pipeline: $commands"
    }
    return [scriptblock]::Create($matches[0].Extent.Text)
}

$owners = @(
    @{ Builder = 'ops/build-full-package.ps1'; Directory = 'ops/full-package/scripts'; Required = @('start-full-windows.ps1', 'windows-console-control.ps1', 'diagnostics.ps1', 'protect-env-windows.ps1', 'smoke-live.ps1') },
    @{ Builder = 'ops/build-posix-managed-packages.ps1'; Directory = 'ops/full-package-posix'; Required = @('start-full-posix.sh', 'process-lifetime.py', 'diagnostics.sh', 'protect-env-posix.sh') }
)
$productionRoot = [System.IO.Path]::GetFullPath($RepoRoot)
$parentFull = [System.IO.Path]::GetFullPath($ScratchParent)
$scratchRoot = Join-Path $parentFull ('ac-copy-list-' + [guid]::NewGuid().ToString('N'))
$failures = [System.Collections.Generic.List[string]]::new()
try {
    foreach ($owner in $owners) {
        $pipeline = Get-CopyPipeline (Join-Path $productionRoot $owner.Builder) $owner.Directory
        $repoRoot = Join-Path $scratchRoot ([System.IO.Path]::GetFileNameWithoutExtension($owner.Builder))
        $targetRoot = Join-Path $repoRoot 'synthetic-runtime'
        $sourceRoot = Join-Path $repoRoot $owner.Directory
        New-Item -ItemType Directory -Force -Path $sourceRoot | Out-Null
        $names = @($owner.Required) + @('runtime-dependency-live-probe.ps1', 'updater-e2e-smoke.ps1', 'migrate-legacy-1.0-windows.ps1', 'synthetic-worklog.md')
        foreach ($name in $names) {
            [System.IO.File]::WriteAllText((Join-Path $sourceRoot $name), 'synthetic input: ' + $name)
        }
        $copied = [System.Collections.Generic.List[string]]::new()
        function Copy-File([string]$Source, [string]$Destination) {
            if (-not [System.IO.Path]::IsPathRooted($Source)) { $Source = Join-Path $repoRoot $Source }
            if (-not [System.IO.Path]::IsPathRooted($Destination)) { $Destination = Join-Path $targetRoot $Destination }
            New-Item -ItemType Directory -Force -Path (Split-Path -Parent $Destination) | Out-Null
            Copy-Item -LiteralPath $Source -Destination $Destination
            $copied.Add([System.IO.Path]::GetFileName($Destination))
        }
        . $pipeline
        foreach ($name in $owner.Required) {
            if ($copied -notcontains $name) { $failures.Add("$($owner.Builder) lost runtime helper $name") }
            elseif ([System.IO.File]::ReadAllText((Join-Path $targetRoot ('scripts/' + $name))) -ne ('synthetic input: ' + $name)) {
                $failures.Add("$($owner.Builder) changed synthetic helper $name")
            }
        }
        foreach ($name in @('runtime-dependency-live-probe.ps1', 'updater-e2e-smoke.ps1', 'migrate-legacy-1.0-windows.ps1', 'synthetic-worklog.md')) {
            if ($copied -contains $name) { $failures.Add("$($owner.Builder) shipped source-only file $name") }
        }
        Write-Output "$($owner.Builder): copied=$($copied -join ',')"
    }
}
finally {
    $resolvedScratch = [System.IO.Path]::GetFullPath($scratchRoot)
    if (-not $resolvedScratch.StartsWith($parentFull.TrimEnd('\', '/') + [System.IO.Path]::DirectorySeparatorChar, [System.StringComparison]::OrdinalIgnoreCase)) {
        throw 'Synthetic cleanup escaped its intended parent'
    }
    if (Test-Path -LiteralPath $resolvedScratch) { Remove-Item -LiteralPath $resolvedScratch -Recurse -Force }
}
if ($failures.Count) { throw ($failures -join "`n") }
Write-Output 'PASS: production runtime-copy pipelines exclude development harnesses and retain runtime helpers.'
