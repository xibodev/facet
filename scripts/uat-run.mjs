import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { spawnSync } from 'node:child_process';
import { validateModelConfig } from './uat-config.mjs';
import { runWithCheckpoints } from './uat-checkpoint.mjs';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
export function configureRun(args, inherited = process.env) {
  const certify = args.includes('--certify');
  const freeModel = args.includes('--free-model');
  if (certify && (freeModel || inherited.UAT_NETWORK_INTERNAL === 'false')) throw new Error('CERTIFICATION_REFUSED: open egress cannot certify the sealed network policy');
  const positional = args.filter(arg => !['--certify', '--free-model'].includes(arg));
  if (positional.length !== 1 || positional[0].startsWith('--') || ['--certify', '--free-model'].some(flag => args.filter(arg => arg === flag).length > 1)) throw new Error('Usage: node scripts/uat-run.mjs <external-evidence-root> [--certify | --free-model]');
  if (freeModel && inherited.UAT_OPENCODE_CONFIG_FILE) throw new Error('--free-model cannot be combined with UAT_OPENCODE_CONFIG_FILE; unset it to select the committed credential-free configuration');
  const evidence = path.resolve(positional[0]);
  const relative = path.relative(root, evidence);
  if (!relative.startsWith(`..${path.sep}`) && !path.isAbsolute(relative)) throw new Error('Supply an evidence root outside the source repository');
  // Resolve existing ancestors as well, so an external symlink cannot point into source.
  let ancestor = evidence;
  while (!fs.existsSync(ancestor)) ancestor = path.dirname(ancestor);
  const realRelative = path.relative(fs.realpathSync(root), fs.realpathSync(ancestor));
  if (!realRelative.startsWith(`..${path.sep}`) && !path.isAbsolute(realRelative)) throw new Error('Supply an evidence root outside the source repository');
  const env = { ...inherited };
  // Compose uses only an explicit model config mount. Also keep host-side probes
  // from inheriting unrelated credentials and user-wide Compose overrides.
  for (const key of Object.keys(env)) {
    if (/TOKEN|SECRET|PASSWORD|API_KEY|ACCESS_KEY|AUTHORIZATION/i.test(key) || /^(OPENCODE_|COMPOSE_|UAT_)/.test(key)) delete env[key];
  }
  env.UAT_PORT = '31000';
  env.UAT_APP_URL = 'http://127.0.0.1:31000';
  env.UAT_REAL_AGENT = '0';
  env.UAT_NETWORK_INTERNAL = 'true';
  env.UAT_CERTIFY = certify ? '1' : '0';
  if (freeModel) {
    env.UAT_REAL_AGENT = '1';
    // Relative to the detached Compose source, never the host checkout or HOME.
    env.UAT_OPENCODE_CONFIG_FILE = './.release-harness/docker/opencode-free.json';
    env.UAT_NETWORK_INTERNAL = 'false';
    env.UAT_TURN_TIMEOUT_MS = '600000';
    validateModelConfig(JSON.parse(fs.readFileSync(path.join(root, env.UAT_OPENCODE_CONFIG_FILE), 'utf8')));
  } else if (inherited.UAT_REAL_AGENT === '1') {
    const config = inherited.UAT_OPENCODE_CONFIG_FILE;
    if (!config || !path.isAbsolute(config) || !fs.statSync(config).isFile()) throw new Error('Real acceptance needs an explicit absolute UAT_OPENCODE_CONFIG_FILE');
    const rel = path.relative(root, fs.realpathSync(config));
    if (!rel.startsWith(`..${path.sep}`) && !path.isAbsolute(rel)) throw new Error('Model configuration must remain outside the repository');
    const settings = JSON.parse(fs.readFileSync(config, 'utf8'));
    validateModelConfig(settings);
    env.UAT_REAL_AGENT = '1';
    env.UAT_OPENCODE_CONFIG_FILE = config;
    // Network access is a second explicit opt-in, never inferred from credentials.
    env.UAT_NETWORK_INTERNAL = inherited.UAT_NETWORK_INTERNAL === 'false' ? 'false' : 'true';
  }
  return { evidence, env, certify, freeModel };
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  const { evidence, env, certify, freeModel } = configureRun(process.argv.slice(2));
  const runId = `facet-uat-${Date.now()}`;
  fs.mkdirSync(evidence, { recursive: true });
  console.log(`UAT mode: ${certify ? 'CERTIFICATION (clean source required)' : 'DEVELOPMENT (--allow-dirty)'}`);
  console.log(`Model mode: ${freeModel ? 'FREE HOSTED opencode/big-pickle (open egress, no credentials)' : env.UAT_REAL_AGENT === '1' ? 'EXPLICIT EXTERNAL CONFIG' : 'OFFLINE (real-agent acceptance unconfigured)'}`);
  const started = Date.now();
  console.log('UAT budgets: setup target <=25m including installation; two turns <=10m each; checks/export reserve 5m; runner 50m; external supervisor >=55m (not 25m).');
  const result = await runWithCheckpoints(process.execPath, [path.join(root, 'node_modules/@xibodev/release-harness-core/bin/release-harness.js'), 'run-local', ...(certify ? [] : ['--allow-dirty']), '--run-id', runId, '--evidence-dir', evidence, '--port-offset', '0'], {
    cwd: root, env, stdio: 'inherit', timeout: 3000000,
    checkpoint: () => console.log(JSON.stringify({ run_id: runId, marker: 'launcher-progress-not-adjudication', elapsed_ms: Date.now() - started, updated_at: new Date().toISOString() }))
  });
  console.log(`Run evidence: ${path.join(evidence, 'runs', runId)}`);
  // Core 1.2.0 can skip teardown after a partial up failure. Clean only this run.
  const cleanup = spawnSync('docker', ['compose', '-p', `rh-${runId}`, '-f', path.join(root, 'docker-compose.test.yml'), 'down', '-v', '--remove-orphans'], { cwd: root, env: { ...env, RUN_ID: runId }, stdio: 'inherit', timeout: 60000 });
  if (result.error) console.error('Harness process failed or exceeded its overall timeout');
  process.exitCode = result.exit_code ?? 3;
  if (cleanup.status !== 0 && process.exitCode === 0) process.exitCode = 3;
}
