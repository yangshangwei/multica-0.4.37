#Requires -Version 7.2
# Dependency-free parser and fixture tests. These do not run an installer.
Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
$scriptPath = Join-Path $PSScriptRoot 'accept-windows-installer.ps1'
$tokens = $null
$errors = $null
$ast = [Management.Automation.Language.Parser]::ParseFile($scriptPath, [ref]$tokens, [ref]$errors)
if ($errors.Count -gt 0) { throw ($errors | Out-String) }
foreach ($name in @('Assert-X64Binary', 'Get-InstallerEvidence', 'Assert-DisposableRunner')) {
    $definition = $ast.Find({ param($node) $node -is [Management.Automation.Language.FunctionDefinitionAst] -and $node.Name -eq $name }, $false)
    if ($null -eq $definition) { throw "Missing function: $name" }
    . ([scriptblock]::Create($definition.Extent.Text))
}

$servicePath = Join-Path $PSScriptRoot 'windows-collaboration-server.ps1'
$serviceAst = [Management.Automation.Language.Parser]::ParseFile($servicePath, [ref]$tokens, [ref]$errors)
if ($errors.Count -gt 0) { throw ($errors | Out-String) }
$resolver = $serviceAst.Find({ param($node) $node -is [Management.Automation.Language.FunctionDefinitionAst] -and $node.Name -eq 'Get-NodeApplication' }, $false)
if ($null -eq $resolver) { throw 'Missing Node application resolver' }
. ([scriptblock]::Create($resolver.Extent.Text))
foreach ($name in @('Test-ServiceStartTime', 'Get-ServiceProcessEvidence')) {
    $definition = $serviceAst.Find({ param($node) $node -is [Management.Automation.Language.FunctionDefinitionAst] -and $node.Name -eq $name }, $false)
    if ($null -eq $definition) { throw "Missing service ownership helper: $name" }
    . ([scriptblock]::Create($definition.Extent.Text))
}


function Assert-Throws {
    param([scriptblock]$Action, [string]$Expected)
    try { & $Action } catch {
        if ($_.Exception.Message -notmatch $Expected) { throw }
        return
    }
    throw "Expected failure: $Expected"
}

$fixtureRoot = Join-Path ([IO.Path]::GetTempPath()) ('multica-acceptance-test-' + [guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $fixtureRoot | Out-Null
$originalRunnerEnvironment = $env:RUNNER_ENVIRONMENT
$originalPath = $env:PATH
try {
    $env:RUNNER_ENVIRONMENT = 'self-hosted'
    Assert-Throws { Assert-DisposableRunner } 'Native Windows x64|Only disposable'
    $env:RUNNER_ENVIRONMENT = $originalRunnerEnvironment
    $path = Join-Path $fixtureRoot 'fixture.exe'
    $bytes = [byte[]]::new(128)
    $bytes[0] = 0x4d; $bytes[1] = 0x5a; $bytes[60] = 64
    $bytes[64] = 0x50; $bytes[65] = 0x45; $bytes[68] = 0x64; $bytes[69] = 0x86
    [IO.File]::WriteAllBytes($path, $bytes)
    Assert-X64Binary $path
    $bytes[68] = 0x4c; $bytes[69] = 0x01
    [IO.File]::WriteAllBytes($path, $bytes)
    Assert-Throws { Assert-X64Binary $path } 'Not a Windows x64'
    $bytes[60] = 255
    [IO.File]::WriteAllBytes($path, $bytes)
    Assert-Throws { Assert-X64Binary $path } 'Invalid PE offset'
    $bytes[0] = 0
    [IO.File]::WriteAllBytes($path, $bytes)
    Assert-Throws { Assert-X64Binary $path } 'Missing DOS header'
    Assert-Throws { Get-InstallerEvidence $path '0.5.2' } 'Expected exact versioned'
    $current = Get-Process -Id $PID
    $stamp = $current.StartTime.ToUniversalTime().ToString('o')
    if (-not (Test-ServiceStartTime $current.StartTime $stamp)) { throw 'Round-trip string start time rejected' }
    $owned = @{ pid = $PID; startedAt = $stamp } | ConvertTo-Json | ConvertFrom-Json
    $proof = Get-ServiceProcessEvidence $owned $current.Path
    if (-not ($proof.present -and $proof.alive -and $proof.path_matches -and $proof.start_matches)) { throw 'JSON-round-tripped owned process proof failed' }
    $foreign = Get-ServiceProcessEvidence $owned (Join-Path $fixtureRoot 'foreign.exe')
    if ($foreign.path_matches) { throw 'Foreign executable accepted' }
    $wrongTime = @{ pid = $PID; startedAt = $current.StartTime.ToUniversalTime().AddTicks(1).ToString('o') } | ConvertTo-Json | ConvertFrom-Json
    if ((Get-ServiceProcessEvidence $wrongTime $current.Path).start_matches) { throw 'Reused PID start time accepted' }
    $invalidTime = @{ pid = $PID; startedAt = 'wrong-start' } | ConvertTo-Json | ConvertFrom-Json
    if ((Get-ServiceProcessEvidence $invalidTime $current.Path).start_matches) { throw 'Malformed process start time accepted' }
    $missing = @{ pid = 0; startedAt = '' } | ConvertTo-Json | ConvertFrom-Json
    $absent = Get-ServiceProcessEvidence $missing $current.Path
    if ($absent.present -or $absent.alive -or $absent.path_matches -or $absent.start_matches) { throw 'Missing process accepted' }
    $firstBin = Join-Path $fixtureRoot 'first node'
    $secondBin = Join-Path $fixtureRoot 'second node'
    New-Item -ItemType Directory -Path $firstBin, $secondBin | Out-Null
    $firstNode = Join-Path $firstBin 'node.exe'
    $secondNode = Join-Path $secondBin 'node.exe'
    # Real application files make command discovery exercise PATH ordering.
    Copy-Item -LiteralPath (Join-Path $PSHOME 'pwsh.exe') -Destination $firstNode
    Copy-Item -LiteralPath (Join-Path $PSHOME 'pwsh.exe') -Destination $secondNode
    $env:PATH = $firstBin + [IO.Path]::PathSeparator + $secondBin
    $nodeCandidates = @(Get-Command node -CommandType Application -All -ErrorAction Stop)
    if ($nodeCandidates.Count -ne 2) { throw 'Expected two fixture Node applications' }
    $resolved = Get-NodeApplication
    if ($resolved -isnot [string] -or $resolved -cne $firstNode) { throw 'Node resolver did not preserve the first PATH application' }
    $env:PATH = $secondBin + [IO.Path]::PathSeparator + $firstBin
    if ((Get-NodeApplication) -cne $secondNode) { throw 'Node resolver ignored reordered PATH' }
    $env:PATH = $fixtureRoot
    try {
        $null = Get-NodeApplication
        throw 'Expected missing Node resolution to fail'
    } catch [System.Management.Automation.CommandNotFoundException] { }
    Write-Host 'PASS: PowerShell parse, persistent-runner rejection, PE/filename fixtures, owned-process evidence, and Node multi-PATH ordering/missing-command checks'
} finally {
    $env:RUNNER_ENVIRONMENT = $originalRunnerEnvironment
    $env:PATH = $originalPath
    Remove-Item -LiteralPath $fixtureRoot -Recurse -Force
}
