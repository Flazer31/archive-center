param(
    [string]$EnvFile = ".\.env.full.local",
    [string]$BaseUrl,
    [string]$SessionId
)

$ErrorActionPreference = "Stop"

function Import-DotEnv([string]$Path) {
    if (Test-Path -LiteralPath $Path -PathType Leaf) {
        Get-Content -LiteralPath $Path | ForEach-Object {
            $line = $_.Trim()
            if ($line -eq "" -or $line.StartsWith("#")) { return }
            $idx = $line.IndexOf("=")
            if ($idx -lt 1) { return }
            [Environment]::SetEnvironmentVariable($line.Substring(0, $idx).Trim(), $line.Substring($idx + 1).Trim(), "Process")
        }
    }
}

function Invoke-Json($Method, $Path, $Body = $null) {
    $uri = "$BaseUrl$Path"
    if ($null -eq $Body) {
        return Invoke-RestMethod -Method $Method -Uri $uri -TimeoutSec 30
    }
    $json = $Body | ConvertTo-Json -Depth 12
    return Invoke-RestMethod -Method $Method -Uri $uri -Body $json -ContentType "application/json" -TimeoutSec 60
}

$packRoot = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
Set-Location $packRoot
Import-DotEnv $EnvFile

if ([string]::IsNullOrWhiteSpace($BaseUrl)) {
    $bind = if ([string]::IsNullOrWhiteSpace($env:AC_BIND_ADDR)) { "127.0.0.1:28080" } else { $env:AC_BIND_ADDR }
    $BaseUrl = "http://$bind"
}
$BaseUrl = $BaseUrl.TrimEnd("/")
if ([string]::IsNullOrWhiteSpace($SessionId)) {
    $SessionId = "full-package-smoke-" + [DateTimeOffset]::UtcNow.ToUnixTimeMilliseconds()
}

$health = Invoke-Json GET "/health"
$ready = Invoke-Json GET "/ready"
$config = Invoke-Json POST "/config/update" @{
    mainProvider = $env:AC_LT_MAIN_PROVIDER
    mainEndpoint = $env:AC_LT_MAIN_ENDPOINT
    mainModel = $env:AC_LT_MAIN_MODEL
    mainApiKey = $env:AC_LT_MAIN_API_KEY
    criticProvider = $env:AC_LT_CRITIC_PROVIDER
    criticEndpoint = $env:AC_LT_CRITIC_ENDPOINT
    criticModel = $env:AC_LT_CRITIC_MODEL
    criticApiKey = $env:AC_LT_CRITIC_API_KEY
    supervisorProvider = $env:AC_LT_SUPERVISOR_PROVIDER
    supervisorEndpoint = $env:AC_LT_SUPERVISOR_ENDPOINT
    supervisorModel = $env:AC_LT_SUPERVISOR_MODEL
    supervisorApiKey = $env:AC_LT_SUPERVISOR_API_KEY
    embeddingProvider = $env:AC_LT_EMBEDDING_PROVIDER
    embeddingEndpoint = $env:AC_LT_EMBEDDING_ENDPOINT
    embeddingModel = $env:AC_LT_EMBEDDING_MODEL
    embeddingApiKey = $env:AC_LT_EMBEDDING_API_KEY
    topK = 5
}
$complete = Invoke-Json POST "/complete-turn" @{
    chat_session_id = $SessionId
    turn_index = 1
    user_input = "Package smoke user asks the archive to remember a brass key."
    assistant_content = "Package smoke assistant records that the brass key is under the blue lantern."
    context_messages = @(
        @{ role = "user"; content = "Remember the brass key." },
        @{ role = "assistant"; content = "The brass key is under the blue lantern." }
    )
}
$prepare = Invoke-Json POST "/prepare-turn" @{
    chat_session_id = $SessionId
    turn_index = 2
    raw_user_input = "Where is the brass key?"
    settings = @{
        injection_enabled = $true
        input_context_enabled = $true
        top_k = 5
        max_injection_chars = 3000
        max_input_context_chars = 800
    }
}
$search = Invoke-Json POST "/search" @{
    chat_session_id = $SessionId
    user_input = "brass key blue lantern"
    top_k = 5
}
$rollback = Invoke-Json DELETE "/rollback/1?chat_session_id=$SessionId&req_source=full_package_smoke"
$sessionDelete = Invoke-Json DELETE "/sessions/$SessionId"

$report = [ordered]@{
    generated_at = [DateTimeOffset]::UtcNow.ToString("o")
    base_url = $BaseUrl
    session_id = $SessionId
    health_status = $health.status
    ready = $ready.ready
    ready_checks = $ready.checks
    config_updated = $config.updated
    complete_turn = @{
        status = $complete.status
        source = $complete.source
        save_ok = $complete.save_ok
        critic_triggered = $complete.critic_triggered
        fail_reasons = $complete.fail_reasons
        warnings = $complete.warnings
    }
    prepare_turn = @{
        status = $prepare.status
        source = $prepare.source
        fallback_reason = $prepare.fallback_reason
        recall_source = $prepare.recall_result.source
        vector_engine = $prepare.recall_result.vector_shadow.engine
        vector_source = $prepare.recall_result.vector_shadow.source
    }
    search = @{
        memory_count = $search.memory_count
        fallback_count = $search.fallback_count
        total_count = $search.total_count
    }
    rollback = @{
        status = $rollback.status
        source = $rollback.source
        vector_delete = $rollback.deletions.vectors
    }
    session_delete = @{
        status = $sessionDelete.status
        source = $sessionDelete.source
        deleted = $sessionDelete.deleted
        vector_cleanup = $sessionDelete.vector_cleanup
    }
}

$outDir = Join-Path $packRoot ".runtime\reports"
New-Item -ItemType Directory -Force -Path $outDir | Out-Null
$outFile = Join-Path $outDir ("full-smoke-" + $SessionId + ".json")
$report | ConvertTo-Json -Depth 12 | Set-Content -LiteralPath $outFile -Encoding UTF8
$report | ConvertTo-Json -Depth 12
