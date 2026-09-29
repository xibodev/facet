// Build a Windows archive from tracked product files only. Run from a clean
// checkout; local projects, credentials, dependencies and UAT recordings never
// enter the distribution. Renderer dependencies are installed by the user.
const fs=require('node:fs/promises');
const path=require('node:path');
const {execFileSync}=require('node:child_process');
const {createHash}=require('node:crypto');

(async()=>{
 const repo=path.resolve(__dirname,'..');
 const version=JSON.parse(await fs.readFile(path.join(repo,'package.json'),'utf8')).version;
 const out=path.resolve(process.argv[2]||path.join(repo,'build','release'));
 const stage=path.join(out,`facet-${version}-windows-amd64`);
 await fs.rm(stage,{recursive:true,force:true});
 await fs.mkdir(path.join(stage,'bin'),{recursive:true});
 const env={...process.env,GOOS:'windows',GOARCH:'amd64',CGO_ENABLED:'0'};
 for(const command of ['facet','facet-ui','facet-module'])execFileSync('go',['build','-trimpath','-o',path.join(stage,'bin',`${command}.exe`),`./cmd/${command}`],{cwd:repo,env,stdio:'inherit'});
 const tracked=execFileSync('git',['ls-files','-z','--cached','--others','--exclude-standard'],{cwd:repo,encoding:'utf8'}).split('\0').filter(Boolean);
 for(const file of tracked){
  if(!/^(skills|packs|agents|schemas)\//.test(file)&&!/^remotion-composer\/(src\/|package(?:-lock)?\.json$|tsconfig\.json$|legacy-composer-manifest\.json$)/.test(file))continue;
  if(/(^|\/)(node_modules|\.env[^/]*|\.git)(\/|$)/.test(file))throw Error(`Unsafe package entry: ${file}`);
  try{await fs.access(path.join(repo,file));}catch(error){if(error.code==='ENOENT')continue;throw error;}
  const target=path.join(stage,'bundle',file);await fs.mkdir(path.dirname(target),{recursive:true});await fs.copyFile(path.join(repo,file),target);
 }
 for(const file of ['LICENSE','THIRD_PARTY_NOTICES.md'])await fs.copyFile(path.join(repo,file),path.join(stage,file));
 await fs.writeFile(path.join(stage,'START-HERE.txt'),`Facet ${version} — Windows amd64\n\nRun bin\\facet.exe ui to open Facet in your browser.\nSettings > Assistant models > Discover free models uses the embedded Studio kernel's provider registry. Availability depends on upstream services.\n\nInstall FFmpeg/FFprobe and Node.js for media production. For Remotion compositions, run in bundle\\remotion-composer:\n  npm ci\n  npx remotion browser ensure\n\nCLI integration: use bin\\facet.exe init <project> --engine copilot --no-launch (or another supported target).\nStudio module: bin\\facet-module.exe exposes the module protocol; its bundled content is in bundle.\n\nThis release includes Windows/Chrome standalone UAT. Other operating systems, every capability pack, paid generation, OAuth sign-in and external host journeys are not certified by those results.\n`);
 const archive=path.join(out,`facet-${version}-windows-amd64.zip`);
 execFileSync('tar',['-a','-c','-f',archive,'-C',stage,'.'],{stdio:'inherit'});
 const sum=createHash('sha256').update(await fs.readFile(archive)).digest('hex');
 await fs.writeFile(path.join(out,'SHA256SUMS.txt'),`${sum}  ${path.basename(archive)}\n`);
 console.log(JSON.stringify({version,stage,archive,sha256:sum}));
})().catch(error=>{console.error(error);process.exit(1)});
