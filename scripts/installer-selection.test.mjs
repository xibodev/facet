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
    app: 'AGENT.md',
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

test('script installers expose the Facet app target without the retired studio target', () => {
  assert.match(bash, /--target opencode\|codex\|claude\|copilot\|app/);
  assert.match(bash, /Which agent should use Facet\? \(opencode, codex, claude, copilot, app\)/);
  assert.doesNotMatch(bash, /--target opencode\|codex\|claude\|copilot\|studio/);
  assert.match(powershell, /\[ValidateSet\('opencode','codex','claude','copilot','app'\)\]/);
  assert.doesNotMatch(powershell, /\[ValidateSet\([^)]*'studio'/);
});

test('script installers migrate legacy standalone receipts to the app target', () => {
  assert.match(bash, /previous_host.+studio.+previous_host=app/s);
  assert.match(powershell, /if \(\$receipt\.host -eq 'studio'\) \{ \$receipt\.host = 'app' \}/);
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

test('script installers default to the complete local runtime profile', () => {
  assert.match(bash, /COMPONENTS=\$\{COMPONENTS:-remotion,piper\}/);
  assert.match(powershell, /\$defaultComponents = 'remotion'/);
  assert.match(powershell, /\$defaultComponents \+= ',piper'/);
  assert.match(powershell, /if \(-not \$Components\) \{ \$Components=\$defaultComponents \}/);
  assert.match(manifest, /^builtin\tedge-tts\t-\tmicrosoft_edge\tall\tall\tall\t0\t/m);
});

test('script installer defaults remain core-only', () => {
  assert.doesNotMatch(bash, /PACKS=.*explainer/);
  assert.doesNotMatch(powershell, /\$Pack\s*=.*explainer/);
  assert.match(bash, /No production-method pack is active; use the core guidance only/);
  assert.match(powershell, /No production-method pack is active; use the core guidance only/);
});

test('public installers do not advertise downloads from the private gflow repository', () => {
  assert.doesNotMatch(manifest, /^component\tgflow\t/m);
  assert.doesNotMatch(bash, /Download gflow|releases\/download\/v.*gflow/s);
  assert.doesNotMatch(powershell, /Download gflow|releases\/download\/v.*gflow/s);
});

test('bash installer accepts an empty production-method selection on Bash 3.2', () => {
  assert.match(bash, /for id in "\$\{raw_packs\[@\]:-\}"; do/);
  assert.match(bash, /packs\\t%s\\n'.+"\$\{SELECTED_PACKS\[\*\]:-\}"/);
});

test('script installers manage bounded instruction sections with ownership metadata', () => {
  for (const source of [bash, powershell]) {
    assert.match(source, /facet:managed:start/);
    assert.match(source, /facet:managed:end/);
    assert.match(source, /instruction-section/);
    assert.match(source, /uninstall/i);
    assert.match(source, /shared runtime/i);
  }
});

test('script launchers expose the installed Piper voice to Facet', () => {
  assert.match(bash, /FACET_PIPER_MODEL/);
  assert.match(powershell, /FACET_PIPER_MODEL/);
});

test('linux installer accepts current browser dependency package names', () => {
  assert.match(bash, /apt-get update/);
  assert.match(bash, /apt-cache show fonts-liberation/);
  assert.match(bash, /package=\$\{package\/\/fonts-liberation\/fonts-liberation2\}/);
});
