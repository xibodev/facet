import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { test } from 'node:test';

const root = new URL('../', import.meta.url);
const bash = readFileSync(new URL('install.sh', root), 'utf8');
const powershell = readFileSync(new URL('install.ps1', root), 'utf8');
const manifest = readFileSync(new URL('installer/manifest.tsv', root), 'utf8');

test('installer manifest covers every supported adapter and production method', () => {
  const instructions = {
    claude: 'CLAUDE.md',
    copilot: '.github/copilot-instructions.md',
    codex: 'AGENTS.md',
    opencode: 'AGENTS.md',
    studio: 'AGENTS.md',
  };
  for (const [adapter, instruction] of Object.entries(instructions)) {
    assert.match(manifest, new RegExp(`^host\\t${adapter}\\t`, 'm'));
    assert.match(manifest, new RegExp(`^instruction\\t${adapter}-instructions\\t-\\t${instruction.replaceAll('.', '\\.')}\\t`, 'm'));
  }
  assert.match(manifest, /^host\tcodex\t-\t\.agents\/skills\t/m);
  assert.doesNotMatch(manifest, /\.codex\/skills/);
  for (const method of ['explainer', 'cinematic', 'screen-demo', 'talking-head', 'social', 'character-animation', 'localization']) {
    assert.match(manifest, new RegExp(`^pack\\t${method}\\t`, 'm'));
  }
});

test('script installers expose explicit repeatable production-method selection', () => {
  assert.match(bash, /--pack\|--production-method/);
  assert.match(bash, /FACET_PACKS/);
  assert.match(bash, /packs\\t%s/);
  assert.match(powershell, /\[Alias\('ProductionMethod'\)\]\[string\[\]\]\$Pack/);
  assert.match(powershell, /FACET_PACKS/);
  assert.match(powershell, /\$_\.Split\(','\)/);
  assert.match(powershell, /packs=\$selectedPacks/);
});

test('script installer defaults remain core-only', () => {
  assert.doesNotMatch(bash, /PACKS=.*explainer/);
  assert.doesNotMatch(powershell, /\$Pack\s*=.*explainer/);
  assert.match(bash, /No production-method pack is active; use the core guidance only/);
  assert.match(powershell, /No production-method pack is active; use the core guidance only/);
});

test('script installers manage bounded instruction sections with ownership metadata', () => {
  for (const source of [bash, powershell]) {
    assert.match(source, /facet:managed:start/);
    assert.match(source, /facet:managed:end/);
    assert.match(source, /instruction-section/);
  }
});
