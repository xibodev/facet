import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";
import ts from "typescript";

const source = readFileSync(new URL("./explainerLayout.ts", import.meta.url), "utf8");
const { outputText } = ts.transpileModule(source, {
  compilerOptions: { target: ts.ScriptTarget.ES2020, module: ts.ModuleKind.ES2020 },
});
const { explainerLayout } = await import(
  `data:text/javascript;base64,${Buffer.from(outputText).toString("base64")}`
);

for (const [width, height, scale, left, top] of [
  [320, 180, 1 / 6, 0, 0],
  [1920, 1080, 1, 0, 0],
  [1080, 1920, 0.5625, 0, 656.25],
  [2560, 1080, 1, 320, 0],
]) {
  test(`${width}x${height} contains and centers the complete logical canvas`, () => {
    const layout = explainerLayout(width, height);
    assert.deepEqual(layout, { width: 1920, height: 1080, scale, left, top });
    assert.equal(layout.left * 2 + layout.width * layout.scale, width);
    assert.equal(layout.top * 2 + layout.height * layout.scale, height);
    assert.ok(layout.left >= 0 && layout.top >= 0);
    assert.equal(layout.width / layout.height, 16 / 9);
  });
}
