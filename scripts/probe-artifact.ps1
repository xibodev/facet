# Custom artifact probe for Facet & Release-Harness (PowerShell)
param(
    [string]$FilePath = "renders/final.mp4"
)

if (-not (Test-Path $FilePath)) {
    Write-Error "Error: Rendered artifact $FilePath does not exist"
    exit 1
}

$fileInfo = Get-Item $FilePath
if ($fileInfo.Length -gt 1000) {
    Write-Output "Artifact $FilePath is valid ($($fileInfo.Length) bytes)"
    exit 0
} else {
    Write-Error "Error: File $FilePath size is too small ($($fileInfo.Length) bytes)"
    exit 1
}
