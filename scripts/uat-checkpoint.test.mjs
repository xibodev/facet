import { test } from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { setTimeout as delay } from 'node:timers/promises';
import { runInNewContext } from 'node:vm';
import { EventEmitter } from 'node:events';
import { atomicJSON, browserCheckpoint, runWithCheckpoints } from './uat-checkpoint.mjs';
import { exportNames, runnerDiagnostics } from './uat-probe.mjs';
import { completedRender } from './uat-proof-fixtures.mjs';
import { processInventory } from './uat-checkpoint-process.mjs';

test('receipt clocks measure observation, not checkpoint writes or provider timestamps', () => {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'facet-uat-receipts-'));
  try {
    let tick = 100, wall = 100000;
    const transcript = [], report = {};
    const checkpoint = browserCheckpoint(dir, report, transcript, [], { now: () => tick, wallNow: () => wall });
    checkpoint.tokenRequested();
    checkpoint.tokenCaptured({ token: 'synthetic-secret' });
    checkpoint.beginTurn(1);
    tick = 150;
    assert.equal(checkpoint.telemetry().lastEvent, null);
    assert.equal(checkpoint.telemetry().eventCount, 0);
    assert.deepEqual(checkpoint.telemetry().activeTools, []);
    const event = structuredClone(completedRender);
    event.data.tool_output = 'safe stdout synthetic-secret';
    checkpoint.received(event);
    assert.equal(transcript[0].receipt.elapsedMs, 50);
    assert.equal(transcript[0].receipt.receivedAt, new Date(wall).toISOString());
    assert.notEqual(transcript[0].receipt.receivedAt, new Date(event.data.raw.timestamp).toISOString());
    assert.deepEqual(checkpoint.telemetry().activeTools, [], 'Completed-only events never invent a start');
    assert.equal(checkpoint.telemetry().lastToolResult.stdout, 'safe stdout [REDACTED]');
    tick = 250; wall = 90000; // Wall-clock correction must not affect elapsed time.
    checkpoint.write('still-waiting');
    assert.equal(report.telemetry.lastEvent.elapsedMs, 50);
    assert.equal(report.telemetry.silenceMs, 100);
    checkpoint.received({ type: 'cc', data: { type: 'tool_use', tool_id: 'a', tool_name: 'bash' } });
    checkpoint.received({ type: 'cc', data: { type: 'tool_use', tool_id: 'b', tool_name: 'read' } });
    checkpoint.received({ type: 'cc', data: { type: 'tool_result', tool_id: 'a', tool_output: 'done' } });
    assert.deepEqual(checkpoint.telemetry().activeTools.map(tool => tool.tool_id), ['b']);
    checkpoint.beginTurn(2);
    assert.equal(checkpoint.telemetry().eventCount, 0);
    assert.equal(checkpoint.telemetry().lastEvent, null);
    assert.equal(checkpoint.telemetry().lastToolResult, null);
    assert.deepEqual(checkpoint.telemetry().activeTools, []);
    assert.equal(transcript.length, 4, 'No synthetic start added to evidence');
  } finally { fs.rmSync(dir, { recursive: true, force: true }); }
});

test('timeout stdout is bounded only after sanitation and withheld during token rotation', () => {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'facet-uat-stdout-'));
  try {
    const checkpoint = browserCheckpoint(dir, {}, [], [], { now: () => 0, wallNow: () => 0 });
    const secret = 's'.repeat(9000);
    checkpoint.tokenRequested();
    checkpoint.received({ type: 'cc', data: { type: 'tool_result', tool_id: 'a', tool_output: `prefix${secret}` } });
    assert.equal(checkpoint.telemetry().lastToolResult.stdout, null);
    checkpoint.tokenCaptured({ token: secret });
    assert.equal(checkpoint.telemetry().lastToolResult.stdout, 'prefix[REDACTED]');
    checkpoint.received({ type: 'cc', data: { type: 'tool_result', tool_output: 'x'.repeat(10000) } });
    assert.equal(checkpoint.telemetry().lastToolResult.stdout.length, 8192);
    assert.equal(checkpoint.telemetry().lastToolResult.stdoutTruncated, true);
    checkpoint.tokenRequested();
    assert.equal(checkpoint.telemetry().lastToolResult.stdout, null);
  } finally { fs.rmSync(dir, { recursive: true, force: true }); }
});

