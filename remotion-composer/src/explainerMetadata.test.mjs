import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";
import ts from "typescript";

// Use the installed compiler without adding a test runner or emitting build files.
const source = readFileSync(new URL("./explainerMetadata.ts", import.meta.url), "utf8");
const { outputText } = ts.transpileModule(source, {
  compilerOptions: { target: ts.ScriptTarget.ES2020, module: ts.ModuleKind.ES2020 },
});
const { calculateExplainerMetadata: metadata } = await import(
  `data:text/javascript;base64,${Buffer.from(outputText).toString("base64")}`
);
const cut = (start = 0, end = 3) => ({ id: "intro", source: "", in_seconds: start, out_seconds: end });

test("explicit 320x180 at 24fps for 3 seconds has exactly 72 frames", () => {
  const props = { width: 320, height: 180, fps: 24, duration_seconds: 3, cuts: [cut()] };
  Object.freeze(props.cuts[0]);
  Object.freeze(props.cuts);
  Object.freeze(props);
  assert.deepEqual(metadata(props), { width: 320, height: 180, fps: 24, durationInFrames: 72 });
});

test("defaults preserve last-cut plus one and 60-second empty fallback", () => {
  assert.deepEqual(metadata({ cuts: [cut()] }), { width: 1920, height: 1080, fps: 30, durationInFrames: 120 });
  assert.equal(metadata({ cuts: [cut(0, 5), cut(0, 2)] }).durationInFrames, 180);
  assert.equal(metadata({ cuts: [cut(0, 3.01)] }).durationInFrames, 121);
  assert.equal(metadata({ cuts: [] }).durationInFrames, 1800);
  assert.equal(metadata({}).durationInFrames, 1800);
  assert.equal(metadata({ fps: 24, cuts: [cut()] }).durationInFrames, 96);
  assert.equal(metadata({ fps: 24, cuts: [] }).durationInFrames, 1440);
});

test("partial profiles, fractional fps, exact frame durations and rounding noise", () => {
  assert.deepEqual(metadata({ width: 640, duration_seconds: 3, cuts: [] }),
    { width: 640, height: 1080, fps: 30, durationInFrames: 90 });
  assert.equal(metadata({ fps: 29.97, duration_seconds: 100, cuts: [cut()] }).durationInFrames, 2997);
  assert.equal(metadata({ fps: 24, duration_seconds: 1 / 24, cuts: [cut(0, 1 / 24)] }).durationInFrames, 1);
  assert.equal(metadata({ fps: 30, duration_seconds: 0.1 + 0.2, cuts: [] }).durationInFrames, 9);
  assert.equal(metadata({ fps: 24, duration_seconds: 3, cuts: [cut(0.021, 3)] }).durationInFrames, 72);
});

for (const field of ["width", "height"]) {
  test(`${field} rejects nonnumeric, nonfinite, nonpositive, odd, fractional and unsafe values`, () => {
    for (const value of [null, "320", true, NaN, Infinity, -Infinity, 0, -2, 321, 320.5, 2 ** 53]) {
      assert.throws(() => metadata({ [field]: value, cuts: [] }), new RegExp(field));
    }
  });
}

for (const field of ["fps", "duration_seconds"]) {
  test(`${field} rejects nonnumeric, nonfinite and nonpositive values`, () => {
    for (const value of [null, "24", true, NaN, Infinity, -Infinity, 0, -1]) {
      assert.throws(() => metadata({ [field]: value, cuts: [] }), new RegExp(field));
    }
  });
}

test("invalid frame counts are rejected instead of rounded or truncated", () => {
  for (const props of [
    { fps: 24, duration_seconds: 0.1 },
    { fps: 24, duration_seconds: 0.001 },
    { fps: 30, duration_seconds: Number.MAX_VALUE },
    { fps: Number.MAX_VALUE, duration_seconds: 3 },
    { fps: 30, duration_seconds: 2 ** 53 },
    { fps: Number.MIN_VALUE, duration_seconds: Number.MIN_VALUE },
    { fps: Number.MAX_VALUE },
  ]) {
    assert.throws(() => metadata({ ...props, cuts: [] }), /frame count/);
  }
});

test("cut timings reject malformed, reversed, subframe, unsafe and out-of-bounds cuts", () => {
  for (const value of [null, {}, "cuts"]) {
    assert.throws(() => metadata({ cuts: value }), /cuts must be an array/);
  }
  for (const value of [null, {}, cut(-1, 2), cut(2, 1), cut(1, 1), cut(0, 0.001),
    cut(NaN, 2), cut(0, Infinity), cut("0", 2), cut(0, "3"), cut(0, 2 ** 53)]) {
    assert.throws(() => metadata({ cuts: [value] }), /cuts\[0\]/);
  }
  for (const value of [cut(0, 3.001), cut(3, 4)]) {
    assert.throws(() => metadata({ fps: 24, duration_seconds: 3, cuts: [value] }), /exceeds duration_seconds/);
  }
});
