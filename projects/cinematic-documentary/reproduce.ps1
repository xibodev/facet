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
    Remove-Item -LiteralPath "projects/cinematic-documentary/review/final-frames" -Recurse -Force -ErrorAction SilentlyContinue
    New-Item -ItemType Directory -Force -Path "projects/cinematic-documentary/review/final-frames" | Out-Null

    # 1. Ensure build
    Invoke-NativeChecked "go" @("build", "-o", "bin/videokit.exe", "./cmd/videokit")

    # 2. Discovery
    Save-VideoKitJson "projects/cinematic-documentary/artifacts/discovery/tools-list.json" @("tools", "list")
    foreach ($tool in @("edge_tts", "wikimedia", "color_grade", "video_stitch", "audio_mix", "output_review")) {
        Save-VideoKitJson "projects/cinematic-documentary/artifacts/discovery/$tool.json" @("tools", "describe", $tool)
    }

    # 3. Voiceover synthesis
    Save-VideoKitJson "projects/cinematic-documentary/artifacts/results/edge-tts.json" @(
        "tools", "run", "edge_tts", "--input", "projects/cinematic-documentary/artifacts/requests/edge-tts.json"
    )

    # 4. Media acquisition (if raw files missing)
    if (-not (Test-Path "projects/cinematic-documentary/assets/raw/voyager_spacecraft.jpg")) {
        Save-VideoKitJson "projects/cinematic-documentary/artifacts/results/wikimedia-spacecraft.json" @(
            "tools", "run", "wikimedia", "--input", "projects/cinematic-documentary/artifacts/requests/wikimedia-spacecraft.json"
        )
    }
    if (-not (Test-Path "projects/cinematic-documentary/assets/raw/golden_record.jpg")) {
        Save-VideoKitJson "projects/cinematic-documentary/artifacts/results/wikimedia-record.json" @(
            "tools", "run", "wikimedia", "--input", "projects/cinematic-documentary/artifacts/requests/wikimedia-record.json"
        )
    }
    if (-not (Test-Path "projects/cinematic-documentary/assets/raw/earth_blue_marble.jpg")) {
        Save-VideoKitJson "projects/cinematic-documentary/artifacts/results/wikimedia-earth.json" @(
            "tools", "run", "wikimedia", "--input", "projects/cinematic-documentary/artifacts/requests/wikimedia-earth.json"
        )
    }
    if (-not (Test-Path "projects/cinematic-documentary/assets/raw/deep_space.jpg")) {
        Save-VideoKitJson "projects/cinematic-documentary/artifacts/results/wikimedia-deep-space.json" @(
            "tools", "run", "wikimedia", "--input", "projects/cinematic-documentary/artifacts/requests/wikimedia-deep-space.json"
        )
    }

    # 5. Motion video generation from raw stills
    Invoke-NativeChecked "ffmpeg" @(
        "-hide_banner", "-loglevel", "error", "-y",
        "-i", "projects/cinematic-documentary/assets/raw/voyager_spacecraft.jpg",
        "-vf", "scale=1920:1080:force_original_aspect_ratio=increase,crop=1920:1080,zoompan=z='min(zoom+0.001,1.15)':x='iw/2-(iw/zoom/2)':y='ih/2-(ih/zoom/2)':d=135:s=1920x1080:fps=30,format=yuv420p",
        "-c:v", "libx264", "-preset", "fast", "-pix_fmt", "yuv420p", "-t", "4.5",
        "projects/cinematic-documentary/assets/video/shot1_raw.mp4"
    )
    Invoke-NativeChecked "ffmpeg" @(
        "-hide_banner", "-loglevel", "error", "-y",
        "-i", "projects/cinematic-documentary/assets/raw/golden_record.jpg",
        "-vf", "scale=1920:1080:force_original_aspect_ratio=increase,crop=1920:1080,zoompan=z='min(zoom+0.0008,1.12)':x='iw/2-(iw/zoom/2)':y='ih/2-(ih/zoom/2)':d=150:s=1920x1080:fps=30,format=yuv420p",
        "-c:v", "libx264", "-preset", "fast", "-pix_fmt", "yuv420p", "-t", "5.0",
        "projects/cinematic-documentary/assets/video/shot2_raw.mp4"
    )
    Invoke-NativeChecked "ffmpeg" @(
        "-hide_banner", "-loglevel", "error", "-y",
        "-i", "projects/cinematic-documentary/assets/raw/earth_blue_marble.jpg",
        "-vf", "scale=1920:1080:force_original_aspect_ratio=increase,crop=1920:1080,zoompan=z='min(zoom+0.0009,1.13)':x='iw/2-(iw/zoom/2)':y='ih/2-(ih/zoom/2)':d=135:s=1920x1080:fps=30,format=yuv420p",
        "-c:v", "libx264", "-preset", "fast", "-pix_fmt", "yuv420p", "-t", "4.5",
        "projects/cinematic-documentary/assets/video/shot3_raw.mp4"
    )
    Invoke-NativeChecked "ffmpeg" @(
        "-hide_banner", "-loglevel", "error", "-y",
        "-i", "projects/cinematic-documentary/assets/raw/deep_space.jpg",
        "-vf", "scale=1920:1080:force_original_aspect_ratio=increase,crop=1920:1080,zoompan=z='min(zoom+0.0007,1.10)':x='iw/2-(iw/zoom/2)':y='ih/2-(ih/zoom/2)':d=135:s=1920x1080:fps=30,format=yuv420p",
        "-c:v", "libx264", "-preset", "fast", "-pix_fmt", "yuv420p", "-t", "4.5",
        "projects/cinematic-documentary/assets/video/shot4_raw.mp4"
    )

    # 6. Ambient soundtrack generation
    Invoke-NativeChecked "ffmpeg" @(
        "-hide_banner", "-loglevel", "error", "-y",
        "-f", "lavfi", "-i", "sine=frequency=155.56:duration=20",
        "-f", "lavfi", "-i", "sine=frequency=233.08:duration=20",
        "-f", "lavfi", "-i", "sine=frequency=311.13:duration=20",
        "-f", "lavfi", "-i", "sine=frequency=466.16:duration=20",
        "-filter_complex", "[0:a]volume=0.25[a0];[1:a]volume=0.20[a1];[2:a]volume=0.18[a2];[3:a]volume=0.12[a3];[a0][a1][a2][a3]amix=inputs=4:normalize=0,lowpass=f=800,aecho=0.8:0.88:60:0.4,afade=t=in:st=0:d=2,afade=t=out:st=16:d=4[a]",
        "-map", "[a]", "-c:a", "pcm_s16le", "-ar", "48000", "-ac", "2",
        "projects/cinematic-documentary/assets/audio/ambient_music.wav"
    )

    # 7. Color grading via color_grade tool
    Save-VideoKitJson "projects/cinematic-documentary/artifacts/results/color-grade-shot1.json" @(
        "tools", "run", "color_grade", "--input", "projects/cinematic-documentary/artifacts/requests/color-grade-shot1.json"
    )
    Save-VideoKitJson "projects/cinematic-documentary/artifacts/results/color-grade-shot2.json" @(
        "tools", "run", "color_grade", "--input", "projects/cinematic-documentary/artifacts/requests/color-grade-shot2.json"
    )
    Save-VideoKitJson "projects/cinematic-documentary/artifacts/results/color-grade-shot3.json" @(
        "tools", "run", "color_grade", "--input", "projects/cinematic-documentary/artifacts/requests/color-grade-shot3.json"
    )
    Save-VideoKitJson "projects/cinematic-documentary/artifacts/results/color-grade-shot4.json" @(
        "tools", "run", "color_grade", "--input", "projects/cinematic-documentary/artifacts/requests/color-grade-shot4.json"
    )

    # 8. Assembly via video_stitch tool
    Save-VideoKitJson "projects/cinematic-documentary/artifacts/results/video-stitch-validate.json" @(
        "tools", "run", "video_stitch", "--input", "projects/cinematic-documentary/artifacts/requests/video-stitch-validate.json"
    )
    Save-VideoKitJson "projects/cinematic-documentary/artifacts/results/video-stitch.json" @(
        "tools", "run", "video_stitch", "--input", "projects/cinematic-documentary/artifacts/requests/video-stitch.json"
    )

    # 9. Attach voiceover to montage video
    Invoke-NativeChecked "ffmpeg" @(
        "-hide_banner", "-loglevel", "error", "-y",
        "-i", "projects/cinematic-documentary/renders/montage.mp4",
        "-i", "projects/cinematic-documentary/assets/audio/voiceover.mp3",
        "-map", "0:v:0", "-map", "1:a:0", "-c:v", "copy", "-c:a", "aac", "-ar", "48000", "-ac", "2",
        "projects/cinematic-documentary/renders/montage_vo.mp4"
    )

    # 10. Sound design & mixing via audio_mix tool
    Save-VideoKitJson "projects/cinematic-documentary/artifacts/results/audio-mix-estimate.json" @(
        "tools", "estimate", "audio_mix", "--input", "projects/cinematic-documentary/artifacts/requests/audio-mix.json"
    )
    Save-VideoKitJson "projects/cinematic-documentary/artifacts/results/audio-mix.json" @(
        "tools", "run", "audio_mix", "--input", "projects/cinematic-documentary/artifacts/requests/audio-mix.json"
    )

    # 11. Technical review via output_review tool
    Save-VideoKitJson "projects/cinematic-documentary/review/report.json" @(
        "tools", "run", "output_review", "--input", "projects/cinematic-documentary/artifacts/requests/output-review.json"
    )

    $review = Get-Content -Raw "projects/cinematic-documentary/review/report.json" | ConvertFrom-Json
    if ($review.result.review_status -ne "pass") {
        throw "output review status is $($review.result.review_status)"
    }
    if (@($review.result.samples).Count -ne 4) {
        throw "output review did not produce exactly four frames"
    }
    Write-Host "Production reproduction and review passed successfully!"
} finally {
    Pop-Location
}
