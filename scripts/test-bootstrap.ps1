# Exercises the real irm | iex dispatch with mocked network and a fixture child.
# No agent settings, dependencies, or product installation are changed.
$ErrorActionPreference = 'Stop'
$repo = Split-Path -Parent $PSScriptRoot
$source = [IO.File]::ReadAllText((Join-Path $repo 'docs/install.ps1'))
$tokens=$null; $errors=$null
[void][Management.Automation.Language.Parser]::ParseInput($source,[ref]$tokens,[ref]$errors)
if ($errors.Count) { throw ($errors | Out-String) }
if (-not $IsWindows) { 'PowerShell bootstrap parses; execution is Windows-only.'; return }
$root = Join-Path ([IO.Path]::GetTempPath()) ('facet-bootstrap-test-' + [guid]::NewGuid().ToString('N'))
$saved = $env:FACET_BOOTSTRAP_TEST
New-Item -ItemType Directory -Path $root | Out-Null
try {
    $archive = Join-Path $root 'fixture.zip'
    $zip = [IO.Compression.ZipFile]::Open($archive,[IO.Compression.ZipArchiveMode]::Create)
    try {
        foreach ($name in @('install.ps1','install.sh','installer/README.md','installer/manifest.tsv','installer/verify.html')) {
            $entry = $zip.CreateEntry($name)
            $writer = [IO.StreamWriter]::new($entry.Open())
            if ($name -eq 'install.ps1') {
                $writer.Write('if ((Read-Host "Fixture prompt") -ne "answer") { throw "Prompt failed" }; [IO.File]::WriteAllText($env:FACET_BOOTSTRAP_TEST, $PSScriptRoot); if ($env:FACET_BOOTSTRAP_FAIL -eq "1") { throw "Child failure" }')
            } else { $writer.Write('fixture') }
            $writer.Dispose()
        }
    } finally { $zip.Dispose() }
    $hash = (Get-FileHash $archive).Hash.ToLowerInvariant()
    $script:bootstrapSource = $source.Replace('108bcf2353c5b20e09b81189ad7006689464f23083fc5dd88e5bc948c15edc2c',$hash)
    $script:archive = $archive
    $script:lastDownload = ''
    function Invoke-RestMethod { param($Uri); if ($Uri -ne 'https://xibodev.github.io/facet/install.ps1') { throw 'Unexpected bootstrap URL' }; $script:bootstrapSource }
    function Invoke-WebRequest { param($Uri,$OutFile,$TimeoutSec); if ($Uri -ne 'https://github.com/xibodev/facet/releases/download/v1.0.3/facet-installer-1.0.3.zip') { throw 'Unexpected release URL' }; $script:lastDownload=$OutFile; Copy-Item -LiteralPath $script:archive -Destination $OutFile }
    function Read-Host { param($Prompt); 'answer' }
    $env:FACET_BOOTSTRAP_TEST = Join-Path $root 'called'
    irm https://xibodev.github.io/facet/install.ps1 | iex
    if (-not (Test-Path $env:FACET_BOOTSTRAP_TEST)) { throw 'Child never ran' }
    if (Test-Path (Split-Path -Parent $script:lastDownload)) { throw 'Bootstrap temporary directory leaked' }
    Remove-Item $env:FACET_BOOTSTRAP_TEST
    $script:bootstrapSource = $script:bootstrapSource.Replace($hash,('0'*64))
    try { irm https://xibodev.github.io/facet/install.ps1 | iex; throw 'Expected checksum failure' } catch { if ($_.Exception.Message -notmatch 'checksum mismatch') { throw } }
    if ((Test-Path $env:FACET_BOOTSTRAP_TEST) -or (Test-Path (Split-Path -Parent $script:lastDownload))) { throw 'Bad checksum executed child or leaked temporary files' }
    $script:bootstrapSource = $source.Replace('108bcf2353c5b20e09b81189ad7006689464f23083fc5dd88e5bc948c15edc2c',$hash)
    $env:FACET_BOOTSTRAP_FAIL = '1'
    try { irm https://xibodev.github.io/facet/install.ps1 | iex; throw 'Expected child failure' } catch { if ($_.Exception.Message -notmatch 'Child failure') { throw } }
    if (Test-Path (Split-Path -Parent $script:lastDownload)) { throw 'Child failure leaked temporary files' }
    'PASS: irm | iex, interactive host handoff, checksum rejection, child failure, and cleanup.'
} finally {
    $env:FACET_BOOTSTRAP_TEST = $saved
    Remove-Item Env:FACET_BOOTSTRAP_FAIL -ErrorAction SilentlyContinue
    Remove-Item -LiteralPath $root -Recurse -Force
}