test('runner timeout evidence uses the deadline clock and drains export before return', async () => {
  let tick = 10, deadline, interval, exported = false;
  const cleared = [];
  const child = new EventEmitter();
  child.kill = signal => { assert.equal(signal, 'SIGKILL'); child.emit('close', null, signal); };
  const runtime = {
    now: () => tick, spawn: () => child,
    setTimeout: (fn, ms) => { assert.equal(ms, 100); deadline = fn; return 'deadline'; },
    setInterval: fn => { interval = fn; return 'interval'; },
    clearTimeout: id => cleared.push(id), clearInterval: id => cleared.push(id)
  };
  const pending = runWithCheckpoints('synthetic', [], { timeout: 100, runtime, checkpoint: async () => { await Promise.resolve(); exported = true; } });
  tick = 110;
  interval();
  deadline();
  const result = await pending;
  assert.equal(result.timed_out, true);
  assert.equal(result.timeout_ms, 100);
  assert.equal(result.timeout_elapsed_ms, 100);
  assert.equal(result.elapsed_ms, 100);
  assert.equal(result.exit_code, null);
  assert.equal(exported, true);
  assert.deepEqual(cleared, ['interval', 'deadline']);
});

test('failed runner writes durable diagnostics even if capture fails, without copying command errors', async () => {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'facet-uat-runner-diagnostic-'));
  try {
    const result = { timed_out: true, exit_code: null, timeout_ms: 100, timeout_elapsed_ms: 101 };
    await runnerDiagnostics('browser', result, dir, async () => {
      await Promise.resolve();
      throw new Error('private command args/env/auth synthetic-secret');
    }, () => 0);
    const file = path.join(dir, 'browser-runner-diagnostics.json');
    const text = fs.readFileSync(file, 'utf8');
    const diagnostic = JSON.parse(text);
    assert.equal(diagnostic.observed_at, '1970-01-01T00:00:00.000Z');
    assert.equal(diagnostic.adjudication, false);
    assert.equal(diagnostic.timeout_elapsed_ms, 101);
    assert.equal(diagnostic.process_inventory.status, 'unavailable');
    assert.ok(!text.includes('synthetic-secret'));
    const inventory = { status: 'complete', processes: [{ pid: 10, ppid: 1, name: 'node', elapsedMs: 5, cpuMs: 0, rssBytes: 1024 }] };
    await runnerDiagnostics('browser', { ...result, timed_out: false, exit_code: 1 }, dir, async () => JSON.stringify(inventory), () => 0);
    assert.deepEqual(JSON.parse(fs.readFileSync(file)).process_inventory, inventory);
    assert.equal(JSON.parse(fs.readFileSync(file)).timed_out, false);
    await runnerDiagnostics('installation', { timed_out: false, exit_code: 0 }, dir, () => { assert.fail('Successful child needs no failure capture'); });
    assert.deepEqual(fs.readdirSync(dir), ['browser-runner-diagnostics.json']);
  } finally { fs.rmSync(dir, { recursive: true, force: true }); }
});

test('bounded proc fixture captures owned active targets and descendants, never arbitrary names or secrets', () => {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), 'facet-uat-proc-'));
  const fixture = (pid, ppid, name, uid = 10001, state = 'S') => {
    const dir = path.join(root, String(pid));
    fs.mkdirSync(dir);
    const fields = Array(22).fill('0');
    fields[0] = state; fields[1] = String(ppid); fields[11] = '120'; fields[12] = '30'; fields[19] = '200';
    fs.writeFileSync(path.join(dir, 'stat'), `${pid} (${name}) ${fields.join(' ')}`);
    fs.writeFileSync(path.join(dir, 'status'), `Uid:\t${uid}\t${uid}\t${uid}\t${uid}\nVmRSS:\t42 kB\n`);
  };
  try {
    fs.writeFileSync(path.join(root, 'uptime'), '12 0');
    fixture(10, 1, 'node');
    fixture(11, 10, 'secret ) arbitrary');
    fixture(12, 11, 'ffmpeg');
    fixture(13, 1, 'opencode');
    fixture(14, 1, 'unrelated-secret');
    fixture(15, 10, 'node', 0);
    fixture(16, 10, 'ffmpeg', 10001, 'Z');
    fs.mkdirSync(path.join(root, '17')); // Disappeared or unreadable process metadata.
    const options = { root, uid: 10001, now: () => 0, clockTicks: 100 };
    const result = processInventory(options);
    assert.deepEqual(result.processes.map(row => row.pid), [10, 11, 12, 13]);
    assert.deepEqual(result.processes[0], { pid: 10, ppid: 1, name: 'node', elapsedMs: 10000, cpuMs: 1500, rssBytes: 43008 });
    assert.equal(result.processes[1].name, 'other');
    assert.equal(result.skipped, 1);
    assert.equal(result.status, 'partial');
    assert.ok(!JSON.stringify(result).includes('secret'));
    assert.equal(processInventory({ ...options, maxRows: 1 }).processes.length, 1);
    assert.equal(processInventory({ ...options, maxProcesses: 1 }).scanned, 1);
    assert.equal(processInventory({ ...options, maxEntries: 0 }).truncated, true);
    let clock = 0;
    assert.equal(processInventory({ ...options, now: () => clock++, budgetMs: 1 }).truncated, true);
    assert.equal(processInventory({ ...options, root: path.join(root, 'absent') }).status, 'unavailable');
    fs.writeFileSync(path.join(root, '10', 'status'), 'x'.repeat(17000));
    assert.equal(processInventory(options).skipped, 2, 'Oversize metadata must not be read without a bound');
  } finally { fs.rmSync(root, { recursive: true, force: true }); }
});

