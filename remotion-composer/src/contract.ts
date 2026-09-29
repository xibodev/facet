export const DEFAULT_COMPOSITION = {
  width: 1920,
  height: 1080,
  fps: 30,
} as const;

type BaseCut = {
  id?: string;
  in_seconds: number;
  out_seconds: number;
  backgroundColor?: string;
  color?: string;
};

export type TextCardCut = BaseCut & {
  type: "text_card";
  text: string;
  fontSize?: number;
};

export type HeroTitleCut = BaseCut & {
  type: "hero_title";
  text: string;
  subtitle?: string;
};

export type StatCardCut = BaseCut & {
  type: "stat_card";
  stat: string;
  label?: string;
};

export type MediaCut = BaseCut & {
  type: "media";
  source: string;
  media_kind: "image" | "video";
  fit?: "contain" | "cover";
  title?: string;
  muted?: boolean;
};

export type ExplainerCut =
  | TextCardCut
  | HeroTitleCut
  | StatCardCut
  | MediaCut;

export type AudioTrack = {
  src: string;
  volume?: number;
  loop?: boolean;
};

export type ExplainerProps = {
  width?: number;
  height?: number;
  fps?: number;
  duration_seconds?: number;
  backgroundColor?: string;
  cuts: ExplainerCut[];
  audio?: {
    narration?: AudioTrack;
    music?: AudioTrack;
  };
};

export type ValidatedComposition = ExplainerProps & {
  width: number;
  height: number;
  fps: number;
  duration_seconds: number;
  durationInFrames: number;
  cuts: ExplainerCut[];
};

const isRecord = (value: unknown): value is Record<string, unknown> =>
  typeof value === "object" && value !== null && !Array.isArray(value);

const finiteNumber = (value: unknown, name: string): number => {
  if (typeof value !== "number" || !Number.isFinite(value)) {
    throw new Error(`${name} must be a finite number`);
  }
  return value;
};

const nonBlank = (value: unknown, name: string): string => {
  if (typeof value !== "string" || value.trim() === "") {
    throw new Error(`${name} must be a nonblank string`);
  }
  return value;
};

const optionalString = (value: unknown, name: string): void => {
  if (value !== undefined) {
    nonBlank(value, name);
  }
};

const validateTrack = (value: unknown, name: string): void => {
  if (value === undefined) return;
  if (!isRecord(value)) throw new Error(`${name} must be an object`);
  nonBlank(value.src, `${name}.src`);
  if (value.volume !== undefined) {
    const volume = finiteNumber(value.volume, `${name}.volume`);
    if (volume < 0 || volume > 1) {
      throw new Error(`${name}.volume must be between 0 and 1`);
    }
  }
  if (value.loop !== undefined && typeof value.loop !== "boolean") {
    throw new Error(`${name}.loop must be a boolean`);
  }
};

