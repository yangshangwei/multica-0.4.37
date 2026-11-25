#Requires -Version 7.2
[CmdletBinding()]
param([Parameter(Mandatory)][string]$StateDirectory, [Parameter(Mandatory)][string]$RepoRoot, [switch]$Stop, [switch]$Probe)
Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
if (-not $IsWindows -or $env:GITHUB_ACTIONS -ne 'true' -or $env:RUNNER_ENVIRONMENT -ne 'github-hosted') { throw 'Disposable GitHub Windows runner required' }
$state = [IO.Path]::GetFullPath($StateDirectory)
$tempRoot = [IO.Path]::GetFullPath($env:RUNNER_TEMP).TrimEnd('\') + '\'
if (-not $state.StartsWith($tempRoot, [StringComparison]::OrdinalIgnoreCase)) { throw 'State must be below RUNNER_TEMP' }
$webRecord = Join-Path $state 'web-private.json'
$apiState = Join-Path $state 'api'
function Get-NodeApplication {
    $path = (Get-Command node -CommandType Application -All -ErrorAction Stop | Select-Object -First 1).Source
    if ($path -isnot [string] -or -not [IO.Path]::IsPathFullyQualified($path) -or -not (Test-Path -LiteralPath $path -PathType Leaf)) { throw 'Node application must resolve to one absolute executable file' }
    return $path
}
function Test-ServiceStartTime {
    param([datetime]$ActualStart, $RecordedStart)
    if ($RecordedStart -is [datetime]) {
        $recorded = $RecordedStart
    } elseif ($RecordedStart -is [string]) {
        try { $recorded = [datetime]::ParseExact($RecordedStart, 'o', [Globalization.CultureInfo]::InvariantCulture, [Globalization.DateTimeStyles]::RoundtripKind) }
        catch { return $false }
    } else { return $false }
    return $ActualStart.ToUniversalTime().Ticks -eq $recorded.ToUniversalTime().Ticks
}
function Get-ServiceProcessEvidence {
    param($Record, [string]$ExpectedPath)
    $process = if ($Record.pid) { Get-Process -Id $Record.pid -ErrorAction SilentlyContinue } else { $null }
    return [ordered]@{
        present = ($null -ne $process)
        alive = ($null -ne $process -and -not $process.HasExited)
        path_matches = ($null -ne $process -and $process.Path -ieq $ExpectedPath)
        start_matches = ($null -ne $process -and (Test-ServiceStartTime $process.StartTime $Record.startedAt))
    }
}
if ($Probe) {
    $apiRecord = Get-Content -LiteralPath (Join-Path $apiState 'server-private.json') -Raw | ConvertFrom-Json
    $web = Get-Content -LiteralPath $webRecord -Raw | ConvertFrom-Json
    [ordered]@{
        source_matches = ($apiRecord.sourceCommit -ceq $env:GITHUB_SHA)
        api = Get-ServiceProcessEvidence $apiRecord (Join-Path $apiState 'business-server.exe')
        web = Get-ServiceProcessEvidence $web $web.node
    } | ConvertTo-Json -Compress -Depth 4
    return
}
function Stop-OwnedWeb {
    param($Record)
    if (-not $Record.pid) { return }
    $process = Get-Process -Id $Record.pid -ErrorAction SilentlyContinue
    if ($null -eq $process) { return }
    if ($process.Path -ine $Record.node -or -not (Test-ServiceStartTime $process.StartTime $Record.startedAt)) { throw 'Web PID ownership mismatch' }
    & taskkill.exe /PID $process.Id /T /F | Out-Null
    if ($LASTEXITCODE -ne 0) { throw 'Owned production Web shutdown failed' }
}
if ($Stop) {
    try {
        if (Test-Path -LiteralPath $webRecord) { Stop-OwnedWeb (Get-Content -LiteralPath $webRecord -Raw | ConvertFrom-Json) }
    } finally { & (Join-Path $PSScriptRoot 'windows-business-server.ps1') -StateDirectory $apiState -RepoRoot $RepoRoot -Stop }
    return
}
if (Test-Path -LiteralPath $state) { throw 'Refusing to reuse collaboration state' }
if ($env:GITHUB_SHA -notmatch '^[a-f0-9]{40}$') { throw 'Exact candidate source required' }
$actual = (& git -C $RepoRoot rev-parse HEAD).Trim()
if ($LASTEXITCODE -ne 0 -or $actual -cne $env:GITHUB_SHA) { throw 'Candidate checkout differs from source SHA' }
New-Item -ItemType Directory -Path $state | Out-Null
@{ pid = $null; node = ''; startedAt = '' } | ConvertTo-Json | Set-Content -LiteralPath $webRecord
$listener = [Net.Sockets.TcpListener]::new([Net.IPAddress]::Loopback, 0)
$listener.Start(); $webPort = $listener.LocalEndpoint.Port; $listener.Stop()
& (Join-Path $PSScriptRoot 'windows-business-server.ps1') -StateDirectory $apiState -RepoRoot $RepoRoot -FrontendPort $webPort
$private = Join-Path $apiState 'server-private.json'
$setup = Get-Content -LiteralPath $private -Raw | ConvertFrom-Json -AsHashtable
if ($setup.environment.DATABASE_URL -notmatch '^postgres://postgres@127\.0\.0\.1:[0-9]+/multica_acceptance\?sslmode=disable$') { throw 'Explicit task database identity missing' }
$bin = Join-Path $state 'bin'
New-Item -ItemType Directory -Path $bin | Out-Null
$daemon = Join-Path $bin 'host-daemon.exe'; $provider = Join-Path $bin 'host-provider.exe'; $cli = Join-Path $bin 'host-cli.exe'; $discovery = Join-Path $bin 'discovery-only.exe'
Push-Location (Join-Path $RepoRoot 'server')
try {
    # Compile only: the native fixture dispatches in init before TestMain.
    go test -c -o $daemon ./internal/daemon
    if ($LASTEXITCODE -ne 0) { throw 'Host fixture compile failed' }
    go build -o $provider (Join-Path $RepoRoot 'e2e/fixtures/collaboration-host-provider.go')
    if ($LASTEXITCODE -ne 0) { throw 'Synthetic host Provider build failed' }
    go build -ldflags "-X main.sourceCommit=$env:GITHUB_SHA" -o $discovery (Join-Path $RepoRoot 'e2e/fixtures/collaboration-discovery-only.go')
    if ($LASTEXITCODE -ne 0) { throw 'Discovery-only CLI build failed' }
    $discoveryBuildInfo = & go version -m $discovery
    if ($LASTEXITCODE -ne 0 -or ($discoveryBuildInfo -join "`n") -notmatch [regex]::Escape("main.sourceCommit=$env:GITHUB_SHA")) { throw 'Discovery-only source stamp mismatch' }
    go build -ldflags "-X main.commit=$env:GITHUB_SHA" -o $cli ./cmd/multica
    if ($LASTEXITCODE -ne 0) { throw 'Source-bound host CLI build failed' }
} finally { Pop-Location }
$api = $setup.api
$webUrl = "http://127.0.0.1:$webPort"
$setup.collaboration = @{ webUrl = $webUrl; daemon = $daemon; provider = $provider; cli = $cli; discovery = $discovery }
$setup | ConvertTo-Json -Depth 8 | Set-Content -LiteralPath $private
$env:REMOTE_API_URL = $api
$env:NEXT_PUBLIC_API_URL = $api
$env:NEXT_PUBLIC_WS_URL = $api.Replace('http:', 'ws:') + '/ws'
Push-Location $RepoRoot
try {
    pnpm --filter @multica/web build
    if ($LASTEXITCODE -ne 0) { throw 'Production Web build failed' }
} finally { Pop-Location }
$node = Get-NodeApplication
$next = Join-Path $RepoRoot 'apps/web/node_modules/next/dist/bin/next'
$arguments = @(('"' + $next + '"'), 'start', '--hostname', '127.0.0.1', '--port', "$webPort")
$web = Start-Process -FilePath $node -ArgumentList $arguments -WorkingDirectory (Join-Path $RepoRoot 'apps/web') -PassThru -RedirectStandardOutput (Join-Path $state 'web.stdout.log') -RedirectStandardError (Join-Path $state 'web.stderr.log')
try {
    $web.Refresh()
    @{ pid = $web.Id; node = $node; startedAt = $web.StartTime.ToUniversalTime().ToString('o') } | ConvertTo-Json | Set-Content -LiteralPath $webRecord
    $ready = $false
    for ($attempt = 0; $attempt -lt 60; $attempt++) {
        if ($web.HasExited) { throw 'Owned production Web exited before readiness; private logs retained' }
        try { if ((Invoke-WebRequest $webUrl -TimeoutSec 3).StatusCode -eq 200) { $ready = $true; break } } catch { }
        Start-Sleep -Seconds 1
    }
    if (-not $ready) { throw 'Owned production Web readiness timeout' }
} catch {
    if (-not $web.HasExited) { Stop-OwnedWeb (Get-Content -LiteralPath $webRecord -Raw | ConvertFrom-Json) }
    throw
}

$env:MULTICA_COLLABORATION_STATE_DIR = $state
if ($env:GITHUB_ENV) { "MULTICA_COLLABORATION_STATE_DIR=$state" | Add-Content $env:GITHUB_ENV }
Write-Output 'Source-bound isolated collaboration API, production Web and fake Host binaries prepared; private state is not an upload artifact.'
