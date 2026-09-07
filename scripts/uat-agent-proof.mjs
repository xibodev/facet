import assert from 'node:assert/strict';
import path from 'node:path';

// Linux container paths are resolved independently of the host running the tests.
// This checks execution records, not claims in agent prose or arbitrary shell text.
export function renderingProof(events, projectPath, nativeSession, facts) {
  assert.ok(typeof projectPath === 'string' && path.posix.isAbsolute(projectPath) && path.posix.normalize(projectPath) === projectPath, 'Expected canonical absolute project path');
  const expectedOutput = path.posix.join(projectPath, 'renders/final.mp4');
  function renderInput(input) {
    const command = input?.command;
    if (typeof command !== 'string' || command.length > 65536 || /[\x00-\x08\x0a-\x1f\x7f]/.test(command)) return null;
    // Only literal words, quoted arguments, && and the exact stderr redirect.
    // Double quotes decode shell-escaped quotes/backslashes, never expansion.
    const word = /2>&1|[A-Za-z0-9_./-]+|'[^']*'|"(?:[^"\\$`!]|\\[^$`!])*"/y;
    const parts = [[]];
    let cursor = 0;
    while (cursor < command.length) {
      if (/[ \t]/.test(command[cursor])) { cursor++; continue; }
      if (command.startsWith('&&', cursor)) {
        if (!parts.at(-1).length || parts.length === 3) return null;
        parts.push([]);
        cursor += 2;
        continue;
      }
      word.lastIndex = cursor;
      const token = word.exec(command)?.[0];
      if (!token || parts.at(-1).length === 7) return null;
      cursor = word.lastIndex;
      if (cursor < command.length && !/[ \t]/.test(command[cursor]) && !command.startsWith('&&', cursor)) return null;
      const quoted = token[0] === '"' || token[0] === "'";
      let value = quoted ? token.slice(1, -1) : token;
      if (token[0] === '"') value = value.replace(/\\(["\\])/g, '$1');
      parts.at(-1).push({ value, quoted });
    }
    const directory = parts[0].length === 2 && !parts[0][0].quoted && parts[0][0].value === 'cd' ? parts[0][1] : null;
    if (directory) parts.shift();
    const matches = parts.map(part => {
      if (![6, 7].includes(part.length) || part.slice(0, 5).some(token => token.quoted)) return null;
      const [binary, tools, operation, tool, flag, argument, redirect] = part.map(token => token.value);
      if (!/^(?:facet|\/[A-Za-z0-9_./-]+\/facet)$/.test(binary) || tools !== 'tools' || !/^(run|estimate)$/.test(operation) || !/^(video_compose|edit|source_edit|video_stitch|video_trimmer|audio_mix|audio_mixer)$/.test(tool) || flag !== '--input') return null;
      if (redirect !== undefined && (redirect !== '2>&1' || part[6].quoted)) return null;
      if (!/^[A-Za-z0-9_./ -]+$/.test(argument)) {
        if (!part[5].quoted) return null;
        try {
          const json = JSON.parse(argument);
          if (!json || typeof json !== 'object' || Array.isArray(json)) return null;
        } catch { return null; }
      }
      return { operation, tool, argument };
    });
    if (matches.some(match => !match) || ![1, 2].includes(matches.length)) return null;
    const last = matches.at(-1);
    if (last.operation !== 'run') return null;
    if (matches.length === 2 && (matches[0].operation !== 'estimate' || matches[0].tool !== last.tool || matches[0].argument !== last.argument)) return null;
    if (directory) assert.equal(directory.value, projectPath, 'Render cd must select the exact project');
    if (input.workdir !== undefined) assert.equal(input.workdir, projectPath, 'Render workdir must select the exact project');
    return { command, tool: last.tool, operations: matches.map(match => match.operation), input };
  }
  // Consume the entire stdout stream as consecutive objects, never extract JSON
  // from logs, filtered fragments, arrays, or a successful-looking substring.
  function results(text) {
    assert.equal(typeof text, 'string', 'Expected Facet JSON output');
    const values = [];
    let start = 0;
    while (start < text.length) {
      if (/[ \t\r\n]/.test(text[start])) { start++; continue; }
      assert.equal(text[start], '{', 'Unrecognized rendering tool output: expected Facet JSON');
      let depth = 0, quoted = false, escaped = false, end = start;
      for (; end < text.length; end++) {
        const char = text[end];
        if (quoted) {
          if (escaped) escaped = false;
          else if (char === '\\') escaped = true;
          else if (char === '"') quoted = false;
        } else if (char === '"') quoted = true;
        else if (char === '{') depth++;
        else if (char === '}' && --depth === 0) break;
      }
      assert.ok(end < text.length && values.length < 2, 'Incomplete or extra Facet JSON output');
      values.push(JSON.parse(text.slice(start, end + 1)));
      start = end + 1;
    }
    return values;
  }
  const calls = new Map();
  const successful = [];
  let finalHash;
  for (const event of events) {
    const data = event.type === 'cc' ? event.data : null;
    if (!data || !['tool_use', 'tool_result'].includes(data.type)) continue;
    const raw = data.raw;
    const part = raw?.part;
    const state = part?.state;
    const input = renderInput(data.tool_input);
    const rawInput = renderInput(state?.input);
    const prior = calls.get(data.tool_id);
    if (!prior && !input && !rawInput) continue;
    assert.ok(typeof data.tool_id === 'string' && data.tool_id, 'Missing rendering call ID');
    assert.ok(typeof data.session_id === 'string' && data.session_id, 'Missing rendering session ID');
    if (nativeSession !== undefined) assert.equal(data.session_id, nativeSession, 'Render does not belong to the active native session');
    assert.equal(data.tool_name, 'bash', 'Rendering tool name mismatch');
    if (data.type === 'tool_use') {
      assert.ok(input, 'Invalid rendering invocation');
      if (prior) {
        assert.equal(prior.completed, false, 'Rendering call ID reused after completion');
        assert.deepEqual(prior.input, data.tool_input, 'Call ID reused with different rendering input');
        assert.equal(prior.session, data.session_id, 'Call ID reused across sessions');
      } else calls.set(data.tool_id, { ...input, session: data.session_id, completed: false });
      continue;
    }
    // OpenCode 1.18.29 run --format json emits one completed tool_use record,
    // normalized by Studio to tool_result. It is its own call/result evidence.
    if (raw !== undefined) {
      assert.equal(raw.type, 'tool_use', 'Unexpected raw OpenCode event type');
      assert.equal(part?.type, 'tool', 'Unexpected raw OpenCode part type');
      assert.equal(part.callID, data.tool_id, 'Raw/normalized call ID mismatch');
      assert.equal(raw.sessionID, data.session_id, 'Raw/normalized session ID mismatch');
      assert.equal(part.sessionID, data.session_id, 'Raw part session ID mismatch');
      assert.equal(part.tool, data.tool_name, 'Raw/normalized tool name mismatch');
      assert.equal(state?.status, 'completed', 'OpenCode rendering call did not complete successfully');
      assert.deepEqual(state.input, data.tool_input, 'Raw/normalized rendering input mismatch');
      assert.equal(state.output, data.tool_output, 'Raw/normalized rendering output mismatch');
      assert.ok(Number.isInteger(state.metadata?.exit) && state.metadata.exit >= 0 && state.metadata.exit <= 255, 'Invalid rendering shell exit');
      assert.ok(state.metadata.truncated === undefined || state.metadata.truncated === false, 'Truncated rendering output');
      if (state.metadata.output !== undefined) assert.equal(state.metadata.output, state.output, 'Raw metadata output mismatch');
      assert.ok(input && rawInput, 'Invalid rendering invocation');
    } else assert.ok(prior, 'Completed-only rendering result needs raw OpenCode evidence');
    const call = prior || { ...input, session: data.session_id, completed: false };
    assert.equal(call.completed, false, 'Duplicate rendering result');
    if (data.tool_input !== undefined) assert.deepEqual(data.tool_input, call.input, 'Rendering result input differs from call');
    call.completed = true;
    calls.set(data.tool_id, call);
    assert.ok(data.is_error === undefined || typeof data.is_error === 'boolean', 'Invalid rendering error flag');
    assert.equal(data.session_id, call.session, 'Rendering result session mismatch');
    const outputs = results(data.tool_output);
    assert.ok(outputs.length > 0 && outputs.length <= call.operations.length, 'Facet result count mismatch');
    for (const [index, output] of outputs.entries()) {
      assert.equal(output.tool, call.tool, 'Facet result tool mismatch');
      assert.equal(output.operation, call.operations[index], 'Facet result operation mismatch');
      assert.equal(typeof output.ok, 'boolean', 'Invalid Facet result status');
      if (!output.ok) {
        assert.equal(index, outputs.length - 1, 'Execution continued after failed Facet command');
        assert.ok(typeof output.error?.code === 'string' && output.error.code.trim() && typeof output.error.message === 'string' && output.error.message.trim(), 'Missing structured Facet failure');
        assert.ok(output.result === undefined, 'Failed Facet result contains success evidence');
      }
    }
    const output = outputs.at(-1);
    if (!output.ok) {
      if (raw !== undefined) assert.notEqual(state.metadata.exit, 0, 'Facet failure masked by zero shell exit');
      else assert.equal(data.is_error, true, 'Paired Facet failure needs an error flag');
      // A recorded failure is not delivery; only later successful recovery counts.
      successful.length = 0;
      continue;
    }
    assert.equal(outputs.length, call.operations.length, 'Facet result count mismatch');
    assert.ok(data.is_error === undefined || data.is_error === false, 'Rendering tool reported an error');
    if (raw !== undefined) assert.equal(state.metadata.exit, 0, 'Rendering shell exit must be zero');
    const rendered = output.result?.output;
    assert.ok(typeof rendered === 'string' && rendered && !/[\x00-\x1f\\]/.test(rendered), 'Invalid Facet render output path');
    const resolved = path.posix.resolve(projectPath, rendered);
    assert.ok(resolved.startsWith(`${projectPath}/`), 'Facet render output escaped the project');
    // An intermediate render is valid execution, but cannot prove the final file.
    const hash = output.result?.output_facts?.sha256;
    if (hash !== undefined) assert.match(hash, /^[a-f0-9]{64}$/, 'Invalid Facet output SHA256');
    if (resolved === expectedOutput) {
      successful.length = 0;
      finalHash = hash;
      successful.push({ call_id: data.tool_id, tool: call.tool, output: expectedOutput, successful: true });
    }
  }
  assert.ok(successful.length > 0, 'No correlated successful Facet rendering call');
  for (const call of calls.values()) assert.ok(call.completed, 'Rendering call has no result');
  if (facts !== undefined) {
    assert.match(finalHash, /^[a-f0-9]{64}$/, 'Missing Facet output SHA256');
    assert.match(facts?.sha256, /^[a-f0-9]{64}$/, 'Missing independent media SHA256');
    assert.equal(finalHash, facts.sha256, 'Facet output SHA256 differs from delivered media');
  }
  return successful;
}
