[CmdletBinding()]
param()

$ErrorActionPreference = "Stop"
$projectDir = $PSScriptRoot
$repoRoot = (Resolve-Path (Join-Path $projectDir "../..")).Path
$videokit = Join-Path $repoRoot "bin/videokit.exe"

function Invoke-NativeChecked {
    param(
        [Parameter(Mandatory)] [string] $Program,
        [Parameter(Mandatory)] [string[]] $Arguments
    )

    & $Program @Arguments
    if ($LASTEXITCODE -ne 0) {
        throw "$Program failed with exit code $LASTEXITCODE"
    }
}

function Save-VideoKitJson {
    param(
        [Parameter(Mandatory)] [string] $OutputPath,
        [Parameter(Mandatory)] [string[]] $Arguments
    )

    $json = & $videokit @Arguments | Out-String
    if ($LASTEXITCODE -ne 0) {
        throw "videokit failed with exit code ${LASTEXITCODE}: $($Arguments -join ' ')"
    }
    $parsed = $json | ConvertFrom-Json
    if (-not $parsed.ok) {
        throw "videokit returned a failed envelope: $($Arguments -join ' ')"
    }
    [IO.File]::WriteAllText($OutputPath, $json.TrimEnd() + [Environment]::NewLine, [Text.UTF8Encoding]::new($false))
}

Push-Location $repoRoot
try {
    foreach ($directory in @(
        "bin",
        "projects/phase2-source-edit/assets/source",
        "projects/phase2-source-edit/artifacts/discovery",
        "projects/phase2-source-edit/artifacts/results",
        "projects/phase2-source-edit/renders",
        "projects/phase2-source-edit/review"
    )) {
        New-Item -ItemType Directory -Force -Path $directory | Out-Null
    }

    foreach ($path in @(
        "projects/phase2-source-edit/assets/source/source.mp4",
        "projects/phase2-source-edit/renders/edit.mp4",
        "projects/phase2-source-edit/renders/final.mp4",
        "projects/phase2-source-edit/artifacts/source-review.json",
        "projects/phase2-source-edit/artifacts/results/frame-sample.json",
        "projects/phase2-source-edit/artifacts/results/source-edit-estimate.json",
        "projects/phase2-source-edit/artifacts/results/source-edit.json",
        "projects/phase2-source-edit/artifacts/results/audio-mix-estimate.json",
        "projects/phase2-source-edit/artifacts/results/audio-mix.json",
        "projects/phase2-source-edit/review/report.json"
    )) {
        Remove-Item -LiteralPath $path -Force -ErrorAction SilentlyContinue
    }
    Remove-Item -LiteralPath "projects/phase2-source-edit/review/source-frames" -Recurse -Force -ErrorAction SilentlyContinue
    Remove-Item -LiteralPath "projects/phase2-source-edit/review/final-frames" -Recurse -Force -ErrorAction SilentlyContinue

    Invoke-NativeChecked "ffmpeg" @(
        "-hide_banner", "-loglevel", "error", "-y",
        "-f", "lavfi", "-i", "testsrc2=size=1280x720:rate=30:duration=4",
        "-f", "lavfi", "-i", "sine=frequency=440:sample_rate=48000:duration=4",
        "-f", "lavfi", "-i", "smptebars=size=1280x720:rate=30:duration=4",
        "-f", "lavfi", "-i", "sine=frequency=554.37:sample_rate=48000:duration=4",
        "-f", "lavfi", "-i", "yuvtestsrc=size=1280x720:rate=30:duration=4",
        "-f", "lavfi", "-i", "sine=frequency=659.25:sample_rate=48000:duration=4",
        "-f", "lavfi", "-i", "rgbtestsrc=size=1280x720:rate=30:duration=4",
        "-f", "lavfi", "-i", "sine=frequency=880:sample_rate=48000:duration=4",
        "-filter_complex", "[0:v]format=yuv420p[v0];[2:v]format=yuv420p[v1];[4:v]format=yuv420p[v2];[6:v]format=yuv420p[v3];[v0][1:a][v1][3:a][v2][5:a][v3][7:a]concat=n=4:v=1:a=1[v][a]",
        "-map", "[v]", "-map", "[a]", "-c:v", "libx264", "-pix_fmt", "yuv420p",
        "-c:a", "aac", "-ar", "48000", "-ac", "2", "-movflags", "+faststart",
        "projects/phase2-source-edit/assets/source/source.mp4"
    )

    Invoke-NativeChecked "go" @("build", "-o", "bin/videokit.exe", "./cmd/videokit")

    Save-VideoKitJson "projects/phase2-source-edit/artifacts/discovery/tools-list.json" @("tools", "list")
    foreach ($tool in @("media_probe", "frame_sample", "source_edit", "audio_mix", "output_review")) {
        Save-VideoKitJson "projects/phase2-source-edit/artifacts/discovery/$tool.json" @("tools", "describe", $tool)
    }

    Save-VideoKitJson "projects/phase2-source-edit/artifacts/source-review.json" @(
        "tools", "run", "media_probe", "--input", "projects/phase2-source-edit/artifacts/requests/media-probe.json"
    )
    Save-VideoKitJson "projects/phase2-source-edit/artifacts/results/frame-sample.json" @(
        "tools", "run", "frame_sample", "--input", "projects/phase2-source-edit/artifacts/requests/frame-sample.json"
    )
    Save-VideoKitJson "projects/phase2-source-edit/artifacts/results/source-edit-estimate.json" @(
        "tools", "estimate", "source_edit", "--input", "projects/phase2-source-edit/artifacts/edit.json"
    )
    Save-VideoKitJson "projects/phase2-source-edit/artifacts/results/source-edit.json" @(
        "tools", "run", "source_edit", "--input", "projects/phase2-source-edit/artifacts/edit.json"
    )
    Save-VideoKitJson "projects/phase2-source-edit/artifacts/results/audio-mix-estimate.json" @(
        "tools", "estimate", "audio_mix", "--input", "projects/phase2-source-edit/artifacts/requests/audio-mix.json"
    )
    Save-VideoKitJson "projects/phase2-source-edit/artifacts/results/audio-mix.json" @(
        "tools", "run", "audio_mix", "--input", "projects/phase2-source-edit/artifacts/requests/audio-mix.json"
    )
    Save-VideoKitJson "projects/phase2-source-edit/review/report.json" @(
        "tools", "run", "output_review", "--input", "projects/phase2-source-edit/artifacts/requests/output-review.json"
    )

    $review = Get-Content -Raw "projects/phase2-source-edit/review/report.json" | ConvertFrom-Json
    if ($review.result.review_status -ne "pass") {
        throw "output review status is $($review.result.review_status)"
    }
    if (@($review.result.samples).Count -ne 4) {
        throw "output review did not produce exactly four frames"
    }
} finally {
    Pop-Location
}
