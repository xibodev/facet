# Parse the script-owned Windows installer and check preflight without writes.
$ErrorActionPreference = 'Stop'
$path = Join-Path (Split-Path -Parent $PSScriptRoot) 'install.ps1'
$tokens=$null; $errors=$null
[void][Management.Automation.Language.Parser]::ParseFile($path,[ref]$tokens,[ref]$errors)
if ($errors.Count) { throw ($errors | Out-String) }
try { & $path -NonInteractive; throw 'Expected preflight failure.' } catch {
    if ($_.Exception.Message -notmatch 'requires -Target and -ProjectDir|Use install.sh') { throw }
}
'Script installer parser and preflight passed.'
