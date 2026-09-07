import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { execFile } from 'node:child_process';
import { promisify } from 'node:util';
import { atomicJSON, runWithCheckpoints } from './uat-checkpoint.mjs';

const exec = promisify(execFile);
export const exportNames = {
  installation: ['checks.json', 'install-build.log', 'proof-tests.log', 'renderer-fixture.mp4', 'renderer-props.json', 'renderer.log', 'facet-compose-fixture.mp4', 'facet-compose-props.json', 'facet-compose.log'],
  browser: ['progress.json', 'checkpoint.json', 'result.json', 'console.json', 'transcript.json', 'trace.zip', '01-created.png', '02-turn.png', '03-turn.png', 'turn-1-playback.png', 'turn-2-playback.png', 'failure.png', 'failed-candidate.mp4', 'turn-1.mp4', 'turn-2.mp4', 'turn-1-frame.png', 'turn-2-frame.png', 'turn-1-download.mp4', 'turn-2-download.mp4']
};

export async function runnerDiagnostics(track, result, evidence, capture, wallNow = Date.now) {
  if (!result.timed_out && result.exit_code === 0) return;
  const diagnostic = { track, adjudication: false, observed_at: new Date(wallNow()).toISOString(), timed_out: result.timed_out, timeout_ms: result.timeout_ms, timeout_elapsed_ms: result.timeout_elapsed_ms };
  try {
    diagnostic.process_inventory = JSON.parse(await capture());
  } catch { diagnostic.process_inventory = { status: 'unavailable', reason: 'bounded-container-capture-failed', timeoutMs: 30000, processes: [] }; }
  atomicJSON(path.join(evidence, `${track}-runner-diagnostics.json`), diagnostic);
}

export async function probe() {
  // Harness 1.2.0 passes cwd, not RUN_ID/evidenceDir, to custom child commands.
  const source = process.cwd();
  const runId = process.env.UAT_RUN_ID || path.basename(path.dirname(source));
  if (!/^[a-zA-Z0-9_-]+$/.test(runId)) throw new Error('Unsafe UAT run identifier');
  const evidence = process.env.UAT_EVIDENCE_PATH || path.resolve(source, '../../../runs', runId, 'evidence');
  fs.mkdirSync(evidence, { recursive: true });
  const docker = async args => (await exec('docker', args, { encoding: 'utf8', timeout: 30000, maxBuffer: 1024 * 1024 })).stdout;
  const ids = (await docker(['ps', '--filter', `label=com.xibodev.release-harness.run-id=${runId}`, '--filter', 'label=com.docker.compose.service=studio', '--format', '{{.ID}}'])).trim().split(/\s+/).filter(Boolean);
  if (ids.length !== 1) throw new Error(`Expected exactly one running Studio container for this run, found ${ids.length}`);
  const container = ids[0];
  const networkNames = Object.keys(JSON.parse(await docker(['inspect', '--format', '{{json .NetworkSettings.Networks}}', container])));
  const networks = await Promise.all(networkNames.map(async name => ({ name, internal: (await docker(['network', 'inspect', '--format', '{{.Internal}}', name])).trim() === 'true' })));
  const effectiveNetwork = { studio_networks: networks, studio_internal_only: networks.length > 0 && networks.every(n => n.internal) };
  atomicJSON(path.join(evidence, 'effective-network.json'), effectiveNetwork);
  if (process.env.UAT_CERTIFY === '1' && !effectiveNetwork.studio_internal_only) throw new Error('CERTIFICATION_REFUSED: observed Studio network permits open egress');
  const image = (await docker(['inspect', '--format', '{{.Image}}', container])).trim();
  const results = [], copied = new Map(), started = Date.now();
  const progress = track => {
    const state = { run_id: runId, image_id: image, status: 'running', final: false, adjudication: false, track, elapsed_ms: Date.now() - started, updated_at: new Date().toISOString(), results };
    atomicJSON(path.join(evidence, 'docker-uat.partial.json'), state);
    console.log(JSON.stringify(state));
  };
  async function exportTrack(track, complete = false) {
    // One metadata query, no per-file existence subprocesses or repeated media
    // copies. During execution only atomic browser checkpoints are publishable.
    const names = complete ? exportNames[track] : track === 'browser' ? ['progress.json', 'checkpoint.json'] : [];
    if (!names.length) return;
    const directory = `/home/facet/uat-evidence/${track}`;
    const listing = `const fs=require('node:fs');const [dir,names]=process.argv.slice(1);let allowed=JSON.parse(names);if(dir.endsWith('/browser')){let final=false;try{const r=JSON.parse(fs.readFileSync(dir+'/result.json','utf8'));final=r.final===true&&r.status==='complete'}catch{}if(!final)allowed=allowed.filter(n=>n==='progress.json'||n==='checkpoint.json')}console.log(JSON.stringify(allowed.flatMap(name=>{try{const s=fs.lstatSync(dir+'/'+name);return s.isFile()?[{name,signature:s.size+':'+s.mtimeMs}]:[]}catch(e){if(e.code==='ENOENT')return [];throw e}})))`;
    const files = JSON.parse(await docker(['exec', container, 'node', '-e', listing, directory, JSON.stringify(names)]));
    const target = path.join(evidence, track);
    fs.mkdirSync(target, { recursive: true });
    for (const { name, signature } of files) {
      if (!names.includes(name)) throw new Error('Unexpected export path');
      const key = `${track}/${name}`;
      if (copied.get(key) === signature) continue;
      const destination = path.join(target, name);
      await docker(['cp', `${container}:${directory}/${name}`, `${destination}.copying`]);
      fs.renameSync(`${destination}.copying`, destination);
      copied.set(key, signature);
    }
  }
  for (const [track, script] of [['installation', 'uat-installation.mjs'], ['browser', 'uat-browser.mjs']]) {
    progress(track);
    const result = await runWithCheckpoints('docker', ['exec', container, 'node', `/opt/uat/${script}`, `/home/facet/uat-evidence/${track}`], {
      timeout: track === 'browser' ? 1350000 : 600000,
      checkpoint: async () => { progress(track); await exportTrack(track); }
    });
    results.push({ track, ...result });
    // Killing host docker exec does not stop the container worker. Capture in
    // that namespace, as its configured user, before returning to core sealing.
    await runnerDiagnostics(track, result, evidence, () => docker(['exec', container, 'node', '/opt/uat/uat-checkpoint-process.mjs']));
    // Export installation before starting browser, and final browser artifacts
    // only after the child has closed AND its final result is published. Killing
    // docker exec does not prove its container process stopped writing files.
    await exportTrack(track, true);
    progress(`${track}-exported`);
  }
  atomicJSON(path.join(evidence, 'docker-uat.json'), { run_id: runId, image_id: image, effective_network: effectiveNetwork, status: 'complete', final: true, results });
  console.log(JSON.stringify({ run_id: runId, results }));
  if (results.some(r => r.exit_code !== 0 || r.checkpoint_failed)) process.exitCode = 1;
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  probe().catch(() => { console.error('UAT probe failed; partial evidence is not adjudication'); process.exitCode = 1; });
}
