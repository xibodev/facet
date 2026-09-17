#requires -Version 7.0
<#
.SYNOPSIS
Install a published Facet Windows release without a Go toolchain or checkout.
.EXAMPLE
pwsh -File ./install-release.ps1
.EXAMPLE
pwsh -File ./install-release.ps1 -Mode standalone -NonInteractive -NoPath -NoShortcuts
#>
[CmdletBinding()]
param(
    [ValidateSet('standalone', 'cli', 'module')][string]$Mode,
    [string]$Version = '1.0.2',
    [string]$InstallDir = '',
    [ValidateSet('claude', 'copilot', 'codex', 'opencode')][string]$Target,
    [string]$ProjectDir = '',
    [string]$StudioExecutable = '',
    [switch]$NonInteractive,
    [switch]$NoPath,
    [switch]$NoShortcuts,
    [switch]$SetupRenderer,
    [switch]$Launch,
    [string]$ArchivePath = '',
    [string]$ChecksumPath = ''
)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
if (-not $IsWindows) { throw 'This release installer supports Windows. No Linux/macOS binary is published in this release.' }
if ([Runtime.InteropServices.RuntimeInformation]::OSArchitecture -ne 'X64') { throw 'This release contains Windows x64 binaries only.' }
if ($Version -notmatch '^\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?$') { throw 'Version must be an explicit release version, for example 1.0.2.' }
if ($NonInteractive -and -not $Mode) { throw '-NonInteractive requires -Mode standalone, cli, or module.' }
if ([bool]$ArchivePath -ne [bool]$ChecksumPath) { throw 'Offline installation requires both -ArchivePath and -ChecksumPath.' }

function Confirm-Choice([string]$Prompt, [bool]$Default = $true) {
    $suffix = if ($Default) { '[Y/n]' } else { '[y/N]' }
    $answer = Read-Host "$Prompt $suffix"
    if (-not $answer.Trim()) { return $Default }
    return $answer -match '^(?i)y(es)?$'
}

function Require-Command([string]$Name) {
    $commands = @(Get-Command $Name -CommandType Application -ErrorAction SilentlyContinue)
    if (-not $commands.Count) { throw "Required command '$Name' is not installed or not on PATH." }
    return $commands[0].Source
}

function Invoke-Checked([string]$Program, [string[]]$Arguments) {
    & $Program @Arguments
    if ($LASTEXITCODE -ne 0) { throw "Command failed ($LASTEXITCODE): $Program" }
}

