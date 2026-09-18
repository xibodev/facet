import assert from "node:assert/strict";
import test from "node:test";

import {
  DEFAULT_COMPOSITION,
  validateComposition,
} from "../dist/contract.js";

const textCut = {
  id: "intro",
  type: "text_card",
  text: "Facet explains",
  in_seconds: 0,
  out_seconds: 2,
};

test("normalizes the explicit explainer profile to an exact frame count", () => {
  const result = validateComposition({
    width: 1280,
    height: 720,
    fps: 24,
    duration_seconds: 2,
    cuts: [textCut],
  });
  assert.equal(result.width, 1280);
  assert.equal(result.height, 720);
  assert.equal(result.fps, 24);
  assert.equal(result.durationInFrames, 48);
});

test("uses public defaults and the last cut end when duration is omitted", () => {
  const result = validateComposition({cuts: [textCut]});
  assert.deepEqual(
    {
      width: result.width,
      height: result.height,
      fps: result.fps,
      durationInFrames: result.durationInFrames,
    },
    {...DEFAULT_COMPOSITION, durationInFrames: 60},
  );
});

test("accepts only the retained scene primitives", () => {
  for (const cut of [
    textCut,
    {...textCut, type: "hero_title", subtitle: "One clear point"},
    {...textCut, type: "stat_card", stat: "42%", label: "faster"},
    {...textCut, type: "media", source: "asset-000.png", media_kind: "image"},
    {...textCut, type: "media", source: "asset-000.mp4", media_kind: "video"},
  ]) {
    assert.doesNotThrow(() => validateComposition({cuts: [cut]}));
  }
  assert.throws(
    () => validateComposition({cuts: [{...textCut, type: "bar_chart"}]}),
    /unsupported type "bar_chart"/,
  );
});

test("rejects blank content and invalid timelines", () => {
  for (const props of [
    {cuts: [{...textCut, text: "   "}]},
    {cuts: [{...textCut, type: "hero_title", text: ""}]},
    {cuts: [{...textCut, type: "stat_card", stat: ""}]},
    {cuts: [{...textCut, type: "media", source: ""}]},
    {cuts: [{...textCut, out_seconds: 0}]},
    {duration_seconds: 1, cuts: [textCut]},
    {fps: 30, duration_seconds: 1.01, cuts: [{...textCut, out_seconds: 1}]},
  ]) {
    assert.throws(() => validateComposition(props));
  }
});

test("validates dimensions and optional narration and music", () => {
  assert.doesNotThrow(() =>
    validateComposition({
      cuts: [textCut],
      audio: {
        narration: {src: "voice.wav", volume: 1},
        music: {src: "music.wav", volume: 0.2, loop: true},
      },
    }),
  );
  for (const props of [
    {width: 1279, cuts: [textCut]},
    {height: 0, cuts: [textCut]},
    {fps: Number.NaN, cuts: [textCut]},
    {cuts: [textCut], audio: {narration: {src: ""}}},
    {cuts: [textCut], audio: {music: {src: "music.wav", volume: 2}}},
  ]) {
    assert.throws(() => validateComposition(props));
  }
});