const validateCut = (
  value: unknown,
  index: number,
  fps: number,
  durationSeconds: number,
  previousEnd: number,
): ExplainerCut => {
  if (!isRecord(value)) throw new Error(`cut ${index} must be an object`);
  const prefix = `cut ${index}`;
  const start = finiteNumber(value.in_seconds, `${prefix}.in_seconds`);
  const end = finiteNumber(value.out_seconds, `${prefix}.out_seconds`);
  if (start < 0 || end <= start) {
    throw new Error(`${prefix} must satisfy 0 <= in_seconds < out_seconds`);
  }
  if (start < previousEnd) {
    throw new Error(`${prefix} overlaps or is out of order`);
  }
  if (end > durationSeconds) {
    throw new Error(`${prefix}.out_seconds exceeds duration_seconds`);
  }
  const startFrame = Math.floor(start * fps + 1e-9);
  const endFrame = Math.ceil(end * fps - 1e-9);
  if (endFrame <= startFrame) {
    throw new Error(`${prefix} must span at least one frame`);
  }

  const type = nonBlank(value.type, `${prefix}.type`);
  optionalString(value.backgroundColor, `${prefix}.backgroundColor`);
  optionalString(value.color, `${prefix}.color`);
  switch (type) {
    case "text_card": {
      nonBlank(value.text, `${prefix}.text`);
      if (value.fontSize !== undefined) {
        const fontSize = finiteNumber(value.fontSize, `${prefix}.fontSize`);
        if (fontSize <= 0) throw new Error(`${prefix}.fontSize must be positive`);
      }
      break;
    }
    case "hero_title":
      nonBlank(value.text, `${prefix}.text`);
      optionalString(value.subtitle, `${prefix}.subtitle`);
      break;
    case "stat_card":
      nonBlank(value.stat, `${prefix}.stat`);
      optionalString(value.label, `${prefix}.label`);
      break;
    case "media":
      nonBlank(value.source, `${prefix}.source`);
      if (value.media_kind !== "image" && value.media_kind !== "video") {
        throw new Error(`${prefix}.media_kind must be "image" or "video"`);
      }
      if (
        value.fit !== undefined &&
        value.fit !== "contain" &&
        value.fit !== "cover"
      ) {
        throw new Error(`${prefix}.fit must be "contain" or "cover"`);
      }
      optionalString(value.title, `${prefix}.title`);
      if (value.muted !== undefined && typeof value.muted !== "boolean") {
        throw new Error(`${prefix}.muted must be a boolean`);
      }
      break;
    default:
      throw new Error(`${prefix} has unsupported type "${type}"`);
  }
  return value as ExplainerCut;
};

export const validateComposition = (input: unknown): ValidatedComposition => {
  if (!isRecord(input)) throw new Error("composition props must be an object");

  const widthValue = input.width ?? DEFAULT_COMPOSITION.width;
  const heightValue = input.height ?? DEFAULT_COMPOSITION.height;
  const fps = input.fps ?? DEFAULT_COMPOSITION.fps;
  for (const [name, value] of [
    ["width", widthValue],
    ["height", heightValue],
  ] as const) {
    const number = finiteNumber(value, name);
    if (!Number.isSafeInteger(number) || number <= 0 || number % 2 !== 0) {
      throw new Error(`${name} must be a positive even safe integer`);
    }
  }
  const width = widthValue as number;
  const height = heightValue as number;
  const frameRate = finiteNumber(fps, "fps");
  if (frameRate <= 0) throw new Error("fps must be positive");

  if (!Array.isArray(input.cuts) || input.cuts.length === 0) {
    throw new Error("cuts must be a nonempty array");
  }
  const lastEnd = input.cuts.reduce((latest, value, index) => {
    if (!isRecord(value)) throw new Error(`cut ${index} must be an object`);
    const end = finiteNumber(value.out_seconds, `cut ${index}.out_seconds`);
    return Math.max(latest, end);
  }, 0);
  const durationSeconds =
    input.duration_seconds === undefined
      ? lastEnd
      : finiteNumber(input.duration_seconds, "duration_seconds");
  if (durationSeconds <= 0) {
    throw new Error("duration_seconds must be positive");
  }
  const frameCount = durationSeconds * frameRate;
  if (!Number.isSafeInteger(frameCount) || frameCount <= 0) {
    throw new Error(
      "duration_seconds * fps must be a positive safe integer frame count",
    );
  }

  let previousEnd = 0;
  const cuts = input.cuts.map((cut, index) => {
    const validated = validateCut(
      cut,
      index,
      frameRate,
      durationSeconds,
      previousEnd,
    );
    previousEnd = validated.out_seconds;
    return validated;
  });

  if (input.audio !== undefined) {
    if (!isRecord(input.audio)) throw new Error("audio must be an object");
    validateTrack(input.audio.narration, "audio.narration");
    validateTrack(input.audio.music, "audio.music");
    if (input.audio.narration === undefined && input.audio.music === undefined) {
      throw new Error("audio must contain narration or music");
    }
  }
  optionalString(input.backgroundColor, "backgroundColor");

  return {
    ...(input as ExplainerProps),
    width,
    height,
    fps: frameRate,
    duration_seconds: durationSeconds,
    durationInFrames: frameCount,
    cuts,
  };
};
