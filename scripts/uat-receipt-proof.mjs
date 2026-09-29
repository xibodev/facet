import fs from 'node:fs';
import path from 'node:path';
import assert from 'node:assert/strict';
import { randomUUID } from 'node:crypto';
import { digest, readReceiptFile, realFacet, receiptRoot } from './uat-facet-recorder.mjs';

export function receiptIndex(directory) {
  const index = {};
  let total = 0;
  for (const name of fs.readdirSync(directory)) {
    assert.match(name, /^[a-f0-9-]{36}\.(json|pending)$/);
    assert.ok(Object.keys(index).length < 256, 'Too many receipts');
    const bytes = readReceiptFile(path.join(directory, name));
    total += bytes.length;
    assert.ok(total <= 32 * 1024 * 1024, 'Turn receipt byte limit');
    index[name] = digest(bytes);
  }
  return index;
}

export function beginReceiptTurn(root = receiptRoot) {
  fs.mkdirSync(root, { recursive: true, mode: 0o700 });
  assert.equal(fs.realpathSync(root), path.resolve(root), 'Unsafe receipt root');
  const turnId = randomUUID(), directory = path.join(root, turnId);
  fs.mkdirSync(directory, { mode: 0o700 });
  const baseline = receiptIndex(directory);
  fs.writeFileSync(path.join(root, 'active.json.writing'), JSON.stringify({ turnId }), { mode: 0o600 });
  fs.renameSync(path.join(root, 'active.json.writing'), path.join(root, 'active.json'));
  return { turnId, directory, baseline, startedAt: Date.now() };
}

export function collectReceipts(turn) {
  const index = receiptIndex(turn.directory);
  const receipts = [];
  for (const [name, hash] of Object.entries(index)) {
    assert.ok(!(name in turn.baseline), 'Old receipt in turn');
    assert.ok(name.endsWith('.json'), 'Incomplete recorder call');
    const bytes = readReceiptFile(path.join(turn.directory, name));
    assert.equal(digest(bytes), hash, 'Receipt changed during collection');
    const receipt = JSON.parse(bytes);
    assert.equal(name, `${receipt.id}.json`, 'Receipt ID/source mismatch');
    assert.equal(receipt.source, undefined, 'Receipt cannot supply a source path');
    receipts.push({ source: name, sha256: hash, receipt });
  }
  return { index, receipts };
}

export function receiptProof(captured, events, projectPath, nativeSession, facts, { startedAt, completedAt, executableSha256 }) {
  assert.ok(nativeSession && path.posix.isAbsolute(projectPath) && path.posix.normalize(projectPath) === projectPath);
  assert.match(executableSha256, /^[a-f0-9]{64}$/);
  assert.match(facts?.sha256, /^[a-f0-9]{64}$/);
  const final = path.posix.join(projectPath, 'renders/final.mp4');
  const successful = [];
  const ids = new Set();
  let finalHash;
  for (const { source, receipt: r } of [...captured.receipts].sort((a, b) => a.receipt.completedAt - b.receipt.completedAt)) {
    assert.match(source, /^[a-f0-9-]{36}\.json$/);
    assert.equal(source, `${r.id}.json`);
    assert.equal(r.source, undefined, 'Receipt cannot supply a source path');
    assert.ok(!ids.has(r.id), 'Duplicate receipt'); ids.add(r.id);
    assert.equal(r.version, 1); assert.equal(r.kind, 'instrumented-real-facet');
    assert.ok(Number.isInteger(r.startedAt) && Number.isInteger(r.completedAt) && r.startedAt >= startedAt && r.completedAt >= r.startedAt && r.completedAt <= completedAt, 'Stale/out-of-turn receipt');
    assert.equal(r.executable, realFacet, 'Wrong executable');
    assert.equal(r.executableSha256, executableSha256, 'Wrong installed binary fingerprint');
    assert.equal(r.executableUnchanged, true, 'Executable changed during call');
    assert.equal(r.complete, true, 'Partial/failed capture');
    assert.ok(Array.isArray(r.argv) && r.argv.every(v => typeof v === 'string'));
    const [tools, operation, tool, flag, input] = r.argv;
    if (tools !== 'tools' || operation !== 'run' || !/^(video_compose|edit|source_edit|video_stitch|video_trimmer|audio_mix|audio_mixer)$/.test(tool)) continue;
    assert.equal(r.cwd, projectPath, 'Wrong render cwd');
    assert.equal(r.argv.length, 5); assert.equal(flag, '--input'); assert.ok(input);
    if (r.exit !== 0 || r.signal !== null) { successful.length = 0; continue; }
    const output = JSON.parse(r.stdout); // Entire envelope, not a successful substring.
    assert.equal(output.ok, true); assert.equal(output.tool, tool); assert.equal(output.operation, 'run');
    const rendered = output.result?.output;
    assert.ok(typeof rendered === 'string' && !/[\x00-\x1f\\]/.test(rendered));
    const resolved = path.posix.resolve(projectPath, rendered);
    assert.ok(resolved.startsWith(`${projectPath}/`), 'Output escaped project');
    if (resolved !== final) continue;
    finalHash = output.result?.output_facts?.sha256;
    // Supporting session evidence, NOT a claim that the recorder knows native IDs.
    // The child receipt proves argv; shell text only associates the observed turn.
    const supporting = events.find(event => {
      const d = event.type === 'cc' && event.data, raw = d?.raw, p = raw?.part, s = p?.state;
      if (d?.type !== 'tool_result' || d.session_id !== nativeSession || d.tool_name !== 'bash' || d.is_error === true) return false;
      if (raw?.type !== 'tool_use' || raw.sessionID !== nativeSession || p?.type !== 'tool' || p?.sessionID !== nativeSession || p?.callID !== d.tool_id || p?.tool !== 'bash' || s?.status !== 'completed') return false;
      if (!Number.isInteger(s.time?.start) || !Number.isInteger(s.time?.end) || s.time.start < startedAt || s.time.end > completedAt || s.time.start > r.startedAt || s.time.end < r.completedAt) return false;
      if (JSON.stringify(s.input) !== JSON.stringify(d.tool_input) || s.output !== d.tool_output || s.metadata?.exit !== 0) return false;
      if (d.tool_input.workdir !== undefined && d.tool_input.workdir !== projectPath) return false;
      const command = d.tool_input.command;
      return typeof command === 'string' && command.length <= 65536 && new RegExp(`(?:^|&&)\\s*(?:/opt/uat/bin/)?facet\\s+tools\\s+run\\s+${tool}\\s+--input\\s+`).test(command);
    });
    assert.ok(supporting, 'No current native-session Facet command supporting receipt');
    successful.length = 0;
    successful.push({ receipt_id: r.id, source, tool, output: final, successful: true, supporting_call_id: supporting.data.tool_id, binding: 'turn-time-cwd-and-observed-command-not-native-attestation' });
  }
  assert.ok(successful.length, 'No successful current Facet receipt for final media');
  assert.equal(finalHash, facts.sha256, 'Receipt hash differs from final media');
  return successful;
}
