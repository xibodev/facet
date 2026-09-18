# Website bootstrap for: irm https://xibodev.github.io/facet/install.ps1 | iex
# The release's install.ps1 owns host selection and all installation policy.
& {
    $ErrorActionPreference = 'Stop'
    Set-StrictMode -Off
    $PSModuleAutoLoadingPreference = 'All'
    if ($PSVersionTable.PSEdition -ne 'Core') {
        $env:PSModulePath = (Join-Path $env:SystemRoot 'System32/WindowsPowerShell/v1.0/Modules') + ';' + $env:PSModulePath
    }
    Import-Module Microsoft.PowerShell.Utility
    $ProgressPreference = 'SilentlyContinue'
    if ($env:OS -ne 'Windows_NT') { throw 'Use the curl command on Linux/macOS.' }
    Add-Type -AssemblyName System.IO.Compression.FileSystem
    $version = '1.0.4'
    $expected = '1d30c34a0958c50459d2341f5f479f2089c28f931a52e6503d6fceb9b801b3b7'
    $url = "https://github.com/xibodev/facet/releases/download/v$version/facet-installer-$version.zip"
    $temp = Join-Path ([IO.Path]::GetTempPath()) ('facet-bootstrap-' + [guid]::NewGuid().ToString('N'))
    New-Item -ItemType Directory -Path $temp | Out-Null
    try {
        Write-Host "Downloading Facet $version installer..."
        $archive = Join-Path $temp 'installer.zip'
        for ($attempt=1; $attempt -le 3; $attempt++) {
            try { Invoke-WebRequest -UseBasicParsing -Uri $url -OutFile $archive -TimeoutSec 180; break }
            catch { if ($attempt -eq 3) { throw }; Write-Host "Download interrupted; retrying ($attempt/3)..."; Start-Sleep -Seconds ($attempt*2) }
        }
        if ((Get-Item -LiteralPath $archive).Length -gt 1MB -or (Get-FileHash -LiteralPath $archive -Algorithm SHA256).Hash -ne $expected) {
            throw 'Installer checksum mismatch; nothing executed.'
        }
        $zip = [IO.Compression.ZipFile]::OpenRead($archive)
        try {
            $names = @($zip.Entries | ForEach-Object FullName | Sort-Object)
            $allowed = @('install.ps1','install.sh','installer/README.md','installer/manifest.tsv','installer/verify.html') | Sort-Object
            if ($names.Count -ne $allowed.Count -or (Compare-Object $names $allowed -CaseSensitive)) { throw 'Unexpected installer package layout.' }
        } finally { $zip.Dispose() }
        $package = Join-Path $temp 'package'
        Expand-Archive -LiteralPath $archive -DestinationPath $package
        # Invoke in this host so Read-Host remains interactive under irm | iex.
        $options = @{}
        foreach ($pair in @(@('FACET_TARGET','Target'),@('FACET_PROJECT','ProjectDir'),@('FACET_INSTALL_DIR','InstallDir'),@('FACET_COMPONENTS','Components'))) {
            $value = [Environment]::GetEnvironmentVariable($pair[0])
            if ($value) { $options[$pair[1]]=$value }
        }
        if ($env:FACET_YES -eq '1') { $options.NonInteractive=$true }
        if ($env:FACET_ACTION -and ([IO.File]::ReadAllText((Join-Path $package 'install.ps1')) -match '\[string\]\$Action')) { $options.Action=$env:FACET_ACTION }
        if ($PSVersionTable.PSVersion.Major -lt 7) {
            # Older published packages can require pwsh. Prefer an existing
            # compatible host; do not silently install another shell.
            $requires = [IO.File]::ReadAllText((Join-Path $package 'install.ps1')) -match '(?m)^#requires -Version 7'
            if ($requires) {
                $pwsh = Get-Command pwsh -ErrorAction SilentlyContinue
                if (-not $pwsh) { throw 'This published package requires PowerShell 7. The next installer release supports Windows PowerShell directly; meanwhile run in pwsh or use the manual package.' }
                & $pwsh.Source -NoProfile -File (Join-Path $package 'install.ps1') @options
                if ($LASTEXITCODE -ne 0) { throw "Installer exited with $LASTEXITCODE" }
            } else { & (Join-Path $package 'install.ps1') @options }
        } else { & (Join-Path $package 'install.ps1') @options }
    } finally {
        Remove-Item -LiteralPath $temp -Recurse -Force
    }
}
