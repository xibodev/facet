#requires -Version 5.1
<#
.SYNOPSIS
Install the prebuilt Facet capability for an agentic CLI.
.DESCRIPTION
Run from the installer package (install.ps1 and installer/manifest.tsv).
Authentication is owned by the selected CLI. No product setup command is used.
#>
[CmdletBinding()]
param(
    [string]$Version = $env:FACET_VERSION,
    [ValidateSet('opencode','codex','claude','copilot','studio')][string]$Target,
    [string]$ProjectDir = $env:FACET_PROJECT,
    [string]$InstallDir = $env:FACET_INSTALL_DIR,
    [string]$Components = $env:FACET_COMPONENTS,
    [Alias('ProductionMethod')][string[]]$Pack,
    [ValidateSet('add','repair','update')][string]$Action = 'add',
    [switch]$MigrateLegacy,
    [switch]$Plain,
    [string]$ArchivePath,
    [string]$ChecksumPath,
    [switch]$NonInteractive,
    [switch]$SkipVerify
)
$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'
$PSNativeCommandUseErrorActionPreference = $false
$PSModuleAutoLoadingPreference = 'All'
if ($PSVersionTable.PSEdition -ne 'Core') {
    $env:PSModulePath = (Join-Path $env:SystemRoot 'System32/WindowsPowerShell/v1.0/Modules') + ';' + $env:PSModulePath
}
Import-Module Microsoft.PowerShell.Utility
Set-StrictMode -Version Latest
if ($env:OS -ne 'Windows_NT') { throw 'Use install.sh on Linux/macOS.' }
if (-not $PSCommandPath) { throw 'Run install.ps1 from the downloaded installer package, not a pipe.' }
Add-Type -AssemblyName System.IO.Compression.FileSystem
if (-not $Target -and $env:FACET_TARGET) { $Target = $env:FACET_TARGET }
if ($env:FACET_YES -eq '1') { $NonInteractive = $true }
if ($env:FACET_ACTION -and -not $PSBoundParameters.ContainsKey('Action')) { $Action = $env:FACET_ACTION }
if ($Action -notin @('add','repair','update')) { throw 'Action must be add, repair, or update.' }
$componentsExplicit = -not [string]::IsNullOrWhiteSpace($Components)
$packsExplicit = $PSBoundParameters.ContainsKey('Pack') -or -not [string]::IsNullOrWhiteSpace($env:FACET_PACKS)
if (-not $Pack -and $env:FACET_PACKS) { $Pack = @($env:FACET_PACKS.Split(',') | Where-Object { $_.Trim() } | ForEach-Object { $_.Trim() }) }
$logRoot = if ($env:FACET_LOG_DIR) { $env:FACET_LOG_DIR } else { Join-Path $HOME '.facet/logs' }
New-Item -ItemType Directory -Path $logRoot -Force | Out-Null
$logFile = Join-Path $logRoot ('install-' + (Get-Date -Format 'yyyyMMdd-HHmmss') + '-' + [guid]::NewGuid().ToString('N') + '.log')
$detail = [bool]$PSBoundParameters.ContainsKey('Verbose')
$rich = -not $Plain -and $env:FACET_PLAIN -ne '1' -and -not $env:NO_COLOR -and -not $NonInteractive
try { $rich = $rich -and -not [Console]::IsInputRedirected -and -not [Console]::IsOutputRedirected } catch { $rich=$false }
function Section([string]$Label) {
    Write-Host "`n$Label" -ForegroundColor Cyan
}
function Select-Choice([string]$Title,[string[]]$Labels,[string[]]$Values,[string[]]$Defaults,[switch]$Multiple) {
    if (-not $rich) {
        for ($i=0; $i -lt $Labels.Count; $i++) { Write-Host "  $($i+1). $($Labels[$i])" }
        return Ask $Title ($Defaults -join ',')
    }
    $index=0
    $checked=@{}
    for ($i=0; $i -lt $Values.Count; $i++) {
        if ($Values[$i] -in $Defaults) { $checked[$i]=$true; if (-not $Multiple) { $index=$i } }
    }
    Write-Host "`n? $Title" -ForegroundColor Cyan
    $top=[Console]::CursorTop
    $oldCursor=[Console]::CursorVisible
    try {
        [Console]::CursorVisible=$false
        while ($true) {
            [Console]::SetCursorPosition(0,$top)
            $width=[Math]::Max(20,[Console]::WindowWidth-2)
            for ($i=0; $i -lt $Labels.Count; $i++) {
                $pointer=if ($i -eq $index) {'>'} else {' '}
                $mark=if ($Multiple) {if ($checked.ContainsKey($i)) {'[x]'} else {'[ ]'}} else {if ($i -eq $index) {'(*)'} else {'( )'}}
                $line="  $pointer $mark $($Labels[$i])"
                if ($line.Length -gt $width) { $line=$line.Substring(0,$width-3)+'...' }
                Write-Host $line.PadRight($width) -ForegroundColor $(if ($i -eq $index) {'Green'} else {'Gray'})
            }
            $hint=if ($Multiple) {'  Up/Down: move | Space: toggle | Enter: confirm | Esc: cancel'} else {'  Up/Down: move | Enter: confirm | Esc: cancel'}
            Write-Host $hint.Substring(0,[Math]::Min($hint.Length,$width)).PadRight($width) -ForegroundColor DarkGray
            $key=[Console]::ReadKey($true)
            switch ($key.Key) {
                'UpArrow' { $index=($index+$Labels.Count-1)%$Labels.Count }
                'DownArrow' { $index=($index+1)%$Labels.Count }
                'Spacebar' { if ($Multiple) { if ($checked.ContainsKey($index)) {$checked.Remove($index)} else {$checked[$index]=$true} } }
                'Escape' { throw 'Installation cancelled.' }
                'Enter' {
                    $result=@()
                    for ($i=0; $i -lt $Values.Count; $i++) { if (($Multiple -and $checked.ContainsKey($i)) -or (-not $Multiple -and $i -eq $index)) { $result+=$Values[$i] } }
                    if (-not $result.Count) { $result=@('none') }
                    [Console]::SetCursorPosition(0,$top)
                    for ($i=0; $i -le $Labels.Count; $i++) { Write-Host (' '*$width) }
                    [Console]::SetCursorPosition(0,$top)
                    Write-Host "  Selected: $($result -join ', ')" -ForegroundColor Green
                    return ($result -join ',')
                }
            }
        }
    } finally { [Console]::CursorVisible=$oldCursor }
}
function Redact([string]$Text) {
    return ($Text -replace '(https?://)[^/\s@]+@','$1<redacted>@' -replace '([?&][^=\s&]+)=([^&\s]+)','$1=<redacted>')
}
function Step([string]$Label,[scriptblock]$Work) {
    Write-Host "  -> $Label"
    try {
        & $Work 2>&1 | ForEach-Object {
            $line = Redact "$_"
            Add-Content -LiteralPath $logFile -Value $line -Encoding UTF8
            if ($detail) { Write-Host $line }
        }
        Write-Host "  OK $Label"
    } catch {
        Add-Content -LiteralPath $logFile -Value (Redact $_.Exception.Message) -Encoding UTF8
        throw "$Label failed. Earlier project bindings remain unchanged. Details: $logFile"
    }
}
function Relative-Path([string]$Root,[string]$Path) { return $Path.Substring($Root.TrimEnd('\','/').Length + 1).Replace('\','/') }
$facetSectionStart = '<!-- facet:managed:start -->'
$facetSectionEnd = '<!-- facet:managed:end -->'
function Find-FacetSection([string]$Content,[string]$Path) {
    $startPattern = '(?m)^' + [regex]::Escape($facetSectionStart) + '\r?$'
    $endPattern = '(?m)^' + [regex]::Escape($facetSectionEnd) + '\r?$'
    $starts = [regex]::Matches($Content,$startPattern).Count
    $ends = [regex]::Matches($Content,$endPattern).Count
    if ($starts -eq 0 -and $ends -eq 0) { return $null }
    if ($starts -ne 1 -or $ends -ne 1) { throw "Malformed Facet section in $Path" }
    $pattern = '(?ms)^' + [regex]::Escape($facetSectionStart) + '\r?\n.*?^' + [regex]::Escape($facetSectionEnd) + '\r?$'
    $match = [regex]::Match($Content,$pattern)
    if (-not $match.Success) { throw "Malformed Facet section in $Path" }
    return $match
}
function Text-Hash([string]$Text) {
    $Text = $Text.Replace("`r`n","`n").TrimEnd("`r","`n")
    $sha = [Security.Cryptography.SHA256]::Create()
    try { return ([BitConverter]::ToString($sha.ComputeHash([Text.Encoding]::UTF8.GetBytes($Text)))).Replace('-','').ToLowerInvariant() }
    finally { $sha.Dispose() }
}
function Merge-FacetSection([string]$Path,[string]$Section,[bool]$Owned) {
    if (-not (Test-Path -LiteralPath $Path)) { return $Section + "`n" }
    $item = Get-Item -LiteralPath $Path -Force
    if (-not $item.PSIsContainer -and -not ($item.Attributes -band [IO.FileAttributes]::ReparsePoint)) {
        $content = [IO.File]::ReadAllText($Path)
        $match = Find-FacetSection $content $Path
        if (-not $match) {
            if ($content -and -not $content.EndsWith("`n")) { $content += "`n" }
            return $content + $Section + "`n"
        }
        if (-not $Owned) { throw "Preserving unmanaged Facet section in $Path" }
        return $content.Substring(0,$match.Index) + $Section + $content.Substring($match.Index + $match.Length)
    }
    throw "Preserving unsafe governing instruction: $Path"
}
function Download([string]$Url,[string]$Destination) {
    for ($attempt=1; $attempt -le 3; $attempt++) {
        try { Invoke-WebRequest -UseBasicParsing -Uri $Url -OutFile $Destination -TimeoutSec 180; return } catch {
            if ($attempt -eq 3) { throw }
            Write-Host "  Download interrupted; retrying ($attempt/3)..."
            Start-Sleep -Seconds ($attempt*2)
        }
    }
}
$manifest = @(Import-Csv -LiteralPath (Join-Path $PSScriptRoot 'installer/manifest.tsv') -Delimiter "`t")
function Definition([string]$Id) {
    $rows = @($manifest | Where-Object id -EQ $Id)
    if ($rows.Count -ne 1) { throw "Invalid manifest entry: $Id" }
    return $rows[0]
}
if ((Definition layout).version -ne '1') { throw 'Unsupported installer manifest schema.' }
function Ask([string]$Prompt,[string]$Default) {
    $answer = Read-Host "$Prompt [$Default]"
    if (-not $answer.Trim()) { return $Default }
    return $answer.Trim()
}
function Resolve-Program([string]$Name) {
    $found = @(Get-Command $Name -CommandType Application -ErrorAction SilentlyContinue)
    if (-not $found.Count) { throw "Required command not found: $Name" }
    return $found[0].Source
}
function Run([string]$Program,[string[]]$Arguments) {
    $resolved = Resolve-Program $Program
    # Windows PowerShell represents native stderr as ErrorRecords. Warnings from
    # npm/pip must not turn a successful exit into a failed installation.
    $ErrorActionPreference = 'Continue'
    & $resolved @Arguments
    if ($LASTEXITCODE -ne 0) { throw "Command failed ($LASTEXITCODE): $Program" }
}
function Confirm-Install([string]$Description) {
    Write-Host $Description
}
function Assert-RealAncestors([string]$Path) {
    while ($Path) {
        $item = Get-Item -LiteralPath $Path -Force -ErrorAction SilentlyContinue
        if ($item -and (($item.Attributes -band [IO.FileAttributes]::ReparsePoint) -or -not $item.PSIsContainer)) { throw "Unsafe directory ancestor: $Path" }
        $Path = Split-Path -Parent $Path
    }
}
function Assert-New([string]$Path) {
    if (Get-Item -LiteralPath $Path -Force -ErrorAction SilentlyContinue) { throw "Preserving existing entry: $Path" }
    Assert-RealAncestors (Split-Path -Parent $Path)
}
function Verify-Checksum([string]$Archive,[string]$Sums,[string]$Name) {
    $expected = @([IO.File]::ReadAllLines($Sums) | ForEach-Object {
        if ($_ -match '^([a-fA-F0-9]{64})\s+\*?(.+)$' -and $Matches[2] -ceq $Name) { $Matches[1] }
    })
    if ($expected.Count -ne 1 -or (Get-FileHash -LiteralPath $Archive -Algorithm SHA256).Hash -ne $expected[0]) { throw "Checksum mismatch: $Name" }
}
function Expand-SafeZip([string]$Archive,[string]$Destination) {
    $zip = [IO.Compression.ZipFile]::OpenRead($Archive)
    try {
        $seen = [Collections.Generic.HashSet[string]]::new([StringComparer]::OrdinalIgnoreCase)
        $total = [long]0
        foreach ($entry in $zip.Entries) {
            $name = $entry.FullName.Replace('\','/')
            while ($name.StartsWith('./')) { $name = $name.Substring(2) }
            if (-not $name) { continue }
            if ($name.StartsWith('/') -or $name.Contains(':') -or $name -match '(^|/)\.\.(/|$)|[\x00-\x1f]') { throw "Unsafe archive path: $name" }
            foreach ($part in $name.TrimEnd('/').Split('/')) {
                if (-not $part -or $part -eq '.' -or $part.EndsWith('.') -or $part.EndsWith(' ') -or $part -match '^(?i:CON|PRN|AUX|NUL|COM[1-9]|LPT[1-9])(?:\.|$)') { throw "Unsafe archive path: $name" }
            }
            if (-not $seen.Add($name.TrimEnd('/'))) { throw "Duplicate archive entry: $name" }
            $type = ($entry.ExternalAttributes -shr 16) -band 0xF000
            if ($type -notin @(0,0x8000,0x4000)) { throw 'Archive links and special files are unsupported.' }
            $total += $entry.Length
            if ($total -gt 1GB) { throw 'Archive exceeds extraction limit.' }
            $dest = [IO.Path]::GetFullPath((Join-Path $Destination $name))
            if (-not $dest.StartsWith($Destination + [IO.Path]::DirectorySeparatorChar,[StringComparison]::OrdinalIgnoreCase)) { throw 'Archive leaves destination.' }
        }
        foreach ($entry in $zip.Entries) {
            $name = $entry.FullName.Replace('\','/')
            while ($name.StartsWith('./')) { $name = $name.Substring(2) }
            if (-not $name) { continue }
            $dest = Join-Path $Destination $name
            if ($name.EndsWith('/')) { New-Item -ItemType Directory -Path $dest -Force | Out-Null }
            else {
                New-Item -ItemType Directory -Path (Split-Path -Parent $dest) -Force | Out-Null
                [IO.Compression.ZipFileExtensions]::ExtractToFile($entry,$dest,$false)
            }
        }
    } finally { $zip.Dispose() }
}
function Install-System([string]$Id) {
    $package = (Definition $Id).windows
    Confirm-Install "Install missing dependency with winget: $package"
    if (-not (Get-Command winget -ErrorAction SilentlyContinue)) { throw "winget is unavailable. Install $Id using its vendor installer, then rerun setup. Log: $logFile" }
    Step "Install $Id" { Run 'winget' @('install','--id',$package,'--exact','--source','winget','--accept-package-agreements','--accept-source-agreements') }
    $env:PATH = [Environment]::GetEnvironmentVariable('Path','Machine') + ';' + [Environment]::GetEnvironmentVariable('Path','User') + ';' + $env:PATH
}
if ($NonInteractive -and (-not $Target -or -not $ProjectDir)) { throw '-NonInteractive requires -Target and -ProjectDir.' }
if (-not $Version) { $Version = (Definition facet).version }
if ($Version -notmatch '^\d+\.\d+\.\d+(?:-[A-Za-z0-9.-]+)?$') { throw 'Invalid release version.' }
if ([bool]$ArchivePath -ne [bool]$ChecksumPath) { throw 'Supply both -ArchivePath and -ChecksumPath.' }
$arch = switch ([Runtime.InteropServices.RuntimeInformation]::OSArchitecture.ToString()) { 'X64' {'amd64'} 'Arm64' {'arm64'} default {throw 'Unsupported architecture.'} }
Write-Host "`n  FACET" -ForegroundColor Cyan
Write-Host '  Video production for your agent.' -ForegroundColor DarkGray
Write-Host "  Installer $Version | Windows $arch"
Section '[1/3] Choose your setup'
$detected = @($manifest | Where-Object kind -EQ 'host' | Where-Object { Get-Command $_.id -ErrorAction SilentlyContinue })
if (-not $Target) {
    $defaultHost = if ($detected.Count) { $detected[0].id } else { 'opencode' }
    Write-Host "  Detected CLIs: $(@($detected | ForEach-Object id) -join ', ')"
    $hostRows=@($manifest | Where-Object kind -EQ 'host')
    $labels=@($hostRows | ForEach-Object { "$($_.capability)" + $(if (Get-Command $_.id -ErrorAction SilentlyContinue) {' - detected'} else {' - not detected'}) })
    $Target = Select-Choice 'Which agent should use Facet?' $labels @($hostRows | ForEach-Object id) @($defaultHost)
    if ($Target -match '^[1-5]$') { $Target=$hostRows[[int]$Target-1].id }
}
$hostDef = Definition $Target
if ($hostDef.kind -ne 'host') { throw 'Unsupported CLI.' }
$instructionDef = Definition "$Target-instructions"
if ($instructionDef.kind -ne 'instruction' -or [IO.Path]::IsPathRooted($instructionDef.value) -or $instructionDef.value -match '(^|[/\\])\.\.([/\\]|$)') { throw 'Invalid governing instruction path.' }
$instructionRel = $instructionDef.value
if (-not $ProjectDir) { $ProjectDir = Ask 'Project directory' '.' }
$ProjectDir = [IO.Path]::GetFullPath($ProjectDir)
if (-not $Components) {
    $savedState = Join-Path $ProjectDir '.facet-install/installation.json'
    if (Test-Path $savedState) { $Components = (([IO.File]::ReadAllText($savedState) | ConvertFrom-Json).components -join ',') }
    if (-not $Components) { $Components='remotion' }
}
if (-not $packsExplicit) {
    $savedState = Join-Path $ProjectDir '.facet-install/installation.json'
    if (Test-Path $savedState) {
        $saved = [IO.File]::ReadAllText($savedState) | ConvertFrom-Json
        if ($saved.PSObject.Properties.Name -contains 'packs') { $Pack = @($saved.packs) }
    }
}
$selectedPacks = @($Pack | Where-Object { $_ } | ForEach-Object { $_.Split(',') } | ForEach-Object { $_.Trim().ToLowerInvariant() } | Where-Object { $_ })
if (@($selectedPacks | Select-Object -Unique).Count -ne $selectedPacks.Count) { throw 'Duplicate production method.' }
foreach ($id in $selectedPacks) {
    $def = Definition $id
    if ($def.kind -ne 'pack') { throw "Unknown production method: $id" }
}
if (-not $InstallDir) { $InstallDir = Join-Path $HOME ".facet/releases/$Version-windows-$arch" }
$InstallDir = [IO.Path]::GetFullPath($InstallDir)
if (($ProjectDir + $InstallDir) -match '[\x00-\x1f]') { throw 'Control characters are unsupported in installation paths.' }
$choices = @($manifest | Where-Object kind -EQ 'component')
Write-Host 'Core: FFmpeg and FFprobe. Optional downloads (approximate; platform/cache dependent):'
for ($i=0; $i -lt $choices.Count; $i++) {
    $id=$choices[$i].id
    $status = if ($id -in $Components.Split(',')) {'[x]'} else {'[ ]'}
    if ($choices[$i].windows -notin @('all',$arch)) { $status='[unavailable]' }
    Write-Host "$status $($i+1). ${id}: $($choices[$i].size) - $($choices[$i].capability)"
}
Write-Host 'Media providers are optional and may require credentials/account access. CLI authentication is assumed.'
if (-not $NonInteractive) {
    $available=@($choices | Where-Object { $_.windows -in @('all',$arch) })
    if ($rich) {
        $Components = Select-Choice 'Optional production tools (uncheck all for core editing)' @($available | ForEach-Object { "$($_.id) | $($_.size) | $($_.capability)" }) @($available | ForEach-Object id) @($Components.Split(',')) -Multiple
    } else { $Components = Ask 'Enter your selection (e.g. 1,2); Enter keeps checked items, none selects core only' $Components }
}
$selected = @($Components.Split(',') | ForEach-Object { $v=$_.Trim(); if ($v -match '^[1-4]$') { $choices[[int]$v-1].id } else { $v } })
if (@($selected | Select-Object -Unique).Count -ne $selected.Count -or ($selected.Count -gt 1 -and 'none' -in $selected)) { throw 'Duplicate or conflicting dependency selection.' }
foreach ($id in $selected) {
    if ($id -eq 'none') { continue }
    $def = Definition $id
    if ($def.kind -ne 'component' -or $def.windows -notin @('all',$arch)) { throw "Component unavailable: $id on windows/$arch" }
}
$skill = Join-Path $ProjectDir "$($hostDef.value)/facet"
$state = Join-Path $ProjectDir '.facet-install'
$instruction = Join-Path $ProjectDir $instructionRel
$previous = $null
$legacyMigration = $false
$instructionOwned = $false
if (Test-Path -LiteralPath $state) {
    Assert-RealAncestors $state
    $ownedFile = Join-Path $state 'managed-files.json'
    $previous = [IO.File]::ReadAllText((Join-Path $state 'installation.json')) | ConvertFrom-Json
    if ($previous.host -ne $Target) { throw 'This project belongs to another CLI integration; select that CLI or a fresh project.' }
    if (-not (Test-Path -LiteralPath $ownedFile)) {
        if (-not $NonInteractive) { $MigrateLegacy = (Ask 'Migrate older integration? Its complete files will be kept in a project backup' 'n') -in @('y','yes') }
        if (-not $MigrateLegacy) { throw 'Older integration has no ownership hashes. Rerun with -MigrateLegacy to preserve a full backup before replacing it.' }
        $legacyMigration = $true
        Assert-RealAncestors $skill
        if (@(Get-ChildItem -LiteralPath $state,$skill -Recurse -Force | Where-Object { $_.Attributes -band [IO.FileAttributes]::ReparsePoint }).Count) { throw 'Refusing legacy migration containing links.' }
    } else {
    $owned = [IO.File]::ReadAllText($ownedFile) | ConvertFrom-Json
    foreach ($file in $owned) {
        if ($file.path -match '(^|/)\.\.(/|$)|:|\\' -or $file.path.StartsWith('/')) { throw 'Invalid project ownership record.' }
        $path = Join-Path $ProjectDir $file.path
        Assert-RealAncestors (Split-Path -Parent $path)
        $item = Get-Item -LiteralPath $path -Force -ErrorAction Stop
        if ($item.Attributes -band [IO.FileAttributes]::ReparsePoint -or (Get-FileHash -LiteralPath $path).Hash -ne $file.sha256) { throw "Preserving modified project file: $($file.path)" }
    }
    $actualFiles = @(Get-ChildItem -LiteralPath $state,$skill -File -Recurse | Where-Object FullName -NE $ownedFile)
    if ($actualFiles.Count -ne @($owned).Count) { throw 'Preserving extra files in installer-owned project directories.' }
    }
    $instructionRecord = Join-Path $state 'instruction-section.json'
    if (Test-Path -LiteralPath $instructionRecord) {
        $record = [IO.File]::ReadAllText($instructionRecord) | ConvertFrom-Json
        if ($record.path -cne $instructionRel -or $record.sha256 -notmatch '^[a-f0-9]{64}$') { throw 'Invalid governing instruction ownership record.' }
        $instructionItem = Get-Item -LiteralPath $instruction -Force -ErrorAction Stop
        if ($instructionItem.PSIsContainer -or $instructionItem.Attributes -band [IO.FileAttributes]::ReparsePoint) { throw 'Managed governing instruction is missing or replaced.' }
        $section = Find-FacetSection ([IO.File]::ReadAllText($instruction)) $instructionRel
        if (-not $section -or (Text-Hash $section.Value) -cne $record.sha256) { throw 'Preserving modified Facet instruction section.' }
        $instructionOwned = $true
    }
    if (-not $NonInteractive -and -not $PSBoundParameters.ContainsKey('Action') -and -not $env:FACET_ACTION) {
        $Action = Select-Choice 'What should setup do?' @('Add components - keep existing tools','Repair - build and verify a replacement','Update - switch this project to the selected version') @('add','repair','update') @('add')
        if ($Action -match '^[1-3]$') { $Action=@('add','repair','update')[[int]$Action-1] }
        if ($Action -notin @('add','repair','update')) { throw 'Choose add, repair, or update.' }
    }
    if ($Action -eq 'add' -and $previous.version -ne $Version) { throw 'Choose update to change the product version.' }
    if ($Action -eq 'add') {
        $selected = @(@($previous.components) + @($selected | Where-Object { $_ -ne 'none' }) | Where-Object { $_ -ne 'none' } | Select-Object -Unique)
        if (-not $selected.Count) { $selected=@('none') }
        $previousPacks = if ($previous.PSObject.Properties.Name -contains 'packs') { @($previous.packs) } else { @() }
        $selectedPacks = @($previousPacks + $selectedPacks | Where-Object { $_ } | Select-Object -Unique)
        $InstallDir = $previous.installation
    }
} else { Assert-New $skill; Assert-New $state }
Assert-RealAncestors $InstallDir
# An existing runtime is immutable during repair/additions: build a sibling
# generation and only rebind this project after every selected check passes.
$reuse = $false
$runtimeState = Join-Path $InstallDir 'components.json'
if (Test-Path -LiteralPath $runtimeState) {
    $ready = [IO.File]::ReadAllText($runtimeState) | ConvertFrom-Json
    $missing = @($selected | Where-Object { $_ -ne 'none' -and $_ -notin @($ready.components) })
    $reuse = $Action -ne 'repair' -and $ready.version -eq $Version -and -not $missing.Count
}
if ((Test-Path -LiteralPath $InstallDir) -and -not $reuse) {
    if (-not (Test-Path -LiteralPath (Join-Path $InstallDir 'facet-install.json'))) { throw 'Existing installation is not managed by these scripts.' }
    $InstallDir += '-generation-' + [guid]::NewGuid().ToString('N').Substring(0,8)
}
Write-Host "`n  Agent: $Target`n  Project: $ProjectDir`n  Action: $Action`n  Production methods: $(if ($selectedPacks.Count) {$selectedPacks -join ', '} else {'core only'})`n  Components: $($selected -join ', ')`n  Runtime: $InstallDir"
Write-Host '  Existing managed runtimes are retained until a verified replacement is ready.'
Write-Host "  Detailed log: $logFile"
if (-not $NonInteractive -and (Ask 'Continue?' 'y') -notin @('y','yes')) { throw 'Installation cancelled.' }
Section '[2/3] Install and verify capabilities'
$savedPath = $env:PATH
$temp = Join-Path ([IO.Path]::GetTempPath()) ('facet-install-' + [guid]::NewGuid().ToString('N'))
$stage = ''; $projectStage = ''
$oldState=''; $oldSkill=''
$instructionTouched=$false; $instructionExisted=$false; $instructionBackup=$null
$newRuntime = -not (Test-Path -LiteralPath $InstallDir)
$committed = $false
New-Item -ItemType Directory -Path $temp | Out-Null
try {
    $name = "facet-$Version-windows-$arch.zip"
    if (-not $ArchivePath -and -not $reuse) {
        $base = "https://github.com/$((Definition facet).value)/releases/download/v$Version"
        $cache = Join-Path $HOME '.facet/cache'; New-Item -ItemType Directory -Path $cache -Force | Out-Null
        $ArchivePath = Join-Path $cache $name; $ChecksumPath = Join-Path $temp 'SHA256SUMS.txt'
        Step 'Check release download' { Download "$base/SHA256SUMS.txt" $ChecksumPath }
        $cached = $false
        if (Test-Path $ArchivePath) { try { Verify-Checksum $ArchivePath $ChecksumPath $name; $cached=$true } catch {} }
        if (-not $cached) { Step 'Download Facet' { Download "$base/$name" "$ArchivePath.partial"; Verify-Checksum "$ArchivePath.partial" $ChecksumPath $name; Move-Item -LiteralPath "$ArchivePath.partial" -Destination $ArchivePath -Force } }
        else { Write-Host '  OK Reusing verified download' }
    }
    $hash = ''
    if ($ArchivePath) { Verify-Checksum $ArchivePath $ChecksumPath $name; $hash = (Get-FileHash -LiteralPath $ArchivePath -Algorithm SHA256).Hash }
    $receiptFile = Join-Path $InstallDir 'facet-install.json'
    if (Test-Path -LiteralPath $InstallDir) {
        if (-not (Test-Path -LiteralPath $receiptFile)) { throw 'Existing installation is not managed by these scripts.' }
        $receipt = [IO.File]::ReadAllText($receiptFile) | ConvertFrom-Json
        if ($receipt.schema -ne 1 -or ($hash -and $receipt.archive_sha256 -ne $hash) -or $receipt.version -ne $Version) { throw 'Installation receipt mismatch; choose Repair.' }
        if (-not @($receipt.files | Where-Object { $_.path.Replace('\','/') -eq 'bin/facet.exe' }).Count) { throw 'Installation receipt is missing the product binary.' }
        foreach ($file in $receipt.files) {
            if ($file.path -match '(^|[/\\])\.\.([/\\]|$)|:' -or [IO.Path]::IsPathRooted($file.path)) { throw 'Invalid receipt path.' }
            $path = Join-Path $InstallDir $file.path
            Assert-RealAncestors (Split-Path -Parent $path)
            $info = Get-Item -LiteralPath $path -Force
            if ($info.Attributes -band [IO.FileAttributes]::ReparsePoint -or (Get-FileHash -LiteralPath $path).Hash -ne $file.sha256) { throw "Installed file changed: $($file.path)" }
        }
    } else {
        $parent = Split-Path -Parent $InstallDir
        New-Item -ItemType Directory -Path $parent -Force | Out-Null
        $stage = Join-Path $parent ('.facet-stage-' + [guid]::NewGuid().ToString('N'))
        New-Item -ItemType Directory -Path $stage | Out-Null
        Expand-SafeZip $ArchivePath $stage
        foreach ($required in @('bin/facet.exe','bundle/skills/facet/SKILL.md','bundle/packs/explainer/SKILL.md','bundle/remotion-composer/package-lock.json')) {
            if (-not (Test-Path -LiteralPath (Join-Path $stage $required) -PathType Leaf)) { throw "Release missing $required" }
        }
        $reported = & (Join-Path $stage 'bin/facet.exe') version
        if ($LASTEXITCODE -ne 0 -or "$reported".Trim() -ne "facet v$Version") { throw 'Binary version mismatch.' }
        $files = @(Get-ChildItem -LiteralPath $stage -Recurse -File | ForEach-Object { @{path=(Relative-Path $stage $_.FullName); sha256=(Get-FileHash -LiteralPath $_.FullName).Hash} })
        @{schema=1;version=$Version;archive_sha256=$hash;files=$files} | ConvertTo-Json -Depth 5 | Set-Content -LiteralPath (Join-Path $stage 'facet-install.json') -Encoding utf8
        for ($attempt=0; $attempt -lt 10; $attempt++) {
            try { Move-Item -LiteralPath $stage -Destination $InstallDir; $stage=''; break } catch { if ($attempt -eq 9) {throw}; Start-Sleep -Milliseconds 300 }
        }
    }
    $facet = Join-Path $InstallDir 'bin/facet.exe'
    $deps = Join-Path $InstallDir 'dependencies'
    New-Item -ItemType Directory -Path $deps -Force | Out-Null
    $paths = @((Join-Path $deps 'gflow'),(Join-Path $deps 'piper/Scripts'))
    $env:PATH = ($paths -join ';') + ';' + $env:PATH
    if (-not (Get-Command ffmpeg -ErrorAction SilentlyContinue) -or -not (Get-Command ffprobe -ErrorAction SilentlyContinue)) { Install-System ffmpeg }
    Step 'Check media tools' { Run ffmpeg @('-version'); Run ffprobe @('-version') }
    if ('remotion' -in $selected -or 'hyperframes' -in $selected) {
        $major = 0
        if (Get-Command node -ErrorAction SilentlyContinue) { $major = [int](& node -p "process.versions.node.split('.')[0]") }
        if ($major -lt [int](Definition node).value) { Install-System node }
        if ([int](& node -p "process.versions.node.split('.')[0]") -lt [int](Definition node).value) { throw 'Node version is too old; update it and rerun.' }
    }
    if ('remotion' -in $selected) { $composer = Join-Path $InstallDir (Definition remotion).value }
    if ('hyperframes' -in $selected) { $hfEntry = Join-Path $deps 'hyperframes/node_modules/hyperframes/bin/hyperframes.mjs' }
    if ('piper' -in $selected) { $python = Join-Path $deps 'piper/Scripts/python.exe'; $voices = Join-Path $deps 'voices' }
    if ($reuse) { Write-Host '  OK Reusing configured dependencies (no package reinstall)' }
    if ('remotion' -in $selected -and -not $reuse) {
        $composer = Join-Path $InstallDir (Definition remotion).value
        Confirm-Install "Install locked Remotion dependencies and headless browser in $composer"
        Step 'Install Remotion packages' { Run npm.cmd @('ci','--prefix',$composer,'--no-audit','--no-fund') }
        Push-Location $composer
        try { Step 'Prepare Remotion browser' { Run node @((Join-Path $composer 'node_modules/@remotion/cli/remotion-cli.js'),'browser','ensure') } } finally { Pop-Location }
    }
    if ('hyperframes' -in $selected -and -not $reuse) {
        $hf = Join-Path $deps 'hyperframes'; New-Item -ItemType Directory -Path $hf -Force | Out-Null
        $package = Join-Path $hf 'package.json'
        if (-not (Test-Path $package)) { @{private=$true;dependencies=@{hyperframes=(Definition hyperframes).version}} | ConvertTo-Json | Set-Content $package }
        Confirm-Install "Install HyperFrames $((Definition hyperframes).version) and browser"
        $npmAction = if (Test-Path (Join-Path $hf 'package-lock.json')) {'ci'} else {'install'}
        Step 'Install HyperFrames packages' { Run npm.cmd @($npmAction,'--prefix',$hf,'--no-audit','--no-fund') }
        $hfEntry = Join-Path $hf 'node_modules/hyperframes/bin/hyperframes.mjs'
        Step 'Prepare HyperFrames browser' { Run node @($hfEntry,'browser','ensure') }
    }
    if ('gflow' -in $selected -and -not $reuse) {
        $def = Definition gflow; $gn = "gflow_$($def.version)_windows_$arch.zip"
        $base = "https://github.com/$($def.value)/releases/download/v$($def.version)"
        Step 'Download gflow' { Download "$base/$gn" (Join-Path $temp $gn); Download "$base/checksums.txt" (Join-Path $temp 'gflow-sums') }
        Verify-Checksum (Join-Path $temp $gn) (Join-Path $temp 'gflow-sums') $gn
        $gd = Join-Path $deps 'gflow'
        if (-not (Test-Path $gd)) { New-Item -ItemType Directory $gd | Out-Null; Expand-SafeZip (Join-Path $temp $gn) $gd }
        Step 'Check gflow executable' { Run (Join-Path $gd 'gflow.exe') @('--help') }
    }
    if ('piper' -in $selected -and -not $reuse) {
        if (-not (Get-Command python -ErrorAction SilentlyContinue)) { Install-System python }
        $venv = Join-Path $deps 'piper'; $python = Join-Path $venv 'Scripts/python.exe'
        if (-not (Test-Path $python)) { Step 'Prepare private Python environment' { Run python @('-m','venv',$venv) } }
        Confirm-Install "Install Piper $((Definition piper).version) and voice $((Definition piper).value)"
        Step 'Install Piper' { Run $python @('-m','pip','install','--only-binary=:all:',"piper-tts==$((Definition piper).version)") }
        $voices = Join-Path $deps 'voices'; New-Item -ItemType Directory $voices -Force | Out-Null
        Step 'Download speech model' { Run $python @('-m','piper.download_voices','--download-dir',$voices,(Definition piper).value) }
    }
    if (-not $SkipVerify) {
      Step 'Verify selected local capabilities' {
        $verify = Join-Path $temp 'verify'; New-Item -ItemType Directory $verify | Out-Null
        $video = Join-Path $verify 'test.mp4'
        Run ffmpeg @('-v','error','-f','lavfi','-i','color=c=blue:s=320x180:r=24:d=1','-c:v','libx264','-pix_fmt','yuv420p',$video)
        if ('remotion' -in $selected) {
            @{paths=@{remotion_composer=$composer}} | ConvertTo-Json | Set-Content (Join-Path $verify '.facet.yaml')
            $video = Join-Path $verify 'render.mp4'
            @{width=320;height=180;fps=24;duration_seconds=1;output_path=$video;cuts=@(@{type='text_card';text='Facet setup';in_seconds=0;out_seconds=1})} | ConvertTo-Json -Depth 6 | Set-Content (Join-Path $verify 'render.json')
            Push-Location $verify
            try { Run $facet @('tools','run','video_compose','--input',(Join-Path $verify 'render.json')) } finally { Pop-Location }
        }
        Run ffprobe @('-v','error','-show_streams',$video); Run ffmpeg @('-v','error','-i',$video,'-f','null','-')
        if ('piper' -in $selected) {
            'Facet setup verification.' | & $python -m piper --model (Join-Path $voices "$((Definition piper).value).onnx") --output_file (Join-Path $verify 'voice.wav')
            if ($LASTEXITCODE -ne 0) { throw 'Piper verification failed.' }
            Run ffmpeg @('-v','error','-i',(Join-Path $verify 'voice.wav'),'-f','null','-')
        }
        if ('hyperframes' -in $selected) {
            $hv = Join-Path $verify 'html'; New-Item -ItemType Directory $hv | Out-Null
            Copy-Item -LiteralPath (Join-Path $PSScriptRoot 'installer/verify.html') -Destination (Join-Path $hv 'index.html')
            Push-Location $hv
            try { Run node @($hfEntry,'render','--output',(Join-Path $hv 'render.mp4'),'--fps','24') } finally { Pop-Location }
            Run ffmpeg @('-v','error','-i',(Join-Path $hv 'render.mp4'),'-f','null','-')
        }
        Write-Host 'Local media verification passed; external providers and host invocation were not tested.'
      }
    } else { Write-Host 'Verification skipped: media readiness is unverified.' }
    if (-not $previous) { Assert-New $skill; Assert-New $state }
    Section '[3/3] Connect your CLI'
    @{version=$Version;components=$selected} | ConvertTo-Json | Set-Content -LiteralPath (Join-Path $InstallDir 'components.json') -Encoding UTF8
    New-Item -ItemType Directory -Path $ProjectDir -Force | Out-Null
    $projectStage = Join-Path $ProjectDir ('.facet-stage-' + [guid]::NewGuid().ToString('N'))
    New-Item -ItemType Directory -Path "$projectStage/state" -Force | Out-Null
    Copy-Item -LiteralPath "$InstallDir/bundle/skills/facet" -Destination "$projectStage/skill" -Recurse
    Copy-Item -LiteralPath "$InstallDir/bundle/packs" -Destination "$projectStage/state/packs" -Recurse
    foreach ($cmd in @('ffmpeg','ffprobe','node','npm.cmd')) { $found=@(Get-Command $cmd -CommandType Application -ErrorAction SilentlyContinue); if ($found.Count) { $paths += Split-Path -Parent $found[0].Source } }
    $pathLiteral = (($paths -join ';') + ';').Replace("'","''")
    $binaryLiteral = $facet.Replace("'","''")
    @('$ErrorActionPreference = ''Stop''',"`$env:PATH = '$pathLiteral' + `$env:PATH","& '$binaryLiteral' @args",'exit $LASTEXITCODE') | Set-Content -LiteralPath "$projectStage/state/run-facet.ps1" -Encoding utf8
    $launcher = (Join-Path $state 'run-facet.ps1').Replace("'","''")
    $methodLine = if ($selectedPacks.Count) { "- Active production methods selected during setup: " + (($selectedPacks | ForEach-Object { "$state/packs/$_/SKILL.md" }) -join ', ') + '.' } else { '- No production-method pack is active; use the core guidance only.' }
    @('', '## This installation', "- Invoke Facet with: & '$launcher' followed by the normal arguments. Use this launcher instead of bare facet in examples.", "- Resolve packs/... under $state. Read the relevant pack's SKILL.md on demand.", $methodLine, "- Optional components selected: $($selected -join ','). Media services report missing credentials/session requirements when used.") | Add-Content -LiteralPath "$projectStage/skill/SKILL.md"
    if ('piper' -in $selected) { "- Piper model: $(Join-Path $voices "$((Definition piper).value).onnx")" | Add-Content "$projectStage/skill/SKILL.md" }
    if ('hyperframes' -in $selected) { "- Invoke the pinned HyperFrames renderer with node `"$hfEntry`" followed by its arguments; do not use unpinned npx." | Add-Content "$projectStage/skill/SKILL.md" }
    @{schema=1;version=$Version;installation=$InstallDir;host=$Target;components=$selected;packs=$selectedPacks} | ConvertTo-Json | Set-Content "$projectStage/state/installation.json"
    $sectionLines = @(
        $facetSectionStart,
        '## Facet',
        '- Facet manages only this bounded section; keep project-specific instructions outside it.',
        "- Read core guidance at ``$($hostDef.value)/facet/SKILL.md``.",
        "- Invoke Facet through ``$state/run-facet.ps1``."
    )
    if ($selectedPacks.Count) {
        $sectionLines += '- Active production methods: ' + (($selectedPacks | ForEach-Object { "``$state/packs/$_/SKILL.md``" }) -join ', ') + '.'
    } else {
        $sectionLines += '- No production-method pack is active; use core guidance only.'
    }
    $sectionLines += $facetSectionEnd
    $facetSection = $sectionLines -join "`n"
    $mergedInstruction = Merge-FacetSection $instruction $facetSection $instructionOwned
    @{path=$instructionRel;sha256=(Text-Hash $facetSection)} | ConvertTo-Json | Set-Content "$projectStage/state/instruction-section.json" -Encoding UTF8
    $managed = @()
    foreach ($pair in @(@{source="$projectStage/state";prefix='.facet-install'},@{source="$projectStage/skill";prefix="$($hostDef.value)/facet"})) {
        $managed += @(Get-ChildItem -LiteralPath $pair.source -Recurse -File | ForEach-Object { @{path=($pair.prefix + '/' + (Relative-Path $pair.source $_.FullName));sha256=(Get-FileHash -LiteralPath $_.FullName).Hash} })
    }
    ConvertTo-Json -InputObject @($managed) -Depth 4 | Set-Content "$projectStage/state/managed-files.json" -Encoding UTF8
    New-Item -ItemType Directory -Path (Split-Path -Parent $skill) -Force | Out-Null
    $oldState="$projectStage/old-state"; $oldSkill="$projectStage/old-skill"
    try {
        if ($previous) { Move-Item -LiteralPath $state -Destination $oldState; Move-Item -LiteralPath $skill -Destination $oldSkill }
        Move-Item -LiteralPath "$projectStage/state" -Destination $state
        Move-Item -LiteralPath "$projectStage/skill" -Destination $skill
        $instructionExisted = Test-Path -LiteralPath $instruction
        if ($instructionExisted) { $instructionBackup = [IO.File]::ReadAllBytes($instruction) }
        New-Item -ItemType Directory -Path (Split-Path -Parent $instruction) -Force | Out-Null
        $instructionTouched = $true
        [IO.File]::WriteAllText($instruction,$mergedInstruction,[Text.UTF8Encoding]::new($false))
    } catch {
        if (Test-Path $oldState) { if (Test-Path $state) { Remove-Item $state -Recurse -Force }; Move-Item $oldState $state }
        if (Test-Path $oldSkill) { if (Test-Path $skill) { Remove-Item $skill -Recurse -Force }; Move-Item $oldSkill $skill }
        throw
    }
    if ($legacyMigration) {
        $backup = Join-Path $ProjectDir ('.facet-backup-' + [guid]::NewGuid().ToString('N'))
        New-Item -ItemType Directory $backup | Out-Null
        Move-Item -LiteralPath $oldState -Destination (Join-Path $backup 'state')
        Move-Item -LiteralPath $oldSkill -Destination (Join-Path $backup 'skill')
        Write-Host "Original integration preserved at: $backup"
    }
    $committed=$true
    Write-Host "`nFacet is ready for $Target." -ForegroundColor Green
    Write-Host "  Open a new $Target session in: $ProjectDir"
    Write-Host '  Try: Use Facet to make a short title card.'
    Write-Host "Setup log: $logFile"
    Write-Host 'Rerun this installer to add components, repair, or update this project.'
} catch {
    Write-Host "`nSetup incomplete. Rerun with the same choices to retry. Log: $logFile"
    throw
} finally {
    $env:PATH = $savedPath
    if (-not $committed) {
        if ($oldState -and (Test-Path $oldState)) { if (Test-Path $state) { Remove-Item $state -Recurse -Force }; Move-Item $oldState $state }
        if ($oldSkill -and (Test-Path $oldSkill)) { if (Test-Path $skill) { Remove-Item $skill -Recurse -Force }; Move-Item $oldSkill $skill }
        if ($instructionTouched) {
            if ($instructionExisted) { [IO.File]::WriteAllBytes($instruction,$instructionBackup) }
            elseif (Test-Path -LiteralPath $instruction) { Remove-Item -LiteralPath $instruction -Force }
        }
    }
    if ($stage -and (Test-Path -LiteralPath $stage)) { Remove-Item -LiteralPath $stage -Recurse -Force }
    if ($projectStage -and (Test-Path -LiteralPath $projectStage)) { Remove-Item -LiteralPath $projectStage -Recurse -Force }
    Remove-Item -LiteralPath $temp -Recurse -Force
    if ($newRuntime -and -not $committed -and (Test-Path -LiteralPath $InstallDir)) { Remove-Item -LiteralPath $InstallDir -Recurse -Force }
}
