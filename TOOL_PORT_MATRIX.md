# Phase 1 Tool Port Matrix

## Scope And Method

This is the complete but intentionally shallow Phase 1 ledger for clean upstream
OpenMontage at commit `cd9f3c1f03368be87b140af494914b8ee4e3c7a4`.
The census inspected only relative paths, top-level class declarations, inheritance,
and imports under `tools/`; it did not inspect behavior, tests, Facet, Backlot, or
`video-agent-bundle`. Rough dependencies are import-level hints, not verified
runtime contracts. Backlot is outside this census and is not counted.

The 136 concrete identities comprise 117 non-selector `BaseTool` subclasses, 16
stock-source adapters, and 3 shared clients. Four selector `BaseTool` subclasses
are listed separately because they may only become transparent capability facts,
mechanical eligibility checks, and explainable ranking signals. They must not
silently choose or execute a provider; Claude retains the choice.

Priority uses exactly `first vertical slice`, `first release`, `later`, or
`exclude`. `Later` is a deferral, not a promise. In particular, local GPU tools
remain `later` or `exclude` and do not shape the headless first release.

## Family Counts

| Capability family | Concrete identities |
|---|---:|
| Analysis and inspection | 14 |
| Audio and speech | 20 |
| Avatar and lip sync | 4 |
| Capture | 2 |
| Character animation | 6 |
| Enhancement | 6 |
| Graphics and image | 24 |
| Publication and delivery | 1 |
| Subtitles | 1 |
| Video, stock, and composition | 55 |
| Shared provider clients | 3 |
| **Concrete total** | **136** |

Selectors are excluded from those family counts: audio 1, capture 1, graphics 1,
and video 1, for a separate total of 4.

## Concrete Identities

