# Facet Video Kit - Windows PowerShell Interactive Installer
# Usage: powershell -ExecutionPolicy Bypass -File .\install.ps1

param(
    [switch]$Quiet,
    [switch]$NonInteractive,
    [string]$Scope = "on-demand",
    [string]$InstallDir = ""
)

$ErrorActionPreference = "Stop"

Write-Host ''
Write-Host '======================================================' -ForegroundColor Cyan
Write-Host '      Facet - Autonomous Video Production Studio      ' -ForegroundColor Cyan
Write-Host '======================================================' -ForegroundColor Cyan
Write-Host ''

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
if (-not $ScriptDir) {
    $ScriptDir = Get-Location
}

# 1. Check Go Compiler
Write-Host '==> Checking Go compiler environment...' -ForegroundColor Yellow
$GoCmd = Get-Command 'go' -ErrorAction SilentlyContinue
if (-not $GoCmd) {
    Write-Error 'Go compiler (go.dev) was not found in PATH. Please install Go 1.22+ to build Facet from source.'
    exit 1
}
$goVer = go version
Write-Host ('  Found Go compiler: ' + $goVer) -ForegroundColor Green

# 2. Build Binaries
Write-Host '==> Compiling Facet binaries (facet, facet-ui)...' -ForegroundColor Yellow
$LocalBin = Join-Path $ScriptDir 'bin'
if (-not (Test-Path $LocalBin)) {
    New-Item -ItemType Directory -Path $LocalBin -Force | Out-Null
}

$FacetExe = Join-Path $LocalBin 'facet.exe'
$FacetUIExe = Join-Path $LocalBin 'facet-ui.exe'

Push-Location $ScriptDir
try {
    Write-Host '  Building bin/facet.exe...' -ForegroundColor DarkGray
    & go build -o $FacetExe ./cmd/facet
    if ($LASTEXITCODE -ne 0) { throw 'Failed to build cmd/facet' }

    Write-Host '  Building bin/facet-ui.exe...' -ForegroundColor DarkGray
    & go build -o $FacetUIExe ./cmd/facet-ui
    if ($LASTEXITCODE -ne 0) { throw 'Failed to build cmd/facet-ui' }
}
finally {
    Pop-Location
}
Write-Host '  Compiled successfully.' -ForegroundColor Green

# 3. Setup Install Location
if (-not $InstallDir) {
    $InstallDir = Join-Path $env:LOCALAPPDATA 'Programs/Facet/bin'
}
$UserFacetBin = Join-Path $env:USERPROFILE '.facet/bin'
$UserFacetBundle = Join-Path $env:USERPROFILE '.facet/bundle'

foreach ($d in @($InstallDir, $UserFacetBin, $UserFacetBundle)) {
    if (-not (Test-Path $d)) {
        New-Item -ItemType Directory -Path $d -Force | Out-Null
    }
}

Copy-Item -Path $FacetExe -Destination (Join-Path $InstallDir 'facet.exe') -Force
Copy-Item -Path $FacetUIExe -Destination (Join-Path $InstallDir 'facet-ui.exe') -Force
Copy-Item -Path $FacetExe -Destination (Join-Path $UserFacetBin 'facet.exe') -Force
Copy-Item -Path $FacetUIExe -Destination (Join-Path $UserFacetBin 'facet-ui.exe') -Force

# Copy Bundle Knowledge Assets to ~/.facet/bundle
Write-Host '==> Installing central bundle assets...' -ForegroundColor Yellow
$BundleFolders = @('skills', 'pipeline_defs', 'schemas', 'styles')
foreach ($folder in $BundleFolders) {
    $src = Join-Path $ScriptDir $folder
    $dst = Join-Path $UserFacetBundle $folder
    if (Test-Path $src) {
        if (-not (Test-Path $dst)) {
            New-Item -ItemType Directory -Path $dst -Force | Out-Null
        }
        Copy-Item -Path ($src + '/*') -Destination $dst -Recurse -Force
    }
}
Write-Host ('  Installed central bundle to ' + $UserFacetBundle) -ForegroundColor Green

