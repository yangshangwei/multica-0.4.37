#Requires -Version 7.2
[CmdletBinding()]
param(
    [Parameter(Mandatory)][string]$ArtifactRoot,
    [Parameter(Mandatory)][string]$ExpectedVersion,
    [Parameter(Mandatory)][string]$ReportPath,
    [ValidateSet('x64', 'ia32')][string]$ExpectedArch = 'x64',
    [ValidateRange(1, 600)][int]$InstallerTimeoutSeconds = 300,
    [ValidateRange(1, 60)][int]$CliTimeoutSeconds = 30
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

function Wait-SmokeProcess {
    param([System.Diagnostics.Process]$Process, [int]$TimeoutSeconds, [string]$Label)
    if (-not $Process.WaitForExit($TimeoutSeconds * 1000)) {
        try { $Process.Kill($true) } catch { }
        throw "$Label timed out after $TimeoutSeconds seconds"
    }
    $Process.WaitForExit()
    $Process.Refresh()
}

function Get-PeMachine {
    param([string]$Path)
    $reader = [System.IO.BinaryReader]::new([System.IO.File]::OpenRead($Path))
    try {
        if ($reader.ReadUInt16() -ne 0x5A4D) { throw "Missing DOS header: $Path" }
        $reader.BaseStream.Position = 0x3C
        $offset = $reader.ReadUInt32()
        if ($offset -gt $reader.BaseStream.Length - 6) { throw "Invalid PE offset: $Path" }
        $reader.BaseStream.Position = $offset
        if ($reader.ReadUInt32() -ne 0x00004550) { throw "Missing PE signature: $Path" }
        return $reader.ReadUInt16()
    } finally {
        $reader.Dispose()
    }
}

$ReportPath = [System.IO.Path]::GetFullPath($ReportPath)
$ExpectedVersion = $ExpectedVersion.Trim() -replace '^v', ''
$expectedPeMachine = if ($ExpectedArch -eq 'ia32') { 0x014c } else { 0x8664 }
$expectedGoArch = if ($ExpectedArch -eq 'ia32') { '386' } else { 'amd64' }
$peMachineHex = '0x{0:x}' -f $expectedPeMachine
$verification = [ordered]@{
    schema_version = 1
    scope = "native_windows_${ExpectedArch}_installer_and_bundled_cli"
    status = 'failed'
    expected_version = $ExpectedVersion
    expected_arch = $ExpectedArch
    source_commit = $env:GITHUB_SHA
    source_ref = $env:GITHUB_REF
    started_at = [DateTime]::UtcNow.ToString('o')
    finished_at = $null
    gui_tested = $false
    report_model_tested = $false
    installer = $null
    desktop = $null
    cli = $null
    error = $null
}

try {
    if (-not $IsWindows -or [System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture.ToString() -ne 'X64') {
        throw 'This smoke test requires a native Windows x64 runner'
    }
    if ([string]::IsNullOrWhiteSpace($env:RUNNER_TEMP)) { throw 'RUNNER_TEMP is required for an isolated installation' }
    $pattern = [regex]::new('^multica-desktop-(?<version>\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?(?:\+[0-9A-Za-z.-]+)?)-windows-' + [regex]::Escape($ExpectedArch) + '\.exe$', 'IgnoreCase')
    $installers = @(Get-ChildItem -LiteralPath $ArtifactRoot -Recurse -File -Filter "*-windows-$ExpectedArch.exe" |
        Where-Object { $pattern.IsMatch($_.Name) })
    if ($installers.Count -ne 1) { throw "Expected exactly one versioned Windows $ExpectedArch installer; found $($installers.Count)" }
    $installer = $installers[0]
    $installerVersion = $pattern.Match($installer.Name).Groups['version'].Value
    $verification.installer = [ordered]@{
        path = $installer.FullName
        version = $installerVersion
        sha256 = (Get-FileHash -LiteralPath $installer.FullName -Algorithm SHA256).Hash.ToLowerInvariant()
        bytes = $installer.Length
        exit_code = $null
    }
    if ($installerVersion -cne $ExpectedVersion) { throw 'Installer filename version does not match the packaging version' }
    if ($env:GITHUB_REF_TYPE -eq 'tag' -and ($env:GITHUB_REF_NAME -replace '^v', '') -cne $ExpectedVersion) {
        throw 'Installer version does not match the workflow tag'
    }

    $smokeRoot = Join-Path $env:RUNNER_TEMP ('multica-installer-smoke-' + [guid]::NewGuid().ToString('N'))
    $installDirectory = Join-Path $smokeRoot 'installed'
    New-Item -ItemType Directory -Path $smokeRoot | Out-Null
    # NSIS /S suppresses app launch without --force-run. Its /D override must
    # be the final, unquoted argument, including when RUNNER_TEMP has spaces.
    $installerProcess = Start-Process -FilePath $installer.FullName -ArgumentList "/S /D=$installDirectory" -PassThru
    Wait-SmokeProcess -Process $installerProcess -TimeoutSeconds $InstallerTimeoutSeconds -Label 'Installer'
    $verification.installer.exit_code = $installerProcess.ExitCode
    if ($installerProcess.ExitCode -ne 0) { throw "Installer exited with code $($installerProcess.ExitCode)" }

    $desktopPath = Join-Path $installDirectory 'Multica.exe'
    $cliPath = Join-Path $installDirectory 'resources/app.asar.unpacked/resources/bin/multica.exe'
    foreach ($path in @($desktopPath, $cliPath)) {
        if (-not (Test-Path -LiteralPath $path -PathType Leaf)) { throw "Installed binary is missing: $path" }
        if ((Get-PeMachine -Path $path) -ne $expectedPeMachine) { throw "Installed binary is not ${ExpectedArch}: $path" }
    }
    $productVersion = [System.Diagnostics.FileVersionInfo]::GetVersionInfo($desktopPath).ProductVersion
    $numericVersion = [regex]::Match($ExpectedVersion, '^\d+\.\d+\.\d+').Value
    if ($productVersion -notmatch ('^' + [regex]::Escape($numericVersion) + '(?:\.\d+)?$')) {
        throw 'Installed desktop product version does not match the installer version'
    }
    $verification.desktop = [ordered]@{ path = $desktopPath; pe_machine = $peMachineHex; product_version = $productVersion }

    # Execute only the CLI inside this installation. Never launch Multica.exe,
    # look up a CLI on PATH, or start a daemon, agent, or desktop service.
    $stdoutPath = Join-Path $smokeRoot 'cli-version.stdout.json'
    $stderrPath = Join-Path $smokeRoot 'cli-version.stderr.txt'
    $verification.cli = [ordered]@{ path = $cliPath; pe_machine = $peMachineHex; arguments = @('version', '--output', 'json'); exit_code = $null; stdout = $null; stderr = $null; reported = $null }
    $cliProcess = Start-Process -FilePath $cliPath -ArgumentList 'version --output json' -WorkingDirectory $smokeRoot -PassThru -NoNewWindow -RedirectStandardOutput $stdoutPath -RedirectStandardError $stderrPath
    Wait-SmokeProcess -Process $cliProcess -TimeoutSeconds $CliTimeoutSeconds -Label 'Bundled CLI version'
    $verification.cli.exit_code = $cliProcess.ExitCode
    $verification.cli.stdout = [System.IO.File]::ReadAllText($stdoutPath)
    $verification.cli.stderr = [System.IO.File]::ReadAllText($stderrPath)
    if ($cliProcess.ExitCode -ne 0) { throw "Bundled CLI version exited with code $($cliProcess.ExitCode)" }
    $cliVersion = $verification.cli.stdout | ConvertFrom-Json -AsHashtable
    $verification.cli.reported = $cliVersion
    if ($cliVersion['os'] -cne 'windows' -or $cliVersion['arch'] -cne $expectedGoArch) { throw 'Bundled CLI reports a different OS or architecture' }
    if (($cliVersion['version'] -replace '^v', '') -cne $ExpectedVersion) { throw 'Bundled CLI version does not match the installer version' }
    $verification.status = 'passed'
    Write-Host "Windows $ExpectedArch installer and bundled CLI verified: $ExpectedVersion"
} catch {
    $verification.error = $_.Exception.Message
    throw
} finally {
    $verification.finished_at = [DateTime]::UtcNow.ToString('o')
    [System.IO.Directory]::CreateDirectory([System.IO.Path]::GetDirectoryName($ReportPath)) | Out-Null
    [System.IO.File]::WriteAllText($ReportPath, ($verification | ConvertTo-Json -Depth 8), [System.Text.UTF8Encoding]::new($false))
    Write-Host "Verification report: $ReportPath"
}
