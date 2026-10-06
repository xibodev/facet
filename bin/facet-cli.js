#!/usr/bin/env node
'use strict';

// npm supplies this launcher, not the Facet runtime. It runs an existing native
// `facet` binary and never installs one. It never resolves `facet` through PATH
// (that would find this launcher again) and never runs another platform's binary.
const { spawn } = require('child_process');
const path = require('path');
const os = require('os');
const fs = require('fs');

const GOOS = { win32: 'windows', darwin: 'darwin', linux: 'linux' };
const GOARCH = { x64: 'amd64', arm64: 'arm64' };

const MISSING = [
  'Facet native binary not found. The npm package is only a launcher; install the Facet runtime once per user:',
  '  macOS/Linux: curl -fsSL https://xibodev.github.io/facet/install.sh | sh',
  '  Windows:     irm https://xibodev.github.io/facet/install.ps1 | iex',
  'Or set FACET_BIN to an existing facet executable.',
].join('\n');

// Search order: an explicit FACET_BIN, the active user runtime installed by the
// Facet installer, then a binary for exactly this platform shipped beside this file.
function candidates() {
  const platform = os.platform();
  const exe = platform === 'win32' ? '.exe' : '';
  const list = [];
  if (process.env.FACET_BIN) list.push(process.env.FACET_BIN);
  let home = '';
  try { home = os.homedir(); } catch { /* No resolvable home directory. */ }
  if (home) list.push(path.join(home, '.facet', 'current', 'bin', `facet${exe}`));
  const goos = GOOS[platform];
  const goarch = GOARCH[os.arch()];
  if (goos && goarch) list.push(path.join(__dirname, `facet-${goos}-${goarch}${exe}`));
  return list;
}

const bin = candidates().find(candidate => fs.existsSync(candidate));

if (!bin) {
  console.error(MISSING);
  process.exit(1);
}

const child = spawn(bin, process.argv.slice(2), { stdio: 'inherit' });

child.on('error', (err) => {
  console.error(`Failed to execute the facet binary (${bin}): ${err.message}`);
  process.exit(1);
});

child.on('exit', (code, signal) => {
  if (signal) {
    process.kill(process.pid, signal);
  } else {
    process.exit(code ?? 0);
  }
});
