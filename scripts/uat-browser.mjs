import fs from 'node:fs';
import path from 'node:path';
import assert from 'node:assert/strict';
import { execFileSync } from 'node:child_process';
import { chromium } from 'playwright';
import AdmZip from 'adm-zip';
import { inspectMedia, decodedAgentMedia } from './uat-media.mjs';
import { renderingProof } from './uat-agent-proof.mjs';
import { atomicJSON, browserCheckpoint } from './uat-checkpoint.mjs';
import { captureProcessInventory } from './uat-checkpoint-process.mjs';
import { validateModelConfig } from './uat-config.mjs';
import { beginReceiptTurn, collectReceipts, receiptIndex, receiptProof } from './uat-receipt-proof.mjs';

const out = process.argv[2] || '/home/facet/uat-evidence/browser';
fs.mkdirSync(out, { recursive: true });
const report = { track: 'real-agent-browser', passed: false, checks: [], turns: [] };
const consoleLog = [], transcript = [], pending = new Set();
const receiptSnapshots = [];
const checkpoint = browserCheckpoint(out, report, transcript, consoleLog);
const { redact } = checkpoint;
checkpoint.write();
const checkpointTimer = setInterval(() => checkpoint.write(), 5000);
checkpointTimer.unref();
const timeout = Number(process.env.UAT_TURN_TIMEOUT_MS || 240000);
assert.ok(Number.isFinite(timeout) && timeout >= 1000 && timeout <= 600000, 'Invalid UAT_TURN_TIMEOUT_MS');
const browser = await chromium.launch({ executablePath: '/usr/bin/chromium', headless: true, args: ['--no-sandbox'] });
const context = await browser.newContext({ viewport: { width: 1440, height: 1000 } });
await context.tracing.start({ screenshots: false, snapshots: true, sources: false });
const page = await context.newPage();
page.setDefaultTimeout(30000);
page.setDefaultNavigationTimeout(30000);
const slug = `uat-agent-${Date.now()}`;
let projectPath;
try {
  assert.equal(fs.existsSync('/home/facet/studio/projects'), false, 'Test setup must not create studio/projects');
  page.on('console', msg => consoleLog.push({ type: msg.type(), text: msg.text() }));
  page.on('pageerror', error => consoleLog.push({ type: 'pageerror', text: error.message }));
  page.on('dialog', async dialog => { consoleLog.push({ type: 'dialog', text: dialog.message() }); await dialog.dismiss(); });
  page.on('request', request => {
    if (new URL(request.url()).pathname === '/api/session-token') checkpoint.tokenRequested();
  });
  page.on('response', response => {
    if (new URL(response.url()).pathname === '/api/session-token') {
      const task = response.json().then(data => checkpoint.tokenCaptured(data)).catch(() => {}).finally(() => pending.delete(task));
      pending.add(task);
    }
  });
  await page.exposeFunction('uatEvent', event => { checkpoint.received(event); });
  await page.addInitScript(() => {
    const Native = window.EventSource;
    window.__uatEvents = [];
    window.EventSource = class extends Native {
      constructor(...args) {
        super(...args);
        for (const type of ['session', 'cc', 'end', 'process_exit', 'error']) this.addEventListener(type, event => {
          let data;
          try { data = JSON.parse(event.data); } catch { data = { message: 'unparseable or transport error' }; }
          const item = { type, data };
          window.__uatEvents.push(item);
          window.uatEvent(item);
        });
      }
    };
  });
  checkpoint.write('create-project-for-native-studio');
  await page.goto('http://127.0.0.1:8788/', { waitUntil: 'domcontentloaded' });
  await page.locator('#newProjectButton').click();
  await page.locator('#newProjectName').fill(slug);
  await page.locator('#newProjectSlug').fill(slug);
  const createdResponse = page.waitForResponse(r => new URL(r.url()).pathname === '/api/catalog/new' && r.request().method() === 'POST');
  await page.locator('#createProjectButton').click();
  const created = await createdResponse;
  assert.equal(created.status(), 200, 'Create production failed');
  const project = await created.json();
  projectPath = project.path;
  assert.ok(projectPath?.startsWith('/home/facet/'), 'Project escaped disposable HOME');
  await page.locator('#workspaceTitle').waitFor({ state: 'visible' });
  await page.screenshot({ path: path.join(out, '01-created.png'), fullPage: true });
  assert.equal(fs.existsSync(path.join(projectPath, 'renders/final.mp4')), false, 'Fresh project already contains output');
  report.checks.push({ name: 'create-project-for-native-studio', passed: true });

  checkpoint.write('external-catalog-details-and-media-isolation');
  const base = 'http://127.0.0.1:8788';
  const detailsResponse = await context.request.get(`${base}/api/projects/${encodeURIComponent(slug)}`);
  assert.equal(detailsResponse.status(), 200, 'External catalog project details unavailable without studio/projects');
  const details = await detailsResponse.json();
  assert.equal(details.path, projectPath);
  assert.equal(details.engine, 'studio');
  assert.equal(details.stages.master, false);
  assert.ok(!details.video_url, 'Fresh project inherited another production video');
  const mediaPrefix = `/api/media/catalog/${details.slug}/`;
  let briefBytesMatch = false;
  if (details.brief_url) {
    assert.ok(details.brief_url.startsWith(mediaPrefix), 'Brief URL is not scoped to the canonical catalog project');
    const brief = await context.request.get(`${base}${details.brief_url}`);
    assert.equal(brief.status(), 200);
    const briefRelative = decodeURIComponent(new URL(details.brief_url, base).pathname.slice(mediaPrefix.length));
    const briefFile = path.resolve(projectPath, briefRelative);
    assert.ok(briefFile.startsWith(`${projectPath}/`), 'Brief escaped project directory');
    assert.equal(await brief.text(), fs.readFileSync(briefFile, 'utf8'));
    briefBytesMatch = true;
  }
  const denied = [];
  for (const url of [`/api/projects/${slug}-missing`, `/api/media/catalog/${slug}/renders/final.mp4`, '/api/media/etc/passwd', `/api/media/catalog/${slug}/%2e%2e%2f%2e%2e%2f%2e%2e%2fetc/passwd`]) {
    const response = await context.request.get(`${base}${url}`, { maxRedirects: 0 });
    assert.ok([400, 403, 404].includes(response.status()), `Out-of-scope or absent resource unexpectedly served: ${url}`);
    denied.push({ url, status: response.status() });
  }
  report.checks.push({ name: 'external-catalog-details-and-media-isolation', passed: true, facts: { engine: details.engine, absent_master: true, brief_present: Boolean(details.brief_url), brief_bytes_match: briefBytesMatch, denied } });

  checkpoint.write('real-agent-configuration');
  if (process.env.UAT_REAL_AGENT !== '1') throw new Error('REAL_AGENT_UNCONFIGURED: set UAT_REAL_AGENT=1 and mount a dedicated model configuration explicitly; no fake agent is permitted');
  const config = JSON.parse(fs.readFileSync(process.env.OPENCODE_CONFIG, 'utf8'));
  validateModelConfig(config);
  report.model = config.model;
  const modelStatus = await (await context.request.get(`${base}/api/models`)).json();
  if (!modelStatus.configured || modelStatus.active_model !== config.model) {
    await page.locator('#settingsButton').click();
    const discoveredResponse = page.waitForResponse(r => new URL(r.url()).pathname === '/api/models/discover' && r.request().method() === 'POST');
    await page.locator('#discoverModelsButton').click();
    const discovered = await discoveredResponse;
    assert.equal(discovered.status(), 200, 'Native model discovery failed');
    await page.locator(`#modelSelect option[value="${config.model}"]`).waitFor({ state: 'attached' });
    await page.locator('#modelSelect').selectOption(config.model);
    const savedResponse = page.waitForResponse(r => new URL(r.url()).pathname === '/api/models' && r.request().method() === 'POST');
    await page.locator('#saveModelButton').click();
    const saved = await savedResponse;
    assert.equal(saved.status(), 200, 'Native model selection failed');
    await page.locator('[data-close="settingsDialog"]').click();
  }
  await page.waitForFunction(() => !document.querySelector('#promptInput').disabled);
  const prompts = [
    'Make a 3-second title card that says "Hello from Facet", with a blue background and an audible tone. Export it at 320x180, 24fps, H.264 video and AAC audio to renders/final.mp4. Use the installed Facet toolbox and local assets only, with no paid APIs or network asset downloads.',
    'Change the title to "See you soon" and the background to green. Keep the same duration, dimensions, frame rate, codecs and audible tone, and export the revised video to renders/final.mp4 using the installed Facet toolbox.'
  ];
  report.prompts = [];
  for (let i = 0; i < prompts.length; i++) {
    const before = await page.evaluate(() => window.__uatEvents.length);
    const prompt = prompts[i];
    report.prompts.push({ turn: i + 1, text: prompt });
    checkpoint.write(`turn-${i + 1}-submit`);
    await page.locator('#promptInput').fill(prompt);
    checkpoint.beginTurn(i + 1);
    const receiptTurn = beginReceiptTurn();
    await page.locator('#sendButton').click();
    checkpoint.write(`turn-${i + 1}-wait`);
    const waitStarted = performance.now();
    try {
      await page.waitForFunction(start => window.__uatEvents.slice(start).some(e => e.type === 'end'), before, { timeout });
    } catch (error) {
      if (error.name === 'TimeoutError') {
        report.timeout_diagnostics = {
          reason: 'turn-end-not-observed-before-deadline', adjudication: false,
          timeoutMs: timeout, waitElapsedMs: performance.now() - waitStarted,
          observedAt: new Date().toISOString(), telemetry: checkpoint.telemetry()
        };
        checkpoint.write(`turn-${i + 1}-timeout`);
        report.timeout_diagnostics.processInventory = await captureProcessInventory();
        checkpoint.write(`turn-${i + 1}-timeout-captured`);
      }
      throw error;
    }
    const events = await page.evaluate(start => window.__uatEvents.slice(start), before);
    const receiptCompletedAt = Date.now();
    const capturedReceipts = collectReceipts(receiptTurn);
    receiptSnapshots.push({ directory: receiptTurn.directory, index: capturedReceipts.index });
    report.receipts ??= [];
    report.receipts.push({ turn: i + 1, startedAt: receiptTurn.startedAt, completedAt: receiptCompletedAt, baseline: receiptTurn.baseline, ...capturedReceipts });
    checkpoint.write(`turn-${i + 1}-completion-and-media`);
    const end = events.find(e => e.type === 'end');
    assert.equal(end.data.ok, true, `Turn ${i + 1} did not complete successfully`);
    assert.equal(end.data.alive, true, `Turn ${i + 1} lost its conversation`);
    assert.equal(fs.realpathSync(path.join(projectPath, 'renders/final.mp4')), path.join(projectPath, 'renders/final.mp4'), 'Final media path must not traverse symlinks');
    const facts = inspectMedia(path.join(projectPath, 'renders/final.mp4'), { minDuration: 2.5, maxDuration: 4.5 });
    const video = facts.streams.find(s => s.codec_type === 'video');
    assert.equal(video.width, 320); assert.equal(video.height, 180);
    for (const rate of [video.avg_frame_rate, video.r_frame_rate]) {
      const [numerator, denominator] = String(rate).split('/').map(Number);
      assert.ok(Number.isFinite(numerator / denominator) && Math.abs(numerator / denominator - 24) < 0.01, 'Expected 24fps');
    }
    checkpoint.write(`turn-${i + 1}-session-and-tool-proof`);
    const sessions = events.filter(e => e.type === 'session').map(e => e.data);
    const native = sessions.find(s => s.native_id)?.native_id;
    assert.ok(native, 'No native CLI session evidence');
    const session = sessions[0]?.id;
    const toolProof = capturedReceipts.receipts.length
      ? receiptProof(capturedReceipts, events, projectPath, native, facts, {
        startedAt: receiptTurn.startedAt, completedAt: receiptCompletedAt,
        executableSha256: fs.readFileSync('/opt/uat/facet.sha256', 'utf8').split(/\s/)[0]
      })
      : renderingProof(events, projectPath, native, facts);
    assert.deepEqual(receiptIndex(receiptTurn.directory), capturedReceipts.index, 'Receipts mutated after collection');
    checkpoint.write(`turn-${i + 1}-decoded-media-and-revision`);
    const decoded = decodedAgentMedia(path.join(projectPath, 'renders/final.mp4'));
    if (i) {
      assert.equal(native, report.turns[0].native_id, 'Revision did not resume native session');
      assert.equal(session, report.turns[0].session_id, 'Revision replaced Studio session');
      assert.notEqual(facts.sha256, report.turns[0].facts.sha256, 'Revision did not change rendered bytes');
      assert.notEqual(decoded.decoded_frame_sha256, report.turns[0].decoded.decoded_frame_sha256, 'Revision did not change decoded visual content at one second');
    }
    fs.copyFileSync(path.join(projectPath, 'renders/final.mp4'), path.join(out, `turn-${i + 1}.mp4`));
    execFileSync('ffmpeg', ['-v', 'error', '-y', '-ss', '1', '-i', path.join(out, `turn-${i + 1}.mp4`), '-frames:v', '1', path.join(out, `turn-${i + 1}-frame.png`)], { timeout: 30000, stdio: ['ignore', 'pipe', 'pipe'] });
    const turn = { native_id: native, session_id: session, facts, decoded, tool_proof: toolProof };
    report.turns.push(turn);
    await page.screenshot({ path: path.join(out, `0${i + 2}-turn.png`), fullPage: true });
    // Verify the first deliverable before submitting the revision; do not let a
    // final-only playback check conceal an unusable initial output.
    checkpoint.write(`turn-${i + 1}-playback`);
    await page.waitForFunction(() => { const v = document.querySelector('#projectVideo'); return !v.hidden && v.readyState >= 2 && v.duration > 0; }, null, { timeout: 30000 });
    await page.locator('#projectVideo').evaluate(async v => { v.pause(); v.currentTime = 0; await v.play(); });
    await page.waitForFunction(() => document.querySelector('#projectVideo').currentTime > 0.3, null, { timeout: 15000 });
    turn.playback = await page.locator('#projectVideo').evaluate(v => ({ duration: v.duration, currentTime: v.currentTime, error: v.error?.code || null }));
    assert.equal(turn.playback.error, null);
    await page.locator('#projectVideo').evaluate(v => v.pause());
    checkpoint.write(`turn-${i + 1}-download`);
    const downloadPromise = page.waitForEvent('download');
    await page.locator('#videoLink').click();
    const download = await downloadPromise;
    assert.equal(await download.failure(), null);
    const downloaded = path.join(out, `turn-${i + 1}-download.mp4`);
    await download.saveAs(downloaded);
    turn.download = inspectMedia(downloaded, { minDuration: 2.5, maxDuration: 4.5 });
    assert.equal(turn.download.sha256, facts.sha256, 'Browser downloaded stale or different rendered bytes');
    report.checks.push({ name: `turn-${i + 1}-playback-and-download`, passed: true });
    await page.screenshot({ path: path.join(out, `turn-${i + 1}-playback.png`), fullPage: true });
    checkpoint.write(`turn-${i + 1}-verified`);
  }
  for (const snapshot of receiptSnapshots) assert.deepEqual(receiptIndex(snapshot.directory), snapshot.index, 'Receipts mutated before completion');
  report.passed = true;
} catch (error) {
  report.error = redact(error.message);
  process.exitCode = 1;
  checkpoint.write('failure-diagnostics');
  await page.screenshot({ path: path.join(out, 'failure.png'), fullPage: true }).catch(() => {});
} finally {
  checkpoint.write('finalizing');
  await Promise.allSettled([...pending]);
  const steps = transcript.filter(e => e.type === 'cc' && e.data?.raw?.type === 'step_finish');
  report.observed = {
    native_ids: [...new Set(transcript.filter(e => e.type === 'session').map(e => e.data.native_id).filter(Boolean))],
    normalized_tool_uses: transcript.filter(e => e.type === 'cc' && e.data?.type === 'tool_use').length,
    normalized_tool_results: transcript.filter(e => e.type === 'cc' && e.data?.type === 'tool_result').length,
    model_steps: steps.length,
    reported_costs: steps.map(e => e.data.raw.part?.cost),
    end_events: transcript.filter(e => e.type === 'end').map(e => e.data)
  };
  // Preserve an actual failed deliverable for diagnosis, never as a passed turn.
  const candidate = projectPath && path.join(projectPath, 'renders/final.mp4');
  if (!report.passed && candidate && fs.existsSync(candidate) && fs.lstatSync(candidate).isFile() && fs.realpathSync(candidate).startsWith(`${fs.realpathSync(projectPath)}/`)) {
    const saved = path.join(out, 'failed-candidate.mp4');
    fs.copyFileSync(candidate, saved);
    report.failed_candidate = { file: 'failed-candidate.mp4', accepted: false };
    try {
      report.failed_candidate.facts = inspectMedia(saved, { minDuration: 2.5, maxDuration: 4.5 });
      report.failed_candidate.decoded = decodedAgentMedia(saved);
    } catch (error) { report.failed_candidate.error = redact(error.message); }
  }
  checkpoint.write('trace-and-no-pageerrors');
  await context.tracing.stop({ path: path.join(out, 'trace-private.zip') });
  await browser.close();
  clearInterval(checkpointTimer);
  await Promise.allSettled([...pending]);
  assert.ok(checkpoint.ready(), 'Session token capture incomplete; refusing content and trace export');
  const errors = consoleLog.filter(e => e.type === 'pageerror');
  report.checks.push({ name: 'no-pageerrors', passed: errors.length === 0, count: errors.length });
  if (errors.length) {
    report.passed = false;
    report.error = [report.error, 'Browser reported JavaScript errors'].filter(Boolean).join('; ');
    process.exitCode = 1;
  }
  // Raw traces contain session-token URLs/bodies. Sanitize every text resource
  // before export, and never retain or export the original archive.
  const raw = new AdmZip(path.join(out, 'trace-private.zip'));
  const safe = new AdmZip();
  for (const entry of raw.getEntries()) {
    if (entry.isDirectory) continue;
    const bytes = entry.getData();
    const text = bytes.toString('utf8');
    if (!text.includes('\uFFFD') && !bytes.includes(0)) safe.addFile(entry.entryName, Buffer.from(redact(text)));
    else if (/\.(png|jpeg|jpg|woff2?)$/i.test(entry.entryName)) safe.addFile(entry.entryName, bytes);
  }
  safe.writeZip(path.join(out, 'trace.zip.writing'));
  fs.renameSync(path.join(out, 'trace.zip.writing'), path.join(out, 'trace.zip'));
  fs.unlinkSync(path.join(out, 'trace-private.zip'));
  atomicJSON(path.join(out, 'console.json'), consoleLog, redact);
  atomicJSON(path.join(out, 'transcript.json'), transcript, redact);
  atomicJSON(path.join(out, 'result.json'), { ...report, status: 'complete', final: true }, redact);
  console.log(redact(JSON.stringify({ track: report.track, passed: report.passed, error: report.error, completed_turns: report.turns.length })));
}