# 4. PATH Configuration
Write-Host '==> Configuring User PATH environment...' -ForegroundColor Yellow
$UserPath = [Environment]::GetEnvironmentVariable('Path', 'User')
if (-not $UserPath) { $UserPath = '' }

$PathsToAdd = @($InstallDir, $UserFacetBin)
$PathModified = $false

foreach ($p in $PathsToAdd) {
    if ($UserPath -notlike ('*' + $p + '*')) {
        $UserPath = $p + ';' + $UserPath
        $PathModified = $true
    }
    if ($env:Path -notlike ('*' + $p + '*')) {
        $env:Path = $p + ';' + $env:Path
    }
}

if ($PathModified) {
    [Environment]::SetEnvironmentVariable('Path', $UserPath, 'User')
    Write-Host '  Updated Windows User PATH in registry.' -ForegroundColor Green
} else {
    Write-Host '  User PATH already contains Facet directories.' -ForegroundColor DarkGray
}

# 5. Discover Agent CLIs on Machine
Write-Host ''
Write-Host '==> [1/3] Discovering installed Agent CLIs...' -ForegroundColor Cyan

$DiscoveredCLIs = @{}

$CheckList = @(
    @{ Name = 'Claude Code'; Executables = @('claude.exe', 'claude.cmd', 'claude.ps1', 'claude'); Key = 'claude' },
    @{ Name = 'OpenCode'; Executables = @('opencode.exe', 'opencode.cmd', 'opencode.ps1', 'opencode'); Key = 'opencode' },
    @{ Name = 'OpenAI Codex'; Executables = @('codex.exe', 'codex.cmd', 'codex.ps1', 'codex'); Key = 'codex' },
    @{ Name = 'GitHub Copilot'; Executables = @('copilot.exe', 'copilot.cmd', 'copilot.ps1', 'copilot', 'github-copilot-cli'); Key = 'copilot' }
)

foreach ($item in $CheckList) {
    $found = $null
    foreach ($exe in $item.Executables) {
        $cmd = Get-Command $exe -ErrorAction SilentlyContinue
        if ($cmd) {
            $found = $cmd.Source
            break
        }
        if ($env:APPDATA) {
            $npmPath = Join-Path $env:APPDATA ('npm/' + $exe)
            if (Test-Path $npmPath) {
                $found = $npmPath
                break
            }
        }
        $localBinPath = Join-Path $env:USERPROFILE ('.local/bin/' + $exe)
        if (Test-Path $localBinPath) {
            $found = $localBinPath
            break
        }
    }
    if ($found) {
        $DiscoveredCLIs[$item.Key] = @{ Name = $item.Name; Path = $found }
        Write-Host ('  [✓] ' + $item.Name + ': Found at ' + $found) -ForegroundColor Green
    } else {
        Write-Host ('  [ ] ' + $item.Name + ': Not found in PATH') -ForegroundColor DarkGray
    }
}

# 6. Skill Scope Selection
Write-Host ''
$ChosenScope = $Scope
if (-not $Quiet -and -not $NonInteractive) {
    Write-Host '==> [2/3] Choose Skill Scope Preference:' -ForegroundColor Cyan
    Write-Host '  [1] On-Demand: Skills active only in projects initialized with facet init (Recommended)' -ForegroundColor White
    Write-Host '  [2] Global: Skills always active across all CLI terminal sessions' -ForegroundColor White
    $choice = Read-Host 'Select option [1/2] (default: 1)'
    if ($choice -eq '2') {
        $ChosenScope = 'global'
    } else {
        $ChosenScope = 'on-demand'
    }
}

