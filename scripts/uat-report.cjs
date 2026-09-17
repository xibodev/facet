// Build an evidence index from completed runs; never computes release verdicts.
const fs=require('node:fs/promises'),path=require('node:path');
const {pathToFileURL}=require('node:url');
(async()=>{
 const [out,...dirs]=process.argv.slice(2);
 const runs=await Promise.all(dirs.map(async dir=>({dir,data:JSON.parse(await fs.readFile(path.join(dir,'report.json'),'utf8'))})));
 const esc=s=>String(s??'').replaceAll('&','&amp;').replaceAll('<','&lt;').replaceAll('>','&gt;').replaceAll('"','&quot;');
 const link=(file,label)=>`<a href="${pathToFileURL(file).href}">${esc(label)}</a>`;
 let html='<!doctype html><html lang="en"><meta charset="utf-8"><title>Facet standalone UAT</title><style>body{font:16px/1.6 system-ui;max-width:1100px;margin:40px auto;padding:20px;color:#222}table{border-collapse:collapse;width:100%}td,th{border:1px solid #ccc;padding:8px;text-align:left}code{overflow-wrap:anywhere}img{max-width:100%;border:1px solid #ccc}section{margin:36px 0}</style><h1>Facet standalone UAT</h1><p>Windows / Chrome. Tested through browser controls in isolated application homes. Fixture-based integration and live-model production are reported separately. This is not release certification or exhaustive acceptance of all providers and platforms.</p>';
 for(const {dir,data}of runs){
  html+=`<section><h2>${esc(data.scope||data.model||'Browser production run')}</h2><p>Recorded result: <strong>${esc(data.status)}</strong></p><p>Binary SHA-256: <code>${esc(data.sha256)}</code></p><p>${link(path.join(dir,'report.json'),'Full JSON evidence')}</p><table><tr><th>Scenario</th><th>Result</th></tr>`;
  for(const c of data.cases||data.checks?.map(title=>({title,result:data.status}))||[]){html+=`<tr><td>${esc(c.title)}</td><td>${esc(c.result)}</td></tr>`;}
  html+='</table>';
  if(data.composition)html+=`<p>Native composition: ${esc(JSON.stringify(data.composition))}. No audio stream; title frame inspected.</p>`;
  if(data.audio)html+=`<p>Audio mixing: ${esc(JSON.stringify(data.audio))}. Decoded reference-tone signal verified; subjective speech/music quality not tested.</p>`;
  for(const file of await fs.readdir(dir)){if(file.endsWith('.png'))html+=`<details><summary>${esc(file)}</summary><img src="${pathToFileURL(path.join(dir,file)).href}" alt="${esc(file)}"></details>`;else if(file==='trace.zip')html+=`<p>${link(path.join(dir,file),'Chrome trace')}</p>`;}
  for(const folder of ['video','browser-video']){try{for(const file of await fs.readdir(path.join(dir,folder)))html+=`<p>${link(path.join(dir,folder,file),'Browser recording')}</p>`;}catch{}}
  html+='</section>';
 }
 html+='<h2>Defects found and fixed during this UAT</h2><ul><li>Duplicate/traversing project folder creation</li><li>Unsaved document loss and modal keyboard focus escape</li><li>Missing rendered-output entries and preview selection reset</li><li>Review/acceptance targeting default rather than selected output</li><li>Review report source ambiguity</li><li>Unreadable empty-state heading, content-obscuring sticky header, overflowing file picker</li></ul><h2>Not verified</h2><ul><li>macOS/Linux and Safari/Firefox</li><li>Paid cloud generation and provider-specific OAuth</li><li>Every capability pack, long-form production, external CLI and full Studio host acceptance</li><li>Subjective narration/music quality and screen-reader audit</li><li>Race detector: CGO/compiler unavailable</li></ul><p>Passing these scenarios does not establish those untested capabilities.</p></html>';
 await fs.mkdir(out,{recursive:true});await fs.writeFile(path.join(out,'index.html'),html);await fs.writeFile(path.join(out,'runs.json'),JSON.stringify(runs.map(r=>({directory:r.dir,status:r.data.status,sha256:r.data.sha256})),null,2));console.log(path.join(out,'index.html'));
})().catch(e=>{console.error(e);process.exit(1)});
