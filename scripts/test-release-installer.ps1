#requires -Version 7.0
param([Parameter(Mandatory)][string]$ArchivePath, [Parameter(Mandatory)][string]$ChecksumPath)
$ErrorActionPreference = 'Stop'
$installer = Join-Path (Split-Path -Parent $PSScriptRoot) 'install-release.ps1'
$tokens = $null; $errors = $null
[void][Management.Automation.Language.Parser]::ParseFile($installer, [ref]$tokens, [ref]$errors)
if ($errors.Count) { throw ($errors | Out-String) }
$root = Join-Path ([IO.Path]::GetTempPath()) ('facet-installer-tests-' + [guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $root | Out-Null
$common = @{ArchivePath=[IO.Path]::GetFullPath($ArchivePath); ChecksumPath=[IO.Path]::GetFullPath($ChecksumPath); NonInteractive=$true; NoPath=$true; NoShortcuts=$true}
$beforePath = [Environment]::GetEnvironmentVariable('Path','User')
$results = [Collections.Generic.List[string]]::new()
function Expect-Failure([scriptblock]$Run,[string]$Message) {
    try { & $Run; throw 'TEST: unexpectedly succeeded' } catch {
        if ($_.Exception.Message -notlike "*$Message*") { throw }
    }
}
try {
    $install = Join-Path $root 'release with spaces'
    & $installer @common -Mode standalone -InstallDir $install
    if (-not (Test-Path "$install/bin/facet.exe")) { throw 'No executable installed.' }
    $hash = (Get-FileHash "$install/bin/facet.exe").Hash
    & $installer @common -Mode standalone -InstallDir $install
    if ((Get-FileHash "$install/bin/facet.exe").Hash -ne $hash) { throw 'Reinstall changed executable.' }
    $results.Add('real release: first install and verified reinstall into a path with spaces')
    $unmanaged = Join-Path $root 'unmanaged'; New-Item -ItemType Directory $unmanaged | Out-Null
    'user content' | Set-Content (Join-Path $unmanaged 'keep.txt')
    Expect-Failure { & $installer @common -Mode standalone -InstallDir $unmanaged } 'not managed'
    if ([IO.File]::ReadAllText((Join-Path $unmanaged 'keep.txt')).Trim() -ne 'user content') { throw 'Changed unmanaged data.' }
    $results.Add('unmanaged install directory preserved')
    $badChecksum = Join-Path $root 'bad-checksums.txt'
    ('0' * 64 + '  facet-1.0.2-windows-amd64.zip') | Set-Content $badChecksum
    $bad = $common.Clone(); $bad.ChecksumPath = $badChecksum
    $badDestination = Join-Path $root 'not-created'
    Expect-Failure { & $installer @bad -Mode standalone -InstallDir $badDestination } 'checksum mismatch'
    if (Test-Path $badDestination) { throw 'Checksum failure created installation.' }
    $results.Add('checksum mismatch refused before destination writes')
    $malicious = Join-Path $root 'unsafe.zip'
    $zip = [IO.Compression.ZipFile]::Open($malicious, [IO.Compression.ZipArchiveMode]::Create)
    try { $entry = $zip.CreateEntry('../outside.txt'); $writer = [IO.StreamWriter]::new($entry.Open()); $writer.Write('must not escape'); $writer.Dispose() } finally { $zip.Dispose() }
    $unsafeChecksums = Join-Path $root 'unsafe-checksums.txt'
    ((Get-FileHash $malicious -Algorithm SHA256).Hash + '  facet-1.0.2-windows-amd64.zip') | Set-Content $unsafeChecksums
    $unsafe = $common.Clone(); $unsafe.ArchivePath = $malicious; $unsafe.ChecksumPath = $unsafeChecksums
    Expect-Failure { & $installer @unsafe -Mode standalone -InstallDir (Join-Path $root 'unsafe-destination') } 'Unsafe archive path'
    if (Test-Path (Join-Path $root 'outside.txt')) { throw 'Archive escaped staging.' }
    $results.Add('checksummed traversal archive rejected and staging cleaned')
    foreach ($target in @('copilot','claude','codex','opencode')) {
        $project = Join-Path $root "project-$target"
        New-Item -ItemType Directory $project | Out-Null
        'User instructions, preserve exactly.' | Set-Content (Join-Path $project 'AGENTS.md')
        & $installer @common -Mode cli -InstallDir $install -Target $target -ProjectDir $project
        if ([IO.File]::ReadAllText((Join-Path $project 'AGENTS.md')).Trim() -ne 'User instructions, preserve exactly.') { throw 'User instructions overwritten.' }
        Expect-Failure { & $installer @common -Mode cli -InstallDir $install -Target $target -ProjectDir $project } 'Existing Facet project'
        $results.Add("$target project integration verified; existing instructions preserved")
    }
    $hostScript = Join-Path $root 'studio-host.cmd'
    '@echo off', 'echo %*>>"%FACET_INSTALLER_HOST_LOG%"', 'exit /b 0' | Set-Content $hostScript
    $env:FACET_INSTALLER_HOST_LOG = Join-Path $root 'host.log'
    & $installer @common -Mode module -InstallDir $install -StudioExecutable $hostScript
    $hostCalls = [IO.File]::ReadAllText($env:FACET_INSTALLER_HOST_LOG)
    if ($hostCalls -notmatch 'modules-add --help' -or $hostCalls -notmatch 'modules-add .*facet-module.exe') { throw 'Incorrect host registration commands.' }
    $results.Add('module mode delegates modules-add to supplied host (fixture, not real Studio acceptance)')
    # Exercise the actual prompts via a scoped Read-Host replacement. Answers
    # decline profile writes, renderer downloads and launch in this fixture.
    $answers = [Collections.Generic.Queue[string]]::new()
    foreach ($answer in @('1', 'n', 'n', 'n', 'n', 'y')) { $answers.Enqueue($answer) }
    function Read-Host { param([string]$Prompt); if ($answers.Count -eq 0) { throw "Unexpected prompt: $Prompt" }; return $answers.Dequeue() }
    & $installer -InstallDir (Join-Path $root 'interactive') -ArchivePath $common.ArchivePath -ChecksumPath $common.ChecksumPath
    if ($answers.Count -ne 0) { throw 'Interactive prompts were not all exercised.' }
    Remove-Item Function:Read-Host
    $results.Add('interactive standalone menu and opt-out prompts exercised')
    if ([Environment]::GetEnvironmentVariable('Path','User') -ne $beforePath) { throw 'Isolated tests modified user PATH.' }
    $results.Add('user PATH unchanged; no shortcuts requested')
    @{status='passed';root=$root;checks=$results} | ConvertTo-Json -Depth 4
} finally {
    Remove-Item Env:FACET_INSTALLER_HOST_LOG -ErrorAction SilentlyContinue
    # Retain disposable evidence for inspection; never touch real install/profile.
}
