// Opt-in live model acceptance. Uses the browser's settings and conversation
// controls in a disposable Facet home, never the operator's provider settings.
const {chromium}=require('playwright');
const fs=require('node:fs/promises'),path=require('node:path'),os=require('node:os');
const {spawn,execFileSync}=require('node:child_process');
const {createHash}=require('node:crypto');
(async()=>{
 const base=await fs.mkdtemp(path.join(os.tmpdir(),'opencode','facet-live-browser-'));
 const home=path.join(base,'home'),root=path.join(base,'workspace');await fs.mkdir(home);await fs.mkdir(path.join(root,'.facet'),{recursive:true});
 await fs.writeFile(path.join(root,'.facet','catalog.json'),JSON.stringify({version:'1.0',default_root:path.join(root,'productions'),projects:[]}));
 const app=spawn(path.resolve(process.argv[2]),['ui','--dir',root,'--port','18970','--no-open'],{env:{...process.env,FACET_HOME:home,LOCALAPPDATA:home,HOME:home,USERPROFILE:home},stdio:['ignore','pipe','pipe']});
 let logs='';app.stdout.on('data',d=>logs+=d);app.stderr.on('data',d=>logs+=d);
  const report={base,binary:path.resolve(process.argv[2]),sha256:createHash('sha256').update(await fs.readFile(path.resolve(process.argv[2]))).digest('hex'),attempts:[],errors:[]};let browser,context;
 try{
  for(let i=0;i<100&&!logs.includes('URL:');i++)await new Promise(r=>setTimeout(r,100));
  const origin=logs.match(/URL: (http:\/\/[^\s]+)/)?.[1];if(!origin)throw Error('App did not start');
   browser=await chromium.launch({channel:'chrome',headless:true});context=await browser.newContext({viewport:{width:1440,height:1000},recordVideo:{dir:path.join(base,'browser-video')}});await context.tracing.start({screenshots:true,snapshots:true,sources:false});const page=await context.newPage();page.setDefaultTimeout(45000);
  page.on('pageerror',e=>report.errors.push(e.message));await page.goto(origin);
  await page.locator('#btnModelSettings').click();await page.locator('#btnDiscoverModels').click();
  await page.waitForFunction(()=>!document.querySelector('#btnDiscoverModels').disabled);
  report.discovery=await page.locator('#modelFeedback').innerText();
  const choices=await page.locator('#modelSelect option').evaluateAll(options=>options.map(o=>o.value).filter(Boolean));
  // Bounded live smoke: one candidate per discovered instance, no vendor list.
  const seen=new Map();const candidates=choices.filter(id=>{const instance=id.split('/')[0];const count=seen.get(instance)||0;if(count>=2)return false;seen.set(instance,count+1);return true});
  if(!candidates.length)throw Error('Discovery produced no selectable models');
  for(const id of candidates){
   await page.locator('#modelSelect').selectOption(id);await page.locator('#btnTestModel').click();await page.waitForFunction(()=>!document.querySelector('#btnTestModel').disabled);
   const feedback=await page.locator('#modelFeedback').innerText();report.attempts.push({id,feedback});
   if(feedback.includes('Connected successfully')){report.selected=id;await page.locator('#btnSaveModel').click();await page.waitForFunction(()=>!document.querySelector('#btnSaveModel').disabled);break;}
  }
  if(!report.selected)throw Error('No discovered candidate passed a real model response check');
  await page.locator('#btnQuickNew').click();await page.locator('#newProdName').fill('Live native capability');await page.locator('#btnSubmitNewProd').click();
  await page.waitForFunction(()=>document.querySelector('#modalNewProduction').hidden&&!document.querySelector('#promptInput').disabled);
  await page.locator('#promptInput').fill('Create artifacts/captions.srt using your native subtitle_gen tool. Exactly one cue: text "Facet live capability check", start 0 seconds, end 2 seconds. No narration, music, paid services, or video generation. Then briefly confirm the actual file.');await page.locator('#sendButton').click();
  await page.waitForFunction(()=>!document.querySelector('#promptInput').disabled,null,{timeout:180000});
  report.conversation=await page.locator('#activityList').innerText();
  const file=path.join(root,'productions','live-native-capability','artifacts','captions.srt');
  const content=await fs.readFile(file,'utf8');if(!content.includes('Facet live capability check')||!content.includes('00:00:02,000'))throw Error('Caption content does not match requested cue');
  report.artifact=content;
  const source=path.join(base,'source.mp4');
  execFileSync('ffmpeg',['-hide_banner','-loglevel','error','-f','lavfi','-i','testsrc2=size=320x180:rate=24','-t','2','-c:v','libx264','-pix_fmt','yuv420p',source]);
  await page.locator('[data-nav=assets]').click();await page.getByLabel('Import source media').setInputFiles(source);
  await page.waitForFunction(()=>document.querySelector('#artifactsPanel').textContent.includes('source.mp4'));
  await page.locator('#promptInput').fill('Use your native video_trimmer tool to cut the supplied assets/source.mp4 from 0 to 1 second, re-encode with libx264, and write renders/final.mp4. Keep it silent. No paid services or generated footage. Inspect the result with media_probe and report its actual duration and size.');await page.locator('#sendButton').click();
  await page.waitForFunction(()=>!document.querySelector('#promptInput').disabled,null,{timeout:180000});
  await page.waitForFunction(()=>document.querySelector('#masterVideo').readyState>=2);
  await page.locator('#playButton').click();await page.waitForFunction(()=>document.querySelector('#masterVideo').currentTime>0.2);
  const video=path.join(root,'productions','live-native-capability','renders','final.mp4');
  const probe=JSON.parse(execFileSync('ffprobe',['-v','error','-show_format','-show_streams','-of','json',video],{encoding:'utf8'}));
  if(Math.abs(Number(probe.format.duration)-1)>0.1||probe.streams[0].width!==320)throw Error('Rendered media differs from request');
  execFileSync('ffmpeg',['-hide_banner','-loglevel','error','-i',video,'-frames:v','1',path.join(base,'frame.png')]);
  report.media={duration:probe.format.duration,width:probe.streams[0].width,streams:probe.streams.length};
  if(process.argv.includes('--composition')) {
   await page.locator('#promptInput').fill('Create a new silent 2-second title-card video at renders/title.mp4. First save this exact JSON as serialized text in artifacts/title_props.json: {"output":"renders/title.mp4","width":640,"height":360,"fps":24,"duration_seconds":2,"cuts":[{"id":"title","type":"hero_title","text":"FACET CREATES","subtitle":"A real native render","in_seconds":0,"out_seconds":2}]}. Then call native video_compose with only {"input_path":"artifacts/title_props.json"}, which loads saved props. No image generation, TTS, music, paid services or renderer substitutions. Inspect with media_probe and frame_sample, report actual profile.');await page.locator('#sendButton').click();
   await page.waitForFunction(()=>!document.querySelector('#promptInput').disabled,null,{timeout:240000});
   const title=path.join(root,'productions','live-native-capability','renders','title.mp4');
   const facts=JSON.parse(execFileSync('ffprobe',['-v','error','-show_format','-show_streams','-of','json',title],{encoding:'utf8'}));
   if(Math.abs(Number(facts.format.duration)-2)>0.1||facts.streams[0].width!==640||facts.streams[0].avg_frame_rate!=='24/1')throw Error('Title-card output does not match requested profile');
   if(facts.streams.some(s=>s.codec_type==='audio'))throw Error('Silent title card unexpectedly contains an audio track');
   const visible=await page.locator('#activityList').textContent();
   if(!visible.includes('remotion_render')||!visible.includes('"ok":true,"tool":"video_compose"'))throw Error('No successful native Remotion result');
   execFileSync('ffmpeg',['-hide_banner','-loglevel','error','-ss','1','-i',title,'-frames:v','1',path.join(base,'title-frame.png')]);
   report.composition={duration:facts.format.duration,width:facts.streams[0].width};
   await page.locator('[data-nav=assets]').click();
   await page.getByRole('link').filter({hasText:'title_props.json'}).first().click();
   await page.getByRole('button',{name:'Render saved composition',exact:true}).click();
   await page.waitForFunction(()=>document.querySelector('#artifactPreviewContent').textContent.includes('Rendered. Open the output'),null,{timeout:120000});
   report.directRender='Saved composition rendered through the artifact view without an assistant turn';
  }
  if(process.argv.includes('--audio')) {
   const tone=path.join(base,'reference-tone.wav');
   execFileSync('ffmpeg',['-v','error','-f','lavfi','-i','sine=frequency=440:duration=2:sample_rate=48000','-ac','2',tone]);
   await page.locator('[data-nav=assets]').click();await page.getByLabel('Import source media').setInputFiles(tone);
   await page.waitForFunction(()=>document.querySelector('#artifactsPanel').textContent.includes('reference-tone.wav'));
   await page.locator('#promptInput').fill('Mix my supplied assets/reference-tone.wav into the one-second renders/final.mp4, writing renders/with-audio.mp4. Use native audio_mix with video renders/final.mp4, music input assets/reference-tone.wav, music gain_db -6, duration video, output renders/with-audio.mp4, overwrite true. Do not generate audio or use any paid provider. Then inspect actual audio/video streams.');await page.locator('#sendButton').click();
   await page.waitForFunction(()=>!document.querySelector('#promptInput').disabled,null,{timeout:180000});
   const mixed=path.join(root,'productions','live-native-capability','renders','with-audio.mp4');
   const facts=JSON.parse(execFileSync('ffprobe',['-v','error','-show_format','-show_streams','-of','json',mixed],{encoding:'utf8'}));
   if(!facts.streams.some(s=>s.codec_type==='audio'&&s.codec_name==='aac')||Math.abs(Number(facts.format.duration)-1)>0.1)throw Error('Mixed media lacks expected audio or duration');
   const pcm=execFileSync('ffmpeg',['-v','error','-i',mixed,'-vn','-f','f32le','-ac','1','-ar','8000','pipe:1']);let energy=0;for(let i=0;i+4<=pcm.length;i+=4)energy+=pcm.readFloatLE(i)**2;
   const rms=Math.sqrt(energy/(pcm.length/4));if(rms<0.005)throw Error('Mixed audio is effectively silent');
   report.audio={duration:facts.format.duration,codec:'aac',decodedSamples:pcm.length/4,rms};
   await page.locator('[data-nav=assets]').click();await page.getByRole('link').filter({hasText:'with-audio.mp4'}).first().click();await page.waitForFunction(()=>document.querySelector('#masterVideo').readyState>=2);await page.locator('#playButton').click();await page.waitForFunction(()=>document.querySelector('#masterVideo').currentTime>0.2);
  }
  report.conversation=await page.locator('#activityList').innerText();
  await page.screenshot({path:path.join(base,'live.png'),fullPage:true});report.status='passed';
 }catch(error){report.status='failed';report.error=error.stack;process.exitCode=1;if(browser)await browser.contexts()[0].pages()[0].screenshot({path:path.join(base,'failure.png'),fullPage:true});}
  finally{if(context){await context.tracing.stop({path:path.join(base,'trace.zip')}).catch(()=>{});await context.close();}if(browser)await browser.close();app.kill();await fs.writeFile(path.join(base,'report.json'),JSON.stringify(report,null,2));await fs.writeFile(path.join(base,'server.log'),logs);console.log(JSON.stringify(report,null,2));}
})();
