import type { ExplainerProps } from "./Explainer";

// Pure metadata calculation: no asset loading, browser, or rendering required.
export function calculateExplainerMetadata(props: ExplainerProps) {
  const width = props.width === undefined ? 1920 : props.width;
  const height = props.height === undefined ? 1080 : props.height;
  const fps = props.fps === undefined ? 30 : props.fps;
  for (const [name, value] of Object.entries({ width, height })) {
    if (!Number.isSafeInteger(value) || value <= 0 || value % 2 !== 0) {
      throw new Error(`Explainer ${name} must be a positive, even safe integer`);
    }
  }
  if (!Number.isFinite(fps) || fps <= 0) {
    throw new Error("Explainer fps must be a positive finite number");
  }

  const explicitDuration = props.duration_seconds;
  let durationInFrames: number | undefined;
  if (explicitDuration !== undefined) {
    if (!Number.isFinite(explicitDuration) || explicitDuration <= 0) {
      throw new Error("Explainer duration_seconds must be a positive finite number");
    }
    const frames = explicitDuration * fps;
    durationInFrames = Math.round(frames);
    // Permit floating-point multiplication noise, not fractional-frame requests.
    const tolerance = Math.min(1e-7, Number.EPSILON * Math.abs(frames) * 8);
    if (!Number.isSafeInteger(durationInFrames) || durationInFrames <= 0 ||
        Math.abs(frames - durationInFrames) > tolerance) {
      throw new Error("Explainer duration_seconds * fps must be a positive safe integer frame count");
    }
  }

  const cuts = props.cuts === undefined ? [] : props.cuts;
  if (!Array.isArray(cuts)) {
    throw new Error("Explainer cuts must be an array");
  }
  let lastEnd = 0;
  for (const [index, cut] of cuts.entries()) {
    if (!cut || !Number.isFinite(cut.in_seconds) || !Number.isFinite(cut.out_seconds) ||
        cut.in_seconds < 0 || cut.out_seconds <= cut.in_seconds) {
      throw new Error(`Explainer cuts[${index}] requires finite 0 <= in_seconds < out_seconds`);
    }
    const from = Math.round(cut.in_seconds * fps);
    const end = Math.round(cut.out_seconds * fps);
    if (!Number.isSafeInteger(from) || !Number.isSafeInteger(end) || end <= from) {
      throw new Error(`Explainer cuts[${index}] must span at least one frame with safe integer frame boundaries`);
    }
    if (explicitDuration !== undefined && cut.out_seconds > explicitDuration) {
      throw new Error(`Explainer cuts[${index}] exceeds duration_seconds`);
    }
    lastEnd = Math.max(lastEnd, cut.out_seconds);
  }

  if (durationInFrames === undefined) {
    // Preserve the shipped final-fade padding and empty-composition fallback.
    durationInFrames = Math.ceil((cuts.length === 0 ? 60 : lastEnd + 1) * fps);
  }
  if (!Number.isSafeInteger(durationInFrames) || durationInFrames <= 0) {
    throw new Error("Explainer duration must resolve to a positive safe integer frame count");
  }
  return { width, height, fps, durationInFrames };
}
