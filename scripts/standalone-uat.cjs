// Standalone acceptance scenarios through normal Chrome UI controls.
// The local model fixture makes failure/recovery reproducible; live provider
// acceptance is a separate native-live-browser.cjs run, never conflated here.
const {chromium}=require('playwright');
const fs=require('node:fs/promises');
const path=require('node:path');
const os=require('node:os');
const http=require('node:http');
const crypto=require('node:crypto');
const {spawn,execFileSync}=require('node:child_process');
const assert=require('node:assert/strict');

(async()=>{
 await fs.mkdir(path.join(os.tmpdir(),'opencode'),{recursive:true});
 const base=await fs.mkdtemp(path.join(os.tmpdir(),'opencode','facet-complete-uat-'));
 const home=path.join(base,'home'),root=path.join(base,'workspace');
 await fs.mkdir(home);await fs.mkdir(path.join(root,'.facet'),{recursive:true});
 await fs.writeFile(path.join(root,'.facet','catalog.json'),JSON.stringify({version:'1.0',default_root:path.join(root,'productions'),projects:[]}));
 const binary=path.resolve(process.argv[2]);
 const report={scope:'Facet standalone Windows / Chrome',started:new Date().toISOString(),base,binary,sha256:crypto.createHash('sha256').update(await fs.readFile(binary)).digest('hex'),model:'local deterministic HTTP fixture; not live AI',cases:[],pageErrors:[],dialogs:[]};
 const requests=[];
 const model=http.createServer(async(req,res)=>{
  if(req.method==='GET'){res.setHeader('Content-Type','application/json');res.end(JSON.stringify({object:'list',data:[{id:'uat-model',object:'model'},{id:'uat-other',object:'model'}]}));return;}
  let raw='';try {for await(const chunk of req)raw+=chunk;} catch(error) {if(error.code==='ECONNRESET')return;throw error;}
  const input=JSON.parse(raw);requests.push(input);
  const latest=input.messages?.filter(m=>m.role==='user').at(-1)?.content||'';
  if(latest==='hold request'){res.writeHead(200,{'Content-Type':'text/event-stream'});res.write(':waiting\n\n');return;}
  const answer=`UAT response (${input.model}): ${latest}`;
  if(input.stream){res.writeHead(200,{'Content-Type':'text/event-stream'});res.end(`data: ${JSON.stringify({choices:[{index:0,delta:{content:answer},finish_reason:null}]})}\n\ndata: {"choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}\n\ndata: [DONE]\n\n`);}
  else{res.setHeader('Content-Type','application/json');res.end(JSON.stringify({choices:[{message:{role:'assistant',content:answer},finish_reason:'stop'}]}));}
 });await new Promise(r=>model.listen(0,'127.0.0.1',r));
 let app,log='',origin,browser,context,page;
 async function start(){
  log='';app=spawn(binary,['ui','--dir',root,'--port','18990','--no-open'],{env:{...process.env,FACET_HOME:home,LOCALAPPDATA:home,HOME:home,USERPROFILE:home},stdio:['ignore','pipe','pipe']});
  app.stdout.on('data',d=>log+=d);app.stderr.on('data',d=>log+=d);
  for(let i=0;i<100&&!log.includes('URL:');i++)await new Promise(r=>setTimeout(r,100));
  origin=log.match(/URL: (http:\/\/[^\s]+)/)?.[1];assert.ok(origin,'application failed to start');
 }
 async function stop(){if(app&&app.exitCode===null){const exited=new Promise(r=>app.once('exit',r));app.kill();await exited;}}
 async function shot(name){const file=path.join(base,`${name}.png`);await page.screenshot({path:file,fullPage:true});return file;}
 async function scenario(id,title,fn){const entry={id,title,started:new Date().toISOString()};report.cases.push(entry);try{await fn();entry.result='pass';}catch(error){entry.result='fail';entry.error=error.message;entry.screenshot=await shot(id+'-failure').catch(()=>null);await page.keyboard.press('Escape').catch(()=>{});}entry.finished=new Date().toISOString();await fs.writeFile(path.join(base,'report.json'),JSON.stringify(report,null,2));}
 async function create(name){await page.locator('#btnQuickNew').click();await page.locator('#newProdName').fill(name);const slug=await page.locator('#newProdSlug').inputValue();await page.locator('#btnSubmitNewProd').click();await page.waitForFunction(slug=>document.querySelector('#modalNewProduction').hidden&&document.querySelector('#projectSelect').value===slug&&!document.querySelector('#promptInput').disabled,slug);}
 async function send(text){await page.waitForFunction(()=>!document.querySelector('#promptInput').disabled);await page.locator('#promptInput').fill(text);await page.locator('#sendButton').click();await page.waitForFunction(text=>document.querySelector('#activityList').textContent.includes(`: ${text.split('\n')[0]}`),text);await page.waitForFunction(()=>!document.querySelector('#promptInput').disabled);}
 async function choose(slug){await page.locator('#projectSelect').selectOption(slug);await page.waitForFunction(slug=>document.querySelector('#projectSelect').value===slug&&!document.querySelector('#promptInput').disabled,slug);}
 try{
  await start();browser=await chromium.launch({channel:'chrome',headless:!process.argv.includes('--headed')});
  context=await browser.newContext({viewport:{width:1440,height:1000},recordVideo:{dir:path.join(base,'video')}});
  await context.tracing.start({screenshots:true,snapshots:true,sources:false});page=await context.newPage();page.setDefaultTimeout(12000);
  page.on('pageerror',e=>report.pageErrors.push(e.message));page.on('dialog',async d=>{report.dialogs.push(d.message());await d.dismiss();});
  await page.goto(origin);
  await scenario('01','First run identifies Facet and shows an honest unconfigured state',async()=>{
   assert.equal(await page.title(),'Facet — Video production');await page.locator('#btnModelSettings').click();
   await page.waitForFunction(()=>document.querySelector('#modelSelect').textContent.includes('No models configured'));
   assert.equal(await page.locator('#modelSelect').inputValue(),'');assert.ok((await page.locator('#modelFeedback').innerText()).includes('get started'));
   await shot('01-first-run');
  });
  await scenario('02','Connect a provider through UI, discover its catalog, test and save selection',async()=>{
   await page.getByText('Connect a provider or local model server',{exact:true}).click();
   await page.locator('#providerSelect').selectOption('openai');await page.locator('#providerEndpoint').fill(`http://127.0.0.1:${model.address().port}/v1`);
   await page.locator('#connectProvider').click();await page.waitForFunction(()=>document.querySelector('#modelSelect').textContent.includes('uat-model'));
   await page.locator('#modelSelect').selectOption('openai/uat-model');await page.locator('#btnTestModel').click();await page.waitForFunction(()=>document.querySelector('#modelFeedback').textContent.includes('Connected successfully'));
   await page.locator('#btnSaveModel').click();await page.waitForFunction(()=>!document.querySelector('#btnSaveModel').disabled);
   assert.equal(JSON.parse(await fs.readFile(path.join(home,'config.json'),'utf8')).agents.defaults.model_name,'openai/uat-model');
  });
  await scenario('03','Project creation and Unicode/multiline conversation',async()=>{await create('UAT Alpha');await send('Remember ALPHA-ONLY. Café 日本語 🎬\nSecond line');assert.ok(requests.at(-1).messages.some(m=>m.content?.includes('日本語')));});
  const alpha=path.join(root,'productions','uat-alpha');
  await scenario('04','Duplicate project creation is rejected without overwriting user material',async()=>{
   const brief=path.join(alpha,'artifacts','brief.md');await fs.writeFile(brief,'# USER-OWNED CONTENT\n');
   await page.locator('#btnQuickNew').click();await page.locator('#newProdName').fill('UAT Alpha');
   const response=page.waitForResponse(r=>new URL(r.url()).pathname==='/api/catalog/new'&&r.request().method()==='POST');await page.locator('#btnSubmitNewProd').click();
   assert.ok((await response).status()>=400,'duplicate creation reported success');assert.equal(await fs.readFile(brief,'utf8'),'# USER-OWNED CONTENT\n');
  });
  if(await page.locator('#btnCancelNewModal').isVisible())await page.locator('#btnCancelNewModal').click();
  await scenario('04b','Project folder traversal is rejected before creating anything outside productions',async()=>{
   await page.locator('#btnQuickNew').click();await page.locator('#newProdName').fill('Invalid folder');await page.locator('#newProdSlug').fill('../escaped');
   const response=page.waitForResponse(r=>new URL(r.url()).pathname==='/api/catalog/new'&&r.request().method()==='POST');await page.locator('#btnSubmitNewProd').click();assert.ok((await response).status()>=400);
   assert.equal(await fs.stat(path.join(root,'escaped')).then(()=>true,()=>false),false);await page.locator('#btnCancelNewModal').click();
  });
  await scenario('05','Multiple project conversations are isolated',async()=>{
   await create('UAT Beta');await send('BETA-ONLY');const request=requests.findLast(r=>r.messages.at(-1)?.content==='BETA-ONLY');assert.ok(!request.messages.some(m=>m.content?.includes('ALPHA-ONLY')));
   await choose('uat-alpha');await send('Recall my earlier note');const resumed=requests.findLast(r=>r.messages.at(-1)?.content==='Recall my earlier note');assert.ok(resumed.messages.some(m=>m.content?.includes('ALPHA-ONLY')));
  });
  await scenario('06','Application process restart restores project conversation',async()=>{
   await page.reload();await page.waitForFunction(()=>document.querySelector('#activityList').textContent.includes('ALPHA-ONLY'));
   await stop();await start();await page.goto(origin);await page.waitForFunction(()=>document.querySelector('#activityList').textContent.includes('ALPHA-ONLY'));
   await send('After app restart');assert.ok(requests.findLast(r=>r.messages.at(-1)?.content==='After app restart').messages.some(m=>m.content?.includes('ALPHA-ONLY')));
  });
  const outside=path.join(base,'existing folder with spaces');await fs.mkdir(outside);await fs.writeFile(path.join(outside,'brief.md'),'# Imported project\n');
  await scenario('07','Open an existing folder with spaces and preserve its content',async()=>{
   await page.locator('#btnQuickOpen').click();await page.locator('#openFolderPath').fill(outside);await page.locator('#btnSubmitOpenFolder').click();
   await page.waitForFunction(()=>document.querySelector('#modalOpenFolder').hidden&&document.querySelector('#projectSelect').value==='existing-folder-with-spaces'&&!document.querySelector('#promptInput').disabled);assert.equal(await fs.readFile(path.join(outside,'brief.md'),'utf8'),'# Imported project\n');
   await send('Existing folder request');
  });
  await scenario('08','Invalid folder errors are visible and recoverable',async()=>{
   await page.locator('#btnQuickOpen').click();await page.locator('#openFolderPath').fill(path.join(base,'does-not-exist'));
   const response=page.waitForResponse(r=>new URL(r.url()).pathname==='/api/catalog/open'&&r.request().method()==='POST');await page.locator('#btnSubmitOpenFolder').click();assert.ok((await response).status()>=400);
   await page.locator('#btnCancelOpenModal').click();
  });
  await choose('uat-alpha');
  const source=path.join(base,'source.mp4');execFileSync('ffmpeg',['-v','error','-f','lavfi','-i','testsrc2=size=320x180:rate=24','-t','2','-c:v','libx264','-pix_fmt','yuv420p',source]);
  await scenario('09','Import collision preserves source bytes and reports a useful error',async()=>{
   await page.locator('[data-nav=assets]').click();await page.getByLabel('Import source media').setInputFiles(source);await page.waitForFunction(()=>document.querySelector('#artifactsPanel').textContent.includes('source.mp4'));
   await page.getByLabel('Import source media').setInputFiles(source);await page.waitForFunction(()=>document.querySelector('#liveText').textContent.includes('already exists'));
   assert.deepEqual(await fs.readFile(path.join(alpha,'assets','source.mp4')),await fs.readFile(source));
  });
  await fs.mkdir(path.join(alpha,'renders'),{recursive:true});
  await fs.copyFile(source,path.join(alpha,'renders','final.mp4'));
  execFileSync('ffmpeg',['-v','error','-f','lavfi','-i','color=c=blue:size=320x180:rate=24','-t','1','-c:v','libx264','-pix_fmt','yuv420p',path.join(alpha,'renders','alternate.mp4')]);
  await scenario('09b','Selected alternate output remains selected across automatic refresh',async()=>{
   await page.waitForFunction(()=>document.querySelector('#masterVideo').readyState>=2);
   await page.reload();await choose('uat-alpha');await page.locator('[data-nav=assets]').click();
   await page.getByRole('link').filter({hasText:'alternate.mp4'}).first().click();
   await page.waitForFunction(()=>document.querySelector('#masterVideo').currentSrc.includes('alternate.mp4'));
   await page.waitForTimeout(3800);
   assert.ok((await page.locator('#masterVideo').evaluate(v=>v.currentSrc)).includes('alternate.mp4'),'refresh silently switched output');
  });
  await scenario('09c','Review uses selected output and mismatched expectations produce a failed check',async()=>{
   await page.locator('[data-nav=review]').click();for(const [id,value]of Object.entries({reviewWidth:'640',reviewHeight:'360',reviewFPS:'24',reviewDuration:'1'}))await page.locator('#'+id).fill(value);
   await page.locator('#runReview').click();await page.waitForFunction(()=>!document.querySelector('#runReview').disabled);
   const review=JSON.parse(await fs.readFile(path.join(alpha,'review','report.json'),'utf8'));
   assert.ok(JSON.stringify(review).includes('alternate.mp4'),'review targeted a different output');
   assert.ok(review.result.gates.some(g=>g.status==='fail'),'mismatched profile reported passing');
  });
  await scenario('09d','Acceptance records the selected file, not the default output',async()=>{
   await page.locator('#acceptOutput').click();assert.ok((await page.locator('#reviewFeedback').innerText()).includes('confirm your decision'));
   await page.locator('#confirmReview').check();await page.locator('#acceptOutput').click();await page.waitForFunction(()=>document.querySelector('#reviewFeedback').textContent.includes('exact file digest'));
   const approval=JSON.parse(await fs.readFile(path.join(alpha,'review','acceptance.json'),'utf8'));
   assert.equal(approval.sha256,crypto.createHash('sha256').update(await fs.readFile(path.join(alpha,'renders','alternate.mp4'))).digest('hex'));
  });
  await scenario('09e','Review report identifies its source when preview shows a different output',async()=>{
   await page.locator('[data-nav=assets]').click();await page.getByRole('link').filter({hasText:'final.mp4'}).first().click();await page.locator('[data-nav=review]').click();
   assert.ok((await page.locator('#propertiesPanel').innerText()).includes('different output than the current preview'));
  });
  await page.locator('[data-nav=assets]').click();
  const document=path.join(alpha,'artifacts','edit.json');await fs.writeFile(document,'{"note":"original"}');
  await scenario('10','Invalid JSON and stale edits are rejected; original content survives',async()=>{
   await page.waitForFunction(()=>document.querySelector('#artifactsPanel').textContent.includes('edit.json'));
   await page.getByRole('link').filter({hasText:'edit.json'}).first().click();await page.getByRole('button',{name:'Edit document',exact:true}).click();
   await page.getByLabel('Production document content').fill('{invalid');await page.getByRole('button',{name:'Save document',exact:true}).click();await page.waitForFunction(()=>document.querySelector('#artifactPreviewContent').textContent.includes('not valid'));
   assert.equal(await fs.readFile(document,'utf8'),'{"note":"original"}');await fs.writeFile(document,'{"note":"external change"}');
   await page.getByLabel('Production document content').fill('{"note":"my edit"}');await page.getByRole('button',{name:'Save document',exact:true}).click();await page.waitForFunction(()=>document.querySelector('#artifactPreviewContent').textContent.includes('changed since'));
   assert.equal(await fs.readFile(document,'utf8'),'{"note":"external change"}');
  });
  await scenario('11','Unsaved edits warn before another material replaces the editor',async()=>{
   const before=report.dialogs.length;await page.getByRole('link').filter({hasText:'brief.md'}).first().click();assert.ok(report.dialogs.length>before,'unsaved edits discarded without a warning');
  });
  await scenario('12','Modal keyboard focus stays inside, Escape restores the opener',async()=>{
   await page.locator('#btnQuickNew').click();await page.locator('#btnSubmitNewProd').focus();await page.keyboard.press('Tab');
   assert.equal(await page.evaluate(()=>document.querySelector('#modalNewProduction').contains(document.activeElement)),true,'Tab escaped modal');
   await page.keyboard.press('Escape');assert.equal(await page.evaluate(()=>document.activeElement.id),'btnQuickNew');
  });
  await page.keyboard.press('Escape');
  // Discard the deliberately stale editor before testing navigation/reload.
  page.removeAllListeners('dialog');page.on('dialog',async d=>{report.dialogs.push(d.message());await d.accept();});
  await page.getByRole('link').filter({hasText:'brief.md'}).first().click();
  await scenario('13','Theme persists after reload',async()=>{await page.locator('#appearanceToggle').click();const theme=await page.locator('html').getAttribute('data-theme');await page.reload();assert.equal(await page.locator('html').getAttribute('data-theme'),theme);});
  await scenario('14','All navigation is reachable at desktop, tablet and phone widths',async()=>{
   for(const width of [1440,1024,820,390,320]){await page.setViewportSize({width,height:900});for(const target of ['projects','settings','produce','assets','review']){if(await page.locator('#inspectorClose').isVisible())await page.locator('#inspectorClose').click();await page.locator(`[data-nav=${target}]`).click();}
    assert.equal(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth),true,`overflow at ${width}`);await shot(`layout-${width}`);
    if(await page.locator('#inspectorClose').isVisible())await page.locator('#inspectorClose').click();
    await page.locator('[data-nav=settings]').click();await shot(`settings-${width}`);
   }
  });
  await page.setViewportSize({width:1440,height:1000});
  await scenario('15','No uncaught browser errors during acceptance scenarios',async()=>assert.deepEqual(report.pageErrors,[]));
  report.notTested=['macOS/Linux execution and Safari/Firefox','Paid cloud generation and provider-specific OAuth sign-in','Every capability pack, long-form production and external CLI/Studio host acceptance','Subjective speech/music quality: audio fixture is a measured reference tone'];
 }catch(error){report.harnessError=error.stack;}
 finally{
  if(context){await context.tracing.stop({path:path.join(base,'trace.zip')}).catch(()=>{});await context.close();}
  if(browser)await browser.close();await stop();model.closeAllConnections();model.close();
  report.finished=new Date().toISOString();report.status=report.harnessError||report.cases.some(c=>c.result==='fail')?'failed':'passed';
  await fs.writeFile(path.join(base,'report.json'),JSON.stringify(report,null,2));await fs.writeFile(path.join(base,'server.log'),log);
  console.log(JSON.stringify(report,null,2));if(report.status!=='passed')process.exitCode=1;
 }
})();
