#Requires -Version 7.2
[CmdletBinding()]
param(
    [Parameter(Mandatory)][string]$PreviousInstallerPath,
    [Parameter(Mandatory)][string]$PreviousVersion,
    [Parameter(Mandatory)][string]$CandidateDirectory,
    [Parameter(Mandatory)][string]$ExpectedVersion,
    [Parameter(Mandatory)][string]$StateDirectory,
    [Parameter(Mandatory)][string]$ReportPath
)
Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
$TimeoutSeconds = 300
# Import only audited safety/process functions; never execute the lifecycle body.
$tokens = $null; $parseErrors = $null
$ast = [Management.Automation.Language.Parser]::ParseFile((Join-Path $PSScriptRoot 'accept-windows-installer.ps1'), [ref]$tokens, [ref]$parseErrors)
if ($parseErrors.Count) { throw 'Cannot parse lifecycle safety helpers' }
foreach ($name in @('Assert-DisposableRunner', 'Get-MulticaRegistration', 'Assert-NoExistingMultica', 'Invoke-BoundedProcess', 'Get-SignatureEvidence', 'Get-InstallerEvidence')) {
    $function = $ast.Find({ param($node) $node -is [Management.Automation.Language.FunctionDefinitionAst] -and $node.Name -eq $name }.GetNewClosure(), $false)
    if ($null -eq $function) { throw "Missing safety helper: $name" }
    . ([scriptblock]::Create($function.Extent.Text))
}
$report = [ordered]@{ status = 'failed'; scope = 'installed_windows_electron_updater'; source_commit = $env:GITHUB_SHA; error = $null; cleanup = 'not_started'; previous_installer = $null }
$ownedInstall = $false
try {
    Assert-DisposableRunner
    Assert-NoExistingMultica
    $state = [IO.Path]::GetFullPath($StateDirectory)
    $temporary = [IO.Path]::GetFullPath($env:RUNNER_TEMP).TrimEnd('\') + '\'
    if (-not $state.StartsWith($temporary, [StringComparison]::OrdinalIgnoreCase) -or (Test-Path -LiteralPath $state)) { throw 'StateDirectory must be a new child of RUNNER_TEMP' }
    $ancestor = Get-Item -LiteralPath ([IO.Path]::GetDirectoryName($state))
    while ($null -ne $ancestor) {
        if ($ancestor.Attributes -band [IO.FileAttributes]::ReparsePoint) { throw 'StateDirectory ancestors must not contain reparse points' }
        $ancestor = $ancestor.Parent
    }
    if ([version]($PreviousVersion -split '[-+]')[0] -ge [version]($ExpectedVersion -split '[-+]')[0]) { throw 'ExpectedVersion numeric version must be greater than baseline' }
    $report.previous_installer = Get-InstallerEvidence $PreviousInstallerPath $PreviousVersion
    New-Item -ItemType Directory -Path $state | Out-Null
    $install = Join-Path $state 'installed'
    $ownedInstall = $true
    Invoke-BoundedProcess $PreviousInstallerPath "/S /currentuser /D=$install" 'install_updater_baseline'
    foreach ($registration in @(Get-MulticaRegistration)) {
        if ($registration.hive -ne 'CurrentUser' -or ($registration.location -and [IO.Path]::GetFullPath($registration.location).TrimEnd('\') -ine $install.TrimEnd('\'))) { throw 'Baseline installer escaped per-user isolated installation' }
    }
    $env:MULTICA_UPDATE_OWNED_INSTALL = $install
    & node (Join-Path $PSScriptRoot 'windows-update-probe.mjs') --executable (Join-Path $install 'Multica.exe') --previous-version $PreviousVersion --candidate-directory $CandidateDirectory --expected-version $ExpectedVersion --state-directory $state --report (Join-Path $state 'probe.json')
    $probeExit = $LASTEXITCODE
    if (Test-Path -LiteralPath (Join-Path $state 'probe.json')) { $report.probe = Get-Content -Raw -LiteralPath (Join-Path $state 'probe.json') | ConvertFrom-Json -AsHashtable }
    if ($probeExit -ne 0) { throw "Installed updater probe exited with code $probeExit" }
    $report.status = 'passed'
} catch {
    $report.error = $_.Exception.Message
} finally {
    if ($ownedInstall) {
        try {
            Get-CimInstance Win32_Process | Where-Object { $_.ExecutablePath -and $_.ExecutablePath.StartsWith($install + '\', [StringComparison]::OrdinalIgnoreCase) } | ForEach-Object { Stop-Process -Id $_.ProcessId -Force -ErrorAction SilentlyContinue }
            $uninstaller = Join-Path $install 'Uninstall Multica.exe'
            if (-not (Test-Path -LiteralPath $uninstaller)) { throw 'Owned uninstaller missing' }
            Invoke-BoundedProcess $uninstaller "/S /currentuser _?=$install" 'uninstall_updater_candidate'
            if (Test-Path -LiteralPath (Join-Path $install 'Multica.exe')) { throw 'Uninstall retained desktop executable' }
            if (@(Get-MulticaRegistration).Count) { throw 'Uninstall retained registration' }
            $report.cleanup = 'passed'
        } catch { $report.cleanup = $_.Exception.Message; $report.status = 'failed' }
    }
    [IO.Directory]::CreateDirectory([IO.Path]::GetDirectoryName([IO.Path]::GetFullPath($ReportPath))) | Out-Null
    [IO.File]::WriteAllText([IO.Path]::GetFullPath($ReportPath), ($report | ConvertTo-Json -Depth 20), [Text.UTF8Encoding]::new($false))
}
if ($report.status -ne 'passed') { throw $report.error }
