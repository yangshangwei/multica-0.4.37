#Requires -Version 7.2
[CmdletBinding()]
param([Parameter(Mandatory)][string]$StateDirectory, [Parameter(Mandatory)][string]$RepoRoot, [switch]$Stop)
Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
if (-not $IsWindows -or $env:GITHUB_ACTIONS -ne 'true' -or $env:RUNNER_ENVIRONMENT -ne 'github-hosted') { throw 'Disposable GitHub Windows runner required' }
$state = [IO.Path]::GetFullPath($StateDirectory)
$tempRoot = [IO.Path]::GetFullPath($env:RUNNER_TEMP).TrimEnd('\') + '\'
if (-not $state.StartsWith($tempRoot, [StringComparison]::OrdinalIgnoreCase)) { throw 'State must be below RUNNER_TEMP' }
$private = Join-Path $state 'server-private.json'
if ($Stop) {
    if (-not (Test-Path $private)) { return }
    $record = Get-Content -Raw $private | ConvertFrom-Json
    $process = if ($record.pid) { Get-Process -Id $record.pid -ErrorAction SilentlyContinue } else { $null }
    if ($null -ne $process) {
        if ($process.Path -ne $record.serverExe) { throw 'Backend PID ownership mismatch' }
        Stop-Process -Id $process.Id
    }
    if (Test-Path (Join-Path $state 'pgdata/postmaster.pid')) {
        & (Join-Path $record.pgBin 'pg_ctl.exe') -D (Join-Path $state 'pgdata') -m fast -w stop
        if ($LASTEXITCODE -ne 0) { throw 'PostgreSQL shutdown failed' }
    }
    return
}
if (Test-Path $state) { throw 'Refusing to reuse an existing business state directory' }
New-Item -ItemType Directory $state | Out-Null
$pgCandidates = @()
if ($env:PGBIN) { $pgCandidates += $env:PGBIN }
$pgCandidates += @(Get-ChildItem 'C:\Program Files\PostgreSQL' -Directory -ErrorAction SilentlyContinue | Sort-Object Name -Descending | ForEach-Object { Join-Path $_.FullName 'bin' })
$pgBin = $pgCandidates | Where-Object { Test-Path (Join-Path $_ 'initdb.exe') } | Select-Object -First 1
if (-not $pgBin) { throw 'Runner PostgreSQL initdb/pg_ctl installation missing' }
function Get-FreePort {
    $listener = [Net.Sockets.TcpListener]::new([Net.IPAddress]::Loopback, 0)
    $listener.Start(); $port = $listener.LocalEndpoint.Port; $listener.Stop(); return $port
}
$pgPort = Get-FreePort
$apiPort = Get-FreePort
$proxyPort = Get-FreePort
$env:DATABASE_URL = "postgres://postgres@127.0.0.1:$pgPort/multica_acceptance?sslmode=disable"
$env:PORT = "$apiPort"
$env:APP_ENV = 'development'
$env:JWT_SECRET = [Convert]::ToHexString([Security.Cryptography.RandomNumberGenerator]::GetBytes(32))
$env:MULTICA_AUTH_MODE = 'password'
$env:MULTICA_PASSWORD_LIMITER_MODE = 'single'
$env:MULTICA_PLATFORM_ADMIN_ENABLED = 'true'
$env:MULTICA_MANAGED_INSTALLATIONS_ENABLED = 'true'
$env:MULTICA_DEPLOYMENT_ID = [guid]::NewGuid().ToString()
$env:MULTICA_DEVICE_AUTH_ENABLED = 'false'
$env:ALLOW_SIGNUP = 'true'
$env:MULTICA_LLM_API_KEY = ''
$env:FRONTEND_ORIGIN = "http://127.0.0.1:$proxyPort"
& (Join-Path $pgBin 'initdb.exe') -D (Join-Path $state 'pgdata') -U postgres -A trust --no-locale --encoding=UTF8
if ($LASTEXITCODE -ne 0) { throw 'initdb failed' }
@{ version = 1; pid = $null; serverExe = ''; pgBin = $pgBin } | ConvertTo-Json | Set-Content $private
& (Join-Path $pgBin 'pg_ctl.exe') -D (Join-Path $state 'pgdata') -l (Join-Path $state 'postgres.log') -o "-h 127.0.0.1 -p $pgPort" -w start
if ($LASTEXITCODE -ne 0) { throw 'PostgreSQL startup failed' }
& (Join-Path $pgBin 'createdb.exe') -h 127.0.0.1 -p $pgPort -U postgres multica_acceptance
if ($LASTEXITCODE -ne 0) { throw 'Database creation failed' }
$serverExe = Join-Path $state 'business-server.exe'
if ($env:GITHUB_SHA -notmatch '^[a-f0-9]{40}$') { throw 'Exact source commit required' }
Push-Location (Join-Path $RepoRoot 'server')
try {
    go build -ldflags "-X main.commit=$env:GITHUB_SHA" -o $serverExe ./cmd/server
    if ($LASTEXITCODE -ne 0) { throw 'Backend build failed' }
    go run ./cmd/migrate up
    if ($LASTEXITCODE -ne 0) { throw 'Migrations failed' }
    go build -o (Join-Path $state 'fixture-claude.exe') (Join-Path $RepoRoot 'apps/desktop/scripts/windows-business-provider.go')
    if ($LASTEXITCODE -ne 0) { throw 'Fixture provider build failed' }
    $process = Start-Process $serverExe -PassThru -RedirectStandardOutput (Join-Path $state 'backend.stdout.log') -RedirectStandardError (Join-Path $state 'backend.stderr.log')
} finally { Pop-Location }
$environment = @{}
foreach ($key in @('DATABASE_URL','PORT','APP_ENV','JWT_SECRET','MULTICA_AUTH_MODE','MULTICA_PASSWORD_LIMITER_MODE','MULTICA_PLATFORM_ADMIN_ENABLED','MULTICA_MANAGED_INSTALLATIONS_ENABLED','MULTICA_DEPLOYMENT_ID','MULTICA_DEVICE_AUTH_ENABLED','ALLOW_SIGNUP','MULTICA_LLM_API_KEY','FRONTEND_ORIGIN')) { $environment[$key] = [Environment]::GetEnvironmentVariable($key) }
$api = "http://127.0.0.1:$apiPort"
@{ version = 1; pid = $process.Id; serverExe = $serverExe; pgBin = $pgBin; api = $api; proxyPort = $proxyPort; repo = $RepoRoot; environment = $environment } | ConvertTo-Json -Depth 5 | Set-Content $private
$ready = $false
function Get-BackendFailure {
    $text = (Get-Content (Join-Path $state 'backend.stderr.log') -Tail 12 -ErrorAction SilentlyContinue) -join "`n"
    foreach ($value in @($env:JWT_SECRET, $env:DATABASE_URL)) { if ($value) { $text = $text.Replace($value, '[redacted]') } }
    return $text -replace '(?i)(token|password|secret|authorization)=[^\s]+', '$1=[redacted]'
}
for ($attempt = 0; $attempt -lt 90; $attempt++) {
    try {
        $health = Invoke-RestMethod "$api/health"
        $readiness = Invoke-RestMethod "$api/readyz"
        if ($health.status -eq 'ok' -and $health.pid -eq $process.Id -and $health.commit -eq $env:GITHUB_SHA -and $readiness.status -eq 'ok') { $ready = $true; break }
    } catch { }
    if ($process.HasExited) { throw "Backend exited before readiness: $(Get-BackendFailure)" }
    Start-Sleep -Seconds 1
}
if (-not $ready) { throw "Backend readiness timeout: $(Get-BackendFailure)" }
if ($env:GITHUB_ENV) {
    "MULTICA_BUSINESS_STATE_DIR=$state" | Add-Content $env:GITHUB_ENV
    "MULTICA_BUSINESS_API=$api" | Add-Content $env:GITHUB_ENV
    "MULTICA_BUSINESS_SERVER_EXE=$serverExe" | Add-Content $env:GITHUB_ENV
}
Write-Output 'Disposable PostgreSQL and password/managed backend ready. Private state must not be uploaded.'
