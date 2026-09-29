import React from "react";
import {
  AbsoluteFill,
  Audio,
  Img,
  OffthreadVideo,
  Sequence,
  useVideoConfig,
} from "remotion";

import {resolveAsset} from "./assets";
import {
  AudioTrack,
  ExplainerCut,
  ExplainerProps,
  validateComposition,
} from "./contract";

const fontFamily =
  'Inter, ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif';

const frameRange = (cut: ExplainerCut, fps: number) => {
  const from = Math.floor(cut.in_seconds * fps + 1e-9);
  const end = Math.ceil(cut.out_seconds * fps - 1e-9);
  return {from, durationInFrames: end - from};
};

const cardStyle = (
  cut: ExplainerCut,
  defaults: {backgroundColor: string; color: string},
): React.CSSProperties => ({
  backgroundColor: cut.backgroundColor ?? defaults.backgroundColor,
  color: cut.color ?? defaults.color,
  fontFamily,
  justifyContent: "center",
  alignItems: "center",
  textAlign: "center",
  padding: "8%",
});

const Scene: React.FC<{cut: ExplainerCut}> = ({cut}) => {
  if (cut.type === "text_card") {
    return (
      <AbsoluteFill style={cardStyle(cut, {backgroundColor: "#111827", color: "#f9fafb"})}>
        <div
          style={{
            fontSize: cut.fontSize ?? 80,
            fontWeight: 700,
            lineHeight: 1.08,
            maxWidth: "86%",
          }}
        >
          {cut.text}
        </div>
      </AbsoluteFill>
    );
  }
  if (cut.type === "hero_title") {
    return (
      <AbsoluteFill style={cardStyle(cut, {backgroundColor: "#0f172a", color: "#f8fafc"})}>
        <div style={{fontSize: 104, fontWeight: 800, lineHeight: 1.02}}>
          {cut.text}
        </div>
        {cut.subtitle ? (
          <div style={{fontSize: 38, lineHeight: 1.3, marginTop: 28, opacity: 0.82}}>
            {cut.subtitle}
          </div>
        ) : null}
      </AbsoluteFill>
    );
  }
  if (cut.type === "stat_card") {
    return (
      <AbsoluteFill style={cardStyle(cut, {backgroundColor: "#052e2b", color: "#ecfdf5"})}>
        <div style={{fontSize: 152, fontWeight: 900, lineHeight: 1}}>
          {cut.stat}
        </div>
        {cut.label ? (
          <div style={{fontSize: 42, lineHeight: 1.2, marginTop: 24}}>
            {cut.label}
          </div>
        ) : null}
      </AbsoluteFill>
    );
  }

  const source = resolveAsset(cut.source);
  return (
    <AbsoluteFill
      style={{
        backgroundColor: cut.backgroundColor ?? "#020617",
        justifyContent: "center",
        alignItems: "center",
      }}
    >
      {cut.media_kind === "video" ? (
        <OffthreadVideo
          src={source}
          muted={cut.muted}
          style={{width: "100%", height: "100%", objectFit: cut.fit ?? "contain"}}
        />
      ) : (
        <Img
          src={source}
          style={{width: "100%", height: "100%", objectFit: cut.fit ?? "contain"}}
        />
      )}
      {cut.title ? (
        <div
          style={{
            position: "absolute",
            left: "5%",
            right: "5%",
            bottom: "6%",
            color: cut.color ?? "#ffffff",
            fontFamily,
            fontSize: 42,
            fontWeight: 700,
            textShadow: "0 2px 12px rgba(0,0,0,0.8)",
          }}
        >
          {cut.title}
        </div>
      ) : null}
    </AbsoluteFill>
  );
};

const Track: React.FC<{track: AudioTrack}> = ({track}) => (
  <Audio
    src={resolveAsset(track.src)}
    volume={track.volume ?? 1}
    loop={track.loop ?? false}
  />
);

export const Explainer: React.FC<ExplainerProps> = (props) => {
  const config = useVideoConfig();
  const composition = validateComposition({
    ...props,
    width: config.width,
    height: config.height,
    fps: config.fps,
    duration_seconds: config.durationInFrames / config.fps,
  });

  return (
    <AbsoluteFill style={{backgroundColor: composition.backgroundColor ?? "#000000"}}>
      {composition.cuts.map((cut, index) => {
        const range = frameRange(cut, composition.fps);
        return (
          <Sequence
            key={cut.id ?? `${cut.type}-${index}`}
            from={range.from}
            durationInFrames={range.durationInFrames}
            premountFor={Math.min(composition.fps, range.durationInFrames)}
          >
            <Scene cut={cut} />
          </Sequence>
        );
      })}
      {composition.audio?.music ? <Track track={composition.audio.music} /> : null}
      {composition.audio?.narration ? (
        <Track track={composition.audio.narration} />
      ) : null}
    </AbsoluteFill>
  );
};
