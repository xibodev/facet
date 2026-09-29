import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import { execFileSync } from 'node:child_process';
import { inspectMedia } from './uat-media.mjs';
import { sanitizer } from './uat-sanitize.mjs';

const evidence = process.argv[2] || '/home/facet/uat-evidence/installation';
fs.mkdirSync(evidence, { recursive: true });
const { redact } = sanitizer();
const report = { track: 'deterministic-installation-renderer', real_agent_acceptance: false, checks: [] };
fs.writeFileSync(path.join(evidence, 'install-build.log'), redact(fs.readFileSync('/home/facet/uat-install-build.log', 'utf8')));
function run(command, args, options = {}) {
  return execFileSync(command, args, { encoding: 'utf8', timeout: 120000, stdio: ['ignore', 'pipe', 'pipe'], ...options });
}
function check(name, fn) {
  try { const facts = fn(); report.checks.push({ name, passed: true, facts }); }
  catch (error) { report.checks.push({ name, passed: false, error: redact(error.message) }); process.exitCode = 1; }
}
check('installed-toolchain', () => {
  const versions = {};
  for (const [command, args] of [['facet', ['--version']], ['opencode', ['--version']], ['node', ['--version']], ['ffmpeg', ['-version']], ['ffprobe', ['-version']], ['chromium', ['--version']]]) {
    versions[command] = run(command, args).split('\n')[0];
  }
  assert.match(versions.opencode, /1\.18\.29/);
  return versions;
});
check('sanitizer-negative-control', () => {
  const safe = sanitizer();
  safe.add('uat-secret-test-only-123');
  const text = safe.redact('{"token":"uat-secret-test-only-123","url":"/api/chat?token=uat-secret-test-only-123&prompt=test"}');
  assert.ok(!text.includes('uat-secret-test-only-123'));
  assert.match(text, /REDACTED/);
  return { literal_and_query_token_removed: true };
});
check('agent-proof-and-escaped-secret-negative-controls', () => {
  const log = run('node', ['--test', '/opt/uat/uat-proof.test.mjs']);
  fs.writeFileSync(path.join(evidence, 'proof-tests.log'), redact(log));
  return { synthetic_only: true, real_agent_acceptance: false };
});
check('installed-bundle-and-project-init', () => {
  const home = process.env.HOME;
  assert.ok(fs.existsSync(path.join(home, '.facet/bundle/skills')));
  assert.ok(fs.existsSync(path.join(home, '.facet/bundle/packs')));
  const project = fs.mkdtempSync(path.join(home, 'uat-install-project-'));
  run('facet', ['init', project, '--engine', 'opencode', '--no-launch']);
  assert.ok(fs.existsSync(path.join(project, 'facet.lock.json')));
  assert.ok(fs.existsSync(path.join(project, '.opencode/skills/facet/SKILL.md')));
  return { project_initialized_outside_source: true };
});
check('strict-media-probe-negative-controls', () => {
  const dir = fs.mkdtempSync(path.join(process.env.HOME, 'uat-probes-'));
  const valid = path.join(dir, 'valid.mp4');
  run('ffmpeg', ['-v', 'error', '-y', '-f', 'lavfi', '-i', 'color=c=blue:s=320x180:r=24:d=1', '-f', 'lavfi', '-i', 'sine=frequency=440:duration=1', '-c:v', 'libx264', '-pix_fmt', 'yuv420p', '-c:a', 'aac', '-shortest', valid]);
  const facts = inspectMedia(valid);
  const garbage = path.join(dir, 'garbage.mp4');
  fs.writeFileSync(garbage, Buffer.alloc(4096, 65));
  const truncated = path.join(dir, 'truncated.mp4');
  fs.writeFileSync(truncated, fs.readFileSync(valid).subarray(0, 512));
  const silent = path.join(dir, 'silent.mp4');
  run('ffmpeg', ['-v', 'error', '-y', '-i', valid, '-an', '-c:v', 'copy', silent]);
  const wrong = path.join(dir, 'wrong.mp4');
  run('ffmpeg', ['-v', 'error', '-y', '-i', valid, '-c:v', 'mpeg4', '-c:a', 'aac', wrong]);
  for (const file of [path.join(dir, 'missing.mp4'), garbage, truncated, silent, wrong]) assert.throws(() => inspectMedia(file), path.basename(file));
  assert.throws(() => inspectMedia(valid, { minDuration: 5 }));
  assert.throws(() => inspectMedia(valid, { maxDuration: 0.5 }));
  for (const file of [valid, garbage, truncated, silent, wrong]) {
    let passed = true;
    try { run('bash', ['/opt/uat/probe-artifact.sh', file]); } catch { passed = false; }
    assert.equal(passed, file === valid);
  }
  return { valid: facts, rejected: ['missing', 'garbage-over-1000-bytes', 'truncated', 'missing-audio', 'wrong-codec', 'short-duration', 'long-duration'] };
});
check('real-remotion-render-no-fallback', () => {
  const candidates = [path.join(process.env.HOME, '.facet/bundle/remotion-composer'), path.join(process.env.HOME, '.facet/runtimes/remotion/current'), path.join(process.env.HOME, '.facet/remotion-composer')];
  const composer = candidates.find(p => fs.existsSync(path.join(p, 'node_modules/.bin/remotion')));
  assert.ok(composer, 'Installer must install Remotion and dependencies under HOME');
  const output = path.join(evidence, 'renderer-fixture.mp4');
  const props = path.join(evidence, 'renderer-props.json');
  fs.writeFileSync(props, JSON.stringify({ cuts: [{ id: 'uat-fixture', type: 'hero_title', text: 'Deterministic renderer fixture', in_seconds: 0, out_seconds: 1 }] }));
  const log = run(path.join(composer, 'node_modules/.bin/remotion'), ['render', path.join(composer, 'src/index.tsx'), 'Explainer', output, '--props', props, '--browser-executable=/usr/bin/chromium', '--concurrency=1', '--frames=0-29', '--log=error'], { cwd: composer, timeout: 180000 });
  fs.writeFileSync(path.join(evidence, 'renderer.log'), redact(log));
  return inspectMedia(output, { audio: false, maxDuration: 3 });
});
check('installed-facet-video-compose-with-audio', () => {
  const project = fs.mkdtempSync(path.join(process.env.HOME, 'uat-toolbox-project-'));
  run('facet', ['init', project, '--engine', 'opencode', '--no-launch']);
  fs.mkdirSync(path.join(project, 'narration'), { recursive: true });
  fs.mkdirSync(path.join(project, 'artifacts'), { recursive: true });
  const audio = path.join(project, 'narration/fixture.wav');
  run('ffmpeg', ['-v', 'error', '-y', '-f', 'lavfi', '-i', 'sine=frequency=523:duration=2', audio]);
  const props = {
    cuts: [{ id: 'toolbox-fixture', type: 'hero_title', text: 'Installed Facet renderer fixture', in_seconds: 0, out_seconds: 2 }],
    audio_path: audio, output: 'renders/final.mp4', timeout_seconds: 180
  };
  const input = path.join(project, 'artifacts/props.json');
  fs.writeFileSync(input, JSON.stringify(props));
  assert.equal(fs.existsSync(path.join(project, 'renders/final.mp4')), false);
  let log;
  try {
    log = run('facet', ['tools', 'run', 'video_compose', '--input', input], { cwd: project, timeout: 210000 });
  } catch (error) {
    fs.writeFileSync(path.join(evidence, 'facet-compose.log'), redact(JSON.stringify({ status: error.status, signal: error.signal, stdout: String(error.stdout || ''), stderr: String(error.stderr || '') }, null, 2)));
    throw error;
  }
  fs.writeFileSync(path.join(evidence, 'facet-compose.log'), redact(log));
  fs.copyFileSync(input, path.join(evidence, 'facet-compose-props.json'));
  const file = path.join(project, 'renders/final.mp4');
  const facts = inspectMedia(file, { minDuration: 1.5, maxDuration: 2.5 });
  const video = facts.streams.find(s => s.codec_type === 'video');
  assert.equal(video.width, 1920); assert.equal(video.height, 1080);
  assert.equal(video.avg_frame_rate, '30/1');
  fs.copyFileSync(file, path.join(evidence, 'facet-compose-fixture.mp4'));
  // A broken explicit runtime pin must fail, not manufacture fallback output.
  fs.writeFileSync(path.join(project, '.facet.yaml'), 'paths:\n  remotion_composer: /nonexistent-uat-runtime\n');
  props.output = 'renders/must-not-exist.mp4';
  fs.writeFileSync(input, JSON.stringify(props));
  assert.throws(() => run('facet', ['tools', 'run', 'video_compose', '--input', input], { cwd: project }));
  assert.equal(fs.existsSync(path.join(project, props.output)), false);
  return { cwd_outside_source: true, actual_audio_fixture: true, broken_runtime_rejected_without_output: true, facts };
});
fs.writeFileSync(path.join(evidence, 'checks.json'), redact(JSON.stringify(report, null, 2)));
console.log(JSON.stringify({ track: report.track, checks: report.checks.map(({ name, passed }) => ({ name, passed })) }));
