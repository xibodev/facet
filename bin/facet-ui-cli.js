#!/usr/bin/env node

const { spawn } = require('child_process');
const path = require('path');
const os = require('os');
const fs = require('fs');

function getPlatformBinary(name) {
  if (process.env.FACET_BIN && fs.existsSync(process.env.FACET_BIN)) {
    return process.env.FACET_BIN;
  }

  const platform = os.platform(); // 'win32', 'darwin', 'linux'
  const arch = os.arch(); // 'x64', 'arm64'

  let targetArch = arch;
  if (arch === 'ia32') targetArch = '386';

  let binName = `${name}-${platform}-${targetArch}`;
  if (platform === 'win32') {
    binName = `${name}-windows-amd64.exe`;
  }

  // 1. Direct platform binary in bin/
  const platformBin = path.join(__dirname, binName);
  if (fs.existsSync(platformBin)) {
    return platformBin;
  }

  // 2. Generic name in bin/ (facet-ui.exe / facet-ui)
  const genericBin = path.join(__dirname, platform === 'win32' ? `${name}.exe` : name);
  if (fs.existsSync(genericBin)) {
    return genericBin;
  }

  // 3. User installation path
  const home = os.homedir();
  const userInstall = path.join(home, '.facet', 'bin', platform === 'win32' ? `${name}.exe` : name);
  if (fs.existsSync(userInstall)) {
    return userInstall;
  }
  if (platform === 'win32') {
    const legacyInstall = path.join(process.env.LOCALAPPDATA || path.join(home, 'AppData', 'Local'), 'Programs', 'Facet', 'bin', `${name}.exe`);
    if (fs.existsSync(legacyInstall)) return legacyInstall;
  }

  // Never resolve our own npm shim through PATH.
  return null;
}

const bin = getPlatformBinary('facet-ui');
const args = process.argv.slice(2);

if (!bin) {
  console.error('Facet Studio native binary is missing. npm supplies a launcher, not a binary installer. Clone https://github.com/xibodev/facet and run bash /path/to/facet/install.sh (Linux/macOS), or install.ps1 from the checkout (Windows).');
  process.exit(1);
}

const child = spawn(bin, args, { stdio: 'inherit' });

child.on('error', (err) => {
  console.error(`Failed to execute facet-ui binary (${bin}):`, err.message);
  process.exit(1);
});

child.on('exit', (code, signal) => {
  if (signal) {
    process.kill(process.pid, signal);
  } else {
    process.exit(code ?? 0);
  }
});