<!-- ledger:concrete:start -->
| Name | Capability family | Upstream relative path | Rough dependencies | Proposed treatment | Priority |
|---|---|---|---|---|---|
| `AudioEnergy` | analysis and inspection | `tools/analysis/audio_energy.py` | FFmpeg/ffprobe, JSON | port as deterministic audio analysis | first release |
| `AudioProbe` | analysis and inspection | `tools/analysis/audio_probe.py` | ffprobe, subprocess, JSON | port into `media_probe` | first vertical slice |
| `AzureSpeechToText` | analysis and inspection | `tools/analysis/azure_stt.py` | Azure Speech HTTP, credential | port cloud STT contract | first release |
| `CompositionValidator` | analysis and inspection | `tools/analysis/composition_validator.py` | `AudioProbe`, media files | port into `output_review` | first vertical slice |
| `DashscopeAsr` | analysis and inspection | `tools/analysis/dashscope_asr.py` | DashScope HTTP, credential | port cloud STT contract | later |
| `FaceTracker` | analysis and inspection | `tools/analysis/face_tracker.py` | media analysis runtime | defer until reframing need | later |
| `FrameSampler` | analysis and inspection | `tools/analysis/frame_sampler.py` | FFmpeg, JSON | port into `frame_sample` | first vertical slice |
| `SceneDetect` | analysis and inspection | `tools/analysis/scene_detect.py` | FFmpeg/media analysis | port deterministic scene facts | first release |
| `Transcriber` | analysis and inspection | `tools/analysis/transcriber.py` | local transcription runtime | defer local model execution | later |
| `TranscriptFetcher` | analysis and inspection | `tools/analysis/transcript_fetcher.py` | network transcript source | port source transcript fetch | first release |
| `VideoAnalyzer` | analysis and inspection | `tools/analysis/video_analyzer.py` | media files, JSON | port useful media facts | first release |
| `VideoDownloader` | analysis and inspection | `tools/analysis/video_downloader.py` | network downloader | port source acquisition | first release |
| `VideoUnderstand` | analysis and inspection | `tools/analysis/video_understand.py` | external CLI/model | defer pending production need | later |
| `VisualQA` | analysis and inspection | `tools/analysis/visual_qa.py` | sampled frames/provider | port technical evidence into `output_review` | first vertical slice |
| `AudioEnhance` | audio and speech | `tools/audio/audio_enhance.py` | audio provider/runtime | port when enhancement needed | later |
| `AudioMixer` | audio and speech | `tools/audio/audio_mixer.py` | FFmpeg/audio files | port into `audio_mix` | first vertical slice |
| `AzureTTS` | audio and speech | `tools/audio/azure_tts.py` | Azure Speech HTTP, credential | port cloud TTS contract | first release |
| `ComfyUIMusic` | audio and speech | `tools/audio/comfyui_music.py` | ComfyUI client, local GPU | defer local GPU route | later |
| `DashscopeTTS` | audio and speech | `tools/audio/dashscope_tts.py` | DashScope HTTP, credential | port provider wrapper | later |
| `DoubaoTTS` | audio and speech | `tools/audio/doubao_tts.py` | Doubao HTTP, credential | port provider wrapper | later |
| `ElevenLabsTTS` | audio and speech | `tools/audio/elevenlabs_tts.py` | ElevenLabs HTTP, credential | port cloud TTS contract | first release |
| `FalElevenLabsMusic` | audio and speech | `tools/audio/fal_elevenlabs_music.py` | fal HTTP, credential | port provider wrapper | later |
| `FalElevenLabsTTS` | audio and speech | `tools/audio/fal_elevenlabs_tts.py` | fal HTTP, credential | port provider wrapper | later |
| `FishAudioTTS` | audio and speech | `tools/audio/fish_audio_tts.py` | Fish Audio HTTP, credential | port provider wrapper | later |
| `FreesoundMusic` | audio and speech | `tools/audio/freesound_music.py` | Freesound HTTP, credential | port open/stock audio search | first release |
| `GoogleMusic` | audio and speech | `tools/audio/google_music.py` | Google API, credential | port provider wrapper | later |
| `GoogleTTS` | audio and speech | `tools/audio/google_tts.py` | Google API, shared credential | port cloud TTS contract | first release |
| `KlingTTS` | audio and speech | `tools/audio/kling_tts.py` | Kling client, callbacks, audio probe | port via shared Kling client | later |
| `MusicGen` | audio and speech | `tools/audio/music_gen.py` | local GPU/model | defer local GPU route | later |
| `MusicLibrary` | audio and speech | `tools/audio/music_library.py` | filesystem, FFmpeg | port supplied music library | first release |
| `OpenAITTS` | audio and speech | `tools/audio/openai_tts.py` | OpenAI HTTP, credential | port cloud TTS contract | first release |
| `PiperTTS` | audio and speech | `tools/audio/piper_tts.py` | Piper binary/local model | defer local model route | later |
| `PixabayMusic` | audio and speech | `tools/audio/pixabay_music.py` | Pixabay HTTP | port stock audio search | first release |
| `SunoMusic` | audio and speech | `tools/audio/suno_music.py` | Suno HTTP, credential | defer provider wrapper | later |
| `KlingAvatar` | avatar and lip sync | `tools/avatar/kling_avatar.py` | Kling client, media upload | port cloud avatar route | first release |
| `KlingLipSync` | avatar and lip sync | `tools/avatar/kling_lip_sync.py` | Kling client, media upload | port cloud lip-sync route | first release |
| `LipSync` | avatar and lip sync | `tools/avatar/lip_sync.py` | local binary/model | defer local model route | later |
| `TalkingHead` | avatar and lip sync | `tools/avatar/talking_head.py` | local files/model | defer local model route | later |
| `CapRecorder` | capture | `tools/capture/cap_recorder.py` | Cap CLI, subprocess, OS | port deterministic capture invocation | first release |
| `ScreenRecorder` | capture | `tools/capture/screen_recorder.py` | OS capture, subprocess | port deterministic recording | first release |
| `ActionTimelineCompiler` | character animation | `tools/character/character_animation.py` | artifact schemas, JSON | port mechanical timeline compiler | first release |
| `CharacterAnimationReviewer` | character animation | `tools/character/character_animation.py` | artifact schemas, renderer evidence | port deterministic review facts | first release |
| `CharacterRigRenderer` | character animation | `tools/character/character_animation.py` | artifact schemas, renderer | port renderer invocation | first release |
| `CharacterSpecGenerator` | character animation | `tools/character/character_animation.py` | artifact schemas, JSON | move semantics to Claude; retain validation | first release |
| `PoseLibraryBuilder` | character animation | `tools/character/character_animation.py` | artifact schemas, files | port deterministic artifact builder | first release |
| `SvgRigBuilder` | character animation | `tools/character/character_animation.py` | artifact schemas, SVG | port deterministic rig builder | first release |
| `BgRemove` | enhancement | `tools/enhancement/bg_remove.py` | image provider/runtime | port non-GPU provider route | first release |
| `ColorGrade` | enhancement | `tools/enhancement/color_grade.py` | media provider/runtime | port deterministic media transform | first release |
| `EyeEnhance` | enhancement | `tools/enhancement/eye_enhance.py` | image provider/runtime | defer specialized enhancement | later |
| `FaceEnhance` | enhancement | `tools/enhancement/face_enhance.py` | image provider/runtime | defer specialized enhancement | later |
| `FaceRestore` | enhancement | `tools/enhancement/face_restore.py` | image provider/runtime | defer specialized enhancement | later |
| `Upscale` | enhancement | `tools/enhancement/upscale.py` | provider/runtime, temp files | port non-GPU upscale route | first release |
| `Atlas3D` | graphics and image | `tools/graphics/atlas_3d.py` | Atlas HTTP, requests | port cloud 3D contract | later |
| `AtlasImage` | graphics and image | `tools/graphics/atlas_image.py` | Atlas client/models | port via shared Atlas client | later |
| `BlenderWorld` | graphics and image | `tools/graphics/blender_world.py` | Blender binary | defer existing-runtime invocation | later |
| `CodeSnippet` | graphics and image | `tools/graphics/code_snippet.py` | renderer/runtime | port code-native visual output | first release |
| `ComfyUIImage` | graphics and image | `tools/graphics/comfyui_image.py` | ComfyUI client, local GPU | defer local GPU route | later |
| `DashscopeImage` | graphics and image | `tools/graphics/dashscope_image.py` | DashScope HTTP, credential | port provider wrapper | later |
| `DiagramGen` | graphics and image | `tools/graphics/diagram_gen.py` | JSON, image renderer | port deterministic diagram mechanics | first release |
| `Fal3D` | graphics and image | `tools/graphics/fal_3d.py` | fal HTTP, requests | port cloud 3D contract | later |
| `FluxImage` | graphics and image | `tools/graphics/flux_image.py` | cloud API, credential | port cloud image contract | first release |
| `GoogleImagen` | graphics and image | `tools/graphics/google_imagen.py` | Google API, shared credential | port cloud image contract | first release |
| `GrokImage` | graphics and image | `tools/graphics/grok_image.py` | Grok HTTP, credential | port provider wrapper | later |
| `HunyuanImage` | graphics and image | `tools/graphics/hunyuan_image.py` | cloud API, credential | port provider wrapper | later |
| `ImageGen` | graphics and image | `tools/graphics/image_gen.py` | image API, credential | port useful provider contract | first release |
| `KlingOfficialImage` | graphics and image | `tools/graphics/kling_official_image.py` | Kling client, elements, omni | port via shared Kling client | first release |
| `LocalDiffusion` | graphics and image | `tools/graphics/local_diffusion.py` | local GPU/model | defer local GPU route | later |
| `MathAnimate` | graphics and image | `tools/graphics/math_animate.py` | animation binary, subprocess | defer existing-runtime invocation | later |
| `MiniMaxImage` | graphics and image | `tools/graphics/minimax_image.py` | MiniMax HTTP, credential | port provider wrapper | later |
| `OpenAIImage` | graphics and image | `tools/graphics/openai_image.py` | OpenAI HTTP, credential | port cloud image contract | first release |
| `PexelsImage` | graphics and image | `tools/graphics/pexels_image.py` | Pexels HTTP, credential | port stock image search | first release |
| `PixabayImage` | graphics and image | `tools/graphics/pixabay_image.py` | Pixabay HTTP, credential | port stock image search | first release |
| `RecraftImage` | graphics and image | `tools/graphics/recraft_image.py` | Recraft HTTP, credential | port provider wrapper | later |
| `SeedreamImage` | graphics and image | `tools/graphics/seedream_image.py` | cloud API, credential | port provider wrapper | later |
| `ThreeJSAssetCatalog` | graphics and image | `tools/graphics/threejs_asset_catalog.py` | network, ZIP, filesystem | port asset catalog mechanics | first release |
| `ThreeJSWorld` | graphics and image | `tools/graphics/threejs_world.py` | Three.js/browser runtime | port existing-runtime invocation | first release |
| `ExportBundle` | publication and delivery | `tools/publishers/export_bundle.py` | filesystem, JSON | port local delivery bundling | first release |
| `SubtitleGen` | subtitles | `tools/subtitle/subtitle_gen.py` | subtitle provider/runtime | port subtitle generation contract | first release |
| `AtlasVideo` | video, stock, and composition | `tools/video/atlas_video.py` | Atlas client/models | port via shared Atlas client | later |
| `AutoReframe` | video, stock, and composition | `tools/video/auto_reframe.py` | FFmpeg/media analysis | port deterministic reframing | first release |
| `ClipSearch` | video, stock, and composition | `tools/video/clip_search.py` | corpus/index facts | port transparent clip search | first release |
| `CogVideoVideo` | video, stock, and composition | `tools/video/cogvideo_video.py` | local GPU/model | defer local GPU route | later |
| `ComfyUIVideo` | video, stock, and composition | `tools/video/comfyui_video.py` | ComfyUI client, local GPU | defer local GPU route | later |
| `CorpusBuilder` | video, stock, and composition | `tools/video/corpus_builder.py` | filesystem, stock URLs | port corpus acquisition facts | first release |
| `DirectClipSearch` | video, stock, and composition | `tools/video/direct_clip_search.py` | FFmpeg, network sources | port transparent direct search | first release |
| `GeminiOmniFalVideo` | video, stock, and composition | `tools/video/gemini_omni_fal.py` | fal HTTP, credential | port provider wrapper | later |
| `GeminiOmniVideo` | video, stock, and composition | `tools/video/gemini_omni_video.py` | Gemini HTTP, credential | port cloud video contract | first release |
| `GreenScreenComposite` | video, stock, and composition | `tools/video/green_screen_composite.py` | NumPy, Pillow, FFmpeg | port deterministic compositing | first release |
| `GreenScreenProcessor` | video, stock, and composition | `tools/video/green_screen_processor.py` | FFmpeg/OS runtime | port deterministic processing | first release |
| `GrokVideo` | video, stock, and composition | `tools/video/grok_video.py` | Grok HTTP, credential | port provider wrapper | later |
| `HeyGenVideo` | video, stock, and composition | `tools/video/heygen_video.py` | HeyGen HTTP, credential | port cloud avatar-video contract | first release |
| `HiggsFieldVideo` | video, stock, and composition | `tools/video/higgsfield_video.py` | Higgsfield HTTP, credential | port provider wrapper | later |
| `HunyuanCloudVideo` | video, stock, and composition | `tools/video/hunyuan_cloud_video.py` | cloud API, credential | port provider wrapper | later |
| `HunyuanVideo` | video, stock, and composition | `tools/video/hunyuan_video.py` | local GPU/model | defer local GPU route | later |
| `HyperFramesCompose` | video, stock, and composition | `tools/video/hyperframes_compose.py` | HyperFrames CLI, subprocess | port existing-runtime invocation | first release |
| `JimengVideo` | video, stock, and composition | `tools/video/jimeng_video.py` | Jimeng HTTP/signing, credential | port provider wrapper | later |
| `KlingOfficialVideo` | video, stock, and composition | `tools/video/kling_official_video.py` | Kling client, media, schemas | port cloud video contract | first release |
| `KlingVideo` | video, stock, and composition | `tools/video/kling_video.py` | Kling HTTP, credential | consolidate with official route | later |
| `LTXVideoLocal` | video, stock, and composition | `tools/video/ltx_video_local.py` | local GPU/model | defer local GPU route | later |
| `LTXVideoModal` | video, stock, and composition | `tools/video/ltx_video_modal.py` | Modal cloud, credential | port cloud route if needed | later |
| `MiniMaxFalVideo` | video, stock, and composition | `tools/video/minimax_fal_video.py` | fal HTTP, credential | port provider wrapper | later |
| `MiniMaxVideo` | video, stock, and composition | `tools/video/minimax_video.py` | MiniMax HTTP, credential | port provider wrapper | later |
| `PexelsVideo` | video, stock, and composition | `tools/video/pexels_video.py` | Pexels HTTP, credential | port simple stock video tool | first release |
| `PixabayVideo` | video, stock, and composition | `tools/video/pixabay_video.py` | Pixabay HTTP, credential | port simple stock video tool | first release |
| `RemotionCaptionBurn` | video, stock, and composition | `tools/video/remotion_caption_burn.py` | Remotion runtime, JSON | port existing-runtime invocation | first release |
| `RunwayVideo` | video, stock, and composition | `tools/video/runway_video.py` | Runway HTTP, credential | port cloud video contract | first release |
| `SeedanceArkVideo` | video, stock, and composition | `tools/video/seedance_ark.py` | Ark HTTP, FFmpeg, credential | port provider wrapper | later |
| `SeedanceReplicate` | video, stock, and composition | `tools/video/seedance_replicate.py` | Replicate HTTP, credential | port provider wrapper | later |
| `SeedanceVideo` | video, stock, and composition | `tools/video/seedance_video.py` | cloud API, credential | port provider wrapper | later |
| `ShowcaseCard` | video, stock, and composition | `tools/video/showcase_card.py` | image/video renderer | port code-native card output | first release |
| `SilenceCutter` | video, stock, and composition | `tools/video/silence_cutter.py` | FFmpeg, JSON | port deterministic edit mechanic | first release |
| `SoraVideo` | video, stock, and composition | `tools/video/sora_video.py` | OpenAI HTTP, credential | port cloud video contract | first release |
| `VeoVideo` | video, stock, and composition | `tools/video/veo_video.py` | Google API, shared credential | port cloud video contract | first release |
| `VideoCompose` | video, stock, and composition | `tools/video/video_compose.py` | FFmpeg, subprocess, JSON | port mechanics into `source_edit` | first vertical slice |
| `VideoStitch` | video, stock, and composition | `tools/video/video_stitch.py` | FFmpeg, JSON | port deterministic stitch mechanic | first release |
| `VideoTrimmer` | video, stock, and composition | `tools/video/video_trimmer.py` | FFmpeg, JSON | port mechanics into `source_edit` | first vertical slice |
| `WanVideo` | video, stock, and composition | `tools/video/wan_video.py` | local GPU/model engine | defer local GPU route | later |
| `ArchiveOrgSource` | video, stock, and composition | `tools/video/stock_sources/archive_org.py` | stock adapter base, network | port open-media adapter | first release |
| `CoverrSource` | video, stock, and composition | `tools/video/stock_sources/coverr.py` | stock adapter base, credential | port stock adapter | first release |
| `DarefulSource` | video, stock, and composition | `tools/video/stock_sources/dareful.py` | stock adapter base, network | port stock adapter | first release |
| `ESASource` | video, stock, and composition | `tools/video/stock_sources/esa.py` | stock adapter base, network | port open-media adapter | first release |
| `JAXASource` | video, stock, and composition | `tools/video/stock_sources/jaxa.py` | stock adapter base, network | port open-media adapter | first release |
| `LibraryOfCongressSource` | video, stock, and composition | `tools/video/stock_sources/loc.py` | stock adapter base, network | port open-media adapter | first release |
| `MixkitSource` | video, stock, and composition | `tools/video/stock_sources/mixkit.py` | stock adapter base, network | port stock adapter | first release |
| `NARASource` | video, stock, and composition | `tools/video/stock_sources/nara.py` | stock adapter base, network | port open-media adapter | first release |
| `NasaSource` | video, stock, and composition | `tools/video/stock_sources/nasa.py` | stock adapter base, credential | port open-media adapter | first release |
| `NOAASource` | video, stock, and composition | `tools/video/stock_sources/noaa.py` | stock adapter base, network | port open-media adapter | first release |
| `PexelsSource` | video, stock, and composition | `tools/video/stock_sources/pexels.py` | stock adapter base, credential | port canonical Pexels adapter | first release |
| `PixabayVideoSource` | video, stock, and composition | `tools/video/stock_sources/pixabay_video.py` | stock adapter base, credential | port canonical Pixabay adapter | first release |
| `Pond5PublicDomainSource` | video, stock, and composition | `tools/video/stock_sources/pond5_pd.py` | stock adapter base, network | port public-domain adapter | first release |
| `UnsplashSource` | video, stock, and composition | `tools/video/stock_sources/unsplash.py` | stock adapter base, credential | port stock image adapter | first release |
| `VidevoSource` | video, stock, and composition | `tools/video/stock_sources/videvo.py` | stock adapter base, credential | port stock adapter | later |
| `WikimediaSource` | video, stock, and composition | `tools/video/stock_sources/wikimedia.py` | stock adapter base, network | port open-media adapter | first release |
| `AtlasClient` | shared provider clients | `tools/atlas_client.py` | HTTP, upload/poll/download | port once if Atlas tools retained | later |
| `ComfyUIClient` | shared provider clients | `tools/_comfyui/client.py` | requests, local ComfyUI/GPU | defer local GPU client | later |
| `KlingClient` | shared provider clients | `tools/_kling/client.py` | requests, Kling schemas/errors | port once for Kling tools | first release |
<!-- ledger:concrete:end -->