test('partial checkpoint withholds pending tokens and sanitizes buffered content at export', () => {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'facet-uat-checkpoint-'));
  try {
    const secret = 'synthetic-quote"-slash\\-newline\n-token';
    const report = { passed: true, checks: [], turns: [] };
    let nested = secret;
    for (let i = 0; i < 8; i++) nested = JSON.stringify({ output: nested });
    const transcript = [{ message: secret, nested, url: encodeURIComponent(secret) }];
    const consoleLog = [{ text: secret }];
    const checkpoint = browserCheckpoint(dir, report, transcript, consoleLog);
    checkpoint.write('before-navigation');
    assert.equal(fs.existsSync(path.join(dir, 'checkpoint.json')), false);
    checkpoint.tokenRequested();
    checkpoint.write('token-response-pending');
    assert.equal(fs.existsSync(path.join(dir, 'checkpoint.json')), false);
    checkpoint.tokenCaptured({ token: secret });
    checkpoint.write('turn-1-wait');
    const file = path.join(dir, 'checkpoint.json');
    const text = fs.readFileSync(file, 'utf8');
    const partial = JSON.parse(text);
    assert.equal(partial.status, 'running');
    assert.equal(partial.final, false);
    assert.equal(partial.adjudication, false);
    assert.equal(partial.report.passed, false);
    assert.equal(report.passed, true, 'Checkpoint must not alter acceptance state');
    assert.equal(partial.transcript[0].message, '[REDACTED]');
    assert.equal(partial.transcript[0].url, '[REDACTED]');
    assert.equal(partial.console[0].text, '[REDACTED]');
    let decoded = partial.transcript[0].nested;
    for (let i = 0; i < 8; i++) decoded = JSON.parse(decoded).output;
    assert.equal(decoded, '[REDACTED]');
    assert.equal(transcript[0].message, secret, 'Raw token is retained only in memory for later sanitation');
    const second = 'synthetic-second-token';
    checkpoint.tokenRequested();
    transcript.push({ message: second });
    checkpoint.write();
    assert.equal(fs.readFileSync(file, 'utf8'), text, 'Pending rotation cannot publish unregistered content');
    assert.equal(JSON.parse(fs.readFileSync(path.join(dir, 'progress.json'))).content_withheld, true);
    checkpoint.tokenCaptured({ token: '' });
    assert.equal(checkpoint.ready(), false, 'Invalid capture fails closed');
    checkpoint.tokenCaptured({ token: second });
    checkpoint.write('turn-2-wait');
    assert.equal(JSON.parse(fs.readFileSync(file)).transcript[1].message, '[REDACTED]');
    assert.ok(fs.readdirSync(dir).every(name => ['checkpoint.json', 'progress.json'].includes(name)));
  } finally { fs.rmSync(dir, { recursive: true, force: true }); }
});

test('atomic JSON replaces valid snapshots without retaining intermediate files', () => {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'facet-uat-atomic-'));
  try {
    const file = path.join(dir, 'progress.json');
    atomicJSON(file, { step: 1 });
    atomicJSON(file, { step: 2 });
    assert.deepEqual(JSON.parse(fs.readFileSync(file)), { step: 2 });
    assert.deepEqual(fs.readdirSync(dir), ['progress.json']);
  } finally { fs.rmSync(dir, { recursive: true, force: true }); }
});

test('synthetic running child exports durable partial evidence before timeout, never after return', async () => {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'facet-uat-child-'));
  let exports = 0, active = 0, maxActive = 0;
  try {
    const result = await runWithCheckpoints(process.execPath, ['-e', 'setInterval(()=>{},1000)'], {
      timeout: 200, intervalMs: 10,
      checkpoint: async () => {
        active++;
        maxActive = Math.max(maxActive, active);
        await delay(25);
        atomicJSON(path.join(dir, 'partial.json'), { status: 'running', final: false, adjudication: false, sequence: ++exports });
        active--;
      }
    });
    assert.match(result.error, /timed out/);
    assert.notEqual(result.exit_code, 0);
    assert.equal(maxActive, 1);
    assert.ok(exports >= 2, 'Checkpoints must execute while child is alive');
    assert.equal(result.checkpoint_failed, false);
    const retained = fs.readFileSync(path.join(dir, 'partial.json'), 'utf8');
    assert.equal(JSON.parse(retained).adjudication, false);
    await delay(60);
    assert.equal(fs.readFileSync(path.join(dir, 'partial.json'), 'utf8'), retained);
    assert.equal(active, 0);
  } finally { fs.rmSync(dir, { recursive: true, force: true }); }
});

