#!/usr/bin/env node

const { spawn } = require('child_process');
const path = require('path');
const os = require('os');
const fs = require('fs');

function getPlatformBinary(name) {
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

  // 2. Generic name in bin/ (facet.exe / facet)
  const genericBin = path.join(__dirname, platform === 'win32' ? `${name}.exe` : name);
  if (fs.existsSync(genericBin)) {
    return genericBin;
  }

  // 3. User installation path
  const home = os.homedir();
  const userInstall = platform === 'win32'
    ? path.join(process.env.LOCALAPPDATA || path.join(home, 'AppData', 'Local'), 'Programs', 'Facet', 'bin', `${name}.exe`)
    : path.join(home, '.facet', 'bin', name);
  if (fs.existsSync(userInstall)) {
    return userInstall;
  }

  // 4. Fallback to system PATH
  return name;
}

const bin = getPlatformBinary('facet');
const args = process.argv.slice(2);

const child = spawn(bin, args, { stdio: 'inherit' });

child.on('error', (err) => {
  console.error(`Failed to execute facet binary (${bin}):`, err.message);
  process.exit(1);
});

child.on('exit', (code, signal) => {
  if (signal) {
    process.kill(process.pid, signal);
  } else {
    process.exit(code ?? 0);
  }
});
