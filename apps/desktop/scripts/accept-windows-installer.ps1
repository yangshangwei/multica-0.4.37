#Requires -Version 7.2
[CmdletBinding()]
param(
    [Parameter(Mandatory)][string]$InstallerPath,
    [Parameter(Mandatory)][ValidatePattern('^\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?(?:\+[0-9A-Za-z.-]+)?$')][string]$ExpectedVersion,
    [Parameter(Mandatory)][string]$ReportPath,
    [string]$PreviousInstallerPath,
    [ValidatePattern('^\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?(?:\+[0-9A-Za-z.-]+)?$')][string]$PreviousVersion,
    [switch]$RequireSigned,
    [switch]$BusinessAcceptance,
    [ValidateRange(1, 600)][int]$TimeoutSeconds = 300
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

# This deliberately excludes personal machines and persistent self-hosted runners.
# NSIS owns per-user registry/shortcuts even with a temporary /D destination.
function Assert-DisposableRunner {
    if (-not $IsWindows -or [System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture.ToString() -ne 'X64') {
        throw 'Native Windows x64 is required'
    }
    if ($env:GITHUB_ACTIONS -ne 'true' -or $env:RUNNER_ENVIRONMENT -ne 'github-hosted' -or $env:RUNNER_OS -ne 'Windows') {
        throw 'Only disposable GitHub-hosted Windows runners are supported'
    }
    if ([string]::IsNullOrWhiteSpace($env:RUNNER_TEMP) -or -not (Test-Path -LiteralPath $env:RUNNER_TEMP -PathType Container)) {
        throw 'An existing RUNNER_TEMP directory is required'
    }
    $directory = Get-Item -LiteralPath $env:RUNNER_TEMP
    while ($null -ne $directory) {
        if ($directory.Attributes -band [IO.FileAttributes]::ReparsePoint) { throw 'RUNNER_TEMP must not traverse a reparse point' }
        $directory = $directory.Parent
    }
}

function Get-MulticaRegistration {
    foreach ($hive in @([Microsoft.Win32.RegistryHive]::CurrentUser, [Microsoft.Win32.RegistryHive]::LocalMachine)) {
        foreach ($view in @([Microsoft.Win32.RegistryView]::Registry64, [Microsoft.Win32.RegistryView]::Registry32)) {
            $base = [Microsoft.Win32.RegistryKey]::OpenBaseKey($hive, $view)
            try {
                foreach ($parentPath in @('Software', 'Software\Microsoft\Windows\CurrentVersion\Uninstall')) {
                    $parent = $base.OpenSubKey($parentPath)
                    if ($null -eq $parent) { continue }
                    try {
                        foreach ($name in $parent.GetSubKeyNames()) {
                            $key = $parent.OpenSubKey($name)
                            if ($null -eq $key) { continue }
                            try {
                                $display = [string]$key.GetValue('DisplayName', '')
                                $location = [string]$key.GetValue('InstallLocation', '')
                                $uninstall = [string]$key.GetValue('UninstallString', '')
                                # electron-builder UUID v5 for appId ai.multica.desktop, including stale keys without display names.
                                if ($name -eq 'd8b75c36-d208-59aa-9acd-26838c159dc3' -or "$name $display $location $uninstall" -match '(?i)multica') {
                                    [pscustomobject]@{ hive = "$hive"; view = "$view"; key = "$parentPath\$name"; location = $location }
                                }
                            } finally { $key.Dispose() }
                        }
                    } finally { $parent.Dispose() }
                }
                $protocol = $base.OpenSubKey('Software\Classes\multica')
                if ($null -ne $protocol) {
                    $protocol.Dispose()
                    [pscustomobject]@{ hive = "$hive"; view = "$view"; key = 'Software\Classes\multica'; location = '' }
                }
            } finally { $base.Dispose() }
        }
    }
}

function Assert-NoExistingMultica {
    if (@(Get-Process | Where-Object { $_.ProcessName -match '(?i)multica' }).Count -gt 0) { throw 'An existing Multica process is running' }
    if (@(Get-MulticaRegistration).Count -gt 0) { throw 'Existing Multica registry/protocol state detected' }
    foreach ($root in @($env:APPDATA, $env:LOCALAPPDATA, $env:ProgramFiles, ${env:ProgramFiles(x86)}, $env:USERPROFILE)) {
        if ([string]::IsNullOrWhiteSpace($root)) { continue }
        foreach ($relative in @('Multica', '@multica', '@multicadesktop', '.multica', 'Programs\Multica')) {
            if (Test-Path -LiteralPath (Join-Path $root $relative)) { throw "Existing Multica installation/profile detected under $root" }
        }
    }
    foreach ($folder in @('DesktopDirectory', 'CommonDesktopDirectory', 'Programs', 'CommonPrograms')) {
        $root = [Environment]::GetFolderPath([Environment+SpecialFolder]::$folder)
        if ($root -and @(Get-ChildItem -LiteralPath $root -Filter '*Multica*' -ErrorAction Stop).Count -gt 0) {
            throw "Existing Multica shortcut detected in $root"
        }
    }
}

function Invoke-BoundedProcess {
    param([string]$Path, [string]$Arguments, [string]$Label, [int]$Seconds = $TimeoutSeconds)
    $process = Start-Process -FilePath $Path -ArgumentList $Arguments -PassThru
    if (-not $process.WaitForExit($Seconds * 1000)) {
        try { $process.Kill($true) } catch { }
        throw "$Label timed out after $Seconds seconds"
    }
    $process.WaitForExit()
    $process.Refresh()
    if ($process.ExitCode -ne 0) { throw "$Label exited with code $($process.ExitCode)" }
}

function Get-SignatureEvidence {
    param([string]$Path)
    $signature = Get-AuthenticodeSignature -LiteralPath $Path
    return [ordered]@{
        status = [string]$signature.Status
        subject = if ($signature.SignerCertificate) { $signature.SignerCertificate.Subject } else { $null }
        thumbprint = if ($signature.SignerCertificate) { $signature.SignerCertificate.Thumbprint } else { $null }
    }
}

function Get-InstallerEvidence {
    param([string]$Path, [string]$Version)
    $file = Get-Item -LiteralPath $Path
    if ($file.PSIsContainer -or $file.Name -cne "multica-desktop-$Version-windows-x64.exe") { throw 'Expected exact versioned Windows x64 installer filename' }
    $signature = Get-SignatureEvidence $file.FullName
    return [ordered]@{ path = $file.FullName; version = $Version; sha256 = (Get-FileHash -LiteralPath $file.FullName -Algorithm SHA256).Hash.ToLowerInvariant(); signature = $signature }
}

function Assert-X64Binary {
    param([string]$Path)
    $reader = [IO.BinaryReader]::new([IO.File]::OpenRead($Path))
    try {
        if ($reader.ReadUInt16() -ne 0x5a4d) { throw "Missing DOS header: $Path" }
        $reader.BaseStream.Position = 0x3c
        $offset = $reader.ReadUInt32()
        if ($offset -gt $reader.BaseStream.Length - 6) { throw "Invalid PE offset: $Path" }
        $reader.BaseStream.Position = $offset
        if ($reader.ReadUInt32() -ne 0x4550 -or $reader.ReadUInt16() -ne 0x8664) { throw "Not a Windows x64 PE binary: $Path" }
    } finally { $reader.Dispose() }
}

function Install-AndVerify {
    param($Artifact, [string]$Label)
    $step = [ordered]@{ name = $Label; status = 'failed'; version = $Artifact.version; desktop_signature = $null; cli = $null }
    $report.steps.Add($step)
    # /D must be last and unquoted. /S avoids launching the desktop application.
    Invoke-BoundedProcess $Artifact.path "/S /currentuser /D=$installDirectory" $Label
    $desktop = Join-Path $installDirectory 'Multica.exe'
    $cli = Join-Path $installDirectory 'resources/app.asar.unpacked/resources/bin/multica.exe'
    Assert-X64Binary $desktop
    Assert-X64Binary $cli
    $actual = [Diagnostics.FileVersionInfo]::GetVersionInfo($desktop).ProductVersion
    $numericVersion = [regex]::Match($Artifact.version, '^\d+\.\d+\.\d+').Value
    if ($actual -notmatch ('^' + [regex]::Escape($numericVersion) + '(?:\.\d+)?$')) { throw "Installed desktop version mismatch: $actual" }
    $step.desktop_signature = Get-SignatureEvidence $desktop
    if ($RequireSigned -and $step.desktop_signature.status -ne 'Valid') { throw 'Installed desktop Authenticode signature is not Valid' }
    $registrations = @(Get-MulticaRegistration)
    $locations = @($registrations | Where-Object { $_.location })
    if ($locations.Count -eq 0) { throw 'No installed application registration found' }
    foreach ($entry in $registrations) {
        if ($entry.hive -ne 'CurrentUser') { throw 'Installer created machine-wide registration' }
        if ($entry.location -and [IO.Path]::GetFullPath($entry.location).TrimEnd('\') -ine $installDirectory.TrimEnd('\')) { throw 'Installer escaped the isolated directory' }
    }
    # Only version is executed; never resolve an ambient CLI or start an agent.
    $stdout = Join-Path $testRoot "$Label.stdout.json"
    $stderr = Join-Path $testRoot "$Label.stderr.txt"
    $process = Start-Process -FilePath $cli -ArgumentList 'version --output json' -WorkingDirectory $testRoot -PassThru -NoNewWindow -RedirectStandardOutput $stdout -RedirectStandardError $stderr
    if (-not $process.WaitForExit(30000)) {
        try { $process.Kill($true) } catch { }
        throw 'Bundled CLI version timed out'
    }
    $process.WaitForExit()
    $process.Refresh()
    if ($process.ExitCode -ne 0) { throw "Bundled CLI version exited with code $($process.ExitCode)" }
    $step.cli = [IO.File]::ReadAllText($stdout) | ConvertFrom-Json -AsHashtable
    if ($step.cli['os'] -cne 'windows' -or $step.cli['arch'] -cne 'amd64' -or ($step.cli['version'] -replace '^v', '') -cne $Artifact.version) { throw 'Bundled CLI version/architecture mismatch' }
    if ($BusinessAcceptance -and $Label -in @('install_previous', 'upgrade_candidate')) {
        $phase = if ($Label -eq 'install_previous') { 'baseline' } else { 'verify' }
        if ([string]::IsNullOrWhiteSpace($env:MULTICA_BUSINESS_STATE_DIR)) { throw 'Business state directory missing' }
        & node (Join-Path $PSScriptRoot 'windows-business.mjs') --phase $phase --installed-directory $installDirectory --state-dir $env:MULTICA_BUSINESS_STATE_DIR --report "$env:RUNNER_TEMP/multica-business-$phase.json"
        if ($LASTEXITCODE -ne 0) { throw "Installed business phase failed: $phase" }
        $business = Get-Content -LiteralPath "$env:RUNNER_TEMP/multica-business-$phase.json" -Raw | ConvertFrom-Json
        if ($business.status -ne 'passed') { throw "Business report did not pass: $phase" }
        $step.business = $business.status
    }
    $step.status = 'passed'
}

function Uninstall-AndVerify {
    param([string]$Label, [string]$ExpectedOtherProtocol)
    $step = [ordered]@{ name = $Label; status = 'failed' }
    $report.steps.Add($step)
    $uninstaller = Join-Path $installDirectory 'Uninstall Multica.exe'
    if (-not (Test-Path -LiteralPath $uninstaller -PathType Leaf)) { throw 'Installed uninstaller is missing' }
    # _?= prevents the NSIS parent from exiting before its temporary child.
    Invoke-BoundedProcess $uninstaller "/S /currentuser _?=$installDirectory" $Label
    foreach ($path in @('Multica.exe', 'resources\app.asar', 'resources\app.asar.unpacked\resources\bin\multica.exe')) {
        if (Test-Path -LiteralPath (Join-Path $installDirectory $path)) { throw "Uninstall left application payload: $path" }
    }
    if ($ExpectedOtherProtocol) {
        $key = 'HKCU:\Software\Classes\multica'
        $actual = (Get-Item -LiteralPath "$key\shell\open\command").GetValue('')
        if ($actual -cne $ExpectedOtherProtocol) { throw 'Uninstall changed a protocol owned by another installation' }
        $step.other_protocol_preserved = $true
        # Remove only the exact fixture created immediately before this uninstall.
        Remove-Item -LiteralPath $key -Recurse
    }
    $remaining = @(Get-MulticaRegistration)
    if ($remaining.Count -gt 0) {
        $step.remaining_registration = $remaining
        $protocol = Get-Item 'HKCU:\Software\Classes\multica\shell\open\command' -ErrorAction SilentlyContinue
        if ($protocol) { $step.remaining_protocol_command = $protocol.GetValue('') }
        throw 'Uninstall left Multica registry/protocol state'
    }
    Assert-NoExistingMultica
    $step.status = 'passed'
}

$report = [ordered]@{
    schema_version = 1; scope = 'native_windows_x64_nsis_lifecycle'; status = 'failed'
    source_commit = $env:GITHUB_SHA; started_at = [DateTime]::UtcNow.ToString('o'); finished_at = $null
    require_signed = [bool]$RequireSigned; installer = $null; previous_installer = $null
    upgrade = if ($PreviousInstallerPath) { 'pending' } else { 'not_tested_no_previous_installer' }
    gui_tested = $false; managed_backend_tested = $false; update_feed_tested = $false; user_state_retention_tested = $false
    offline_certificate_trust_tested = $false; steps = [Collections.Generic.List[object]]::new(); error = $null
}
try {
    Assert-DisposableRunner
    Assert-NoExistingMultica
    if ($BusinessAcceptance -and -not $PreviousInstallerPath) { throw 'Business acceptance requires the feature-capable previous installer' }
    if ([bool]$PreviousInstallerPath -ne [bool]$PreviousVersion) { throw 'PreviousInstallerPath and PreviousVersion must be supplied together' }
    $report.installer = Get-InstallerEvidence $InstallerPath $ExpectedVersion
    if ($PreviousInstallerPath) {
        if ([version]($PreviousVersion -split '[-+]')[0] -ge [version]($ExpectedVersion -split '[-+]')[0]) { throw 'PreviousVersion must be older than ExpectedVersion' }
        $report.previous_installer = Get-InstallerEvidence $PreviousInstallerPath $PreviousVersion
    }
    foreach ($artifact in @($report.installer, $report.previous_installer)) {
        if ($null -ne $artifact -and $RequireSigned -and $artifact.signature.status -ne 'Valid') { throw 'Installer Authenticode signature is not Valid' }
    }
    $testRoot = Join-Path ([IO.Path]::GetFullPath($env:RUNNER_TEMP)) ('multica-acceptance-' + [guid]::NewGuid().ToString('N'))
    $installDirectory = Join-Path $testRoot 'installed'
    New-Item -ItemType Directory -Path $testRoot | Out-Null
    if ($report.previous_installer) {
        Install-AndVerify $report.previous_installer 'install_previous'
        Install-AndVerify $report.installer 'upgrade_candidate'
        $report.upgrade = 'passed'
        if ($BusinessAcceptance) {
            $report.gui_tested = $true
            $report.managed_backend_tested = $true
            $report.user_state_retention_tested = $true
        }
    } else {
        Install-AndVerify $report.installer 'install_candidate'
    }
    Uninstall-AndVerify 'uninstall'
    Install-AndVerify $report.installer 'reinstall_candidate'
    if ($BusinessAcceptance) {
        $key = 'HKCU:\Software\Classes\multica'
        if (Test-Path -LiteralPath $key) { throw 'Protocol fixture refuses existing registration' }
        $otherCommand = '"' + (Join-Path $testRoot 'other-handler.exe') + '" "%1"'
        New-Item -Path "$key\shell\open\command" -Force | Out-Null
        Set-Item -LiteralPath "$key\shell\open\command" -Value $otherCommand
        Uninstall-AndVerify 'final_uninstall' -ExpectedOtherProtocol $otherCommand
    } else { Uninstall-AndVerify 'final_uninstall' }
    $report.status = 'passed'
} catch {
    $report.error = $_.Exception.Message
    throw
} finally {
    # Preserve temporary files on failure; runner disposal performs cleanup.
    $report.finished_at = [DateTime]::UtcNow.ToString('o')
    $fullReportPath = [IO.Path]::GetFullPath($ReportPath)
    [IO.Directory]::CreateDirectory([IO.Path]::GetDirectoryName($fullReportPath)) | Out-Null
    [IO.File]::WriteAllText($fullReportPath, ($report | ConvertTo-Json -Depth 10), [Text.UTF8Encoding]::new($false))
    Write-Host "Windows lifecycle evidence: $fullReportPath"
}
