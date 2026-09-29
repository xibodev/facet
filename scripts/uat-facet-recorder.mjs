import fs from 'node:fs';
import path from 'node:path';
import { createHash, randomUUID } from 'node:crypto';
import { spawn } from 'node:child_process';
import { sanitizer } from './uat-sanitize.mjs';

export const realFacet = '/home/facet/.facet/bin/facet';
export const receiptRoot = '/home/facet/uat-receipts';
export const digest = bytes => createHash('sha256').update(bytes).digest('hex');
export const maxCapture = 2 * 1024 * 1024;

// Never follow a receipt-supplied path or symlink. Bounds apply before reading.
export function readReceiptFile(file, max = 16 * 1024 * 1024) {
  const fd = fs.openSync(file, fs.constants.O_RDONLY | (fs.constants.O_NOFOLLOW || 0));
  try {
    const stat = fs.fstatSync(fd);
    if (!stat.isFile() || stat.size > max || fs.lstatSync(file).isSymbolicLink()) throw new Error('Unsafe receipt file');
    return fs.readFileSync(fd);
  } finally { fs.closeSync(fd); }
}

export function activeReceiptDirectory(root = receiptRoot) {
  if (!fs.existsSync(path.join(root, 'active.json'))) return null;
  const active = JSON.parse(readReceiptFile(path.join(root, 'active.json'), 4096));
  if (!/^[a-f0-9-]{36}$/.test(active.turnId)) throw new Error('Invalid receipt turn');
  const directory = path.join(root, active.turnId);
  if (fs.realpathSync(root) !== path.resolve(root) || fs.realpathSync(directory) !== path.resolve(directory)) throw new Error('Unsafe receipt directory');
  return directory;
}

// Injection here is for explicitly synthetic forwarding integration tests only.
// The executable and receipt root in the installed entry point are fixed literals.
export async function forward(executable, argv, directory, { stdout = process.stdout, stderr = process.stderr, limit = maxCapture } = {}) {
  const id = randomUUID();
  const startedAt = Date.now();
  let receipt, pending;
  if (directory) {
    if (Buffer.byteLength(JSON.stringify(argv)) > 65536) throw new Error('Receipt argv limit');
    if (fs.readdirSync(directory).length >= 256) throw new Error('Receipt count limit');
    pending = path.join(directory, `${id}.pending`);
    fs.writeFileSync(pending, '', { flag: 'wx', mode: 0o600 });
    receipt = { version: 1, kind: executable === realFacet ? 'instrumented-real-facet' : 'synthetic-forwarding-fixture', id, startedAt, cwd: process.cwd(), argv, executable, executableSha256: digest(fs.readFileSync(executable)) };
  }
  const child = spawn(executable, argv, { cwd: process.cwd(), env: process.env, stdio: ['inherit', 'pipe', 'pipe'] });
  let failed = false, overflow = false;
  const captures = [[], []], sizes = [0, 0];
  const handlers = new Map();
  for (const signal of ['SIGINT', 'SIGTERM', 'SIGHUP', 'SIGQUIT']) {
    const handler = () => child.kill(signal);
    handlers.set(signal, handler);
    process.on(signal, handler);
  }
  const outputError = error => { failed = true; child.kill(error.code === 'EPIPE' ? 'SIGPIPE' : 'SIGTERM'); };
  stdout.on('error', outputError);
  stderr.on('error', outputError);
  for (const [index, source, target] of [[0, child.stdout, stdout], [1, child.stderr, stderr]]) {
    source.on('data', chunk => {
      if (receipt) {
        sizes[index] += chunk.length;
        if (sizes[index] <= limit) captures[index].push(chunk);
        else overflow = true;
      }
      if (!target.write(chunk)) { source.pause(); target.once('drain', () => source.resume()); }
    });
  }
  child.on('error', () => { failed = true; });
  const result = await new Promise(resolve => child.on('close', (code, signal) => resolve({ code, signal })));
  for (const [signal, handler] of handlers) process.removeListener(signal, handler);
  stdout.removeListener('error', outputError);
  stderr.removeListener('error', outputError);
  if (receipt) {
    const safe = sanitizer();
    for (const [key, value] of Object.entries(process.env)) if (/key|secret|token|password|authorization/i.test(key)) safe.add(value);
    Object.assign(receipt, { completedAt: Date.now(), exit: result.code, signal: result.signal, complete: !failed && !overflow,
      // Never persist a truncated secret prefix when the bounded capture overflows.
      stdout: overflow ? '' : Buffer.concat(captures[0]).toString('utf8'), stderr: overflow ? '' : Buffer.concat(captures[1]).toString('utf8'), bytes: sizes });
    receipt.executableUnchanged = digest(fs.readFileSync(executable)) === receipt.executableSha256;
    const encoded = JSON.stringify(receipt);
    // Preserve child bytes on the pipes; only persisted diagnostic content is redacted.
    const redacted = safe.redact(encoded);
    fs.writeFileSync(pending, redacted, { mode: 0o600 });
    fs.renameSync(pending, path.join(directory, `${id}.json`));
  }
  return { code: failed || overflow ? 74 : result.code, signal: result.signal };
}
