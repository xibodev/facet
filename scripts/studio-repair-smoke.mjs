// Run from any directory: node scripts/studio-repair-smoke.mjs
// Prerequisites: Go, Node 20+, ffmpeg on PATH; from the repository root run
// npm ci then npx --no-install playwright install chromium for the locked browser revision.
// Builds the current Studio and a SYNTHETIC CLI adapter, never invokes a real AI agent.
// Uses an isolated catalog/productions directory and ephemeral loopback port.
// Results, screenshots, and build paths are printed; fixtures remain for inspection.
// --recovery-only skips the broader media/layout smoke test; no video is generated.
import { mkdir, mkdtemp, readFile, writeFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { execFileSync, spawn } from 'node:child_process';
import { createServer } from 'node:net';
import assert from 'node:assert/strict';
import { chromium } from 'playwright';

const repo = fileURLToPath(new URL('../', import.meta.url));
const parent = path.join(tmpdir(), 'opencode');
await mkdir(parent, { recursive: true });
const build = await mkdtemp(path.join(parent, 'facet-studio-smoke-build-'));
const suffix = process.platform === 'win32' ? '.exe' : '';
const candidate = path.join(build, `facet-ui${suffix}`);
const adapter = path.join(build, `codex${suffix}`);
const options = { cwd: repo, stdio: 'inherit', timeout: 30000 };
console.log(`Synthetic adapter regression, NOT real-agent acceptance. Build: ${build}`);
execFileSync('go', ['build', '-o', candidate, './cmd/facet-ui'], { ...options, timeout: 120000 });
execFileSync('go', ['build', '-o', adapter, './internal/studio/testdata/session_fixture.go'], { ...options, timeout: 120000 });
try {
  await recoveryRegression();
  if (!process.argv.includes('--recovery-only')) execFileSync(process.execPath, ['internal/studio/repairs-browser.cjs', candidate, adapter], { ...options, timeout: 60000 });
} catch (error) {
  process.exitCode = error.status || 1;
  console.error(error);
}

async function recoveryRegression() {
  const base = await mkdtemp(path.join(parent, 'facet-session-recovery-synthetic-'));
  const root = path.join(base, 'studio');
  const home = path.join(base, 'home');
  await mkdir(path.join(root, '.facet'), { recursive: true });
  await mkdir(home);
  await writeFile(path.join(root, '.facet', 'catalog.json'), JSON.stringify({ version: '1.0', projects: [] }));
  const port = await new Promise(resolve => {
    const server = createServer();
    server.listen(0, '127.0.0.1', () => {
      const value = server.address().port;
      server.close(() => resolve(value));
    });
  });
  const origin = `http://127.0.0.1:${port}`;
  const child = spawn(candidate, ['--dir', root, '--port', String(port), '--no-open'], {
    env: { ...process.env, HOME: home, USERPROFILE: home, LOCALAPPDATA: home, PATH: build + path.delimiter + process.env.PATH },
    stdio: ['ignore', 'pipe', 'pipe']
  });
  let log = '';
  child.stdout.on('data', data => { log += data; });
  child.stderr.on('data', data => { log += data; });
  let browser;
  const errors = [];
  const requests = [];
  const results = { kind: 'Independent SYNTHETIC session recovery regression; NOT real-agent acceptance', base, checks: [], requests, pageerrors: errors, events: [] };
  try {
    let healthy = false;
    for (let i = 0; i < 100; i++) {
      try { healthy = (await fetch(origin, { signal: AbortSignal.timeout(1000) })).ok; } catch { }
      if (healthy) break;
      await new Promise(resolve => setTimeout(resolve, 100));
    }
    assert.ok(healthy, 'Isolated Studio failed to start');
    browser = await chromium.launch({ headless: true, timeout: 10000 });
    const page = await browser.newPage();
    page.setDefaultTimeout(10000);
    const record = event => results.events.push({ at: new Date().toISOString(), ...event });
    const requestURL = request => {
      const url = new URL(request.url());
      url.searchParams.delete('token');
      return url.pathname + url.search;
    };
    page.on('pageerror', error => {
      errors.push(error.message);
      record({ type: 'pageerror', error: error.message });
    });
    page.on('request', request => {
      const url = new URL(request.url());
      if (url.pathname.startsWith('/api/')) record({ type: 'request', method: request.method(), url: requestURL(request) });
      if (url.pathname === '/api/chat') {
        url.searchParams.delete('token');
        requests.push(Object.fromEntries(url.searchParams));
      }
    });
    page.on('response', response => {
      if (new URL(response.url()).pathname.startsWith('/api/')) record({ type: 'response', url: requestURL(response.request()), status: response.status() });
    });
    for (const type of ['requestfinished', 'requestfailed']) page.on(type, request => {
      if (new URL(request.url()).pathname.startsWith('/api/')) record({ type, url: requestURL(request), error: request.failure()?.errorText });
    });
    await page.exposeFunction('recordSmokeEvent', record);
    await page.addInitScript(() => {
      const snapshot = () => ({
        project: document.querySelector('#projectSelect')?.value,
        engine: document.querySelector('#engineSelect')?.value,
        input: document.querySelector('#promptInput')?.value,
        inputDisabled: document.querySelector('#promptInput')?.disabled,
        sendDisabled: document.querySelector('#sendButton')?.disabled,
        live: document.querySelector('#liveText')?.textContent,
        session: document.querySelector('#sessionControl')?.textContent,
        activity: document.querySelector('#activityList')?.textContent
      });
      window.smokeSnapshot = snapshot;
      let previous;
      const state = () => {
        const current = JSON.stringify(snapshot());
        if (current === previous) return;
        previous = current;
        void window.recordSmokeEvent({ type: 'ui', ...snapshot() });
      };
      new MutationObserver(state).observe(document, { subtree: true, childList: true, characterData: true, attributes: true, attributeFilter: ['disabled'] });
      for (const type of ['input', 'change', 'submit']) document.addEventListener(type, event => {
        if (!['composer', 'promptInput', 'projectSelect', 'engineSelect'].includes(event.target.id)) return;
        void window.recordSmokeEvent({ type, target: event.target.id, ...snapshot() });
        // Runs after the app's submit listener, proving requestSubmit reached it.
        if (type === 'submit') queueMicrotask(() => void window.recordSmokeEvent({ type: 'submit-handled', defaultPrevented: event.defaultPrevented, ...snapshot() }));
      }, true);
    });
    // Only the synthetic codex adapter may receive prompts, regardless of installed CLIs.
    await page.route('**/api/engines', route => route.fulfill({ json: { engines: [{ name: 'codex', available: true }, { name: 'opencode', available: true }] } }));
    await page.route('**/api/chat?**', route => {
      assert.equal(new URL(route.request().url()).searchParams.get('engine'), 'codex');
      return route.continue();
    });
    await page.goto(origin);
    for (const name of ['Synthetic Recovery A', 'Synthetic Recovery B']) {
      await page.locator('#btnQuickNew').click();
      await page.locator('#newProdName').fill(name);
      const slug = await page.locator('#newProdSlug').inputValue();
      await page.locator('#newProdEngine').selectOption('codex');
      await page.locator('#btnSubmitNewProd').click();
      // The modal closes before loadProjects/selectProject finish; the old input may still be enabled.
      await page.waitForFunction(slug => document.querySelector('#modalNewProduction').hidden
        && !document.querySelector('#btnSubmitNewProd').disabled
        && document.querySelector('#projectSelect').value === slug
        && !document.querySelector('#promptInput').disabled, slug);
    }
    await page.locator('#projectSelect').selectOption('synthetic-recovery-a');
    const submit = async text => {
      record({ type: 'normal-submit', text });
      await page.waitForFunction(() => !document.querySelector('#promptInput').disabled);
      await page.locator('#promptInput').fill(text);
      await page.waitForFunction(() => !document.querySelector('#promptInput').disabled && !document.querySelector('#sendButton').disabled);
      await page.locator('#composer').evaluate(form => {
        if (document.querySelector('#promptInput').disabled || document.querySelector('#sendButton').disabled) throw new Error('Composer became blocked before normal submit');
        form.requestSubmit();
      });
      await page.waitForFunction(() => document.querySelector('#liveText').textContent.includes('ready for the next'));
    };
    const holdRecovery = async () => {
      let release;
      const released = new Promise(resolve => { release = resolve; });
      let arrived;
      let arrivalTimer;
      const arrival = new Promise((resolve, reject) => {
        arrived = resolve;
        arrivalTimer = setTimeout(() => reject(new Error('Session recovery request did not arrive')), 10000);
      });
      // Attach immediately so a stalled reload cannot leave an unhandled rejection.
      arrival.catch(() => {});
      await page.route('**/api/session?**', async route => {
        const response = await route.fetch({ timeout: 10000 });
        arrived();
        const failure = await released;
        if (failure === 'network') await route.abort();
        else if (failure === 'http') await route.fulfill({ status: 404, json: { error: 'Synthetic missing session' } });
        else if (failure === 'json') await route.fulfill({ body: '{invalid', contentType: 'application/json' });
        else if (failure === 'identity') await route.fulfill({ json: { ...await response.json(), id: 'stale-session' } });
        else if (failure === 'engine') await route.fulfill({ json: { ...await response.json(), engine: 'opencode' } });
        else if (failure === 'dead') await route.fulfill({ json: { ...await response.json(), alive: false } });
        else await route.fulfill({ response });
      }, { times: 1 });
      try {
        await page.reload();
        await arrival;
      } finally { clearTimeout(arrivalTimer); }
      return async failure => {
        const settled = page.waitForEvent(failure === 'network' ? 'requestfailed' : 'requestfinished', request => new URL(request.url()).pathname === '/api/session');
        release(failure);
        await settled;
        // Flush fetch/json continuation and its finally block, not an arbitrary delay.
        await page.evaluate(() => new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve))));
      };
    };
    const assertPending = async () => {
      const before = requests.length;
      assert.equal(await page.locator('#promptInput').isDisabled(), true);
      assert.equal(await page.locator('#sendButton').isDisabled(), true);
      // Bypass disabled controls to exercise sendPrompt's own guard.
      await page.locator('#composer').evaluate(form => {
        document.querySelector('#promptInput').value = 'Synthetic premature submit';
        form.requestSubmit();
      });
      await page.evaluate(() => new Promise(resolve => requestAnimationFrame(resolve)));
      assert.equal(requests.length, before, 'Pending recovery emitted a chat request');
      assert.equal(await page.locator('#promptInput').inputValue(), 'Synthetic premature submit');
    };
    await submit('Synthetic first turn');
    let release = await holdRecovery();
    await assertPending();
    assert.match(await page.locator('#sessionControl').textContent(), /Restoring/);
    await release();
    await page.waitForFunction(() => document.querySelector('#liveText').textContent.includes('conversation restored'));
    await submit('Synthetic resumed turn');
    assert.equal(requests.length, 2, 'Recovery unexpectedly started a fresh turn');
    assert.ok(requests[1].session);
    const catalog = await (await fetch(origin + '/api/catalog')).json();
    const projectA = catalog.projects.find(project => project.id === 'synthetic-recovery-a');
    const turns = (await readFile(path.join(projectA.path, 'fixture-turns.jsonl'), 'utf8')).trim().split('\n').map(JSON.parse);
    assert.equal(turns.length, 2);
    assert.match(turns[1].args, /resume.*fixture-native-conversation/);
    results.checks.push('Delayed recovery blocks UI and forced submit; exactly two turns, native resume, no fresh start');
    for (const changeContext of [false, true]) {
      let releaseDetail;
      const detailGate = new Promise(resolve => { releaseDetail = resolve; });
      let heldRequest;
      // Only the FIRST recovered detail request hangs; ordinary 3-second polls succeed.
      await page.route('**/api/projects/synthetic-recovery-a', async route => {
        heldRequest = route.request();
        const response = await route.fetch({ timeout: 10000 });
        await detailGate;
        await route.fulfill({ response });
      }, { times: 1 });
      const started = Date.now();
      const sessionLookups = () => results.events.filter(event => event.type === 'request' && event.url.startsWith('/api/session?')).length;
      const beforeLookups = sessionLookups();
      await page.reload();
      await page.waitForFunction(() => document.querySelector('#sessionControl').textContent.includes('Restoring'));
      await assertPending();
      assert.ok(heldRequest, 'The first recovered project detail was not intercepted');
      const poll = await page.waitForResponse(response => response.request() !== heldRequest
        && new URL(response.url()).pathname === '/api/projects/synthetic-recovery-a' && response.ok());
      await poll.finished();
      const polledProject = await poll.json();
      await page.waitForFunction(title => document.querySelector('#workspaceTitle').textContent === title,
        polledProject.name || polledProject.slug || 'synthetic-recovery-a');
      await assertPending();
      let newerSession;
      if (changeContext) {
        await page.locator('#projectSelect').selectOption('synthetic-recovery-b');
        await submit('Synthetic new context before old recovery deadline');
        assert.equal(requests.at(-1).session, undefined);
        await submit('Synthetic resume new context before old recovery deadline');
        newerSession = requests.at(-1).session;
        assert.ok(newerSession);
        // Keep the old selection unresolved past its deadline, with a live new context.
        await page.waitForTimeout(Math.max(0, 11000 - (Date.now() - started)));
      } else {
        const boundMs = 12000; // Ten-second deadline plus browser scheduling tolerance.
        await page.waitForFunction(() => !document.querySelector('#promptInput').disabled
          && !document.querySelector('#sendButton').disabled
          && !document.querySelector('#sessionControl').textContent.includes('Restoring'), null,
        { timeout: Math.max(1, boundMs - (Date.now() - started)) });
        const elapsedMs = Date.now() - started;
        assert.ok(elapsedMs <= boundMs, `Recovery remained pending for ${elapsedMs}ms`);
        record({ type: 'project-recovery-deadline', elapsedMs, boundMs });
      }
      // Release only AFTER the deadline; flush the actual late fetch continuation.
      const finished = page.waitForEvent('requestfinished', request => request === heldRequest);
      releaseDetail();
      await finished;
      await page.evaluate(() => new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve))));
      assert.equal(sessionLookups(), beforeLookups, 'Late selection started stale session restoration');
      assert.doesNotMatch(await page.locator('#liveText').textContent(), /conversation restored/);
      assert.equal(await page.locator('#projectSelect').inputValue(), changeContext ? 'synthetic-recovery-b' : 'synthetic-recovery-a');
      await submit(`Synthetic normal submit after late project recovery (${changeContext ? 'new context' : 'same context'})`);
      assert.equal(requests.at(-1).session, newerSession, 'Recovery revived the saved session or reset the newer session');
      assert.equal(requests.at(-1).dir, changeContext ? catalog.projects.find(project => project.id === 'synthetic-recovery-b').path : projectA.path);
      results.checks.push(changeContext
        ? 'Old project recovery deadline and late detail preserve a newer project conversation'
        : 'Hung first project detail with successful polls clears recovery within 12s; late detail cannot restore session and normal submit starts fresh');
      if (changeContext) {
        await page.locator('#projectSelect').selectOption('synthetic-recovery-a');
        await submit('Synthetic project A after recovery deadline isolation');
      }
    }
    for (const failure of ['http', 'network', 'json', 'identity', 'engine', 'dead']) {
      release = await holdRecovery();
      await assertPending();
      await release(failure);
      await submit(`Synthetic normal start after ${failure} recovery failure`);
      assert.equal(requests.at(-1).session, undefined);
      results.checks.push(`${failure} recovery failure unlocks normal fresh start`);
    }
    // An indefinitely pending server must not lock the composer forever.
    await page.route('**/api/session?**', async route => {
      await new Promise(resolve => setTimeout(resolve, 12000));
      await route.abort().catch(() => {});
    }, { times: 1 });
    await page.reload();
    await page.waitForFunction(() => document.querySelector('#sessionControl').textContent.includes('Restoring'));
    await assertPending();
    await page.waitForFunction(() => !document.querySelector('#promptInput').disabled, null, { timeout: 15000 });
    await submit('Synthetic normal start after recovery timeout');
    assert.equal(requests.at(-1).session, undefined);
    results.checks.push('Hung session recovery times out and permits a normal fresh start');
    release = await holdRecovery();
    await assertPending();
    // Hold the new project's load while the old session resolves.
    let releaseProject;
    const projectGate = new Promise(resolve => { releaseProject = resolve; });
    await page.route('**/api/projects/synthetic-recovery-b', async route => {
      await projectGate;
      await route.continue();
    }, { times: 1 });
    await page.locator('#projectSelect').selectOption('synthetic-recovery-b');
    await release();
    await assertPending();
    releaseProject();
    await submit('Synthetic isolated project B');
    assert.equal(requests.at(-1).session, undefined);
    assert.equal(requests.at(-1).dir, catalog.projects.find(project => project.id === 'synthetic-recovery-b').path);
    results.checks.push('Stale project A response cannot unlock pending project B or leak its session');
    release = await holdRecovery();
    await page.locator('#engineSelect').selectOption('opencode');
    await page.waitForFunction(() => !document.querySelector('#promptInput').disabled);
    await page.locator('#engineSelect').selectOption('codex');
    await release();
    await submit('Synthetic engine context ABA');
    assert.equal(requests.at(-1).session, undefined);
    results.checks.push('Engine switch away and back invalidates old recovery even with matching engine/project');
    await page.route('**/api/close?**', async route => {
      await new Promise(resolve => setTimeout(resolve, 12000));
      await route.abort().catch(() => {});
    }, { times: 1 });
    await page.locator('#projectSelect').selectOption('synthetic-recovery-a');
    await page.waitForFunction(() => !document.querySelector('#promptInput').disabled, null, { timeout: 15000 });
    await submit('Synthetic new project after close timeout');
    assert.equal(requests.at(-1).session, undefined);
    assert.equal(requests.at(-1).dir, projectA.path);
    results.checks.push('Hung session close cannot indefinitely block project switching or reuse the old session');
    assert.deepEqual(errors, []);
    results.status = 'passed';
  } catch (error) {
    results.status = 'failed';
    results.error = error.stack;
    const page = browser?.contexts()[0]?.pages()[0];
    if (page) {
      try { results.diagnostic = await page.evaluate(() => window.smokeSnapshot()); }
      catch (diagnosticError) { results.diagnosticError = diagnosticError.message; }
    }
    throw error;
  } finally {
    try { if (browser) await browser.close(); }
    finally {
      child.kill();
      await writeFile(path.join(base, 'server.log'), log);
      await writeFile(path.join(base, 'results.json'), JSON.stringify(results, null, 2));
      console.log(JSON.stringify({ ...results, events: `${results.events.length} events in results.json` }, null, 2));
    }
  }
}
