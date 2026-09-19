# Opt-in real build/npm validation. All installation, caches and project writes use a new temp root.
param([switch]$KeepArtifacts)
$ErrorActionPreference = 'Stop'
if ($env:OS -ne 'Windows_NT') { throw 'Windows integration test requires Windows.' }
$installer = Join-Path $PSScriptRoot 'install-source.ps1'
$root = Join-Path ([IO.Path]::GetTempPath()) ('facet-install-integration-' + [guid]::NewGuid())
Write-Host "Isolated test root: $root"
$saved = @{}
$shortcutHashes = @{}
$userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
$processPath = $env:Path
try {
    New-Item -ItemType Directory -Path $root | Out-Null
    $homeDir = Join-Path $root 'home'
    $bin = Join-Path $root 'install/bin'
    $bundle = Join-Path $root 'install/bundle'
    $work = Join-Path $root 'work'
    foreach ($shortcut in @((Join-Path ([Environment]::GetFolderPath('Desktop')) 'Facet Studio.lnk'), (Join-Path ([Environment]::GetFolderPath('Programs')) 'Facet/Facet Studio.lnk'))) {
        $shortcutHashes[$shortcut] = if (Test-Path -LiteralPath $shortcut) { (Get-FileHash -LiteralPath $shortcut).Hash } else { $null }
    }
    New-Item -ItemType Directory -Path "$homeDir/.config/facet", $work -Force | Out-Null
    $config = Join-Path $homeDir '.config/facet/config.yaml'
    [IO.File]::WriteAllText($config, "defaults:`n  voice: keep-custom-voice`n")
    $originalConfig = [IO.File]::ReadAllText($config)
    Push-Location $work
    try {
        & $installer -NonInteractive -NoPath -NoShortcuts -Isolated -HomeDir $homeDir -InstallDir $bin
        foreach ($file in @('skills/facet/SKILL.md', 'packs/explainer/SKILL.md', 'remotion-composer/package-lock.json', 'remotion-composer/node_modules/remotion/package.json')) {
            if (-not (Test-Path -LiteralPath (Join-Path $bundle $file) -PathType Leaf)) { throw "Missing installed file: $file" }
        }
        if ([IO.File]::ReadAllText($config) -cne $originalConfig) { throw 'User config was modified.' }
        if ([Environment]::GetEnvironmentVariable('Path', 'User') -cne $userPath -or $env:Path -cne $processPath) { throw 'PATH was modified.' }
        foreach ($shortcut in $shortcutHashes.Keys) {
            $hash = if (Test-Path -LiteralPath $shortcut) { (Get-FileHash -LiteralPath $shortcut).Hash } else { $null }
            if ($hash -cne $shortcutHashes[$shortcut]) { throw "Shortcut changed: $shortcut" }
        }
        foreach ($key in @('USERPROFILE', 'HOME', 'APPDATA', 'LOCALAPPDATA')) {
            $saved[$key] = [Environment]::GetEnvironmentVariable($key, 'Process')
            [Environment]::SetEnvironmentVariable($key, $homeDir, 'Process')
        }
        $facet = Join-Path $bin 'facet.exe'
        & $facet init --help
        if ($LASTEXITCODE -ne 0 -or @(Get-ChildItem -LiteralPath $work -Force).Count) { throw 'CLI help failed or wrote files.' }
        & $facet init project --engine opencode --pack explainer --no-launch
        if ($LASTEXITCODE -ne 0) { throw 'Headless initialization failed.' }
        foreach ($file in @('.opencode/skills/facet/SKILL.md', '.opencode/skills/explainer/SKILL.md', '.facet.yaml')) {
            if (-not (Test-Path -LiteralPath (Join-Path $work "project/$file") -PathType Leaf)) { throw "Missing project file: $file" }
        }
        $projectConfig = [IO.File]::ReadAllText((Join-Path $work 'project/.facet.yaml'))
        if ($projectConfig -notmatch 'keep-custom-voice' -or $projectConfig -notlike '*install*bundle*remotion-composer*') { throw 'Installed runtime discovery or config preservation failed.' }
        & $facet doctor
        if ($LASTEXITCODE -ne 0) { throw 'Doctor command failed (not a render gate).' }
        & node -e "const p=process.argv[1]; for (const m of ['remotion','@remotion/cli','@remotion/renderer']) console.log(require.resolve(m,{paths:[p]})); require(require.resolve('esbuild',{paths:[p]})).transformSync('const n: number = 1', {loader:'ts'});" (Join-Path $bundle 'remotion-composer')
        if ($LASTEXITCODE -ne 0) { throw 'Installed Remotion dependency resolution failed.' }
        try {
            & $installer -NonInteractive -NoPath -NoShortcuts -InstallDir $bin
            throw 'Occupied destination unexpectedly accepted.'
        } catch {
            if ($_.Exception.Message -notmatch 'Refusing to overwrite') { throw }
        }
        Write-Host "PASS: real Windows source install, isolated paths/caches, headless init, config preservation, Remotion resolution. Artifacts: $root"
    } finally { Pop-Location }
} finally {
    foreach ($key in $saved.Keys) { [Environment]::SetEnvironmentVariable($key, $saved[$key], 'Process') }
    if (-not $KeepArtifacts) { Remove-Item -LiteralPath $root -Recurse -Force }
}