Write-Host "`nFacet $Version — Release setup" -ForegroundColor Cyan
if (-not $Mode) {
    Write-Host '  1. Standalone Facet — video workbench with embedded agent runtime'
    Write-Host '  2. CLI integration — add Facet to a project for your agent CLI'
    Write-Host '  3. Studio module — register Facet with an installed Studio host'
    switch (Read-Host 'Choose [1/2/3] (default 1)') {
        '' { $Mode = 'standalone' }
        '1' { $Mode = 'standalone' }
        '2' { $Mode = 'cli' }
        '3' { $Mode = 'module' }
        default { throw 'Choose 1, 2, or 3.' }
    }
}
if (-not $InstallDir) {
    $InstallDir = Join-Path $HOME ".facet/releases/$Version"
    if (-not $NonInteractive) {
        $choice = Read-Host "Install location (Enter for $InstallDir)"
        if ($choice.Trim()) { $InstallDir = $choice.Trim() }
    }
}
$InstallDir = [IO.Path]::GetFullPath($InstallDir).TrimEnd('\', '/')
if ($InstallDir -eq [IO.Path]::GetPathRoot($InstallDir).TrimEnd('\', '/')) { throw 'Choose a dedicated installation directory, not a drive root.' }
if ($Mode -eq 'cli') {
    if (-not $Target -and -not $NonInteractive) { $Target = Read-Host 'Agent CLI: claude, copilot, codex, or opencode' }
    if ($Target -notin @('claude', 'copilot', 'codex', 'opencode')) { throw 'CLI mode requires a supported -Target.' }
    if (-not $ProjectDir -and -not $NonInteractive) { $ProjectDir = Read-Host 'Project folder to integrate' }
    if (-not $ProjectDir) { throw 'CLI mode requires -ProjectDir.' }
    $ProjectDir = [IO.Path]::GetFullPath($ProjectDir)
    foreach ($file in @('.facet.yaml', 'facet.lock.json')) {
        if (Test-Path -LiteralPath (Join-Path $ProjectDir $file)) {
            throw "Existing Facet project detected ($file). Its configuration is preserved. Use the installed facet init command explicitly to refresh that project."
        }
    }
}
if ($Mode -eq 'module') {
    if (-not $StudioExecutable) {
        if (-not $NonInteractive) { $StudioExecutable = Read-Host 'Studio host executable (Enter to find facet-studio-kernel on PATH)' }
        if (-not $StudioExecutable) { $StudioExecutable = Require-Command 'facet-studio-kernel' }
    }
    $StudioExecutable = @(Get-Command $StudioExecutable -CommandType Application -ErrorAction Stop)[0].Source
    # Ask the actual host, rather than guessing which executable owns modules.
    Invoke-Checked $StudioExecutable @('modules-add', '--help')
}
if (-not $NonInteractive) {
    if (-not $SetupRenderer) { $SetupRenderer = Confirm-Choice 'Install locked Remotion dependencies and its headless browser? (requires Node/npm and network)' }
    if (-not $NoPath) { $NoPath = -not (Confirm-Choice 'Add this release to your user PATH?') }
    if ($Mode -eq 'standalone' -and -not $NoShortcuts) { $NoShortcuts = -not (Confirm-Choice 'Create a Start Menu shortcut?') }
    if (-not $Launch -and $Mode -eq 'standalone') { $Launch = Confirm-Choice 'Open Facet after setup?' }
    Write-Host "`nMode: $Mode`nInstall: $InstallDir"
    if (-not (Confirm-Choice 'Continue?')) { return }
}
$npm = ''; $node = ''
if ($SetupRenderer) {
    $node = Require-Command 'node'
    $npm = Require-Command 'npm.cmd'
}

$archiveName = "facet-$Version-windows-amd64.zip"
$temp = Join-Path ([IO.Path]::GetTempPath()) ('facet-release-' + [guid]::NewGuid().ToString('N'))
$stage = ''
New-Item -ItemType Directory -Path $temp | Out-Null
try {
    if (-not $ArchivePath) {
        $base = "https://github.com/xibodev/facet/releases/download/v$Version"
        $ArchivePath = Join-Path $temp $archiveName
        $ChecksumPath = Join-Path $temp 'SHA256SUMS.txt'
        Write-Host "Downloading v$Version from GitHub…"
        Invoke-WebRequest "$base/SHA256SUMS.txt" -OutFile $ChecksumPath
        Invoke-WebRequest "$base/$archiveName" -OutFile $ArchivePath
    }
    $ArchivePath = (Resolve-Path -LiteralPath $ArchivePath).Path
    $ChecksumPath = (Resolve-Path -LiteralPath $ChecksumPath).Path
    $expected = @()
    foreach ($line in [IO.File]::ReadAllLines($ChecksumPath)) {
        if ($line -match '^([a-fA-F0-9]{64})\s+\*?(.+)$' -and $Matches[2] -eq $archiveName) { $expected += $Matches[1].ToLowerInvariant() }
    }
    if ($expected.Count -ne 1) { throw "Checksum file must contain exactly one entry for $archiveName." }
    $actual = (Get-FileHash -LiteralPath $ArchivePath -Algorithm SHA256).Hash.ToLowerInvariant()
    if ($actual -ne $expected[0]) { throw 'Archive checksum mismatch. Nothing has been installed.' }
    Write-Host 'Download checksum verified.' -ForegroundColor Green

    $existing = Test-Path -LiteralPath $InstallDir
    if ($existing) {
        $receiptPath = Join-Path $InstallDir 'facet-install.json'
        if (-not (Test-Path -LiteralPath $receiptPath)) { throw 'Install directory exists and is not managed by this installer. Choose a fresh directory.' }
        $receipt = [IO.File]::ReadAllText($receiptPath) | ConvertFrom-Json
        if ($receipt.version -ne $Version -or $receipt.archive_sha256 -ne $actual) { throw 'Existing installation is a different release. Choose a version-specific directory.' }
        foreach ($file in $receipt.files) {
            if (-not [IO.Path]::IsPathFullyQualified([string]$file.path) -and [string]$file.path -notmatch '(^|[/\\])\.\.([/\\]|$)') {
                $installed = Join-Path $InstallDir $file.path
                if (-not (Test-Path -LiteralPath $installed -PathType Leaf) -or (Get-FileHash -LiteralPath $installed -Algorithm SHA256).Hash.ToLowerInvariant() -ne $file.sha256) { throw "Installed file changed or is missing: $($file.path). Choose a fresh directory to repair." }
            } else { throw 'Invalid installation receipt.' }
        }
        Write-Host "Verified existing v$Version installation."
    } else {
        $parent = Split-Path -Parent $InstallDir
        New-Item -ItemType Directory -Path $parent -Force | Out-Null
        $stage = Join-Path $parent ('.facet-stage-' + [guid]::NewGuid().ToString('N'))
        New-Item -ItemType Directory -Path $stage | Out-Null
        $zip = [IO.Compression.ZipFile]::OpenRead($ArchivePath)
        try {
            $seen = [Collections.Generic.HashSet[string]]::new([StringComparer]::OrdinalIgnoreCase)
            $total = [long]0
            foreach ($entry in $zip.Entries) {
                # tar archives use ./ prefixes; remove only that exact prefix.
                $name = $entry.FullName.Replace('\', '/')
                while ($name.StartsWith('./')) { $name = $name.Substring(2) }
                if (-not $name) { continue }
                if ($name.StartsWith('/') -or $name.Contains(':') -or $name -match '(^|/)\.\.(/|$)') { throw "Unsafe archive path: $name" }
                if (-not $seen.Add($name.TrimEnd('/'))) { throw "Duplicate archive entry: $name" }
                $unixType = ($entry.ExternalAttributes -shr 16) -band 0xF000
                if ($unixType -eq 0xA000) { throw 'Archive symlinks are not supported.' }
                $total += $entry.Length
                if ($total -gt 1GB) { throw 'Archive exceeds the extraction limit.' }
                $destination = [IO.Path]::GetFullPath((Join-Path $stage $name))
                if (-not $destination.StartsWith($stage + [IO.Path]::DirectorySeparatorChar, [StringComparison]::OrdinalIgnoreCase)) { throw 'Archive entry leaves installation directory.' }
                if ($name.EndsWith('/')) { New-Item -ItemType Directory -Path $destination -Force | Out-Null; continue }
                New-Item -ItemType Directory -Path (Split-Path -Parent $destination) -Force | Out-Null
                [IO.Compression.ZipFileExtensions]::ExtractToFile($entry, $destination, $false)
            }
        } finally { $zip.Dispose() }
        foreach ($required in @('bin/facet.exe', 'bin/facet-ui.exe', 'bin/facet-module.exe', 'bundle/skills/facet/SKILL.md', 'bundle/remotion-composer/package-lock.json')) {
            if (-not (Test-Path -LiteralPath (Join-Path $stage $required) -PathType Leaf)) { throw "Release is incomplete: $required" }
        }
        $reported = & (Join-Path $stage 'bin/facet.exe') version
        if ($LASTEXITCODE -ne 0 -or "$reported".Trim() -ne "facet v$Version") { throw 'Binary version does not match the selected release.' }
        $files = @(Get-ChildItem -LiteralPath $stage -Recurse -File | ForEach-Object {
            @{ path = [IO.Path]::GetRelativePath($stage, $_.FullName); sha256 = (Get-FileHash -LiteralPath $_.FullName -Algorithm SHA256).Hash.ToLowerInvariant() }
        })
        @{ version = $Version; archive_sha256 = $actual; files = $files } | ConvertTo-Json -Depth 5 | Set-Content -LiteralPath (Join-Path $stage 'facet-install.json') -Encoding utf8
        Move-Item -LiteralPath $stage -Destination $InstallDir
        $stage = ''
    }

    $bin = Join-Path $InstallDir 'bin'
    $facet = Join-Path $bin 'facet.exe'
    if ($SetupRenderer) {
        $composer = Join-Path $InstallDir 'bundle/remotion-composer'
        Invoke-Checked $npm @('ci', '--prefix', $composer, '--no-audit', '--no-fund')
        Push-Location $composer
        try { Invoke-Checked $node @((Join-Path $composer 'node_modules/@remotion/cli/remotion-cli.js'), 'browser', 'ensure') }
        finally { Pop-Location }
    }
    if ($Mode -eq 'cli') {
        Push-Location (Join-Path $InstallDir 'bundle')
        try { Invoke-Checked $facet @('init', $ProjectDir, '--engine', $Target, '--no-launch') }
        finally { Pop-Location }
        $root = switch ($Target) { 'claude' { '.claude' } 'copilot' { '.github' } 'codex' { '.codex' } 'opencode' { '.opencode' } }
        if (-not (Test-Path -LiteralPath (Join-Path $ProjectDir "$root/skills/facet/SKILL.md"))) { throw 'CLI integration did not create its core skill.' }
        Write-Host "Facet guidance is installed for $Target in $ProjectDir. This is project integration, not marketplace publication."
    }
    if ($Mode -eq 'module') { Invoke-Checked $StudioExecutable @('modules-add', (Join-Path $bin 'facet-module.exe')) }
    if (-not $NoPath) {
        $userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
        if ($bin -notin ($userPath -split ';')) { [Environment]::SetEnvironmentVariable('Path', "$bin;$userPath", 'User') }
        if ($bin -notin ($env:PATH -split ';')) { $env:PATH = "$bin;$env:PATH" }
    }
    if ($Mode -eq 'standalone' -and -not $NoShortcuts) {
        $programs = [Environment]::GetFolderPath('Programs')
        $linkPath = Join-Path $programs "Facet $Version.lnk"
        if (-not (Test-Path -LiteralPath $linkPath)) {
            $shell = New-Object -ComObject WScript.Shell
            $shortcut = $shell.CreateShortcut($linkPath)
            $shortcut.TargetPath = Join-Path $bin 'facet-ui.exe'
            $shortcut.WorkingDirectory = $InstallDir
            $shortcut.Description = 'Facet video production'
            $shortcut.Save()
        } else { Write-Host "Preserved existing shortcut: $linkPath" }
    }
    Write-Host "`nFacet $Version installed at $InstallDir" -ForegroundColor Green
    foreach ($program in @('ffmpeg', 'ffprobe', 'node')) {
        if (-not (Get-Command $program -ErrorAction SilentlyContinue)) { Write-Warning "$program is not on PATH. Install it before using the media operations that require it." }
    }
    if (-not $SetupRenderer) { Write-Host 'Remotion setup deferred. Rerun with -SetupRenderer when Node/npm are available.' }
    Write-Host "Start Facet: & '$facet' ui"
    Write-Host 'In Facet Settings, discover/test/select models. Model discovery is provided by the embedded kernel; free availability depends on upstream services.'
    if ($NoPath) { Write-Host "PATH unchanged. Use the absolute executable path above, or add '$bin' to PATH for CLI integrations." }
    if ($Launch -and $Mode -eq 'standalone') { Start-Process -FilePath (Join-Path $bin 'facet-ui.exe') -WorkingDirectory $InstallDir }
} finally {
    if ($stage -and (Test-Path -LiteralPath $stage)) { Remove-Item -LiteralPath $stage -Recurse -Force }
    Remove-Item -LiteralPath $temp -Recurse -Force
}
