import { test } from 'node:test';
import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import { sanitizer } from './uat-sanitize.mjs';
import { renderingProof as prove } from './uat-agent-proof.mjs';
import { completedRender, project } from './uat-proof-fixtures.mjs';
import { failedSourceEdit, estimatedAudioMix, filteredRewrite, latestProject, deliveredFacts } from './uat-proof-captured-fixtures.mjs';
import { fileURLToPath } from 'node:url';

const renderingProof = events => prove(events, project);

test('synthetic quote/backslash secrets are removed at repeated JSON escaping depths', () => {
  const safe = sanitizer();
  const secret = 'synthetic-quote"-slash\\-line\n-end';
  safe.add(secret);
  for (const value of [secret, encodeURIComponent(secret)]) {
    let nested = { unrelated: value, token: value };
    for (let depth = 0; depth < 8; depth++) {
      const encoded = JSON.stringify(nested);
      const redacted = safe.redact(encoded);
      let decoded = JSON.parse(redacted);
      for (let i = 0; i < depth; i++) decoded = JSON.parse(decoded);
      assert.equal(decoded.unrelated, '[REDACTED]', 'Nested secret was not removed');
      assert.equal(decoded.token, '[REDACTED]', 'Nested token was not removed');
      nested = encoded;
    }
  }
});

function pair(command = 'facet tools run video_compose --input artifacts/props.json') {
  return [
    { type: 'cc', data: { type: 'tool_use', tool_id: 'render-call', session_id: 'synthetic-session', tool_name: 'bash', tool_input: { command } } },
    { type: 'cc', data: { type: 'tool_result', tool_id: 'render-call', session_id: 'synthetic-session', tool_name: 'bash', tool_output: JSON.stringify({ ok: true, tool: 'video_compose', operation: 'run', result: { output: 'renders/final.mp4' } }) } }
  ];
}
test('synthetic events require correlated successful rendering execution, never text matches', () => {
  assert.equal(renderingProof(pair()).length, 1);
  for (const command of ['echo facet tools run video_compose --input props.json', 'false; facet tools run video_compose --input props.json', 'facet tools run media_probe --input props.json', 'facet tools run video_compose --input props.json || true', 'sh render.sh']) {
    assert.throws(() => renderingProof(pair(command)));
  }
  for (const mutate of [
    events => { events[1].data.is_error = true; },
    events => { events[1].data.is_error = 'false'; },
    events => { events[1].data.tool_id = 'unrelated'; },
    events => { events[1].data.session_id = 'another'; },
    events => { events[1].data.tool_output = 'render succeeded'; },
    events => { events[1].data.tool_output = JSON.stringify({ ok: false }); },
    events => { events[1].data.raw = { part: { state: { metadata: { exit: 1 } } } }; },
    events => { events.reverse(); },
    events => { events.shift(); },
    events => { events[0].data.type = 'text_delta'; },
    events => { events.push(events[1]); }
  ]) {
    const events = pair(); mutate(events); assert.throws(() => renderingProof(events));
  }
});

test('real sanitized OpenCode completed-only render proves exact project output', () => {
  const proof = prove([completedRender], project, completedRender.data.session_id);
  assert.deepEqual(proof, [{ call_id: completedRender.data.tool_id, tool: 'video_compose', output: `${project}/renders/final.mp4`, successful: true }]);
  assert.throws(() => prove([completedRender], project, 'another-session'));
  assert.throws(() => prove([completedRender], '/another-project'));
  assert.throws(() => prove([completedRender]));
});

// All variants below are synthetic mutations, not new real UAT evidence.
function variant({ command, workdir, output } = {}) {
  const event = structuredClone(completedRender);
  const data = event.data, state = data.raw.part.state;
  if (command !== undefined) data.tool_input.command = command;
  if (workdir !== undefined) data.tool_input.workdir = workdir;
  state.input = structuredClone(data.tool_input);
  if (output !== undefined) {
    const result = JSON.parse(data.tool_output);
    result.result.output = output;
    data.tool_output = JSON.stringify(result);
    state.output = data.tool_output;
    state.metadata.output = data.tool_output;
  }
  return event;
}

