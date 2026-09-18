# Windows source installer. Run with -File from a complete, trusted checkout.
# InstallDir is the binary directory; the bundle is installed beside it.
param(
    [switch]$Quiet,
    [switch]$NonInteractive,
    [ValidateSet('on-demand', 'global')][string]$Scope = 'on-demand',
    [string]$InstallDir = '',
    [string]$HomeDir = '',
    [switch]$NoPath,
    [switch]$NoShortcuts,
    [switch]$UpdateShortcuts,
    [switch]$Isolated
)

$ErrorActionPreference = 'Stop'
$sourceHelp = 'Facet requires a complete source checkout; this script does not download source. Run: git clone https://github.com/xibodev/facet.git ; then: pwsh -File /absolute/path/to/facet/scripts/install-source.ps1'
# Invoke-Expression / piped invocations have no script path. Never infer source from cwd.
if (-not $PSCommandPath) { throw $sourceHelp }
$ScriptDir = Split-Path -Parent $PSScriptRoot
foreach ($file in @('go.mod', 'cmd/facet/main.go', 'cmd/facet-ui/main.go', 'skills/facet/SKILL.md', 'packs/explainer/SKILL.md', 'remotion-composer/package.json', 'remotion-composer/package-lock.json', 'remotion-composer/src/index.tsx')) {
    if (-not (Test-Path -LiteralPath (Join-Path $ScriptDir $file) -PathType Leaf)) { throw $sourceHelp }
}
$BundleFolders = @('skills', 'packs', 'schemas', 'remotion-composer', 'agents')
foreach ($folder in $BundleFolders) {
    if (-not (Test-Path -LiteralPath (Join-Path $ScriptDir $folder) -PathType Container)) { throw $sourceHelp }
}
if ($env:OS -ne 'Windows_NT') { throw 'This installer supports Windows. Use install.sh on Linux/macOS.' }
if ($UpdateShortcuts -and ($NoShortcuts -or $Isolated)) { throw '-UpdateShortcuts cannot be combined with -NoShortcuts or -Isolated.' }
if ($HomeDir -and -not $Isolated) { throw '-HomeDir requires -Isolated to prevent unintended user-profile writes.' }
if ($Isolated -and (-not $HomeDir -or -not $InstallDir -or -not $NoPath -or -not $NoShortcuts -or $Scope -ne 'on-demand')) {
    throw '-Isolated requires explicit -HomeDir, -InstallDir, -NoPath, -NoShortcuts and on-demand scope.'
}
if (-not $HomeDir) { $HomeDir = $env:USERPROFILE }
$HomeDir = [IO.Path]::GetFullPath($HomeDir)
if ($Isolated -and $HomeDir.TrimEnd('\', '/') -eq $env:USERPROFILE.TrimEnd('\', '/')) {
    throw '-Isolated requires a home different from the real user profile.'
}
if (-not $InstallDir) { $InstallDir = Join-Path $HomeDir '.facet/bin' }
$InstallDir = [IO.Path]::GetFullPath($InstallDir).TrimEnd('\', '/')
$BundleDir = Join-Path (Split-Path -Parent $InstallDir) 'bundle'
foreach ($target in @((Join-Path $InstallDir 'facet.exe'), (Join-Path $InstallDir 'facet-ui.exe'), $BundleDir)) {
    if (Test-Path -LiteralPath $target) { throw "Refusing to overwrite existing installation files: $target. Choose a fresh -InstallDir (bin directory with a fresh sibling bundle)." }
}

function Copy-BundleTree([string]$Source, [string]$Destination) {
    New-Item -ItemType Directory -Path $Destination -Force | Out-Null
    foreach ($item in Get-ChildItem -LiteralPath $Source -Force) {
        if ($item.Name -in @('node_modules', '.git', 'out', 'dist', '.cache', '.env') -or $item.Name -like '.env.*') { continue }
        if ($item.Attributes -band [IO.FileAttributes]::ReparsePoint) { throw "Refusing bundle link: $($item.FullName)" }
        $target = Join-Path $Destination $item.Name
        if ($item.PSIsContainer) { Copy-BundleTree $item.FullName $target }
        else { Copy-Item -LiteralPath $item.FullName -Destination $target }
    }
}

function Set-FacetShortcut($Shell, [string]$Target, [string]$InstallDir, [string]$HomeDir, [switch]$UpdateShortcuts) {
    $executable = Join-Path $InstallDir 'facet-ui.exe'
    $shortcut = $null
    if (Test-Path -LiteralPath $Target) {
        $local = $env:LOCALAPPDATA
        if (-not $local) { $local = Join-Path $HomeDir 'AppData/Local' }
        $knownTargets = @($executable, (Join-Path $HomeDir '.facet/bin/facet-ui.exe'), (Join-Path $local 'Programs/Facet/bin/facet-ui.exe'))
        $knownTargets = @($knownTargets | ForEach-Object { [IO.Path]::GetFullPath($_) })
        $known = $false
        if ($UpdateShortcuts) {
            try {
                $shortcut = $Shell.CreateShortcut($Target)
                $known = $shortcut.TargetPath -and [IO.Path]::IsPathRooted($shortcut.TargetPath) -and ([IO.Path]::GetFullPath($shortcut.TargetPath) -in $knownTargets) -and -not $shortcut.Arguments
            } catch { $known = $false }
        }
        if (-not $UpdateShortcuts -or -not $known) {
            $command = "& '" + $executable.Replace("'", "''") + "'"
            Write-Warning "Preserving existing shortcut: $Target. It may still launch another installation. Launch this install with: $command . Use -UpdateShortcuts during a fresh install to retarget recognized Facet shortcuts; custom targets or arguments must be changed manually."
            return
        }
    }
    New-Item -ItemType Directory -Path (Split-Path -Parent $Target) -Force | Out-Null
    if (-not $shortcut) { $shortcut = $Shell.CreateShortcut($Target) }
    $shortcut.TargetPath = $executable
    $shortcut.WorkingDirectory = $HomeDir
    $shortcut.Description = 'Facet Studio'
    $shortcut.Save()
}

# Redirect build/package caches as well as application config discovery in isolated tests.
$savedEnv = @{}
try {
    $savedEnv['GOTOOLCHAIN'] = [Environment]::GetEnvironmentVariable('GOTOOLCHAIN', 'Process')
    $env:GOTOOLCHAIN = 'local'
    if ($Isolated) {
        $overrides = @{
            USERPROFILE = $HomeDir; HOME = $HomeDir
            APPDATA = (Join-Path $HomeDir 'AppData/Roaming'); LOCALAPPDATA = (Join-Path $HomeDir 'AppData/Local')
            GOCACHE = (Join-Path $HomeDir 'cache/go-build'); GOMODCACHE = (Join-Path $HomeDir 'cache/go-mod')
            GOPATH = (Join-Path $HomeDir 'go'); GOENV = 'off'
            NPM_CONFIG_CACHE = (Join-Path $HomeDir 'cache/npm'); NPM_CONFIG_USERCONFIG = (Join-Path $HomeDir '.npmrc')
            NPM_CONFIG_GLOBALCONFIG = (Join-Path $HomeDir 'npm-globalrc')
        }
        foreach ($key in $overrides.Keys) {
            $savedEnv[$key] = [Environment]::GetEnvironmentVariable($key, 'Process')
            [Environment]::SetEnvironmentVariable($key, $overrides[$key], 'Process')
        }
    }
    Write-Host 'Checking prerequisites: Go 1.25+, Node.js 18+ with npm, FFmpeg and FFprobe...'
    foreach ($program in @('go', 'node', 'npm', 'ffmpeg', 'ffprobe')) {
        if (-not (Get-Command $program -ErrorAction SilentlyContinue)) {
            throw "Missing prerequisite: $program. Install Go 1.25+, Node.js 18+ with npm and FFmpeg/FFprobe on PATH."
        }
    }
    $goVersion = & go version
    if ($LASTEXITCODE -ne 0 -or "$goVersion" -notmatch 'go(\d+)\.(\d+)' -or [int]$Matches[1] -lt 1 -or ([int]$Matches[1] -eq 1 -and [int]$Matches[2] -lt 25)) {
        throw "Go 1.25+ is required; found $goVersion."
    }
    $nodeVersion = & node --version
    if ($LASTEXITCODE -ne 0 -or "$nodeVersion" -notmatch '^v(\d+)\.' -or [int]$Matches[1] -lt 18) { throw "Node.js 18+ is required; found $nodeVersion." }
    & npm --version
    if ($LASTEXITCODE -ne 0) { throw 'npm prerequisite check failed.' }
    foreach ($program in @('ffmpeg', 'ffprobe')) {
        & $program -version | Out-Null
        if ($LASTEXITCODE -ne 0) { throw "$program prerequisite check failed." }
    }
    New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
    Push-Location $ScriptDir
    try {
        foreach ($name in @('facet', 'facet-ui')) {
            & go build -o (Join-Path $InstallDir "$name.exe") "./cmd/$name"
            if ($LASTEXITCODE -ne 0) { throw "Failed to build cmd/$name." }
        }
    } finally { Pop-Location }

    Write-Host 'Installing skills, packs and Remotion source (excluding dependencies and render/cache outputs)...'
    foreach ($folder in $BundleFolders) {
        Copy-BundleTree (Join-Path $ScriptDir $folder) (Join-Path $BundleDir $folder)
    }
    Write-Host 'Installing locked Remotion dependencies (network access may be required)...'
    & npm ci --prefix (Join-Path $BundleDir 'remotion-composer') --no-audit --no-fund
    if ($LASTEXITCODE -ne 0) { throw 'Remotion npm ci failed; installation is incomplete.' }

    if ($Scope -eq 'global') {
        # Only these two global skill conventions are supported. Never replace user skills.
        foreach ($agentDir in @('.claude/skills', '.config/opencode/skills')) {
            $target = Join-Path $HomeDir "$agentDir/facet"

            # A link left by a previous install can outlive its target: renaming
            # this project left ~/.claude/skills/facet pointing at a directory
            # that no longer existed, so the skill was silently dead in the
            # agent CLI. Test-Path reports $false for a broken link, so the old
            # check neither preserved it nor replaced it -- it skipped, and the
            # dangling link stayed dead across every reinstall. Repair a stale
            # link we own; still never touch real user content.
            $link = Get-Item -LiteralPath $target -Force -ErrorAction SilentlyContinue
            if ($null -ne $link -and $null -ne $link.LinkType -and -not (Test-Path -LiteralPath $target)) {
                Write-Host "Repairing stale Facet skill link: $target"
                Remove-Item -LiteralPath $target -Force -ErrorAction SilentlyContinue
            }

            if (Test-Path -LiteralPath $target) { Write-Warning "Preserving existing global skill: $target"; continue }
            Copy-BundleTree (Join-Path $BundleDir 'skills/facet') $target
        }
        Write-Host 'Global scope: core Facet skill registered for Claude Code and OpenCode only; packs remain project-local via facet init.'
    } else { Write-Host 'On-demand scope: no global agent skills changed. Use facet init to project core and pack skills.' }

    if (-not $NoPath) {
        $userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
        if ($InstallDir -notin ($userPath -split ';')) {
            [Environment]::SetEnvironmentVariable('Path', "$InstallDir;$userPath", 'User')
        }
        if ($InstallDir -notin ($env:Path -split ';')) { $env:Path = "$InstallDir;$env:Path" }
    }
    $createShortcuts = -not $NoShortcuts
    if ($createShortcuts -and -not $Quiet -and -not $NonInteractive) {
        $answer = Read-Host 'Create Desktop and Start Menu shortcuts? [Y/n]'
        $createShortcuts = $answer -notmatch '^[nN]'
    }
    if ($createShortcuts) {
        $shell = New-Object -ComObject WScript.Shell
        foreach ($folder in @([Environment]::GetFolderPath('Desktop'), (Join-Path ([Environment]::GetFolderPath('Programs')) 'Facet'))) {
            $target = Join-Path $folder 'Facet Studio.lnk'
            Set-FacetShortcut $shell $target $InstallDir $HomeDir -UpdateShortcuts:$UpdateShortcuts
        }
    }
    Write-Host "Installed binaries: $InstallDir"
    Write-Host "Installed bundle: $BundleDir"
    Write-Host 'No Facet configuration files were changed. This is a source install, not a standalone release download.'
    Write-Host 'Run facet doctor for diagnostics; doctor is not a render-success gate. Install/authenticate your agent separately.'
    Write-Host 'Rendering requires a supported Chromium browser and its dependencies. No video render was verified by this installer.'
    if ($NoPath) { Write-Host "PATH was not changed. Invoke: & '$InstallDir/facet.exe' doctor" }
} finally {
    foreach ($key in $savedEnv.Keys) { [Environment]::SetEnvironmentVariable($key, $savedEnv[$key], 'Process') }
}