test('child exit, spawn failure and export failure remain failures; last export is awaited', async () => {
  let exported = false;
  const success = await runWithCheckpoints(process.execPath, ['-e', 'process.exit(0)'], {
    timeout: 2000, checkpoint: async () => { await delay(10); exported = true; }
  });
  assert.equal(success.exit_code, 0);
  assert.equal(success.timed_out, false);
  assert.equal(success.timeout_elapsed_ms, null);
  assert.equal(exported, true);
  const failed = await runWithCheckpoints(process.execPath, ['-e', 'process.exit(7)'], {
    timeout: 2000, checkpoint: () => { throw new Error('synthetic private error not exported'); }
  });
  assert.equal(failed.exit_code, 7);
  assert.equal(failed.checkpoint_failed, true);
  assert.ok(!JSON.stringify(failed).includes('private'));
  const absent = await runWithCheckpoints('nonexistent-uat-synthetic-command', [], { timeout: 2000, checkpoint: () => {} });
  assert.match(absent.error, /Runner failed/);
  assert.equal(absent.timed_out, false, 'Spawn failure is not a deadline expiry');
});

test('host allowlist excludes private archives and staging files; installation exports before browser', () => {
  assert.ok(exportNames.browser.includes('checkpoint.json'));
  assert.ok(exportNames.browser.includes('trace.zip'));
  for (const names of Object.values(exportNames)) {
    assert.ok(names.every(name => !/private|writing|copying|[/\\]/.test(name)));
  }
  const probe = fs.readFileSync(new URL('./uat-probe.mjs', import.meta.url), 'utf8');
  assert.match(probe, /await exportTrack\(track, true\);\s+progress\(`\$\{track\}-exported`\);\s+}/);
  assert.match(probe, /track === 'browser' \? \['progress.json', 'checkpoint.json'\] : \[\]/);
  assert.ok(probe.includes("final=r.final===true&&r.status==='complete'"));
  assert.ok(probe.includes("if(!final)allowed=allowed.filter(n=>n==='progress.json'||n==='checkpoint.json')"));
  const browser = fs.readFileSync(new URL('./uat-browser.mjs', import.meta.url), 'utf8');
  assert.ok(browser.indexOf('assert.ok(checkpoint.ready()') < browser.indexOf('const raw = new AdmZip'));
  assert.match(browser, /atomicJSON\(path.join\(out, 'result.json'\), \{ \.\.\.report, status: 'complete', final: true }/);
  assert.ok(browser.includes('checkpoint.received(event)'));
  assert.ok(browser.includes('report.timeout_diagnostics.processInventory = await captureProcessInventory()'));
  assert.ok(probe.indexOf('await runnerDiagnostics(track, result') < probe.indexOf('await exportTrack(track, true)'));
});

test('synthetic host listing withholds unfinished trace/media and rejects directories', () => {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'facet-uat-listing-'));
  try {
    const browserDir = path.join(dir, 'browser');
    fs.mkdirSync(browserDir);
    for (const name of ['progress.json', 'checkpoint.json', 'trace.zip', 'trace-private.zip', 'turn-1.mp4']) fs.writeFileSync(path.join(browserDir, name), '{}');
    const probe = fs.readFileSync(new URL('./uat-probe.mjs', import.meta.url), 'utf8');
    const listing = probe.match(/const listing = `([^`]+)`;/)[1];
    const list = () => {
      let output;
      runInNewContext(listing, {
        require: name => { assert.equal(name, 'node:fs'); return fs; },
        process: { argv: ['node', browserDir.replaceAll('\\', '/'), JSON.stringify(exportNames.browser)] },
        console: { log: text => { output = JSON.parse(text); } }
      });
      return output.map(file => file.name);
    };
    assert.deepEqual(list(), ['progress.json', 'checkpoint.json']);
    atomicJSON(path.join(browserDir, 'result.json'), { final: false, status: 'running' });
    assert.deepEqual(list(), ['progress.json', 'checkpoint.json']);
    atomicJSON(path.join(browserDir, 'result.json'), { final: true, status: 'complete', passed: false });
    assert.deepEqual(list(), ['progress.json', 'checkpoint.json', 'result.json', 'trace.zip', 'turn-1.mp4']);
    // Directories are never copied as though they were allowlisted files.
    fs.mkdirSync(path.join(browserDir, 'console.json'));
    assert.ok(!list().includes('console.json'));
  } finally { fs.rmSync(dir, { recursive: true, force: true }); }
});