## Selectors

These four identities are not part of the 136 concrete total. Their proposed
treatment is a non-executing, transparent facts/ranking surface only: return
candidates, inputs, eligibility signals, rejection reasons, and rationale to
Claude. No selector may silently invoke its top candidate.

<!-- ledger:selectors:start -->
| Name | Capability family | Upstream relative path | Rough dependencies | Proposed treatment | Priority |
|---|---|---|---|---|---|
| `TTSSelector` | audio and speech | `tools/audio/tts_selector.py` | tool metadata | reimplement as transparent facts/ranking only | first release |
| `ScreenCaptureSelector` | capture | `tools/capture/screen_capture_selector.py` | tool metadata | reimplement as transparent facts/ranking only | first release |
| `ImageSelector` | graphics and image | `tools/graphics/image_selector.py` | tool metadata | reimplement as transparent facts/ranking only | first release |
| `VideoSelector` | video, stock, and composition | `tools/video/video_selector.py` | environment, tool metadata | reimplement as transparent facts/ranking only | first release |
<!-- ledger:selectors:end -->

## Incremental Enrichment Rule

This ledger remains shallow until a capability batch approaches implementation.
At that point, enrich only the selected identities and their immediate shared
dependencies with verified schemas, behavior, configuration and cost semantics,
representative inputs/outputs, parity fixtures, and consequential-action facts.
Do not deep-inventory unrelated rows, silently change the donor pin, promote a
priority because credentials happen to exist, or infer behavior from any other
checkout. Record any changed treatment or priority with the production need or
observed defect that justified it.
