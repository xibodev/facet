param([string]$OutDir = 'dist')
$ErrorActionPreference = 'Stop'
$repo = Split-Path $PSScriptRoot -Parent
$out = $ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath($OutDir)
New-Item -ItemType Directory -Force -Path $out | Out-Null
Push-Location $repo
try {
    & go build -o (Join-Path $out 'xibodev.facet.exe') ./cmd/facet
    if ($LASTEXITCODE -ne 0) { throw 'Module build failed' }
    foreach ($item in @('agents/facet-creative.md', 'skills/facet', 'packs', 'remotion-composer/src')) {
        $parent = Join-Path $out (Split-Path $item -Parent)
        New-Item -ItemType Directory -Force -Path $parent | Out-Null
        Copy-Item -LiteralPath (Join-Path $repo $item) -Destination $parent -Recurse -Force
    }
    foreach ($item in @('package.json', 'package-lock.json', 'tsconfig.json', 'legacy-composer-manifest.json')) {
        Copy-Item -LiteralPath (Join-Path $repo "remotion-composer/$item") -Destination (Join-Path $out 'remotion-composer') -Force
    }
    $public = Join-Path $repo 'remotion-composer/public'
    if (Test-Path -LiteralPath $public) {
        Copy-Item -LiteralPath $public -Destination (Join-Path $out 'remotion-composer') -Recurse -Force
    }
    & (Join-Path $out 'xibodev.facet.exe') module describe --json | Out-Null
    if ($LASTEXITCODE -ne 0) { throw 'Module descriptor failed' }
    Write-Output "Built $out/xibodev.facet.exe with declared content. Run npm ci in $out/remotion-composer before rendering."
} finally { Pop-Location }