test('raw and normalized call identity, inputs, outputs and success must agree', () => {
  for (const mutate of [
    d => { d.tool_id = 'other'; },
    d => { d.session_id = 'other'; },
    d => { d.raw.sessionID = 'other'; },
    d => { d.raw.part.sessionID = 'other'; },
    d => { d.raw.part.callID = 'other'; },
    d => { d.raw.part.tool = 'read'; },
    d => { d.tool_name = 'read'; },
    d => { d.raw.type = 'text'; },
    d => { d.raw.part.type = 'text'; },
    d => { d.raw.part.state.status = 'error'; },
    d => { d.raw.part.state.status = 'running'; },
    d => { d.raw.part.state.metadata.exit = 1; },
    d => { d.raw.part.state.metadata.exit = '0'; },
    d => { delete d.raw.part.state.metadata.exit; },
    d => { d.raw.part.state.metadata.output = 'fabricated success'; },
    d => { d.raw.part.state.input.command = 'echo fabricated'; },
    d => { d.tool_input.command = 'echo fabricated'; },
    d => { d.tool_input.workdir = '/other'; },
    d => { d.raw.part.state.input.workdir = '/other'; },
    d => { d.tool_output = 'fabricated success'; },
    d => { d.raw.part.state.output = 'fabricated success'; },
    d => { d.is_error = true; },
    d => { d.is_error = 'false'; },
    d => { delete d.raw; },
    d => { delete d.tool_id; },
    d => { delete d.session_id; }
  ]) {
    const event = structuredClone(completedRender); mutate(event.data);
    assert.throws(() => renderingProof([event]));
  }
  for (const output of ['render succeeded', '{"ok":false}', '{"ok":true,"tool":"edit","operation":"run"}', '{"ok":true,"tool":"video_compose","operation":"estimate"}']) {
    const event = structuredClone(completedRender);
    event.data.tool_output = event.data.raw.part.state.output = event.data.raw.part.state.metadata.output = output;
    assert.throws(() => renderingProof([event]));
  }
  assert.throws(() => renderingProof([{ type: 'cc', data: { type: 'text_delta', content: JSON.stringify(completedRender) } }]));
});

test('direct, exact-project cd and exact workdir are allowed; other shell execution is not', () => {
  const direct = 'facet tools run video_compose --input artifacts/props.json';
  for (const command of [direct, `cd ${project} && ${direct}`, `cd "${project}" && ${direct}`, `cd '${project}' && ${direct}`, '/home/facet/.facet/bin/facet tools run video_compose --input "artifacts/my props.json"']) {
    assert.equal(renderingProof([variant({ command, workdir: project, output: 'renders/final.mp4' })]).length, 1);
  }
  for (const command of [
    `echo ${direct}`, `false; ${direct}`, `${direct} || true`, `${direct} && echo done`,
    `cd /other && ${direct}`, `cd ${project}/../other && ${direct}`, `cd ${project} && ${direct}; true`,
    `cd ${project} && echo ${direct}`, `cd ${project} && cd ${project} && ${direct}`,
    `cd $PROJECT && ${direct}`, `cd "$(pwd)" && ${direct}`, `${direct} > result.json`,
    `${direct}\necho done`, `env X=1 ${direct}`, 'sh render.sh', 'facet tools run video_compose --input "$(echo props.json)"'
  ]) assert.throws(() => renderingProof([variant({ command })]), command);
  assert.throws(() => renderingProof([variant({ command: direct, workdir: '/other' })]));
  assert.throws(() => renderingProof([variant({ command: direct, workdir: `${project}/.` })]));
});

test('literal inline JSON works in paired and completed records without splitting quoted operators', () => {
  const json = JSON.stringify({ composition_id: 'TitleCard', output: 'renders/final.mp4', props: { title: 'A && B | C; <D> (E)', quote: 'say "hello"', path: 'a\\b' } });
  const single = `'${json}'`;
  const double = `"${json.replace(/[\\"]/g, '\\$&')}"`;
  for (const argument of [single, double, `'${JSON.stringify({ props: { title: '$HOME $(id) `id` ! literal' } })}'`]) {
    const direct = `facet tools run video_compose --input ${argument}`;
    for (const command of [direct, `cd '${project}'&&${direct} 2>&1`]) {
      assert.equal(renderingProof(pair(command)).length, 1);
      assert.equal(renderingProof([variant({ command })]).length, 1);
    }
  }
  const command = `cd "${project}" && facet tools estimate video_compose --input ${single} 2>&1 && facet tools run video_compose --input ${double} 2>&1`;
  const estimate = JSON.stringify({ ok: true, tool: 'video_compose', operation: 'estimate', result: {} });
  const event = variant({ command });
  assert.equal(renderingProof([withOutput(event, estimate + '\n' + event.data.tool_output)]).length, 1);
  const events = pair(command);
  events[1].data.tool_output = estimate + '\n' + events[1].data.tool_output;
  assert.equal(renderingProof(events).length, 1);
  assert.throws(() => renderingProof([variant({ command: command.replace(single, `'{}'`) })]));
  const changed = variant({ command });
  changed.data.raw.part.state.input.command = command.replace(single, `'{}'`);
  assert.throws(() => renderingProof([changed]), /input mismatch/);
});

