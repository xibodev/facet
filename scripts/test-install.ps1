# Parser and preflight regression tests; no dependencies, builds or user-profile writes.
param()
$ErrorActionPreference = 'Stop'
$installer = Join-Path (Split-Path -Parent $PSScriptRoot) 'install.ps1'
$tokens = $null
$errors = $null
$ast = [System.Management.Automation.Language.Parser]::ParseFile($installer, [ref]$tokens, [ref]$errors)
if ($errors.Count) { throw ($errors | Out-String) }
$content = [IO.File]::ReadAllText($installer)
try {
    & ([scriptblock]::Create($content)) -Quiet -NonInteractive
    throw 'Piped invocation unexpectedly succeeded.'
} catch {
    if ($_.Exception.Message -notmatch 'complete source checkout') { throw }
}

$root = Join-Path ([IO.Path]::GetTempPath()) ('facet-install-preflight-' + [guid]::NewGuid())
$savedPath = $env:Path
$savedOS = $env:OS
try {
    New-Item -ItemType Directory -Path $root | Out-Null
    $copy = Join-Path $root 'install.ps1'
    Copy-Item -LiteralPath $installer -Destination $copy
    try {
        & $copy -Quiet -NonInteractive
        throw 'Incomplete checkout unexpectedly succeeded.'
    } catch {
        if ($_.Exception.Message -notmatch 'complete source checkout') { throw }
    }
    $env:OS = 'Windows_NT'
    $homeDir = Join-Path $root 'home'
    $bin = Join-Path $root 'install/bin'
    $flags = @{ Quiet = $true; NonInteractive = $true; Isolated = $true; HomeDir = $homeDir; InstallDir = $bin; NoPath = $true; NoShortcuts = $true }
    foreach ($conflict in @(@{ NoShortcuts = $true }, @{ Isolated = $true })) {
        try {
            & $installer -UpdateShortcuts @conflict
            throw 'Conflicting shortcut flags unexpectedly succeeded.'
        } catch {
            if ($_.Exception.Message -notmatch '-UpdateShortcuts cannot be combined') { throw }
        }
    }
    $env:Path = ''
    try {
        & $installer @flags
        throw 'Missing prerequisite unexpectedly succeeded.'
    } catch {
        if ($_.Exception.Message -notmatch 'Missing prerequisite: go') { throw }
    }
    if (Test-Path -LiteralPath $bin) { throw 'Preflight wrote install directory.' }

    # Functions are local mocks. None of these tests invokes a compiler or package manager.
    function go { $global:LASTEXITCODE = 0; 'go version go1.24.9 windows/amd64' }
    function node { $global:LASTEXITCODE = 0; 'v22.0.0' }
    function npm { $global:LASTEXITCODE = 0; '10.0.0' }
    function ffmpeg { $global:LASTEXITCODE = 0 }
    function ffprobe { $global:LASTEXITCODE = 0 }
    try {
        & $installer @flags
        throw 'Old Go unexpectedly succeeded.'
    } catch {
        if ($_.Exception.Message -notmatch 'Go 1.25\+ is required') { throw }
    }
    function go { $global:LASTEXITCODE = 0; 'go version go1.25.0 windows/amd64' }
    function node { $global:LASTEXITCODE = 0; 'v16.20.0' }
    try {
        & $installer @flags
        throw 'Old Node unexpectedly succeeded.'
    } catch {
        if ($_.Exception.Message -notmatch 'Node.js 18\+ is required') { throw }
    }
    function node { $global:LASTEXITCODE = 0; 'v22.0.0' }
    function ffprobe { $global:LASTEXITCODE = 5 }
    try {
        & $installer @flags
        throw 'Broken ffprobe unexpectedly succeeded.'
    } catch {
        if ($_.Exception.Message -notmatch 'ffprobe prerequisite check failed') { throw }
    }
    $flags.NoPath = $false
    try {
        & $installer @flags
        throw 'Unsafe isolated flags unexpectedly succeeded.'
    } catch {
        if ($_.Exception.Message -notmatch '-Isolated requires') { throw }
    }
    $flags.NoPath = $true
    $flags.Scope = 'global'
    try {
        & $installer @flags
        throw 'Global isolated scope unexpectedly succeeded.'
    } catch {
        if ($_.Exception.Message -notmatch '-Isolated requires') { throw }
    }
    if ((Test-Path -LiteralPath $bin) -or (Test-Path -LiteralPath $homeDir)) { throw 'Preflight wrote installation or home files.' }
    Write-Host 'PASS: parser, piped source, incomplete checkout, prerequisites, isolated safety and no preflight writes.'

    # Load only the production shortcut function, never the install body or real COM.
    $functionAst = $ast.Find({ param($node) $node -is [System.Management.Automation.Language.FunctionDefinitionAst] -and $node.Name -eq 'Set-FacetShortcut' }, $true)
    if (-not $functionAst) { throw 'Shortcut function missing.' }
    . ([scriptblock]::Create($functionAst.Extent.Text))
    $local = $env:LOCALAPPDATA
    if (-not $local) { $local = Join-Path $homeDir 'AppData/Local' }
    $executable = Join-Path $bin 'facet-ui.exe'
    $legacy = Join-Path $local 'Programs/Facet/bin/facet-ui.exe'
    $current = Join-Path $homeDir '.facet/bin/facet-ui.exe'
    $cases = @(
        @{ Name = 'new'; Existing = $false; OldTarget = ''; Update = $false; Saves = 1 },
        @{ Name = 'legacy-preserved'; Existing = $true; OldTarget = $legacy; Update = $false; Saves = 0 },
        @{ Name = 'current-preserved'; Existing = $true; OldTarget = $current; Update = $false; Saves = 0 },
        @{ Name = 'legacy-approved'; Existing = $true; OldTarget = $legacy; Update = $true; Saves = 1 },
        @{ Name = 'current-approved'; Existing = $true; OldTarget = $current; Update = $true; Saves = 1 },
        @{ Name = 'same-approved'; Existing = $true; OldTarget = $executable; Update = $true; Saves = 1 },
        @{ Name = 'arbitrary-preserved'; Existing = $true; OldTarget = (Join-Path $root 'other/app.exe'); Update = $false; Saves = 0 },
        @{ Name = 'arbitrary-opt-in-preserved'; Existing = $true; OldTarget = (Join-Path $root 'other/app.exe'); Update = $true; Saves = 0 },
        @{ Name = 'custom-facet-preserved'; Existing = $true; OldTarget = (Join-Path $root 'custom/facet-ui.exe'); Update = $true; Saves = 0 },
        @{ Name = 'custom-arguments-preserved'; Existing = $true; OldTarget = $legacy; Arguments = '--dir custom'; Update = $true; Saves = 0 },
        @{ Name = 'unreadable-preserved'; Existing = $true; OldTarget = ''; Unreadable = $true; Update = $true; Saves = 0 }
    )
    foreach ($case in $cases) {
        $target = Join-Path $root ("shortcuts/$($case.Name)/Facet Studio.lnk")
        if ($case.Existing) {
            New-Item -ItemType Directory -Path (Split-Path -Parent $target) -Force | Out-Null
            [IO.File]::WriteAllText($target, 'user shortcut sentinel')
        }
        $link = [pscustomobject]@{ TargetPath = $case.OldTarget; Arguments = $case.Arguments; WorkingDirectory = 'keep-work'; Description = 'keep-description'; Saves = 0 }
        $link | Add-Member ScriptMethod Save { $this.Saves++ }
        $shell = [pscustomobject]@{ Link = $link; ExpectedTarget = $target; Calls = 0; Unreadable = $case.Unreadable }
        $shell | Add-Member ScriptMethod CreateShortcut {
            param($path)
            if ($path -cne $this.ExpectedTarget) { throw 'Unexpected shortcut path.' }
            $this.Calls++
            if ($this.Unreadable) { throw 'Unreadable user shortcut.' }
            return $this.Link
        }
        $warnings = @(Set-FacetShortcut $shell $target $bin $homeDir -UpdateShortcuts:$case.Update 3>&1)
        $expectedCalls = if (-not $case.Existing -or $case.Update) { 1 } else { 0 }
        if ($shell.Calls -ne $expectedCalls -or $link.Saves -ne $case.Saves) { throw "Shortcut save mismatch: $($case.Name)" }
        if ($case.Saves) {
            if ($link.TargetPath -cne $executable -or $link.WorkingDirectory -cne $homeDir -or $link.Description -cne 'Facet Studio' -or $warnings.Count) { throw "Shortcut update failed: $($case.Name)" }
        } else {
            if ($link.TargetPath -cne $case.OldTarget -or $link.Arguments -cne $case.Arguments -or $link.WorkingDirectory -cne 'keep-work' -or $link.Description -cne 'keep-description') { throw "User shortcut mutated: $($case.Name)" }
            if ($warnings.Count -ne 1 -or "$warnings" -notmatch '-UpdateShortcuts' -or -not "$warnings".Contains("& '$executable'")) { throw "Missing actionable shortcut warning: $($case.Name)" }
        }
        if ($case.Existing -and [IO.File]::ReadAllText($target) -cne 'user shortcut sentinel') { throw 'Fixture shortcut file mutated.' }
    }
    Write-Host 'PASS: 11 mocked COM shortcut cases; default preservation, approved migration, custom/unreadable link protection and actionable warnings. No user Desktop changes.'
} finally {
    $env:Path = $savedPath
    $env:OS = $savedOS
    Remove-Item -LiteralPath $root -Recurse -Force
}
