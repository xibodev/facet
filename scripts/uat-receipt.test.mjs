import { test } from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { Writable } from 'node:stream';
import { spawnSync } from 'node:child_process';
import { randomUUID } from 'node:crypto';
import { forward, realFacet, digest, activeReceiptDirectory } from './uat-facet-recorder.mjs';
import { beginReceiptTurn, collectReceipts, receiptIndex, receiptProof } from './uat-receipt-proof.mjs';

// All data here is synthetic; none constitutes real Facet/UAT execution evidence.
const project = '/home/facet/production', native = 'synthetic-session';
const hash = 'a'.repeat(64), binary = 'b'.repeat(64);
function sample() {
  const id = randomUUID();
  const input = { command: `cd ${project} && facet tools run video_compose --input artifacts/props.json | tail -n 2` };
  const data = { type: 'tool_result', session_id: native, tool_id: 'synthetic-call', tool_name: 'bash', tool_input: input, tool_output: '}\n' };
  data.raw = { type: 'tool_use', sessionID: native, part: { type: 'tool', callID: data.tool_id, sessionID: native, tool: 'bash', state: { status: 'completed', input, output: data.tool_output, metadata: { exit: 0 }, time: { start: 101, end: 190 } } } };
  const receipt = { version: 1, kind: 'instrumented-real-facet', id, startedAt: 110, completedAt: 180, cwd: project,
    executable: realFacet, executableSha256: binary, executableUnchanged: true, complete: true, exit: 0, signal: null,
    argv: ['tools', 'run', 'video_compose', '--input', 'artifacts/props.json'],
    stdout: JSON.stringify({ ok: true, tool: 'video_compose', operation: 'run', result: { output: 'renders/final.mp4', output_facts: { sha256: hash } } }), stderr: '' };
  return { captured: { receipts: [{ source: `${id}.json`, receipt }] }, events: [{ type: 'cc', data }], bounds: { startedAt: 100, completedAt: 200, executableSha256: binary } };
}
const prove = s => receiptProof(s.captured, s.events, project, native, { sha256: hash }, s.bounds);

test('synthetic receipt supplies complete envelope lost by legitimate downstream tail', () => {
  assert.equal(prove(sample())[0].binding, 'turn-time-cwd-and-observed-command-not-native-attestation');
});

test('receipt rejects stale/hash/wrong executable/path/partial/exit and unsupported session proof', () => {
  for (const mutate of [
    r => { r.startedAt = 99; }, r => { r.completedAt = 201; }, r => { r.cwd = '/tmp/other'; },
    r => { r.executable = '/tmp/facet'; }, r => { r.executableSha256 = 'c'.repeat(64); },
    r => { r.executableUnchanged = false; }, r => { r.kind = 'synthetic-forwarding-fixture'; },
    r => { r.complete = false; }, r => { r.exit = 1; }, r => { r.signal = 'SIGTERM'; },
    r => { r.stdout = r.stdout.slice(0, -2); }, r => { r.stdout += '\nlog'; },
    r => { r.argv[1] = 'estimate'; }, r => { r.argv[2] = 'media_probe'; },
    r => { r.stdout = r.stdout.replace(hash, 'c'.repeat(64)); },
    r => { r.stdout = r.stdout.replace('renders/final.mp4', '../final.mp4'); },
    r => { r.stdout = r.stdout.replace('renders/final.mp4', 'renders/earlier.mp4'); }
  ]) { const s = sample(); mutate(s.captured.receipts[0].receipt); assert.throws(() => prove(s)); }
  for (const mutate of [
    d => { d.session_id = 'other'; }, d => { d.raw.part.state.time.end = 179; },
    d => { d.raw.part.state.time.start = 99; }, d => { d.raw.part.callID = 'other'; },
    d => { d.tool_input.command = 'echo fake'; }, d => { d.raw.part.state.metadata.exit = 1; }
  ]) { const s = sample(); mutate(s.events[0].data); assert.throws(() => prove(s)); }
});

test('latest final receipt must match independently inspected media, not earlier hash', () => {
  const s = sample(), later = structuredClone(s.captured.receipts[0]);
  later.receipt.id = randomUUID(); later.source = `${later.receipt.id}.json`;
  later.receipt.completedAt++;
  later.receipt.stdout = later.receipt.stdout.replace(hash, 'c'.repeat(64));
  s.captured.receipts.push(later);
  assert.throws(() => prove(s));
  s.captured.receipts.reverse(); assert.throws(() => prove(s));
});