test('inline JSON rejects substitutions, injection, malformed literals and unbounded commands', () => {
  const direct = 'facet tools run video_compose --input';
  for (const argument of [
    String.raw`"{\"title\":\"$(id)\"}"`,
    String.raw`"{\"title\":\"\$(id)\"}"`,
    '"{\\"title\\":\\"${HOME}\\"}"',
    '"{\\"title\\":\\"`id`\\"}"',
    String.raw`"{\"title\":\"!123\"}"`,
    `'{}'; touch /tmp/injected`, `'{}' | grep ok`, `'{}' 2>&1 | cat`,
    `'{}' && touch /tmp/injected`, `'{}' || true`, `'{}' &`, `'{}' > result.json`,
    `'{}'$(id)`, `'{"title":"a'$(id)'b"}'`, `$'{}'`, `$(echo '{}')`, `<(echo '{}')`,
    `'{}' # ignored`, `'{}' "2>&1"`, `'{}' 2>&1 2>&1`, `'{}' &&`,
    `'{}'\necho done`, `'{}'\u0000`, `'{"bad":}'`, `'[]'`, `'{"title":"unterminated}'`,
    `"{}'`, `'{"title":"${'a'.repeat(65536)}"}'`
  ]) {
    const command = `${direct} ${argument}`;
    assert.throws(() => renderingProof(pair(command)), command.slice(0, 160));
    assert.throws(() => renderingProof([variant({ command })]), command.slice(0, 160));
  }
});

test('only the exact resolved final output proves delivery; intermediates cannot substitute', () => {
  for (const output of ['renders/final.mp4', './renders/final.mp4', `${project}/renders/final.mp4`]) assert.equal(renderingProof([variant({ output })]).length, 1);
  for (const output of ['../renders/final.mp4', '/other/renders/final.mp4', `${project}-other/renders/final.mp4`, 'renders/other.mp4', 'build/intermediate.mp4', 'renders\\final.mp4', null]) assert.throws(() => renderingProof([variant({ output })]));
  const intermediate = variant({ output: 'build/intermediate.mp4' });
  intermediate.data.tool_id = intermediate.data.raw.part.callID = 'intermediate-call';
  assert.equal(renderingProof([intermediate, completedRender]).length, 1);
  const failed = structuredClone(intermediate);
  failed.data.raw.part.state.metadata.exit = 1;
  assert.throws(() => renderingProof([failed, completedRender]));
});

test('duplicate completion, reused IDs and changed paired calls fail closed', () => {
  assert.throws(() => renderingProof([completedRender, completedRender]), /Duplicate/);
  const start = { type: 'cc', data: { type: 'tool_use', tool_id: completedRender.data.tool_id, session_id: completedRender.data.session_id, tool_name: 'bash', tool_input: structuredClone(completedRender.data.tool_input) } };
  assert.equal(renderingProof([start, completedRender]).length, 1);
  assert.throws(() => renderingProof([completedRender, start]), /reused after completion/);
  const changed = structuredClone(start); changed.data.tool_input.command = 'facet tools run edit --input other.json';
  assert.throws(() => renderingProof([start, changed, completedRender]));
  assert.throws(() => renderingProof([changed, completedRender]));
});

function withOutput(event, output) {
  const copy = structuredClone(event);
  copy.data.tool_output = copy.data.raw.part.state.output = copy.data.raw.part.state.metadata.output = output;
  return copy;
}

test('captured estimate/run audio mix and structured source-edit failure recovery are intact evidence', () => {
  const native = estimatedAudioMix.data.session_id;
  const proof = prove([failedSourceEdit, estimatedAudioMix], latestProject, native);
  assert.deepEqual(proof, [{ call_id: estimatedAudioMix.data.tool_id, tool: 'audio_mix', output: `${latestProject}/renders/final.mp4`, successful: true }]);
  assert.throws(() => prove([failedSourceEdit], latestProject), /No correlated/);
  assert.throws(() => prove([estimatedAudioMix, failedSourceEdit], latestProject), /No correlated/);
  // Actual delivered bytes were rewritten later; the earlier intact success must
  // not certify them. This fixture replay does not amend the historical verdict.
  assert.throws(() => prove([failedSourceEdit, estimatedAudioMix, filteredRewrite], latestProject, native, deliveredFacts), /SHA256 differs/);
  assert.throws(() => prove([filteredRewrite], latestProject, native, deliveredFacts), /No correlated/);
});