if ($ChosenScope -eq 'global') {
    Write-Host '==> Registering global skills for detected CLIs...' -ForegroundColor Yellow
    $GlobalClaudeSkills = Join-Path $env:USERPROFILE '.claude/skills/facet'
    if (-not (Test-Path $GlobalClaudeSkills)) {
        New-Item -ItemType Directory -Path $GlobalClaudeSkills -Force | Out-Null
    }
    Copy-Item -Path ($UserFacetBundle + '/skills/*') -Destination $GlobalClaudeSkills -Recurse -Force
    Write-Host ('  Registered global skill: ' + $GlobalClaudeSkills) -ForegroundColor Green

    $GlobalOpenCodeSkills = Join-Path $env:USERPROFILE '.config/opencode/skills/facet'
    if (-not (Test-Path $GlobalOpenCodeSkills)) {
        New-Item -ItemType Directory -Path $GlobalOpenCodeSkills -Force | Out-Null
    }
    Copy-Item -Path ($UserFacetBundle + '/skills/*') -Destination $GlobalOpenCodeSkills -Recurse -Force
    Write-Host ('  Registered global skill: ' + $GlobalOpenCodeSkills) -ForegroundColor Green
} else {
    Write-Host '  On-demand scope selected. Use facet init to link skills into any workspace.' -ForegroundColor DarkGray
}

# 7. Create Windows Desktop and Start Menu Shortcuts
Write-Host ''
$CreateShortcuts = $true
if (-not $Quiet -and -not $NonInteractive) {
    Write-Host '==> [3/3] Desktop and Start Menu Shortcuts:' -ForegroundColor Cyan
    $scChoice = Read-Host 'Create Desktop and Start Menu shortcuts for Facet Studio? [Y/n] (default: Y)'
    if ($scChoice -and $scChoice.ToLower().StartsWith('n')) {
        $CreateShortcuts = $false
    }
}

if ($CreateShortcuts) {
    try {
        $WshShell = New-Object -ComObject WScript.Shell

        # Desktop Shortcut
        $DesktopPath = [Environment]::GetFolderPath('Desktop')
        $DesktopShortcutPath = Join-Path $DesktopPath 'Facet Studio.lnk'
        $Shortcut = $WshShell.CreateShortcut($DesktopShortcutPath)
        $Shortcut.TargetPath = (Join-Path $InstallDir 'facet-ui.exe')
        $Shortcut.WorkingDirectory = $env:USERPROFILE
        $Shortcut.Description = 'Facet - Autonomous Video Production Studio'
        $Shortcut.Save()
        Write-Host ('  [✓] Created Desktop Shortcut: ' + $DesktopShortcutPath) -ForegroundColor Green

        # Start Menu Shortcut
        $StartMenuPrograms = [Environment]::GetFolderPath('Programs')
        $FacetStartMenu = Join-Path $StartMenuPrograms 'Facet'
        if (-not (Test-Path $FacetStartMenu)) {
            New-Item -ItemType Directory -Path $FacetStartMenu -Force | Out-Null
        }
        $StartMenuShortcutPath = Join-Path $FacetStartMenu 'Facet Studio.lnk'
        $SMShortcut = $WshShell.CreateShortcut($StartMenuShortcutPath)
        $SMShortcut.TargetPath = (Join-Path $InstallDir 'facet-ui.exe')
        $SMShortcut.WorkingDirectory = $env:USERPROFILE
        $SMShortcut.Description = 'Facet - Autonomous Video Production Studio'
        $SMShortcut.Save()
        Write-Host ('  [✓] Created Start Menu Shortcut: ' + $StartMenuShortcutPath) -ForegroundColor Green
    }
    catch {
        Write-Warning ('Could not create Windows shortcuts: ' + $_)
    }
}

# 8. Run Doctor Verification
Write-Host ''
Write-Host '==> Running system doctor verification...' -ForegroundColor Yellow
$FacetInstalled = Join-Path $InstallDir 'facet.exe'
& $FacetInstalled doctor

Write-Host ''
Write-Host '======================================================' -ForegroundColor Green
Write-Host '            Facet Installation Complete!              ' -ForegroundColor Green
Write-Host '======================================================' -ForegroundColor Green
Write-Host 'Commands available in any terminal:' -ForegroundColor White
Write-Host '  - facet doctor             - inspect system runtimes & 33 tools' -ForegroundColor Cyan
Write-Host '  - facet init [my-project]  - initialize workspace and launch agent' -ForegroundColor Cyan
Write-Host '  - facet ui                 - start and open Facet Studio webapp' -ForegroundColor Cyan
Write-Host '  - Double-click Facet Studio on your Desktop to open Studio anytime.' -ForegroundColor Cyan
Write-Host ''
