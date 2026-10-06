import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { test } from 'node:test';

// Facet 2.0 installers: one runtime per user, wiring through `facet wire`,
// nothing written into projects or CLI instruction files by the installer.
const root = new URL('../', import.meta.url);
const bash = readFileSync(new URL('install.sh', root), 'utf8');
const powershell = readFileSync(new URL('install.ps1', root), 'utf8');
const manifest = readFileSync(new URL('installer/manifest.tsv', root), 'utf8');
const pkg = JSON.parse(readFileSync(new URL('package.json', root), 'utf8'));

test('manifest describes the user-wide layout, the release and the components only', () => {
  assert.match(manifest, /^contract\tlayout\t2\t/m);
  assert.match(manifest, new RegExp(`^release\\tfacet\\t${pkg.version.replaceAll('.', '\\.')}\\t`, 'm'));
  for (const component of ['remotion', 'piper', 'hyperframes']) {
    assert.match(manifest, new RegExp(`^component\\t${component}\\t`, 'm'));
  }
  assert.match(manifest, /^builtin\tedge-tts\t-\tmicrosoft_edge\tall\tall\tall\t0\t/m);
  assert.doesNotMatch(manifest, /^(host|instruction|pack)\t/m, 'per-project targets and packs belong to facet wire');
});

test('both installers expose the same actions and options', () => {
  for (const option of ['--action', '--version', '--components', '--wire', '--scope', '--project', '--archive', '--checksums', '--yes', '--no-path', '--skip-verify', '--purge', '--plain', '--verbose']) {
    assert.ok(bash.includes(option), `install.sh lacks ${option}`);
  }
  assert.match(bash, /install\|update\|rollback\|uninstall/);
  assert.match(powershell, /\[ValidateSet\('install','update','rollback','uninstall'\)\]\[string\]\$Action/);
  assert.match(powershell, /\[ValidateSet\('user','project'\)\]\[string\]\$Scope/);
  for (const name of ['Wire', 'ProjectDir', 'ArchivePath', 'ChecksumPath', 'NonInteractive', 'NoPath', 'SkipVerify', 'Purge']) {
    assert.match(powershell, new RegExp(`\\$${name}\\b`), `install.ps1 lacks -${name}`);
  }
});

test('both installers use one runtime per user activated through ~/.facet/current', () => {
  for (const source of [bash, powershell]) {
    assert.match(source, /runtimes/);
    assert.match(source, /current/);
    assert.match(source, /installer\.json/);
    assert.match(source, /components\.json/);
    assert.match(source, /\.facet-files\.sha256/);
  }
  assert.match(bash, /COMPONENTS=\$\{COMPONENTS:-remotion,piper\}/);
});

test('wiring is delegated to facet wire, including removal on uninstall', () => {
  for (const source of [bash, powershell]) {
    assert.match(source, /wire/);
    assert.match(source, /--remove/);
  }
});

test('installers write nothing into projects or instruction files', () => {
  for (const source of [bash, powershell]) {
    assert.doesNotMatch(source, /run-facet/);
    assert.doesNotMatch(source, /facet:managed:start/);
    assert.doesNotMatch(source, /installation\.(json|tsv)/);
    assert.doesNotMatch(source, /migrate-?legacy/i);
    assert.doesNotMatch(source, /\bstudio\b/i);
    assert.doesNotMatch(source, /--target\b/);
  }
});

test('installers point v1 users at the v1.1.0 uninstaller instead of migrating', () => {
  for (const source of [bash, powershell]) {
    assert.match(source, /v1 project integrations are separate/);
    assert.match(source, /--action uninstall/);
  }
});

test('the bash PATH change is a removable, guarded profile block that --no-path skips', () => {
  assert.match(bash, /# >>> facet path >>>/);
  assert.match(bash, /# <<< facet path <<</);
  assert.match(bash, /NO_PATH/);
  assert.match(bash, /remove_profile_block/);
});

test('public installers do not advertise downloads from the private gflow repository', () => {
  assert.doesNotMatch(manifest, /^component\tgflow\t/m);
  assert.doesNotMatch(bash, /Download gflow|releases\/download\/v.*gflow/s);
  assert.doesNotMatch(powershell, /Download gflow|releases\/download\/v.*gflow/s);
});

test('linux installer accepts current browser dependency package names and non-apt systems', () => {
  assert.match(bash, /apt-get update/);
  assert.match(bash, /apt-cache show fonts-liberation/);
  assert.match(bash, /package=\$\{package\/\/fonts-liberation\/fonts-liberation2\}/);
  assert.match(bash, /has no apt-get/);
  assert.match(bash, /browser_libraries_missing/);
});
