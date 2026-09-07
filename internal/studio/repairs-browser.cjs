// Opt-in synthetic adapter regression, NOT real AI acceptance.
// Reproducible entry point: node scripts/studio-repair-smoke.mjs
// Never reads or changes the user catalog.
const { chromium } = require('playwright');
const fs = require('node:fs/promises');
const path = require('node:path');
const os = require('node:os');
const net = require('node:net');
const { spawn, execFileSync } = require('node:child_process');
const { createHash } = require('node:crypto');
const assert = require('node:assert/strict');

(async () => {
  const base = await fs.mkdtemp(path.join(os.tmpdir(), 'opencode', 'facet-studio-repairs-'));
  const root = path.join(base, 'studio');
  const appData = path.join(base, 'appdata');
  await fs.mkdir(path.join(root, '.facet'), { recursive: true });
  await fs.mkdir(appData, { recursive: true });
  await fs.writeFile(path.join(root, '.facet', 'catalog.json'), JSON.stringify({version:'1.0', projects:[]}));
  const port = await new Promise(resolve => { const s = net.createServer(); s.listen(0, '127.0.0.1', () => { const p = s.address().port; s.close(() => resolve(p)); }); });
  const origin = `http://127.0.0.1:${port}`;
  const candidate = path.resolve(process.argv[2]);
  const fixtureDir = path.dirname(path.resolve(process.argv[3]));
  const child = spawn(candidate, ['--dir', root, '--port', String(port), '--no-open'], {
    env: {...process.env, LOCALAPPDATA: appData, HOME: appData, USERPROFILE: appData, PATH: fixtureDir + path.delimiter + process.env.PATH}, stdio: ['ignore','pipe','pipe']
  });
  let serverLog = ''; child.stdout.on('data', d => serverLog += d); child.stderr.on('data', d => serverLog += d);
  let browser;
  const results = { kind: 'deterministic fixtures, NOT AI acceptance', base, origin, candidate, screenshots: [] };
  async function checkLayout(page, label) {
    const geometry = await page.evaluate(() => {
      const rect = el => { const r=el.getBoundingClientRect(); return {left:r.left,right:r.right,top:r.top,bottom:r.bottom,width:r.width,height:r.height}; };
      const viewport = {width:innerWidth,height:innerHeight};
      const workspace = document.querySelector('#productionView');
      return {viewport, workspace:rect(workspace), scrollTop:workspace.scrollTop,
        kicker:rect(document.querySelector('#kicker')),
        header:rect(document.querySelector('.titlebar')), nav:rect(document.querySelector('.sidebar')),
        controls:[...document.querySelectorAll('#projectSelect,#btnQuickNew,#btnQuickOpen,#engineSelect,#sessionControl,.nav-button')].map(el=>({id:el.id||el.dataset.nav,...rect(el)}))};
    });
    results.layout ||= {};
    results.layout[label] = geometry;
    assert.ok(geometry.kicker.top >= geometry.workspace.top - 1, `${label}: kicker clipped above workspace (${geometry.kicker.top} < ${geometry.workspace.top}, scroll ${geometry.scrollTop})`);
    for (const control of geometry.controls) {
      assert.ok(control.left >= -1 && control.right <= geometry.viewport.width+1, `${label}: ${control.id} outside viewport`);
      const container = control.id.includes('Select') || control.id.startsWith('btn') || control.id === 'sessionControl' ? geometry.header : geometry.nav;
      assert.ok(control.top >= container.top-1 && control.bottom <= container.bottom+1, `${label}: ${control.id} clipped vertically`);
      if (geometry.viewport.width <= 820) assert.ok(control.height >= 44, `${label}: ${control.id} touch target below 44px`);
    }
    for(let i=0;i<geometry.controls.length;i++) for(let j=i+1;j<geometry.controls.length;j++) {
      const a=geometry.controls[i],b=geometry.controls[j];
      assert.ok(Math.min(a.right,b.right)-Math.max(a.left,b.left)<=1 || Math.min(a.bottom,b.bottom)-Math.max(a.top,b.top)<=1, `${label}: ${a.id} overlaps ${b.id}`);
    }
  }
  try {
    for (let i=0;i<100;i++) { try { if ((await fetch(origin)).ok) break; } catch {} await new Promise(r=>setTimeout(r,100)); }
    browser = await chromium.launch({headless:true});
    const page = await browser.newPage({viewport:{width:1440,height:1000}, acceptDownloads:true});
    const errors = []; page.on('pageerror', e => errors.push(e.message));
    const requests = []; page.on('request', r => { if (r.url().includes('/api/chat?')) { const u=new URL(r.url()); u.searchParams.delete('token'); requests.push(u.toString()); } });
    await page.goto(origin);
    await page.locator('#btnQuickNew').click();
    await page.locator('#newProdName').fill('Fixture Browser One');
    await page.locator('#newProdEngine').selectOption('codex');
    await page.locator('#btnSubmitNewProd').click();
    await page.waitForFunction(() => document.querySelector('#projectSelect').value === 'fixture-browser-one' && !document.querySelector('#modalNewProduction').hidden === false);
    const catalog = await (await fetch(origin+'/api/catalog')).json();
    assert.equal(catalog.projects.length,1);
    const project = catalog.projects[0].path;
    assert.ok(project.startsWith(appData));
    await fs.mkdir(path.join(project,'renders','review_frames'),{recursive:true});
    await fs.mkdir(path.join(project,'review'),{recursive:true});
    const video = path.join(project,'renders','final.mp4');
    execFileSync('ffmpeg',['-hide_banner','-loglevel','error','-f','lavfi','-i','testsrc2=size=640x360:rate=30','-t','6','-c:v','libx264','-pix_fmt','yuv420p','-movflags','+faststart',video]);
    execFileSync('ffmpeg',['-hide_banner','-loglevel','error','-i',video,'-frames:v','1',path.join(project,'renders','review_frames','sample.jpg')]);
    await fs.writeFile(path.join(project,'review','report.json'),JSON.stringify({result:{review_status:'warn', fixture:true}}));
    const dependencies = path.join(project,'node_modules','fixture'); await fs.mkdir(dependencies,{recursive:true});
    await Promise.all(Array.from({length:1000},(_,i)=>fs.writeFile(path.join(dependencies,`${i}.js`),'// irrelevant dependency fixture')));
    results.listMs = [];
    for(let i=0;i<3;i++){const start=performance.now();const list=await(await fetch(origin+'/api/projects')).json();results.listMs.push(performance.now()-start);assert.equal(list.length,1);}
    await page.waitForFunction(() => document.querySelector('#masterVideo').readyState >= 2);
    await page.locator('#playButton').click();
    await page.waitForFunction(() => document.querySelector('#masterVideo').currentTime > 0.75);
    results.playback = await page.locator('#masterVideo').evaluate(v=>({time:v.currentTime,error:v.error?.code||null,src:v.currentSrc,width:v.videoWidth}));
    const [download] = await Promise.all([page.waitForEvent('download'),page.locator('#videoDownloadLink').click()]);
    const downloaded = path.join(base,'download.mp4'); await download.saveAs(downloaded);
    const hash = b=>createHash('sha256').update(b).digest('hex');
    results.sourceHash=hash(await fs.readFile(video));results.downloadHash=hash(await fs.readFile(downloaded));assert.equal(results.sourceHash,results.downloadHash);
    const shot=path.join(base,'desktop-playback.png');await page.screenshot({path:shot,fullPage:true});results.screenshots.push(shot);
    await checkLayout(page, 'desktop-1440');
    await page.locator('#promptInput').fill('Fixture first turn'); await page.locator('#composer').evaluate(f=>f.requestSubmit());
    await page.waitForFunction(()=>document.querySelector('#composerMeta').textContent.includes('1 turn'));
    await page.waitForFunction(()=>document.querySelector('#liveText').textContent.includes('ready for the next'));
    await page.locator('[data-nav="projects"]').click();
    await page.locator('.project-card').filter({hasText:'Fixture Browser One'}).click();
    await page.reload();
    await page.waitForFunction(()=>document.querySelector('#liveText').textContent.includes('conversation restored'));
    await page.locator('#promptInput').fill('Fixture second turn'); await page.locator('#composer').evaluate(f=>f.requestSubmit());
    await page.waitForFunction(()=>document.querySelector('#composerMeta').textContent.includes('2 turns'));
    await page.waitForFunction(()=>document.querySelector('#liveText').textContent.includes('ready for the next'));
    const turns=(await fs.readFile(path.join(project,'fixture-turns.jsonl'),'utf8')).trim().split('\n').map(JSON.parse);
    assert.equal(turns.length,2);assert.match(turns[1].args,/resume.*fixture-native-conversation/);
    assert.equal(new URL(requests[1]).searchParams.get('session')?.startsWith('s'),true);
    results.nativeResume=true;
    await page.locator('#btnQuickNew').click();await page.locator('#newProdName').fill('Fixture Browser Two');await page.locator('#newProdEngine').selectOption('codex');await page.locator('#btnSubmitNewProd').click();
    await page.waitForFunction(()=>document.querySelector('#projectSelect').value==='fixture-browser-two' && document.querySelector('#modalNewProduction').hidden);
    await page.locator('#promptInput').fill('Fixture other project');await page.locator('#composer').evaluate(f=>f.requestSubmit());
    await page.waitForFunction(()=>document.querySelector('#liveText').textContent.includes('ready for the next'));
    assert.equal(new URL(requests[2]).searchParams.has('session'),false);results.crossProjectFresh=true;
    await page.locator('#projectSelect').selectOption('fixture-browser-one');
    await page.waitForFunction(()=>document.querySelector('#masterVideo').readyState>=2);
    await page.locator('[data-nav="assets"]').click();
    const report=page.locator('a[href*="/catalog/fixture-browser-one/review/report.json"]');assert.ok(await report.count());
    results.reviewURL=await report.first().getAttribute('href');assert.equal((await fetch(origin+results.reviewURL)).status,200);
    for (const width of [390,320,560,561,820]) {
      await page.setViewportSize({width,height:844});
      await page.locator('[data-nav="produce"]').click();
      // All navigation remains directly reachable without horizontal scrolling.
      await page.locator('[data-nav="settings"]').click();
      await page.locator('#settingsBack').click();
      const mobile=path.join(base,`mobile-${width}.png`);await page.screenshot({path:mobile,fullPage:true});results.screenshots.push(mobile);
      await checkLayout(page, `mobile-${width}`);
      if (width === 390) {
        const before = await page.locator('#masterVideo').evaluate(v=>{v.pause();v.currentTime=0;return v.currentTime;});
        await page.locator('#playButton').click();
        await page.waitForFunction(start=>document.querySelector('#masterVideo').currentTime > start+0.5,before);
        const [mobileDownload] = await Promise.all([page.waitForEvent('download'),page.locator('#videoDownloadLink').click()]);
        const mobileFile = path.join(base,'mobile-download.mp4');await mobileDownload.saveAs(mobileFile);
        results.mobileDownloadHash=hash(await fs.readFile(mobileFile));assert.equal(results.mobileDownloadHash,results.sourceHash);
        results.mobilePlayback=true;
        await checkLayout(page, 'mobile-390-after-download');
      }
    }
    assert.deepEqual(errors,[]);results.browserErrors=errors;results.status='passed';
  } catch(e) {results.status='failed';results.error=e.stack; if(browser){const p=browser.contexts()[0]?.pages()[0];if(p){const shot=path.join(base,'failure.png');await p.screenshot({path:shot});results.screenshots.push(shot);}} process.exitCode=1; }
  finally { if(browser)await browser.close();child.kill();await fs.writeFile(path.join(base,'server.log'),serverLog);await fs.writeFile(path.join(base,'results.json'),JSON.stringify(results,null,2));console.log(JSON.stringify({...results,layout: Object.keys(results.layout||{}),resultsFile:path.join(base,'results.json')},null,2)); }
})();
