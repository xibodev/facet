# Website bootstrap for: irm https://xibodev.github.io/facet/install.ps1 | iex
# The release's install.ps1 owns host selection and all installation policy.
& {
    $ErrorActionPreference = 'Stop'
    if ($PSVersionTable.PSVersion.Major -lt 7) {
        throw 'Run this command in PowerShell 7 (pwsh).'
    }
    if (-not $IsWindows) { throw 'Use the curl command on Linux/macOS.' }
    $version = '1.0.3'
    $expected = '108bcf2353c5b20e09b81189ad7006689464f23083fc5dd88e5bc948c15edc2c'
    $url = "https://github.com/xibodev/facet/releases/download/v$version/facet-installer-$version.zip"
    $temp = Join-Path ([IO.Path]::GetTempPath()) ('facet-bootstrap-' + [guid]::NewGuid().ToString('N'))
    New-Item -ItemType Directory -Path $temp | Out-Null
    try {
        Write-Host "Downloading Facet $version installer…"
        $archive = Join-Path $temp 'installer.zip'
        Invoke-WebRequest -Uri $url -OutFile $archive -TimeoutSec 180
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
        & (Join-Path $package 'install.ps1')
    } finally {
        Remove-Item -LiteralPath $temp -Recurse -Force
    }
}
