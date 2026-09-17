#requires -Version 7.0
<#
.SYNOPSIS
Download and run the prebuilt agent-CLI installer.
#>
[CmdletBinding()]
param(
    [string]$Version = '',
    [ValidateSet('opencode','codex','claude','copilot')][string]$Target,
    [string]$ProjectDir,
    [string]$InstallDir,
    [string]$Components = 'remotion',
    [switch]$NonInteractive,
    [switch]$SkipVerify
)
$ErrorActionPreference = 'Stop'
if (-not $IsWindows) { throw 'Download the native facet-install executable for your OS from the release.' }
if ($NonInteractive -and (-not $Target -or -not $ProjectDir)) { throw '-NonInteractive requires -Target and -ProjectDir.' }
if (-not $Version) {
    $release = Invoke-RestMethod 'https://api.github.com/repos/xibodev/facet/releases/latest'
    $Version = $release.tag_name -replace '^v', ''
}
if ($Version -notmatch '^\d+\.\d+\.\d+(?:-[A-Za-z0-9.-]+)?$') { throw 'Invalid release version.' }
$arch = switch ([Runtime.InteropServices.RuntimeInformation]::OSArchitecture.ToString()) {
    'X64' { 'amd64' }
    'Arm64' { 'arm64' }
    default { throw 'Supported architectures: x64 and ARM64.' }
}
$name = "facet-install-$Version-windows-$arch.exe"
$base = "https://github.com/xibodev/facet/releases/download/v$Version"
$temp = Join-Path ([IO.Path]::GetTempPath()) ('facet-bootstrap-' + [guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $temp | Out-Null
try {
    $exe = Join-Path $temp $name
    $sums = Join-Path $temp 'SHA256SUMS.txt'
    Invoke-WebRequest "$base/SHA256SUMS.txt" -OutFile $sums
    Invoke-WebRequest "$base/$name" -OutFile $exe
    $expected = @([IO.File]::ReadAllLines($sums) | ForEach-Object {
        if ($_ -match '^([a-fA-F0-9]{64})\s+\*?(.+)$' -and $Matches[2] -eq $name) { $Matches[1] }
    })
    if ($expected.Count -ne 1 -or (Get-FileHash -LiteralPath $exe -Algorithm SHA256).Hash -ne $expected[0]) { throw 'Installer checksum mismatch.' }
    $arguments = @('--version', $Version, '--components', $Components)
    if ($Target) { $arguments += @('--target', $Target) }
    if ($ProjectDir) { $arguments += @('--project', $ProjectDir) }
    if ($InstallDir) { $arguments += @('--install-dir', $InstallDir) }
    if ($NonInteractive) { $arguments += '--yes' }
    if ($SkipVerify) { $arguments += '--skip-verify' }
    & $exe @arguments
    if ($LASTEXITCODE -ne 0) { throw "Facet installer exited with code $LASTEXITCODE." }
} finally {
    Remove-Item -LiteralPath $temp -Recurse -Force
}
