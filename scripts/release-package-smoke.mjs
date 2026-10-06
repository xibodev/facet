// Validate npm content and version surfaces. npm ships a launcher and guidance,
// never a native binary installer; native distribution is tested by release.yml.
import assert from 'node:assert/strict';
import { execFileSync, spawnSync } from 'node:child_process';
import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';

const root = fileURLToPath(new URL('../', import.meta.url));
const read = file => JSON.parse(readFileSync(new URL(`../${file}`, import.meta.url), 'utf8'));
const manifest = read('package.json');
const lock = read('package-lock.json');
const composerManifest = read('remotion-composer/composer-manifest.json');

assert.equal(lock.version, manifest.version);
assert.equal(lock.packages[''].version, manifest.version);
assert.deepEqual(lock.packages[''].bin, manifest.bin);
assert.deepEqual(lock.packages[''].devDependencies, manifest.devDependencies);
assert.deepEqual(manifest.bin, { facet: 'bin/facet-cli.js' }, 'npm exposes only the facet launcher');

// Release builds stamp the version with -ldflags "-X main.Version=..."; source
// builds identify as the same release, optionally marked -dev.
const facetVersion = (...flags) => execFileSync('go', ['run', ...flags, './cmd/facet', 'version'], {
  cwd: root, encoding: 'utf8', timeout: 300000,
}).trim();
assert.equal(facetVersion('-ldflags', `-X main.Version=${manifest.version}`), `facet v${manifest.version}`);
assert.match(facetVersion(), new RegExp(`^facet v${manifest.version.replaceAll('.', '\\.')}(-dev)?$`));

// A fixed command line through the shell: Windows npm is a cmd shim, and no
// argument here is external input.
const result = spawnSync('npm pack --dry-run --json --ignore-scripts', {
  cwd: root, encoding: 'utf8', timeout: 120000, shell: true, maxBuffer: 10 * 1024 * 1024,
});
if (result.error) throw result.error;
assert.equal(result.status, 0, result.stderr);
const [pack] = JSON.parse(result.stdout);
assert.equal(pack.name, manifest.name);
assert.equal(pack.version, manifest.version);
const files = new Set(pack.files.map(file => file.path.replaceAll('\\', '/')));
for (const file of [
  'package.json', 'bin/facet-cli.js',
  'skills/facet/SKILL.md', 'packs/explainer/SKILL.md', 'agents/facet-creative.md',
  'remotion-composer/package.json', 'remotion-composer/package-lock.json',
  'remotion-composer/tsconfig.json', 'remotion-composer/composer-manifest.json',
  'remotion-composer/src/index.tsx', 'remotion-composer/src/contract.ts',
  'LICENSE', 'THIRD_PARTY_NOTICES.md',
]) assert.ok(files.has(file), `Missing npm content: ${file}`);
assert.ok([...files].some(file => file.startsWith('schemas/')), 'Missing npm content: schemas/');
for (const file of files) {
  assert.doesNotMatch(file, /^(web|remotion-composer\/tests|scripts|cmd|internal)\//, `Not npm content: ${file}`);
  assert.doesNotMatch(file, /(^|\/)(node_modules|\.git|\.env(?:\.[^/]*)?|\.quality-run)(\/|$)/);
  if (file.startsWith('bin/')) {
    assert.match(file, /^bin\/(facet-cli\.js|facet-(linux|darwin|windows)-(amd64|arm64)(\.exe)?)$/, `Unexpected npm bin content: ${file}`);
  }
}
const composerSource = [...files]
  .filter(file => file.startsWith('remotion-composer/src/'))
  .map(file => file.slice('remotion-composer/'.length))
  .sort();
assert.deepEqual(composerSource, [...composerManifest.allowedSourcePaths].sort());
console.log(`PASS: ${manifest.name}@${manifest.version}, facet source and release version stamps, ${files.size} npm files. Native distribution and publishing are not tested here.`);
