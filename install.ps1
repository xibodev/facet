#requires -Version 7.0
<#
.SYNOPSIS
Install the prebuilt Facet capability for an agentic CLI.
.DESCRIPTION
Run from the installer package (install.ps1 and installer/manifest.tsv).
Authentication is owned by the selected CLI. No product setup command is used.
#>
[CmdletBinding()]
param(
    [string]$Version = '',
    [ValidateSet('opencode','codex','claude','copilot')][string]$Target,
    [string]$ProjectDir,
    [string]$InstallDir,
    [string]$Components = 'remotion',
    [string]$ArchivePath,
    [string]$ChecksumPath,
    [switch]$NonInteractive,
    [switch]$SkipVerify
)
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
if (-not $IsWindows) { throw 'Use install.sh on Linux/macOS.' }
if (-not $PSCommandPath) { throw 'Run install.ps1 from the downloaded installer package, not a pipe.' }
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
    & (Resolve-Program $Program) @Arguments
    if ($LASTEXITCODE -ne 0) { throw "Command failed ($LASTEXITCODE): $Program" }
}
function Confirm-Install([string]$Description) {
    Write-Host $Description
    if (-not $NonInteractive -and (Ask 'Continue?' 'y') -notin @('y','yes')) { throw 'Installation cancelled.' }
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
    Run 'winget' @('install','--id',$package,'--exact','--source','winget','--accept-package-agreements','--accept-source-agreements')
    $env:PATH = [Environment]::GetEnvironmentVariable('Path','Machine') + ';' + [Environment]::GetEnvironmentVariable('Path','User') + ';' + $env:PATH
}
if ($NonInteractive -and (-not $Target -or -not $ProjectDir)) { throw '-NonInteractive requires -Target and -ProjectDir.' }
if (-not $Version) { $Version = (Definition facet).version }
if ($Version -notmatch '^\d+\.\d+\.\d+(?:-[A-Za-z0-9.-]+)?$') { throw 'Invalid release version.' }
if ([bool]$ArchivePath -ne [bool]$ChecksumPath) { throw 'Supply both -ArchivePath and -ChecksumPath.' }
$arch = switch ([Runtime.InteropServices.RuntimeInformation]::OSArchitecture.ToString()) { 'X64' {'amd64'} 'Arm64' {'arm64'} default {throw 'Unsupported architecture.'} }
if (-not $Target) { $Target = Ask 'CLI: opencode, codex, claude, copilot' 'opencode' }
$hostDef = Definition $Target
if ($hostDef.kind -ne 'host') { throw 'Unsupported CLI.' }
if (-not $ProjectDir) { $ProjectDir = Ask 'Project directory' '.' }
$ProjectDir = [IO.Path]::GetFullPath($ProjectDir)
if (-not $InstallDir) { $InstallDir = Join-Path $HOME ".facet/releases/$Version-windows-$arch" }
$InstallDir = [IO.Path]::GetFullPath($InstallDir)
if (($ProjectDir + $InstallDir) -match '[\x00-\x1f]') { throw 'Control characters are unsupported in installation paths.' }
$choices = @($manifest | Where-Object kind -EQ 'component')
Write-Host 'Core: FFmpeg and FFprobe. Optional downloads (approximate; platform/cache dependent):'
for ($i=0; $i -lt $choices.Count; $i++) { Write-Host "$($i+1). $($choices[$i].id): $($choices[$i].size) — $($choices[$i].capability)" }
Write-Host 'Media providers are optional and may require credentials/account access. CLI authentication is assumed.'
if (-not $NonInteractive) { $Components = Ask 'Select numbers or names separated by commas; none for core only' $Components }
$selected = @($Components.Split(',') | ForEach-Object { $v=$_.Trim(); if ($v -match '^[1-4]$') { $choices[[int]$v-1].id } else { $v } })
if (@($selected | Select-Object -Unique).Count -ne $selected.Count -or ($selected.Count -gt 1 -and 'none' -in $selected)) { throw 'Duplicate or conflicting dependency selection.' }
foreach ($id in $selected) {
    if ($id -eq 'none') { continue }
    $def = Definition $id
    if ($def.kind -ne 'component' -or $def.windows -notin @('all',$arch)) { throw "Component unavailable: $id on windows/$arch" }
}
$skill = Join-Path $ProjectDir "$($hostDef.value)/facet"
$state = Join-Path $ProjectDir '.facet-install'
Assert-New $skill; Assert-New $state; Assert-RealAncestors $InstallDir
Confirm-Install "Facet $Version -> $InstallDir; CLI $Target -> $ProjectDir; optional: $($selected -join ',')"
$savedPath = $env:PATH
$temp = Join-Path ([IO.Path]::GetTempPath()) ('facet-install-' + [guid]::NewGuid().ToString('N'))
$stage = ''; $projectStage = ''
New-Item -ItemType Directory -Path $temp | Out-Null
try {
    $name = "facet-$Version-windows-$arch.zip"
    if (-not $ArchivePath) {
        $base = "https://github.com/$((Definition facet).value)/releases/download/v$Version"
        $ArchivePath = Join-Path $temp $name; $ChecksumPath = Join-Path $temp 'SHA256SUMS.txt'
        Invoke-WebRequest "$base/$name" -OutFile $ArchivePath
        Invoke-WebRequest "$base/SHA256SUMS.txt" -OutFile $ChecksumPath
    }
    Verify-Checksum $ArchivePath $ChecksumPath $name
    $hash = (Get-FileHash -LiteralPath $ArchivePath -Algorithm SHA256).Hash
    $receiptFile = Join-Path $InstallDir 'facet-install.json'
    if (Test-Path -LiteralPath $InstallDir) {
        if (-not (Test-Path -LiteralPath $receiptFile)) { throw 'Existing installation is not managed by these scripts.' }
        $receipt = [IO.File]::ReadAllText($receiptFile) | ConvertFrom-Json
        if ($receipt.schema -ne 1 -or $receipt.archive_sha256 -ne $hash -or $receipt.version -ne $Version) { throw 'Installation receipt mismatch; choose a fresh directory.' }
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
        $files = @(Get-ChildItem -LiteralPath $stage -Recurse -File | ForEach-Object { @{path=[IO.Path]::GetRelativePath($stage,$_.FullName); sha256=(Get-FileHash -LiteralPath $_.FullName).Hash} })
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
    Run ffmpeg @('-version'); Run ffprobe @('-version')
    if ('remotion' -in $selected -or 'hyperframes' -in $selected) {
        $major = 0
        if (Get-Command node -ErrorAction SilentlyContinue) { $major = [int](& node -p "process.versions.node.split('.')[0]") }
        if ($major -lt [int](Definition node).value) { Install-System node }
        if ([int](& node -p "process.versions.node.split('.')[0]") -lt [int](Definition node).value) { throw 'Node version is too old; update it and rerun.' }
    }
    if ('remotion' -in $selected) {
        $composer = Join-Path $InstallDir (Definition remotion).value
        Confirm-Install "Install locked Remotion dependencies and headless browser in $composer"
        Run npm.cmd @('ci','--prefix',$composer,'--no-audit','--no-fund')
        Push-Location $composer
        try { Run node @((Join-Path $composer 'node_modules/@remotion/cli/remotion-cli.js'),'browser','ensure') } finally { Pop-Location }
    }
    if ('hyperframes' -in $selected) {
        $hf = Join-Path $deps 'hyperframes'; New-Item -ItemType Directory -Path $hf -Force | Out-Null
        $package = Join-Path $hf 'package.json'
        if (-not (Test-Path $package)) { @{private=$true;dependencies=@{hyperframes=(Definition hyperframes).version}} | ConvertTo-Json | Set-Content $package }
        Confirm-Install "Install HyperFrames $((Definition hyperframes).version) and browser"
        $action = if (Test-Path (Join-Path $hf 'package-lock.json')) {'ci'} else {'install'}
        Run npm.cmd @($action,'--prefix',$hf,'--no-audit','--no-fund')
        $hfEntry = Join-Path $hf 'node_modules/hyperframes/bin/hyperframes.mjs'
        Run node @($hfEntry,'browser','ensure')
    }
    if ('gflow' -in $selected) {
        $def = Definition gflow; $gn = "gflow_$($def.version)_windows_$arch.zip"
        $base = "https://github.com/$($def.value)/releases/download/v$($def.version)"
        Invoke-WebRequest "$base/$gn" -OutFile (Join-Path $temp $gn)
        Invoke-WebRequest "$base/checksums.txt" -OutFile (Join-Path $temp 'gflow-sums')
        Verify-Checksum (Join-Path $temp $gn) (Join-Path $temp 'gflow-sums') $gn
        $gd = Join-Path $deps 'gflow'
        if (-not (Test-Path $gd)) { New-Item -ItemType Directory $gd | Out-Null; Expand-SafeZip (Join-Path $temp $gn) $gd }
        Run (Join-Path $gd 'gflow.exe') @('--help')
    }
    if ('piper' -in $selected) {
        if (-not (Get-Command python -ErrorAction SilentlyContinue)) { Install-System python }
        $venv = Join-Path $deps 'piper'; $python = Join-Path $venv 'Scripts/python.exe'
        if (-not (Test-Path $python)) { Run python @('-m','venv',$venv) }
        Confirm-Install "Install Piper $((Definition piper).version) and voice $((Definition piper).value)"
        Run $python @('-m','pip','install','--only-binary=:all:',"piper-tts==$((Definition piper).version)")
        $voices = Join-Path $deps 'voices'; New-Item -ItemType Directory $voices -Force | Out-Null
        Run $python @('-m','piper.download_voices','--download-dir',$voices,(Definition piper).value)
    }
    if (-not $SkipVerify) {
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
    } else { Write-Host 'Verification skipped: media readiness is unverified.' }
    Assert-New $skill; Assert-New $state
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
    @('', '## This installation', "- Invoke Facet with: & '$launcher' followed by the normal arguments. Use this launcher instead of bare facet in examples.", "- Resolve packs/... under $state. Read the relevant pack's SKILL.md on demand.", "- Optional components selected: $($selected -join ','). Media services report missing credentials/session requirements when used.") | Add-Content -LiteralPath "$projectStage/skill/SKILL.md"
    if ('piper' -in $selected) { "- Piper model: $(Join-Path $voices "$((Definition piper).value).onnx")" | Add-Content "$projectStage/skill/SKILL.md" }
    if ('hyperframes' -in $selected) { "- Invoke the pinned HyperFrames renderer with node `"$hfEntry`" followed by its arguments; do not use unpinned npx." | Add-Content "$projectStage/skill/SKILL.md" }
    @{schema=1;version=$Version;installation=$InstallDir;host=$Target;components=$selected} | ConvertTo-Json | Set-Content "$projectStage/state/installation.json"
    New-Item -ItemType Directory -Path (Split-Path -Parent $skill) -Force | Out-Null
    Move-Item -LiteralPath "$projectStage/state" -Destination $state
    try { Move-Item -LiteralPath "$projectStage/skill" -Destination $skill } catch { Remove-Item -LiteralPath $state -Recurse -Force; throw }
    Write-Host "Installed. Start a new $Target session in $ProjectDir and ask it to use Facet."
} finally {
    $env:PATH = $savedPath
    if ($stage -and (Test-Path -LiteralPath $stage)) { Remove-Item -LiteralPath $stage -Recurse -Force }
    if ($projectStage -and (Test-Path -LiteralPath $projectStage)) { Remove-Item -LiteralPath $projectStage -Recurse -Force }
    Remove-Item -LiteralPath $temp -Recurse -Force
}
