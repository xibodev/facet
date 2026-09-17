#requires -Version 7.0
# Parser/contract checks only. Native installer coverage is internal/installer.
$ErrorActionPreference = 'Stop'
$path = Join-Path (Split-Path -Parent $PSScriptRoot) 'install-release.ps1'
$tokens = $null; $errors = $null
[void][Management.Automation.Language.Parser]::ParseFile($path, [ref]$tokens, [ref]$errors)
if ($errors.Count) { throw ($errors | Out-String) }
try {
    & $path -NonInteractive
    throw 'Expected missing-arguments failure.'
} catch {
    if ($_.Exception.Message -notmatch 'requires -Target and -ProjectDir|Download the native') { throw }
}
'Bootstrap parser and noninteractive preflight passed.'
