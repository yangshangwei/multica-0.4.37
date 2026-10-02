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
    Write-Host 'PASS: PowerShell parse, persistent-runner rejection, x64 PE and malformed PE fixtures, filename mismatch (7 checks)'
} finally {
    $env:RUNNER_ENVIRONMENT = $originalRunnerEnvironment
    Remove-Item -LiteralPath $fixtureRoot -Recurse -Force
}
