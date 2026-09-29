import fs from 'node:fs';
import path from 'node:path';
import { spawn } from 'node:child_process';
import { sanitizer } from './uat-sanitize.mjs';

export function atomicJSON(file, value, redact = text => text) {
  const temporary = `${file}.writing`;
  fs.writeFileSync(temporary, redact(JSON.stringify(value, null, 2)), { mode: 0o600 });
  fs.renameSync(temporary, file);
}

// Keep unsanitized events in memory only. Token requests block content publication
// from request dispatch (not response parsing), including failed/hung captures.
export function browserCheckpoint(out, report, transcript, consoleLog, { now = () => performance.now(), wallNow = Date.now } = {}) {
  const { redact, add } = sanitizer();
  const started = now();
  let turnStarted = null, turn = null, turnEventStart = 0, lastToolResult = null;
  const activeTools = new Map();
  let requests = 0, captured = false, phase = 'browser-start';
  const ready = () => captured && requests === 0;
  const telemetry = () => {
    const elapsedMs = now() - started;
    const events = transcript.slice(turnEventStart);
    const last = events.at(-1);
    return {
      receiptSource: 'playwright-binding', elapsedMs, turn,
      turnElapsedMs: turnStarted === null ? null : now() - turnStarted,
      eventCount: events.length,
      lastEvent: last?.receipt ? { ...last.receipt, type: last.type, normalizedType: last.data?.type ?? null } : null,
      silenceMs: last?.receipt ? elapsedMs - last.receipt.elapsedMs : null,
      activeTools: [...activeTools.values()],
      lastToolResult: lastToolResult && {
        ...lastToolResult,
        // Sanitize BEFORE bounding stdout so truncation cannot expose part of a secret.
        stdout: ready() ? redact(lastToolResult.stdout).slice(-8192) : null,
        stdoutTruncated: ready() ? redact(lastToolResult.stdout).length > 8192 : null
      }
    };
  };
  const write = (next = phase) => {
    phase = next;
    report.telemetry = telemetry();
    const progress = { status: 'running', final: false, adjudication: false, phase, updated_at: new Date(wallNow()).toISOString(), elapsed_ms: now() - started, content_withheld: !ready() };
    atomicJSON(path.join(out, 'progress.json'), progress);
    if (ready()) atomicJSON(path.join(out, 'checkpoint.json'), {
      ...progress, report: { ...report, passed: false }, transcript, console: consoleLog
    }, redact);
  };
  return {
    redact, ready, write, telemetry,
    beginTurn(number) {
      turn = number;
      turnStarted = now();
      turnEventStart = transcript.length;
      activeTools.clear();
      lastToolResult = null;
    },
    received(event) {
      const receipt = { receivedAt: new Date(wallNow()).toISOString(), elapsedMs: now() - started, turn, turnElapsedMs: turnStarted === null ? null : now() - turnStarted };
      const item = { ...event, receipt };
      transcript.push(item);
      if (event.type !== 'cc') return;
      const data = event.data ?? {}, part = data.raw?.part;
      const id = data.tool_id || part?.callID;
      const name = data.tool_name || part?.tool;
      const terminal = data.type === 'tool_result' || ['completed', 'error'].includes(part?.state?.status);
      if (terminal) {
        if (id) activeTools.delete(id);
        const stdout = data.tool_output ?? part?.state?.output;
        lastToolResult = { eventIndex: transcript.length - 1, tool_id: id ?? null, tool_name: name ?? null, ...receipt, stdout: typeof stdout === 'string' ? stdout : '' };
      } else if (id && name && (data.type === 'tool_use' || (part?.type === 'tool' && part.state?.status === 'running'))) {
        activeTools.set(id, { tool_id: id, tool_name: name, ...receipt });
      }
    },
    tokenRequested() { requests++; },
    tokenCaptured(data) {
      if (typeof data?.token !== 'string' || data.token.length <= 3) return;
      add(data.token);
      captured = true;
      requests = Math.max(0, requests - 1);
    }
  };
}

// One child, one non-overlapping checkpoint callback. Drain the last export
// before returning so no timer can mutate evidence after the caller seals it.
export async function runWithCheckpoints(command, args, { timeout, checkpoint, intervalMs = 30000, runtime = { now: () => performance.now(), spawn, setInterval, clearInterval, setTimeout, clearTimeout }, ...options }) {
  const started = runtime.now();
  const child = runtime.spawn(command, args, { stdio: 'ignore', ...options });
  let failure, exporting = null, checkpointFailed = false, timedOut = false, timeoutElapsedMs = null;
  const publish = () => {
    if (!exporting) exporting = Promise.resolve().then(checkpoint).catch(() => { checkpointFailed = true; }).finally(() => { exporting = null; });
    return exporting;
  };
  const timer = runtime.setInterval(publish, intervalMs);
  const deadline = runtime.setTimeout(() => { timedOut = true; timeoutElapsedMs = runtime.now() - started; failure = 'Runner failed or timed out'; child.kill('SIGKILL'); }, timeout);
  const result = await new Promise(resolve => {
    child.on('error', () => { failure = 'Runner failed or timed out'; });
    child.on('close', (code, signal) => resolve({ exit_code: code, signal, error: failure }));
  });
  runtime.clearInterval(timer);
  runtime.clearTimeout(deadline);
  await exporting;
  await publish();
  return { ...result, timed_out: timedOut, timeout_ms: timeout, timeout_elapsed_ms: timeoutElapsedMs, checkpoint_failed: checkpointFailed, elapsed_ms: runtime.now() - started };
}
