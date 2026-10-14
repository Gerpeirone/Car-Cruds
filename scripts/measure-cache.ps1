#requires -Version 7.0
<#
.SYNOPSIS
Compara fichas calientes y fallback a MongoDB; no mide caché fría ni capacidad sostenida.
.EXAMPLE
pwsh -File ./scripts/measure-cache.ps1 -DurationSeconds 5
.DESCRIPTION
Usa cmd/load, solo gateway local y datos demo auto-001. Restaura Redis en finally.
Guarda JSON sin secretos en output/cache-{warm,fallback,metrics}.json.
#>
[CmdletBinding()]
param([ValidateSet(3, 5)] [int] $DurationSeconds = 3)
Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
$script:Repo = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
$script:Output = Join-Path $script:Repo 'output'
$script:Compose = @('compose', '--project-directory', $script:Repo, '--file', (Join-Path $script:Repo 'compose.yaml'), '--env-file', (Join-Path $script:Repo '.env'), '--project-name', 'car-cruds')
$script:Phases = [Collections.Generic.List[object]]::new()
$script:Bin = Join-Path $script:Repo ('tmp/cache-load' + $(if ($IsWindows) { '.exe' } else { '' }))
$started = [DateTimeOffset]::UtcNow
$clock = [Diagnostics.Stopwatch]::StartNew()
$restore = $false; $failed = $false; $failureCode = $null; $redisRestored = $null; $redisRestoreError = $null; $warmup = $null
function Assert-Check([bool] $Condition, [string] $Code) { if (-not $Condition) { throw [InvalidOperationException]::new($Code) } }
function Run-Process([string] $File, [string[]] $Arguments, [int] $Seconds = 30) {
    $info = [Diagnostics.ProcessStartInfo]::new()
    $info.FileName = $File; $info.WorkingDirectory = $script:Repo
    $info.UseShellExecute = $false; $info.CreateNoWindow = $true
    $info.RedirectStandardOutput = $true; $info.RedirectStandardError = $true
    foreach ($argument in $Arguments) { $info.ArgumentList.Add($argument) }
    # Se hereda el entorno sin sobrescribir valores: cmd/load usa devconfig.Load.
    $process = [Diagnostics.Process]::new(); $process.StartInfo = $info
    try {
        Assert-Check ($process.Start()) 'PROCESS_START_FAILED'
        $out = $process.StandardOutput.ReadToEndAsync(); $err = $process.StandardError.ReadToEndAsync()
        if (-not $process.WaitForExit($Seconds * 1000)) { $process.Kill($true); $process.WaitForExit(); throw [TimeoutException]::new('PROCESS_TIMEOUT') }
        $text = $out.GetAwaiter().GetResult(); $null = $err.GetAwaiter().GetResult()
        Assert-Check ($process.ExitCode -eq 0) 'PROCESS_COMMAND_FAILED'
        return $text
    } finally { $process.Dispose() }
}
function Compose-Run([string[]] $Arguments) { return Run-Process 'docker' ($script:Compose + $Arguments) }
function Redis-State {
    $rows = @(Compose-Run @('ps', '--all', '--format', 'json', 'redis') | ConvertFrom-Json -AsHashtable)
    Assert-Check ($rows.Count -eq 1 -and $rows[0]['Service'] -eq 'redis') 'REDIS_CONTAINER_MISSING'
    return $rows[0]
}
function Wait-Redis {
    $wait = [Diagnostics.Stopwatch]::StartNew()
    do {
        $state = Redis-State
        if ($state['State'] -eq 'running' -and $state['Health'] -eq 'healthy' -and $wait.Elapsed.TotalSeconds -lt 45) { return }
        Start-Sleep -Milliseconds 500
    } while ($wait.Elapsed.TotalSeconds -lt 45)
    throw [TimeoutException]::new('REDIS_RECOVERY_TIMEOUT')
}
function Fleet-Metrics {
    $totals = @{ hit = 0.0; miss = 0.0; error = 0.0; detail = 0.0 }
    foreach ($service in @('fleet-a', 'fleet-b')) {
        # Las comillas simples conservan el nombre; el token se expande solo en sh del contenedor.
        $shell = 'wget -q -O - -T 5 --header="Authorization: Bearer ${AUTH_SERVICE_TOKEN:?}" http://127.0.0.1:8080/internal/metrics'
        $text = Compose-Run @('exec', '-T', $service, 'sh', '-c', $shell)
        Assert-Check ($text -match '(?m)^go_goroutines\s+[0-9]+') 'METRICS_RESPONSE_INVALID'
        foreach ($line in $text -split '\r?\n') {
            if ($line -match '^fleet_cache_requests_total\{([^}]+)\}\s+([0-9.eE+-]+)(?:\s.*)?$') {
                $labels = $Matches[1]; $value = [double]::Parse($Matches[2], [Globalization.CultureInfo]::InvariantCulture)
                if ($labels -match '(?:^|,)result="(hit|miss|error)"(?:,|$)') { $totals[$Matches[1]] += $value }
            } elseif ($line -match '^fleet_source_reads_total\{([^}]+)\}\s+([0-9.eE+-]+)(?:\s.*)?$') {
                $labels = $Matches[1]; $value = [double]::Parse($Matches[2], [Globalization.CultureInfo]::InvariantCulture)
                if ($labels -match '(?:^|,)operation="detail"(?:,|$)') { $totals['detail'] += $value }
            }
        }
    }
    return @{ sampledAtUtc = [DateTimeOffset]::UtcNow.ToString('o'); sum = $totals; instances = @('fleet-a', 'fleet-b') }
}
function Save-Json([string] $Name, [object] $Value) {
    [IO.File]::WriteAllText((Join-Path $script:Output $Name), (ConvertTo-Json -InputObject $Value -Depth 20), [Text.UTF8Encoding]::new($false))
}
function Measure-Phase([string] $Name) {
    $phase = [ordered]@{ name = $Name; startedAtUtc = [DateTimeOffset]::UtcNow.ToString('o'); status = 'FAIL'; before = $null; after = $null; delta = $null; loadReport = "cache-$Name.json" }
    $script:Phases.Add($phase)
    $phase['before'] = Fleet-Metrics
    $load = Run-Process $script:Bin @('-path', '/v1/vehicles/auto-001', '-role', 'client', '-concurrency', '1,8', '-duration', ($DurationSeconds.ToString() + 's')) | ConvertFrom-Json -AsHashtable
    Save-Json $phase['loadReport'] $load
    $phase['after'] = Fleet-Metrics
    $delta = @{}
    foreach ($key in @('hit', 'miss', 'error', 'detail')) { $delta[$key] = $phase['after']['sum'][$key] - $phase['before']['sum'][$key]; Assert-Check ($delta[$key] -ge 0) 'COUNTER_RESET_INVALID_COMPARISON' }
    $phase['delta'] = $delta
    $phase['hitRate'] = if ($delta['hit'] + $delta['miss'] -gt 0) { $delta['hit'] / ($delta['hit'] + $delta['miss']) } else { $null }
    Assert-Check ($load['stages'].Count -eq 2 -and $load['stages'][0]['concurrency'] -eq 1 -and $load['stages'][1]['concurrency'] -eq 8) 'LOAD_STAGES_INCOMPLETE'
    foreach ($stage in $load['stages']) { Assert-Check ($stage['requests'] -gt 0 -and $stage['transportErrors'] -eq 0 -and $stage['invalidJSON'] -eq 0 -and $stage['httpStatuses']['200'] -eq $stage['requests']) 'LOAD_NOT_ALL_HTTP200' }
    if ($Name -eq 'warm') { Assert-Check ($delta['hit'] -gt 0) 'CACHE_HITS_NOT_OBSERVED' } else { Assert-Check ($delta['error'] -gt 0 -and $delta['detail'] -gt 0) 'CACHE_FALLBACK_NOT_OBSERVED' }
    $phase['status'] = 'PASS'; $phase['completedAtUtc'] = [DateTimeOffset]::UtcNow.ToString('o')
}
Push-Location -LiteralPath $script:Repo
try {
    Assert-Check (Test-Path -LiteralPath (Join-Path $script:Repo '.env') -PathType Leaf) 'ENV_FILE_MISSING'
    $null = [IO.Directory]::CreateDirectory((Join-Path $script:Repo 'tmp')); $null = [IO.Directory]::CreateDirectory($script:Output)
    $go = (Get-Command go -CommandType Application).Source
    $null = Run-Process $go @('build', '-mod=readonly', '-o', $script:Bin, './cmd/load') 120
    Wait-Redis
    $warmup = Run-Process $script:Bin @('-path', '/v1/vehicles/auto-001', '-role', 'client', '-concurrency', '1', '-duration', '1s') | ConvertFrom-Json -AsHashtable
    Assert-Check ($warmup['stages'].Count -eq 1 -and $warmup['stages'][0]['requests'] -gt 0 -and $warmup['stages'][0]['httpStatuses']['200'] -eq $warmup['stages'][0]['requests']) 'GATEWAY_WARMUP_FAILED'
    Measure-Phase 'warm'
    $restore = $true
    $null = Compose-Run @('stop', '--timeout', '5', 'redis')
    Assert-Check ((Redis-State)['State'] -ne 'running') 'REDIS_DID_NOT_STOP'
    Measure-Phase 'fallback'
} catch {
    $failed = $true
    $failureCode = if ($_.Exception.Message -match '^[A-Z][A-Z0-9_]{0,95}$') { $_.Exception.Message } else { 'UNEXPECTED_ERROR' }
} finally {
    if ($restore) {
        try { $null = Compose-Run @('start', 'redis'); Wait-Redis; $redisRestored = $true }
        catch { $failed = $true; $redisRestored = $false; $redisRestoreError = 'REDIS_RESTORE_FAILED'; if ($null -eq $failureCode) { $failureCode = $redisRestoreError } }
    }
    $clock.Stop()
    $report = [ordered]@{
        result = $(if ($failed) { 'FAIL' } else { 'PASS' }); startedAtUtc = $started.ToString('o'); completedAtUtc = [DateTimeOffset]::UtcNow.ToString('o'); elapsedMs = [Math]::Round($clock.Elapsed.TotalMilliseconds, 2)
        project = 'car-cruds'; route = 'GET /v1/vehicles/auto-001'; role = 'client'; concurrency = @(1, 8); requestedStageSeconds = $DurationSeconds
        model = 'Carga cerrada local; generador y servicios comparten recursos'; notes = 'Scrapes secuenciales suman ambas réplicas; los deltas incluyen el preflight y posible tráfico simultáneo. No es una fase sin dependencia de caché: fallback incluye su falla. No se midió caché fría.'
        commands = @(("go build -mod=readonly -o tmp/" + [IO.Path]::GetFileName($script:Bin) + ' ./cmd/load'), ("tmp/" + [IO.Path]::GetFileName($script:Bin) + " -path /v1/vehicles/auto-001 -role client -concurrency 1,8 -duration ${DurationSeconds}s"), 'docker compose stop --timeout 5 redis', 'docker compose start redis')
        warmup = $warmup; phases = @($script:Phases.ToArray()); redisRestored = $redisRestored; redisRestoreError = $redisRestoreError; errorCode = $failureCode
    }
    try { $null = [IO.Directory]::CreateDirectory($script:Output); Save-Json 'cache-metrics.json' $report } catch { $failed = $true; Write-Host 'REPORT_WRITE_FAILED' }
    Pop-Location
    Write-Host ('Cache: ' + $(if ($failed) { 'FAIL' } else { 'PASS' }))
}
if ($failed) { exit 1 }
exit 0