test('synthetic descriptor and alias variants retain exact command and JSON correspondence', () => {
  const direct = 'facet tools run video_compose --input artifacts/props.json';
  for (const command of [direct + ' 2>&1', `cd '${project}'&&${direct} 2>&1`]) {
    assert.equal(renderingProof([variant({ command })]).length, 1);
  }
  // Alias coverage is synthetic; no audio_mixer runtime execution is claimed.
  const alias = structuredClone(estimatedAudioMix);
  alias.data.tool_input.command = alias.data.tool_input.command.replaceAll('audio_mix ', 'audio_mixer ');
  alias.data.raw.part.state.input = structuredClone(alias.data.tool_input);
  const output = alias.data.tool_output.replaceAll('"tool": "audio_mix"', '"tool": "audio_mixer"');
  assert.equal(prove([withOutput(alias, output)], latestProject)[0].tool, 'audio_mixer');
  const run = JSON.parse(estimatedAudioMix.data.tool_output.slice(estimatedAudioMix.data.tool_output.indexOf('\n}\n{') + 3));
  const estimate = { ok: true, tool: 'audio_mix', operation: 'estimate', result: { note: 'braces { } and escaped quote " and slash \\' } };
  assert.equal(prove([withOutput(estimatedAudioMix, JSON.stringify(estimate) + '\n' + JSON.stringify(run))], latestProject).length, 1);
  for (const text of [
    JSON.stringify(run), JSON.stringify(estimate),
    JSON.stringify(run) + JSON.stringify(estimate),
    JSON.stringify([estimate, run]),
    'log\n' + estimatedAudioMix.data.tool_output,
    estimatedAudioMix.data.tool_output + '\nfinished',
    estimatedAudioMix.data.tool_output + '{}',
    estimatedAudioMix.data.tool_output.slice(0, -4),
    JSON.stringify(estimate) + ',\n' + JSON.stringify(run),
    JSON.stringify({ ...estimate, tool: 'source_edit' }) + JSON.stringify(run),
    JSON.stringify({ ...estimate, ok: false, error: { code: 'failed', message: 'failed' } }) + JSON.stringify(run)
  ]) assert.throws(() => prove([withOutput(estimatedAudioMix, text)], latestProject), text);
  assert.throws(() => renderingProof([withOutput(completedRender, completedRender.data.tool_output + completedRender.data.tool_output)]), /result count/);
});

test('synthetic stale hashes fail while latest successful final bytes may supersede earlier bytes', () => {
  const hash = 'a'.repeat(64), stale = 'b'.repeat(64);
  const result = JSON.parse(completedRender.data.tool_output);
  result.result.output_facts = { sha256: hash };
  const event = withOutput(completedRender, JSON.stringify(result));
  assert.equal(prove([event], project, undefined, { sha256: hash }).length, 1);
  assert.throws(() => prove([event], project, undefined, { sha256: stale }), /SHA256 differs/);
  assert.throws(() => prove([event], project, undefined, {}), /SHA256/);
  for (const invalid of [null, '', 'not-a-hash', 123]) {
    result.result.output_facts.sha256 = invalid;
    assert.throws(() => prove([withOutput(completedRender, JSON.stringify(result))], project));
  }
  result.result.output_facts.sha256 = stale;
  const earlier = withOutput(completedRender, JSON.stringify(result));
  earlier.data.tool_id = earlier.data.raw.part.callID = 'earlier-call';
  assert.equal(prove([earlier, event], project, undefined, { sha256: hash }).length, 1);
  assert.throws(() => prove([event, earlier], project, undefined, { sha256: hash }), /SHA256 differs/);
  const hashlessOverwrite = structuredClone(completedRender);
  hashlessOverwrite.data.tool_id = hashlessOverwrite.data.raw.part.callID = 'hashless-overwrite';
  assert.throws(() => prove([event, hashlessOverwrite], project, undefined, { sha256: hash }), /Missing Facet output SHA256/);
  assert.equal(prove([hashlessOverwrite, event], project, undefined, { sha256: hash }).length, 1);
});

