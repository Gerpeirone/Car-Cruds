#requires -Version 7.0
<#
.SYNOPSIS
Ensaya balanceo, degradación y recuperación de los componentes propios.
.DESCRIPTION
Requiere el proyecto car-cruds iniciado desde compose.yaml y .env local.
Utiliza únicamente el gateway de loopback. Detiene servicios específicos,
los restaura en finally y desactiva los vehículos de prueba sin borrar datos.
El informe contiene estados, tiempos y correlación, sin credenciales ni cuerpos.
.EXAMPLE
pwsh -File ./scripts/verify-resilience.ps1
.EXAMPLE
pwsh -File ./scripts/verify-resilience.ps1 -ReportPath output/resilience-run.json
#>
[CmdletBinding()]
param(
    [ValidatePattern('(?i)\.json$')] [string] $ReportPath = 'output/resilience.json',
    [string] $BaseUrl = '',
    [ValidateRange(5, 45)] [int] $RecoveryTimeoutSeconds = 45,
    [ValidateRange(5, 30)] [int] $DockerTimeoutSeconds = 30
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
$script:RepoRoot = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
$script:ComposePath = Join-Path $script:RepoRoot 'compose.yaml'
$script:EnvPath = Join-Path $script:RepoRoot '.env'
$script:ProjectName = 'car-cruds'
$script:RunId = [Guid]::NewGuid().ToString('N')
$script:ShortId = $script:RunId.Substring(0, 12)
$script:RequestNumber = 0
$script:Requests = [Collections.Generic.List[object]]::new()
$script:Steps = [Collections.Generic.List[object]]::new()
$script:RestoreTargets = [Collections.Generic.HashSet[string]]::new()
$script:Fixtures = [Collections.Generic.List[string]]::new()
$script:Restoration = [Collections.Generic.List[object]]::new()
$script:FixtureCleanup = [Collections.Generic.List[object]]::new()
$script:Client = $null
$script:Token = ''
$script:GatewayBase = ''
$script:RequiredServices = @('gateway', 'fleet-lb', 'fleet-a', 'fleet-b', 'mongo', 'redis', 'rabbitmq', 'solr', 'indexer', 'rentals')
$script:StartedAt = [DateTimeOffset]::UtcNow
$script:TotalClock = [Diagnostics.Stopwatch]::StartNew()
$script:Failed = $false
$script:FailureCode = $null
$script:RequestTimeoutSeconds = 8
$script:BaselineId = 'res-' + $script:ShortId + '-base'
$script:RabbitId = 'res-' + $script:ShortId + '-rabbit'
$script:BaselineBrand = 'Resiliencia-' + $script:ShortId + '-base'
$script:RabbitBrand = 'Resiliencia-' + $script:ShortId + '-rabbit'

function Get-SafeFailureCode {
    param([Exception] $Exception)
    # Nunca se copian mensajes de HTTP/Docker: pueden contener URL o datos.
    if ($Exception.Message -match '^[A-Z][A-Z0-9_]{0,95}$') { return $Exception.Message }
    return 'UNEXPECTED_ERROR'
}

function Assert-Condition {
    param([bool] $Condition, [string] $Code)
    if (-not $Condition) { throw [InvalidOperationException]::new($Code) }
}

function Read-LocalSettings {
    Assert-Condition (Test-Path -LiteralPath $script:EnvPath -PathType Leaf) 'ENV_FILE_MISSING'
    $settings = @{}
    foreach ($line in [IO.File]::ReadAllLines($script:EnvPath)) {
        $value = $line.Trim()
        if ($value.Length -eq 0 -or $value.StartsWith('#')) { continue }
        Assert-Condition ($value -match '^(?:export\s+)?([A-Za-z_][A-Za-z0-9_]*)=(.*)$') 'ENV_FILE_INVALID'
        $key = $Matches[1]
        $entry = $Matches[2].Trim()
        if ($entry.Length -ge 2 -and (($entry[0] -eq '"' -and $entry[-1] -eq '"') -or ($entry[0] -eq "'" -and $entry[-1] -eq "'"))) {
            $entry = $entry.Substring(1, $entry.Length - 2)
        }
        $settings[$key] = $entry
    }
    return $settings
}

function Invoke-LocalCompose {
    param([string[]] $Arguments)
    $info = [Diagnostics.ProcessStartInfo]::new()
    $info.FileName = 'docker'
    $info.WorkingDirectory = $script:RepoRoot
    $info.UseShellExecute = $false
    $info.CreateNoWindow = $true
    $info.RedirectStandardOutput = $true
    $info.RedirectStandardError = $true
    foreach ($argument in @('compose', '--project-directory', $script:RepoRoot, '--file', $script:ComposePath, '--env-file', $script:EnvPath, '--project-name', $script:ProjectName) + $Arguments) {
        $info.ArgumentList.Add($argument)
    }
    $process = [Diagnostics.Process]::new()
    $process.StartInfo = $info
    try {
        if (-not $process.Start()) { throw [InvalidOperationException]::new('DOCKER_START_FAILED') }
        $stdout = $process.StandardOutput.ReadToEndAsync()
        $stderr = $process.StandardError.ReadToEndAsync()
        if (-not $process.WaitForExit($DockerTimeoutSeconds * 1000)) {
            $process.Kill($true)
            $process.WaitForExit()
            throw [TimeoutException]::new('DOCKER_TIMEOUT')
        }
        $output = $stdout.GetAwaiter().GetResult()
        $null = $stderr.GetAwaiter().GetResult()
        if ($process.ExitCode -ne 0) { throw [InvalidOperationException]::new('DOCKER_COMMAND_FAILED') }
        return $output
    }
    catch {
        if ($_.Exception.Message -match '^DOCKER_[A-Z_]+$') { throw }
        throw [InvalidOperationException]::new('DOCKER_COMMAND_FAILED')
    }
    finally { $process.Dispose() }
}

function Get-ServiceState {
    param([string] $Service)
    Assert-Condition ($Service -in $script:RequiredServices) 'DOCKER_SERVICE_NOT_ALLOWED'
    $raw = Invoke-LocalCompose @('ps', '--all', '--format', 'json', $Service)
    Assert-Condition (-not [string]::IsNullOrWhiteSpace($raw)) 'COMPOSE_SERVICE_MISSING'
    try {
        $rows = @($raw | ConvertFrom-Json -AsHashtable)
    }
    catch {
        # Compose antiguo puede emitir un objeto JSON por línea.
        $rows = @($raw -split '\r?\n' | Where-Object { $_.Trim().Length -gt 0 } | ForEach-Object { $_ | ConvertFrom-Json -AsHashtable })
    }
    Assert-Condition ($rows.Count -eq 1 -and $rows[0]['Service'] -eq $Service) 'COMPOSE_SERVICE_AMBIGUOUS'
    return $rows[0]
}

function Stop-TestService {
    param([ValidateSet('fleet-b', 'redis', 'rabbitmq', 'solr')] [string] $Service)
    Assert-Condition ((Get-ServiceState $Service)['State'] -eq 'running') 'SERVICE_NOT_RUNNING_BEFORE_STOP'
    # Registrar antes del comando permite restaurar incluso un stop parcial.
    $null = $script:RestoreTargets.Add($Service)
    $null = Invoke-LocalCompose @('stop', '--timeout', '5', $Service)
    Assert-Condition ((Get-ServiceState $Service)['State'] -ne 'running') 'SERVICE_DID_NOT_STOP'
}

function Start-TestService {
    param([ValidateSet('fleet-b', 'redis', 'rabbitmq', 'solr')] [string] $Service)
    $null = Invoke-LocalCompose @('start', $Service)
    Assert-Condition ((Get-ServiceState $Service)['State'] -eq 'running') 'SERVICE_DID_NOT_START'
}

function Wait-ServiceHealthy {
    param([string] $Service)
    $clock = [Diagnostics.Stopwatch]::StartNew()
    do {
        $state = Get-ServiceState $Service
        if ($state['State'] -eq 'running' -and $state['Health'] -eq 'healthy' -and $clock.Elapsed.TotalSeconds -lt $RecoveryTimeoutSeconds) { return }
        Start-Sleep -Milliseconds 500
    } while ($clock.Elapsed.TotalSeconds -lt $RecoveryTimeoutSeconds)
    throw [TimeoutException]::new('SERVICE_HEALTH_RECOVERY_TIMEOUT')
}

function Invoke-OwnApi {
    param(
        [string] $Method,
        [string] $Path,
        [object] $Body = $null,
        [string[]] $AllowedInstances = @('fleet-a', 'fleet-b')
    )
    $script:RequestNumber++
    $requestId = 'res-' + $script:ShortId + '-' + $script:RequestNumber.ToString('D4')
    $clock = [Diagnostics.Stopwatch]::StartNew()
    $request = [Net.Http.HttpRequestMessage]::new([Net.Http.HttpMethod]::new($Method), $script:GatewayBase + $Path)
    $request.Headers.Authorization = [Net.Http.Headers.AuthenticationHeaderValue]::new('Bearer', $script:Token)
    $request.Headers.Add('X-Request-ID', $requestId)
    if ($null -ne $Body) {
        $json = ConvertTo-Json -InputObject $Body -Depth 12 -Compress
        $request.Content = [Net.Http.StringContent]::new($json, [Text.Encoding]::UTF8, 'application/json')
    }
    $response = $null
    try {
        try { $response = $script:Client.SendAsync($request).GetAwaiter().GetResult() }
        catch {
            $script:Requests.Add([ordered]@{ method = $Method; route = $Path.Split('?')[0]; status = 0; instance = 'unavailable'; requestId = $requestId; correlated = $false; elapsedMs = [Math]::Round($clock.Elapsed.TotalMilliseconds, 2); errorCode = 'HTTP_TRANSPORT_FAILED' })
            throw [InvalidOperationException]::new('HTTP_TRANSPORT_FAILED')
        }
        $ids = @()
        $instances = @()
        if ($response.Headers.Contains('X-Request-ID')) { $ids = @($response.Headers.GetValues('X-Request-ID')) }
        if ($response.Headers.Contains('X-Service-Instance')) { $instances = @($response.Headers.GetValues('X-Service-Instance')) }
        $correlated = $ids.Count -eq 1 -and $ids[0] -eq $requestId
        $instanceValid = $instances.Count -eq 1 -and $instances[0] -in $AllowedInstances
        $safeInstance = if ($instanceValid) { $instances[0] } else { 'unexpected' }
        $script:Requests.Add([ordered]@{ method = $Method; route = $Path.Split('?')[0]; status = [int]$response.StatusCode; instance = $safeInstance; requestId = $requestId; correlated = $correlated; elapsedMs = [Math]::Round($clock.Elapsed.TotalMilliseconds, 2) })
        Assert-Condition $correlated 'REQUEST_CORRELATION_INVALID'
        Assert-Condition $instanceValid 'SERVICE_INSTANCE_INVALID'
        $raw = $response.Content.ReadAsStringAsync().GetAwaiter().GetResult()
        try { $decoded = ConvertFrom-Json -InputObject $raw -AsHashtable }
        catch { throw [InvalidOperationException]::new('HTTP_JSON_INVALID') }
        Assert-Condition ($decoded -is [Collections.IDictionary]) 'HTTP_JSON_OBJECT_REQUIRED'
        return [pscustomobject]@{ Status = [int]$response.StatusCode; Instance = $instances[0]; Json = $decoded; RequestId = $requestId; Correlated = $correlated; ElapsedMs = [Math]::Round($clock.Elapsed.TotalMilliseconds, 2) }
    }
    finally {
        if ($null -ne $response) { $response.Dispose() }
        $request.Dispose()
    }
}

function Assert-Vehicle {
    param([object] $Response, [string] $Id, [bool] $Active = $true)
    Assert-Condition ($Response.Status -eq 200) 'VEHICLE_READ_FAILED'
    Assert-Condition ($Response.Json['vehicleId'] -eq $Id -and $Response.Json['active'] -eq $Active -and $Response.Json['currency'] -eq 'ARS' -and $Response.Json['dailyRateMinor'] -eq 5000000 -and $Response.Json['version'] -ge 1) 'VEHICLE_STATE_INVALID'
}

function Measure-FleetWithdrawal {
    param([string] $Id)
    $failures = [Collections.Generic.List[object]]::new()
    $evidence = [ordered]@{
        serviceState = 'stopped'
        plannedRequests = 20
        requests = 0
        successfulRequests = 0
        initialFailures = @()
        initialFailureCount = 0
        consecutiveHealthy = 0
        requiredConsecutiveHealthy = 12
        withdrawalObserved = $false
        observedInstance = 'fleet-a'
        phase = 'passive_detection'
    }
    # Se conserva evidencia parcial si la fase falla, sin ocultar los 503.
    $script:StepEvidence = $evidence
    for ($sample = 1; $sample -le 20; $sample++) {
        # Cada llamada es una observación nueva; no se reintenta desde el cliente.
        $result = Invoke-OwnApi -Method GET -Path ('/v1/vehicles/' + $Id) -AllowedInstances @('fleet-a', 'gateway')
        $evidence['requests'] = $sample
        Assert-Condition $result.Correlated 'REQUEST_CORRELATION_INVALID'
        if ($result.Status -eq 503) {
            Assert-Condition ($result.Instance -eq 'gateway' -and $result.Json['code'] -eq 'SERVICE_UNAVAILABLE') 'FLEET_INITIAL_FAILURE_UNCLASSIFIED'
            $failure = [ordered]@{ request = $sample; requestId = $result.RequestId; status = 503; instance = 'gateway'; errorCode = 'SERVICE_UNAVAILABLE'; elapsedMs = $result.ElapsedMs; phase = $evidence['phase'] }
            $failures.Add($failure)
            $evidence['initialFailures'] = @($failures.ToArray())
            $evidence['initialFailureCount'] = $failures.Count
            $evidence['consecutiveHealthy'] = 0
            Assert-Condition (-not $evidence['withdrawalObserved']) 'FLEET_FAILURE_AFTER_WITHDRAWAL'
            Assert-Condition ($failures.Count -le 3) 'FLEET_INITIAL_FAILURE_LIMIT_EXCEEDED'
            continue
        }
        Assert-Condition ($result.Status -eq 200 -and $result.Instance -eq 'fleet-a') 'FLEET_UNEXPECTED_HTTP_RESULT'
        Assert-Vehicle $result $Id
        $evidence['successfulRequests']++
        $evidence['consecutiveHealthy']++
        if ($evidence['consecutiveHealthy'] -ge 12) {
            $evidence['withdrawalObserved'] = $true
            $evidence['phase'] = 'stable_fleet_a'
        }
    }
    Assert-Condition ($evidence['withdrawalObserved'] -and $evidence['consecutiveHealthy'] -ge 12 -and $evidence['successfulRequests'] -ge 12) 'FLEET_WITHDRAWAL_NOT_OBSERVED'
    return $evidence
}

function New-TestVehicle {
    param([string] $Id, [string] $Brand)
    $script:Fixtures.Add($Id)
    $input = @{ vehicleId = $Id; brand = $Brand; model = 'Demostracion'; category = 'resiliencia'; transmission = 'manual'; seats = 4; dailyRateMinor = 5000000; currency = 'ARS'; active = $true }
    $result = Invoke-OwnApi -Method POST -Path '/v1/vehicles' -Body $input
    Assert-Condition ($result.Status -eq 201 -and $result.Json['vehicleId'] -eq $Id -and $result.Json['active'] -eq $true -and $result.Json['version'] -eq 1) 'FIXTURE_CREATION_FAILED'
    # quote consulta Mongo directamente, sin usar la caché de fichas.
    $quote = Invoke-OwnApi -Method GET -Path ('/v1/vehicles/' + $Id + '/quote')
    Assert-Vehicle $quote $Id
    return $result
}

function Get-TestSearch {
    param([string] $Brand)
    return Invoke-OwnApi -Method GET -Path ('/v1/vehicles/search?brand=' + [Uri]::EscapeDataString($Brand) + '&pageSize=100')
}

function Test-SearchContains {
    param([object] $Response, [string] $Id)
    Assert-Condition ($Response.Status -eq 200) 'SEARCH_READ_FAILED'
    Assert-Condition ($Response.Json.Contains('items') -and $Response.Json.Contains('total')) 'SEARCH_PAGE_INVALID'
    foreach ($item in @($Response.Json['items'])) {
        if ($item['vehicleId'] -eq $Id -and $item['active'] -eq $true -and $item['version'] -ge 1) { return $true }
    }
    return $false
}

function Wait-TestSearch {
    param([string] $Brand, [string] $Id, [switch] $AllowUnavailable)
    $clock = [Diagnostics.Stopwatch]::StartNew()
    $attempts = 0
    $unavailable = 0
    do {
        $attempts++
        $search = Get-TestSearch $Brand
        if ($AllowUnavailable -and $search.Status -eq 503) {
            $unavailable++
            Assert-Condition ($search.Json['code'] -eq 'SEARCH_UNAVAILABLE') 'SEARCH_ERROR_CLASS_INVALID'
        }
        elseif ((Test-SearchContains $search $Id) -and $clock.Elapsed.TotalSeconds -lt $RecoveryTimeoutSeconds) {
            return [ordered]@{ visible = $true; attempts = $attempts; unavailableResponses = $unavailable; elapsedMs = [Math]::Round($clock.Elapsed.TotalMilliseconds, 2) }
        }
        Start-Sleep -Milliseconds 500
    } while ($clock.Elapsed.TotalSeconds -lt $RecoveryTimeoutSeconds)
    throw [TimeoutException]::new('SEARCH_RECOVERY_TIMEOUT')
}

function Assert-TestAvailability {
    param([string] $Id)
    $path = '/v1/availability?vehicleId=' + $Id + '&startsOn=' + $script:StartsOn + '&endsOn=' + $script:EndsOn
    $result = Invoke-OwnApi -Method GET -Path $path -AllowedInstances @('rentals')
    Assert-Condition ($result.Status -eq 200 -and $result.Json['vehicleId'] -eq $Id -and $result.Json['available'] -eq $true -and $result.Json['days'] -eq 3 -and $result.Json['dailyRateMinor'] -eq 5000000 -and $result.Json['totalMinor'] -eq 15000000 -and $result.Json['currency'] -eq 'ARS') 'AVAILABILITY_STATE_INVALID'
}

function Invoke-TestStep {
    param([string] $Name, [scriptblock] $Action)
    $began = [DateTimeOffset]::UtcNow
    $clock = [Diagnostics.Stopwatch]::StartNew()
    $evidence = $null
    $failure = $null
    $status = 'FAIL'
    $script:StepEvidence = $null
    try {
        $evidence = & $Action
        $status = 'PASS'
    }
    catch {
        $status = 'FAIL'
        if ($null -eq $evidence) { $evidence = $script:StepEvidence }
        $failure = Get-SafeFailureCode $_.Exception
        throw
    }
    finally {
        $script:Steps.Add([ordered]@{ name = $Name; startedAtUtc = $began.ToString('o'); status = $status; elapsedMs = [Math]::Round($clock.Elapsed.TotalMilliseconds, 2); evidence = $evidence; errorCode = $failure })
    }
}

function Disable-TestFixture {
    param([string] $Id)
    for ($attempt = 1; $attempt -le 3; $attempt++) {
        $current = Invoke-OwnApi -Method GET -Path ('/v1/vehicles/' + $Id)
        if ($current.Status -eq 404) { return 'not_created' }
        Assert-Condition ($current.Status -eq 200 -and $current.Json['vehicleId'] -eq $Id) 'FIXTURE_CLEANUP_READ_FAILED'
        if ($current.Json['active'] -eq $false) { return 'inactive' }
        $input = @{}
        foreach ($key in @('vehicleId', 'brand', 'model', 'category', 'transmission', 'seats', 'dailyRateMinor', 'currency', 'version')) { $input[$key] = $current.Json[$key] }
        $input['active'] = $false
        $updated = Invoke-OwnApi -Method PUT -Path ('/v1/vehicles/' + $Id) -Body $input
        if ($updated.Status -eq 409 -and $updated.Json['code'] -eq 'VERSION_CONFLICT') { continue }
        Assert-Condition ($updated.Status -eq 200 -and $updated.Json['active'] -eq $false -and $updated.Json['version'] -gt $current.Json['version']) 'FIXTURE_CLEANUP_UPDATE_FAILED'
        $checked = Invoke-OwnApi -Method GET -Path ('/v1/vehicles/' + $Id)
        Assert-Condition ($checked.Status -eq 200 -and $checked.Json['active'] -eq $false -and $checked.Json['version'] -ge $updated.Json['version']) 'FIXTURE_CLEANUP_STATE_INVALID'
        return 'inactive'
    }
    throw [InvalidOperationException]::new('FIXTURE_CLEANUP_VERSION_CONFLICT')
}

Push-Location -LiteralPath $script:RepoRoot
try {
    $null = Invoke-TestStep 'preconditions' {
        Assert-Condition (Test-Path -LiteralPath $script:ComposePath -PathType Leaf) 'COMPOSE_FILE_MISSING'
        $settings = Read-LocalSettings
        Assert-Condition ($settings.ContainsKey('AUTH_ADMIN_TOKEN') -and $settings['AUTH_ADMIN_TOKEN'].Length -ge 24) 'ADMIN_CREDENTIAL_MISSING'
        $script:Token = $settings['AUTH_ADMIN_TOKEN']
        $port = if ($settings.ContainsKey('APP_PORT')) { $settings['APP_PORT'] } else { '8080' }
        $address = if ([string]::IsNullOrWhiteSpace($BaseUrl)) { 'http://127.0.0.1:' + $port } else { $BaseUrl }
        $uri = $null
        Assert-Condition ([Uri]::TryCreate($address, [UriKind]::Absolute, [ref]$uri)) 'GATEWAY_ADDRESS_INVALID'
        Assert-Condition ($uri.Scheme -eq 'http' -and $uri.IsLoopback -and $uri.UserInfo.Length -eq 0 -and $uri.AbsolutePath -eq '/' -and $uri.Query.Length -eq 0 -and $uri.Fragment.Length -eq 0) 'GATEWAY_MUST_BE_LOCAL'
        $script:GatewayBase = $uri.GetLeftPart([UriPartial]::Authority)
        $transport = [Net.Http.HttpClientHandler]::new()
        $transport.AllowAutoRedirect = $false
        $transport.UseProxy = $false
        $script:Client = [Net.Http.HttpClient]::new($transport)
        $script:Client.Timeout = [TimeSpan]::FromSeconds($script:RequestTimeoutSeconds)
        $states = [ordered]@{}
        foreach ($service in $script:RequiredServices) {
            $state = Get-ServiceState $service
            Assert-Condition ($state['State'] -eq 'running') 'COMPOSE_PRECONDITION_NOT_RUNNING'
            $states[$service] = 'running'
        }
        $ready = Invoke-OwnApi -Method GET -Path '/internal/ready' -AllowedInstances @('gateway')
        Assert-Condition ($ready.Status -eq 200 -and $ready.Json['status'] -eq 'ok') 'GATEWAY_PRECONDITION_NOT_READY'
        try { $branch = [TimeZoneInfo]::FindSystemTimeZoneById('America/Buenos_Aires') }
        catch { $branch = [TimeZoneInfo]::FindSystemTimeZoneById('Argentina Standard Time') }
        $today = [TimeZoneInfo]::ConvertTime([DateTimeOffset]::UtcNow, $branch)
        $script:StartsOn = $today.AddDays(2).ToString('yyyy-MM-dd')
        $script:EndsOn = $today.AddDays(5).ToString('yyyy-MM-dd')
        return [ordered]@{ services = $states; gatewayReady = $true; loopbackOnly = $true }
    }
    $null = Invoke-TestStep 'baseline_fixture' {
        $null = New-TestVehicle $script:BaselineId $script:BaselineBrand
        Assert-Vehicle (Invoke-OwnApi -Method GET -Path ('/v1/vehicles/' + $script:BaselineId)) $script:BaselineId
        Assert-TestAvailability $script:BaselineId
        $indexed = Wait-TestSearch $script:BaselineBrand $script:BaselineId
        return [ordered]@{ fixture = $script:BaselineId; authoritativeRead = $true; availability = $true; search = $indexed }
    }
    $null = Invoke-TestStep 'fleet_both_replicas' {
        $seen = [Collections.Generic.HashSet[string]]::new()
        $requests = 0
        do {
            $result = Invoke-OwnApi -Method GET -Path ('/v1/vehicles/' + $script:BaselineId)
            Assert-Vehicle $result $script:BaselineId
            $null = $seen.Add($result.Instance)
            $requests++
            if ($seen.Count -eq 2 -and $requests -ge 8) { break }
            Start-Sleep -Milliseconds 100
        } while ($requests -lt 40)
        Assert-Condition ($seen.Contains('fleet-a') -and $seen.Contains('fleet-b')) 'BOTH_REPLICAS_NOT_OBSERVED'
        return [ordered]@{ instances = @($seen | Sort-Object); successfulRequests = $requests }
    }
    $null = Invoke-TestStep 'fleet_b_stopped' {
        Stop-TestService 'fleet-b'
        return Measure-FleetWithdrawal $script:BaselineId
    }
    $null = Invoke-TestStep 'fleet_b_recovered' {
        Start-TestService 'fleet-b'
        $clock = [Diagnostics.Stopwatch]::StartNew()
        $attempts = 0
        do {
            $attempts++
            $result = Invoke-OwnApi -Method GET -Path ('/v1/vehicles/' + $script:BaselineId)
            Assert-Vehicle $result $script:BaselineId
            if ($result.Instance -eq 'fleet-b' -and $clock.Elapsed.TotalSeconds -lt $RecoveryTimeoutSeconds) { return [ordered]@{ recoveredInstance = 'fleet-b'; attempts = $attempts; elapsedMs = [Math]::Round($clock.Elapsed.TotalMilliseconds, 2); configuredNginxFailTimeoutSeconds = 30 } }
            Start-Sleep -Milliseconds 500
        } while ($clock.Elapsed.TotalSeconds -lt $RecoveryTimeoutSeconds)
        throw [TimeoutException]::new('FLEET_B_RECOVERY_TIMEOUT')
    }
    $null = Invoke-TestStep 'redis_stopped' {
        Stop-TestService 'redis'
        for ($i = 0; $i -lt 2; $i++) { Assert-Vehicle (Invoke-OwnApi -Method GET -Path ('/v1/vehicles/' + $script:BaselineId)) $script:BaselineId }
        Assert-TestAvailability $script:BaselineId
        return [ordered]@{ serviceState = 'stopped'; detailReads = 2; availability = $true; sourceFallback = 'responded_with_valid_vehicle' }
    }
    $null = Invoke-TestStep 'redis_recovered' {
        Start-TestService 'redis'
        Wait-ServiceHealthy 'redis'
        Assert-Vehicle (Invoke-OwnApi -Method GET -Path ('/v1/vehicles/' + $script:BaselineId)) $script:BaselineId
        return [ordered]@{ serviceState = 'healthy'; detailRead = $true }
    }
    $null = Invoke-TestStep 'rabbit_stopped_outbox_retained' {
        Stop-TestService 'rabbitmq'
        Assert-Condition (-not (Test-SearchContains (Get-TestSearch $script:RabbitBrand) $script:RabbitId)) 'RABBIT_FIXTURE_ALREADY_INDEXED'
        $null = New-TestVehicle $script:RabbitId $script:RabbitBrand
        Assert-Vehicle (Invoke-OwnApi -Method GET -Path ('/v1/vehicles/' + $script:RabbitId)) $script:RabbitId
        for ($i = 0; $i -lt 3; $i++) {
            Assert-Condition (-not (Test-SearchContains (Get-TestSearch $script:RabbitBrand) $script:RabbitId)) 'INDEX_CHANGED_WITH_BROKER_STOPPED'
            Start-Sleep -Milliseconds 500
        }
        return [ordered]@{ serviceState = 'stopped'; fixture = $script:RabbitId; created = $true; authoritativeRead = $true; indexVisible = $false; absentChecks = 3 }
    }
    $null = Invoke-TestStep 'rabbit_recovered_backlog_observed' {
        Start-TestService 'rabbitmq'
        $indexed = Wait-TestSearch $script:RabbitBrand $script:RabbitId
        Assert-Vehicle (Invoke-OwnApi -Method GET -Path ('/v1/vehicles/' + $script:RabbitId + '/quote')) $script:RabbitId
        return [ordered]@{ fixture = $script:RabbitId; sourcePreserved = $true; backlogRecoveryObserved = $true; search = $indexed }
    }
    $null = Invoke-TestStep 'solr_stopped_search_isolated' {
        Stop-TestService 'solr'
        $search = Get-TestSearch $script:BaselineBrand
        Assert-Condition ($search.Status -eq 503 -and $search.Json['code'] -eq 'SEARCH_UNAVAILABLE') 'SEARCH_FAILURE_NOT_CLASSIFIED'
        Assert-Vehicle (Invoke-OwnApi -Method GET -Path ('/v1/vehicles/' + $script:BaselineId)) $script:BaselineId
        Assert-TestAvailability $script:BaselineId
        return [ordered]@{ serviceState = 'stopped'; searchStatus = 503; searchErrorCode = 'SEARCH_UNAVAILABLE'; detailRead = $true; availability = $true }
    }
    $null = Invoke-TestStep 'solr_recovered' {
        Start-TestService 'solr'
        $indexed = Wait-TestSearch $script:BaselineBrand $script:BaselineId -AllowUnavailable
        return [ordered]@{ search = $indexed; indexDataPreserved = $true }
    }
}
catch {
    $script:Failed = $true
    $script:FailureCode = Get-SafeFailureCode $_.Exception
}
finally {
    # Cada restauración se intenta aunque haya fallado otra; no se usan down,
    # rm ni eliminación de volúmenes. start conserva los mismos contenedores.
    foreach ($service in @($script:RestoreTargets | Sort-Object)) {
        $clock = [Diagnostics.Stopwatch]::StartNew()
        try {
            Start-TestService $service
            Wait-ServiceHealthy $service
            $script:Restoration.Add([ordered]@{ service = $service; status = 'PASS'; state = 'healthy'; elapsedMs = [Math]::Round($clock.Elapsed.TotalMilliseconds, 2) })
        }
        catch {
            $script:Failed = $true
            $script:Restoration.Add([ordered]@{ service = $service; status = 'FAIL'; errorCode = Get-SafeFailureCode $_.Exception; elapsedMs = [Math]::Round($clock.Elapsed.TotalMilliseconds, 2) })
        }
    }
    foreach ($id in $script:Fixtures) {
        try {
            $state = Disable-TestFixture $id
            $script:FixtureCleanup.Add([ordered]@{ fixture = $id; status = 'PASS'; state = $state })
        }
        catch {
            $script:Failed = $true
            $script:FixtureCleanup.Add([ordered]@{ fixture = $id; status = 'FAIL'; errorCode = Get-SafeFailureCode $_.Exception })
        }
    }
    if ($null -ne $script:Client) { $script:Client.Dispose() }
    $script:Token = ''
    $script:TotalClock.Stop()
    $result = if ($script:Failed) { 'FAIL' } else { 'PASS' }
    $report = [ordered]@{
        schemaVersion = '1.0.0'
        runId = $script:RunId
        result = $result
        startedAtUtc = $script:StartedAt.ToString('o')
        completedAtUtc = [DateTimeOffset]::UtcNow.ToString('o')
        elapsedMs = [Math]::Round($script:TotalClock.Elapsed.TotalMilliseconds, 2)
        project = @{ name = $script:ProjectName; composeFile = 'compose.yaml'; gateway = 'local-loopback' }
        scope = 'Componentes propios; sin integraciones de gimnasio o clínica'
        limits = @{ requestTimeoutSeconds = $script:RequestTimeoutSeconds; dockerTimeoutSeconds = $DockerTimeoutSeconds; recoveryTimeoutSeconds = $RecoveryTimeoutSeconds }
        steps = @($script:Steps.ToArray())
        requests = @($script:Requests.ToArray())
        restoration = @($script:Restoration.ToArray())
        fixtureCleanup = @($script:FixtureCleanup.ToArray())
        errorCode = $script:FailureCode
    }
    try {
        $target = if ([IO.Path]::IsPathRooted($ReportPath)) { [IO.Path]::GetFullPath($ReportPath) } else { [IO.Path]::GetFullPath((Join-Path $script:RepoRoot $ReportPath)) }
        $directory = [IO.Path]::GetDirectoryName($target)
        $null = [IO.Directory]::CreateDirectory($directory)
        [IO.File]::WriteAllText($target, (ConvertTo-Json -InputObject $report -Depth 20), [Text.UTF8Encoding]::new($false))
    }
    catch {
        $script:Failed = $true
        $result = 'FAIL'
        Write-Host 'REPORT_WRITE_FAILED'
    }
    Pop-Location
    Write-Host ('Resiliencia: ' + $result)
}
if ($script:Failed) { exit 1 }
exit 0
