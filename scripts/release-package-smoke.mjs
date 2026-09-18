// Validate npm content separately from the documented native source installation.
import assert from 'node:assert/strict';
import { execFileSync, spawnSync } from 'node:child_process';
import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';

const root = fileURLToPath(new URL('../', import.meta.url));
const manifest = JSON.parse(readFileSync(new URL('../package.json', import.meta.url), 'utf8'));
const lock = JSON.parse(readFileSync(new URL('../package-lock.json', import.meta.url), 'utf8'));
assert.equal(lock.version, manifest.version);
assert.equal(lock.packages[''].version, manifest.version);
assert.deepEqual(lock.packages[''].devDependencies, manifest.devDependencies);

for (const name of ['facet', 'facet-ui']) {
  const output = execFileSync('go', ['run', `./cmd/${name}`, '--version'], {
    cwd: root, encoding: 'utf8', timeout: 120000,
  });
  assert.equal(output.trim(), `${name} v${manifest.version}`);
}

// Windows npm is a cmd shim; every argument here is fixed, not external input.
const result = spawnSync(process.platform === 'win32' ? 'npm.cmd' : 'npm',
  ['pack', '--dry-run', '--json', '--ignore-scripts'], {
    cwd: root, encoding: 'utf8', timeout: 120000,
    shell: process.platform === 'win32', maxBuffer: 10 * 1024 * 1024,
  });
if (result.error) throw result.error;
assert.equal(result.status, 0, result.stderr);
const [pack] = JSON.parse(result.stdout);
assert.equal(pack.name, manifest.name);
assert.equal(pack.version, manifest.version);
const files = new Set(pack.files.map(file => file.path));
for (const file of [
  'package.json', 'bin/facet-cli.js', 'bin/facet-ui-cli.js',
  'skills/facet/SKILL.md', 'packs/explainer/SKILL.md',
  'remotion-composer/package.json', 'remotion-composer/package-lock.json',
  'remotion-composer/tsconfig.json', 'remotion-composer/src/index.tsx',
  'README.md', 'LICENSE',
]) assert.ok(files.has(file), `Missing npm content: ${file}`);
for (const prefix of ['schemas/', 'web/']) {
  assert.ok([...files].some(file => file.startsWith(prefix)), `Missing npm content: ${prefix}`);
}
for (const file of files) {
  assert.doesNotMatch(file, /(^|\/)(node_modules|\.git|\.env(?:\.[^/]*)?|\.quality-run)(\/|$)/);
}
console.log(`PASS: ${manifest.name}@${manifest.version}, both source CLI versions, ${files.size} npm files. Native distribution and publishing are not tested.`);