test('independent facts always require producer hashes including captured hashless Remotion output', () => {
  const sha256 = 'a'.repeat(64);
  for (const events of [pair(), [completedRender]]) {
    assert.throws(() => prove(events, project, undefined, { sha256 }), /Missing Facet output SHA256/);
  }
  const events = pair(`facet tools run video_compose --input '{"composition_id":"TitleCard"}'`);
  const output = JSON.parse(events[1].data.tool_output);
  output.result.output_facts = { sha256 };
  events[1].data.tool_output = JSON.stringify(output);
  assert.equal(prove(events, project, 'synthetic-session', { sha256 }).length, 1);
  assert.throws(() => prove(events, project, 'synthetic-session', { sha256: 'b'.repeat(64) }), /SHA256 differs/);
  for (const facts of [null, {}, { sha256: '' }, { sha256: 123 }]) {
    assert.throws(() => prove(events, project, 'synthetic-session', facts), /SHA256/);
  }
  assert.equal(JSON.parse(completedRender.data.tool_output).result.output_facts, undefined, 'Historical Remotion fixture must remain hashless');
});

test('synthetic probe-only, estimate-only, filtered and masked failure records never prove rendering', () => {
  const direct = 'facet tools run video_compose --input artifacts/props.json';
  for (const command of [
    direct.replace('run', 'estimate'), direct.replace('video_compose', 'media_probe'), direct.replace('video_compose', 'output_review'),
    `${direct} | cat`, `${direct} 2>&1 | grep ok`, `${direct} 2>&1 | head -20`, `${direct} 2>&1 || true`,
    `${direct} 2>&1; true`, `${direct} 2>/dev/null`, `${direct} 1>&2`, `${direct} > /dev/null`, `${direct} 2>&1 && ${direct}`,
    `facet tools estimate edit --input artifacts/props.json && ${direct}`,
    `facet tools estimate video_compose --input other.json && ${direct}`,
    `facet tools estimate video_compose --input artifacts/props.json && ${direct} && echo done`
  ]) assert.throws(() => renderingProof([variant({ command })]), command);
  for (const mutate of [
    e => { e.data.raw.part.state.metadata.exit = 0; },
    e => { e.data.raw.part.state.metadata.exit = '1'; },
    e => { e.data.raw.part.state.status = 'error'; },
    e => { e.data.raw.part.callID = 'wrong'; },
    e => { e.data.session_id = 'wrong'; },
    e => { e.data.raw.part.state.metadata.truncated = true; },
    e => { e.data.is_error = 'true'; }
  ]) {
    const failed = structuredClone(failedSourceEdit); mutate(failed);
    assert.throws(() => prove([failed, estimatedAudioMix], latestProject));
  }
  for (const error of [undefined, {}, { code: 'failed' }, { code: '', message: 'bad' }]) {
    const output = JSON.parse(failedSourceEdit.data.tool_output);
    output.error = error;
    assert.throws(() => prove([withOutput(failedSourceEdit, JSON.stringify(output)), estimatedAudioMix], latestProject));
  }
  const output = JSON.parse(failedSourceEdit.data.tool_output);
  output.result = { output: 'renders/final.mp4' };
  assert.throws(() => prove([withOutput(failedSourceEdit, JSON.stringify(output)), estimatedAudioMix], latestProject));
});

test('synthetic estimate-chain failures stop exactly where recorded and permit later recovery', () => {
  const failure = { ok: false, tool: 'audio_mix', operation: 'estimate', error: { code: 'invalid_input', message: 'Invalid input' } };
  const failed = withOutput(estimatedAudioMix, JSON.stringify(failure));
  failed.data.tool_id = failed.data.raw.part.callID = 'failed-estimate';
  failed.data.raw.part.state.metadata.exit = 1;
  assert.equal(prove([failed, estimatedAudioMix], latestProject).length, 1);
  const estimate = { ok: true, tool: 'audio_mix', operation: 'estimate', result: {} };
  failure.operation = 'run';
  assert.equal(prove([withOutput(failed, JSON.stringify(estimate) + JSON.stringify(failure)), estimatedAudioMix], latestProject).length, 1);
  const events = pair();
  events[1].data.tool_output = JSON.stringify({ ...failure, tool: 'video_compose' });
  events[1].data.is_error = true;
  assert.equal(renderingProof([...events, completedRender]).length, 1);
  events[1].data.is_error = false;
  assert.throws(() => renderingProof([...events, completedRender]));
});

test('certification rejects open egress before reading config, creating files or running Docker', () => {
  const result = spawnSync(process.execPath, [fileURLToPath(new URL('./uat-run.mjs', import.meta.url)), '--certify', '/unused-test-evidence'], { encoding: 'utf8', timeout: 10000, env: { ...process.env, UAT_NETWORK_INTERNAL: 'false', UAT_REAL_AGENT: '1', UAT_OPENCODE_CONFIG_FILE: '/must-not-read' } });
  assert.notEqual(result.status, 0);
  assert.ok(result.stderr.includes('CERTIFICATION_REFUSED'));
  assert.ok(!result.stdout.includes('Materializing'));
});