test('receipt collector rejects pending, baseline reuse, source traversal, mutation and symlinks', t => {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), 'uat-receipt-test-'));
  t.after(() => fs.rmSync(root, { recursive: true, force: true }));
  const turn = beginReceiptTurn(root), r = sample().captured.receipts[0].receipt;
  assert.equal(activeReceiptDirectory(root), turn.directory);
  const file = path.join(turn.directory, `${r.id}.json`);
  fs.writeFileSync(file, JSON.stringify(r));
  const captured = collectReceipts(turn);
  assert.throws(() => collectReceipts({ ...turn, baseline: captured.index }));
  fs.writeFileSync(file, JSON.stringify({ ...r, source: '../../credentials' }));
  assert.throws(() => collectReceipts(turn));
  assert.notDeepEqual(receiptIndex(turn.directory), captured.index);
  fs.unlinkSync(file);
  fs.writeFileSync(path.join(turn.directory, `${r.id}.pending`), '');
  assert.throws(() => collectReceipts(turn));
  fs.unlinkSync(path.join(turn.directory, `${r.id}.pending`));
  if (process.platform !== 'win32') {
    fs.symlinkSync(path.join(root, 'active.json'), file);
    assert.throws(() => collectReceipts(turn));
  }
  fs.writeFileSync(path.join(root, 'active.json'), JSON.stringify({ turnId: '../../other' }));
  assert.throws(() => activeReceiptDirectory(root));
});

test('forwarder executes an actual child unchanged; synthetic fixture never accepted as real Facet', async t => {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), 'uat-forward-test-'));
  t.after(() => fs.rmSync(root, { recursive: true, force: true }));
  // This is a real Node child probe, not a fake Facet CLI or canned success envelope.
  const code = 'process.stdout.write(JSON.stringify({argv:process.argv.slice(1),cwd:process.cwd(),env:process.env.UAT_FORWARD_SYNTHETIC})+"\\nlast line\\n");process.stderr.write("child stderr\\n");process.exitCode=7';
  process.env.UAT_FORWARD_SYNTHETIC = 'inherited';
  t.after(() => { delete process.env.UAT_FORWARD_SYNTHETIC; });
  const argv = ['-e', code, 'space argument', 'quote"argument'];
  const direct = spawnSync(process.execPath, argv);
  const stdout = [], stderr = [];
  const sink = chunks => new Writable({ write(chunk, encoding, done) { chunks.push(Buffer.from(chunk)); done(); } });
  const result = await forward(process.execPath, argv, root, { stdout: sink(stdout), stderr: sink(stderr) });
  assert.equal(result.code, direct.status); assert.equal(result.code, 7);
  assert.deepEqual(Buffer.concat(stdout), direct.stdout); assert.deepEqual(Buffer.concat(stderr), direct.stderr);
  const receipt = JSON.parse(fs.readFileSync(path.join(root, fs.readdirSync(root)[0])));
  assert.equal(receipt.kind, 'synthetic-forwarding-fixture');
  assert.equal(receipt.stdout, direct.stdout.toString()); assert.equal(receipt.stderr, direct.stderr.toString());
  assert.deepEqual(receipt.argv, argv); assert.equal(receipt.cwd, process.cwd());
  assert.equal(receipt.executableSha256, digest(fs.readFileSync(process.execPath)));
  // A downstream tail-equivalent consumes the real forwarded stream, not receipt data.
  assert.equal(Buffer.concat(stdout).toString().trimEnd().split('\n').at(-1), 'last line');
  assert.ok(receipt.stdout.includes('inherited'));
});

test('bounded capture and recorder write failure cannot silently report child success', async t => {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), 'uat-bounded-test-'));
  t.after(() => fs.rmSync(root, { recursive: true, force: true }));
  const sink = () => new Writable({ write(chunk, encoding, done) { done(); } });
  const result = await forward(process.execPath, ['-e', 'process.stdout.write("0123456789")'], root, { limit: 4, stdout: sink(), stderr: sink() });
  assert.equal(result.code, 74);
  assert.equal(JSON.parse(fs.readFileSync(path.join(root, fs.readdirSync(root)[0]))).complete, false);
  await assert.rejects(forward(process.execPath, ['-e', ''], path.join(root, 'absent'), { stdout: sink(), stderr: sink() }));
});

test('receipt redaction does not alter forwarded child bytes', async t => {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), 'uat-redaction-test-'));
  t.after(() => fs.rmSync(root, { recursive: true, force: true }));
  process.env.UAT_SYNTHETIC_TOKEN = 'synthetic-secret-not-a-real-credential';
  t.after(() => { delete process.env.UAT_SYNTHETIC_TOKEN; });
  const output = [];
  const sink = new Writable({ write(chunk, encoding, done) { output.push(Buffer.from(chunk)); done(); } });
  const result = await forward(process.execPath, ['-e', 'process.stdout.write(process.env.UAT_SYNTHETIC_TOKEN)'], root, { stdout: sink, stderr: sink });
  assert.equal(result.code, 0);
  assert.equal(Buffer.concat(output).toString(), process.env.UAT_SYNTHETIC_TOKEN);
  const receipt = JSON.parse(fs.readFileSync(path.join(root, fs.readdirSync(root)[0])));
  assert.equal(receipt.stdout, '[REDACTED]');
});
