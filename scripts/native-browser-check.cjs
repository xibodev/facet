// Deterministic browser integration against the current Facet app, an
// isolated app-target bundle, and a local streaming model fixture.
const { chromium } = require('playwright');
const fs = require('node:fs/promises');
const os = require('node:os');
const path = require('node:path');
const http = require('node:http');
const { spawn, execFileSync } = require('node:child_process');
const { createHash } = require('node:crypto');
const assert = require('node:assert/strict');

(async () => {
  await fs.mkdir(path.join(os.tmpdir(), 'opencode'), { recursive: true });
  const base = await fs.mkdtemp(path.join(os.tmpdir(), 'opencode', 'facet-native-browser-'));
  const root = path.join(base, 'projects-root');
  const home = path.join(base, 'home');
  const binary = path.resolve(process.argv[2]);
  const bundleRoot = path.join(base, 'bundles');
  await fs.mkdir(path.join(root, '.facet'), { recursive: true });
  await fs.mkdir(home);
  await fs.writeFile(path.join(root, '.facet', 'catalog.json'), JSON.stringify({
    version: '1.0',
    default_root: path.join(root, 'productions'),
    projects: []
  }));

  const modelRequests = [];
  const model = http.createServer(async (req, res) => {
    let raw = '';
    try {
      for await (const chunk of req) raw += chunk;
    } catch (error) {
      if (error.code === 'ECONNRESET') return;
      throw error;
    }
    const input = JSON.parse(raw);
    modelRequests.push(input);
    const latest = input.messages.filter(message => message.role === 'user').at(-1)?.content || '';
    if (latest.includes('fail deliberately')) {
      res.writeHead(503);
      res.end('Fixture service unavailable');
      return;
    }
    const last = input.messages.at(-1);
    const makeCaptions = latest === 'Create captions' && last.role !== 'tool';
    const askApproval = latest === 'Request paid operation' && last.role !== 'tool';
    const content = latest === 'Reply with OK.' ? 'OK' : `Visible reply: ${latest}`;
    if (!input.stream) {
      res.setHeader('Content-Type', 'application/json');
      res.end(JSON.stringify({ choices: [{ message: { role: 'assistant', content }, finish_reason: 'stop' }] }));
      return;
    }
    res.writeHead(200, { 'Content-Type': 'text/event-stream' });
    if (askApproval) {
      res.end(`data: ${JSON.stringify({ choices: [{ index: 0, delta: { tool_calls: [{ index: 0, id: 'paid-1', type: 'function', function: { name: 'openai_image', arguments: JSON.stringify({ prompt: 'Approval fixture', output_path: 'assets/never.png' }) } }] }, finish_reason: null }] })}\n\ndata: {"choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}]}\n\ndata: [DONE]\n\n`);
      return;
    }
    if (makeCaptions) {
      res.end(`data: ${JSON.stringify({ choices: [{ index: 0, delta: { tool_calls: [{ index: 0, id: 'captions-1', type: 'function', function: { name: 'subtitle_gen', arguments: JSON.stringify({ segments: [{ text: 'Browser-created captions', start: 0, end: 2 }], format: 'srt', output_path: 'artifacts/captions.srt' }) } }] }, finish_reason: null }] })}\n\ndata: {"choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}]}\n\ndata: [DONE]\n\n`);
      return;
    }
    res.write(`data: ${JSON.stringify({ choices: [{ index: 0, delta: { content: 'Visible ' }, finish_reason: null }] })}\n\n`);
    setTimeout(() => {
      res.write(`data: ${JSON.stringify({ choices: [{ index: 0, delta: { content: content.slice(8) }, finish_reason: null }] })}\n\n`);
      res.end('data: {"choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}\n\ndata: [DONE]\n\n');
    }, 100);
  });
  await new Promise(resolve => model.listen(0, '127.0.0.1', resolve));
  const endpoint = `http://127.0.0.1:${model.address().port}`;
  await fs.writeFile(path.join(home, 'config.json'), JSON.stringify({
    agents: { defaults: { model_name: 'fixture/model', max_llm_retries: 0 } },
    provider_instances: [{
      id: 'fixture',
      provider_kind: 'openai',
      adapter: 'openai-compatible',
      protocol: 'openai',
      endpoint,
      runtime: { streaming: true },
      state: 'enabled'
    }],
    active_models: ['fixture/model', 'fixture/alternate']
  }));
  await fs.writeFile(path.join(home, 'model_catalogs.json'), JSON.stringify({
    entries: {
      fixture: {
        id: 'fixture',
        instance_id: 'fixture',
        provider: 'openai',
        api_base: endpoint,
        api_key_mask: '',
        models: [{ id: 'model' }, { id: 'alternate' }],
        fetched_at: new Date().toISOString()
      }
    }
  }));
  execFileSync(binary, ['bundle', '--target', 'app', '--out', bundleRoot], { cwd: path.resolve(__dirname, '..') });

  const app = spawn(binary, ['ui', '--dir', root, '--port', '18950', '--no-open'], {
    env: {
      ...process.env,
      FACET_HOME: home,
      FACET_BUNDLE_DIR: path.join(bundleRoot, 'app'),
      LOCALAPPDATA: home,
      HOME: home,
      USERPROFILE: home
    },
    stdio: ['ignore', 'pipe', 'pipe']
  });
  let logs = '';
  app.stdout.on('data', data => logs += data);
  app.stderr.on('data', data => logs += data);
  let browser;
  const report = {
    base,
    binary,
    sha256: createHash('sha256').update(await fs.readFile(binary)).digest('hex'),
    checks: [],
    errors: []
  };
  try {
    for (let i = 0; i < 100 && !logs.includes('URL:'); i++) await new Promise(resolve => setTimeout(resolve, 100));
    const origin = logs.match(/URL: (http:\/\/[^\s]+)/)?.[1];
    assert.ok(origin, 'server did not start');
    browser = await chromium.launch({ ...(process.env.CI ? {} : { channel: 'chrome' }), headless: true });
    const page = await browser.newPage({ viewport: { width: 1440, height: 900 } });
    page.on('pageerror', error => report.errors.push(error.message));
    await page.goto(origin);
    assert.equal(await page.title(), 'Facet');
    await page.locator('#settingsButton').click();
    await page.locator('#modelSelect').selectOption('fixture/model');
    await page.locator('#saveModelButton').click();
    await page.waitForFunction(() => document.querySelector('#modelFeedback').textContent.includes('Saved.'));
    await page.locator('[data-close="settingsDialog"]').click();
    report.checks.push('verified app bundle and selected fixture model');

    await page.locator('#newProjectButton').click();
    await page.locator('#newProjectName').fill('Native browser production');
    await page.locator('#newProjectSlug').fill('native-browser-production');
    const createdResponse = page.waitForResponse(response => new URL(response.url()).pathname === '/api/catalog/new' && response.request().method() === 'POST');
    await page.locator('#createProjectButton').click();
    const created = await createdResponse;
    assert.equal(created.status(), 200);
    const project = (await created.json()).path;
    await page.waitForFunction(() => !document.querySelector('#newProjectDialog').open && !document.querySelector('#promptInput').disabled);
    await page.locator('#promptInput').fill('Remember my blue title');
    await page.locator('#sendButton').click();
    await page.waitForFunction(() => document.querySelector('#activity').textContent.includes('Visible reply: Remember my blue title'));
    await page.waitForFunction(() => !document.querySelector('#promptInput').disabled);
    await page.locator('#promptInput').fill('Continue the project');
    await page.locator('#sendButton').click();
    await page.waitForFunction(() => document.querySelector('#activity').textContent.includes('Visible reply: Continue the project'));
    assert.ok(modelRequests.at(-1).messages.some(message => message.content?.includes('Remember my blue title')), 'follow-up lost conversation context');
    assert.ok(modelRequests.at(-1).messages.some(message => message.content?.includes('Facet Video Producer')), 'missing bundled guidance');
    report.checks.push('created project and preserved guided conversation context');

    await page.waitForFunction(() => !document.querySelector('#promptInput').disabled);
    await page.locator('#promptInput').fill('Create captions');
    await page.locator('#sendButton').click();
    await page.waitForFunction(() => document.querySelector('#activity').textContent.includes('Browser-created captions'));
    const captions = await fs.readFile(path.join(project, 'artifacts', 'captions.srt'), 'utf8');
    assert.ok(captions.includes('00:00:00,000 --> 00:00:02,000') && captions.includes('Browser-created captions'));
    report.checks.push('native subtitle tool created the requested project artifact');

    await page.waitForFunction(() => !document.querySelector('#promptInput').disabled);
    await page.locator('#promptInput').fill('Request paid operation');
    await page.locator('#sendButton').click();
    await page.getByRole('button', { name: 'Deny', exact: true }).click();
    await page.waitForFunction(() => !document.querySelector('#promptInput').disabled);
    assert.equal(await fs.stat(path.join(project, 'assets', 'never.png')).then(() => true, () => false), false);
    report.checks.push('approval denial prevented paid tool execution');

    await page.locator('#settingsButton').click();
    await page.locator('#modelSelect').selectOption('fixture/alternate');
    await page.locator('#saveModelButton').click();
    await page.waitForFunction(() => document.querySelector('#modelFeedback').textContent.includes('Saved.'));
    const modelStatus = await page.evaluate(() => fetch('/api/models').then(response => response.json()));
    assert.equal(modelStatus.active_model, 'fixture/alternate');
    await page.locator('[data-close="settingsDialog"]').click();
    report.checks.push('model switch persisted for the next conversation');

    await page.reload();
    await page.waitForFunction(() => document.querySelector('#activity').textContent.includes('Remember my blue title'));
    await page.setViewportSize({ width: 390, height: 844 });
    await page.locator('#settingsButton').click();
    assert.equal(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth), true);
    assert.deepEqual(report.errors, []);
    report.checks.push('reload restored conversation and mobile layout remained bounded');
    report.status = 'passed';
  } catch (error) {
    report.status = 'failed';
    report.error = error.stack;
    process.exitCode = 1;
    if (browser) await browser.contexts()[0].pages()[0].screenshot({ path: path.join(base, 'failure.png'), fullPage: true });
  } finally {
    if (browser) await browser.close();
    app.kill();
    model.closeAllConnections();
    model.close();
    await fs.writeFile(path.join(base, 'report.json'), JSON.stringify(report, null, 2));
    await fs.writeFile(path.join(base, 'server.log'), logs);
    console.log(JSON.stringify(report, null, 2));
  }
})();
