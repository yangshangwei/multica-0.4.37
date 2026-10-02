#Requires -Version 7.2
$ErrorActionPreference = 'Stop'
$tokens = $null; $errors = $null
$ast = [Management.Automation.Language.Parser]::ParseFile((Join-Path $PSScriptRoot 'windows-update-acceptance.ps1'), [ref]$tokens, [ref]$errors)
if ($errors.Count) { throw ($errors | Out-String) }
$names = @($ast.ParamBlock.Parameters | ForEach-Object { $_.Name.VariablePath.UserPath })
foreach ($name in @('PreviousInstallerPath', 'PreviousVersion', 'CandidateDirectory', 'ExpectedVersion', 'StateDirectory', 'ReportPath')) {
    if ($name -notin $names) { throw "Missing updater acceptance input: $name" }
}
Write-Host 'Windows updater acceptance parser and interface passed'
