import assert from "node:assert/strict";
import {readFile, readdir} from "node:fs/promises";
import path from "node:path";
import test from "node:test";
import {fileURLToPath} from "node:url";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const manifest = JSON.parse(
  await readFile(path.join(root, "legacy-composer-manifest.json"), "utf8"),
);

const listFiles = async (directory) => {
  const entries = await readdir(directory, {withFileTypes: true});
  const files = [];
  for (const entry of entries) {
    const absolute = path.join(directory, entry.name);
    if (entry.isDirectory()) {
      files.push(...(await listFiles(absolute)));
    } else {
      files.push(path.relative(root, absolute).replaceAll("\\", "/"));
    }
  }
  return files;
};

test("only the independently authored source manifest is present", async () => {
  const actual = (await listFiles(path.join(root, "src"))).sort();
  assert.deepEqual(actual, [...manifest.allowedSourcePaths].sort());
  for (const legacyPath of manifest.bannedLegacyPaths) {
    assert.ok(!actual.includes(legacyPath), `${legacyPath} returned`);
  }
  const source = (
    await Promise.all(actual.map((file) => readFile(path.join(root, file), "utf8")))
  ).join("\n");
  for (const token of manifest.bannedLegacyTokens) {
    assert.ok(!source.includes(token), `legacy token ${token} returned`);
  }
});
