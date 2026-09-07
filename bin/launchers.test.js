const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const { test } = require('node:test');

for (const name of ['facet', 'facet-ui']) {
  for (const platform of ['linux', 'darwin', 'win32']) {
    const paths = platform === 'win32' ? path.win32 : path.posix;
    const home = platform === 'win32' ? 'C:\\Users\\Test User' : '/home/test';
    const packageBin = paths.join(home, 'package', 'bin');
    const filename = platform === 'win32' ? `${name}.exe` : name;
    const native = paths.join(home, '.facet', 'bin', filename);
    const generic = paths.join(packageBin, filename);
    const bundled = paths.join(packageBin, platform === 'win32' ? `${name}-windows-amd64.exe` : `${name}-${platform}-x64`);
    const explicit = paths.join(home, 'custom install', filename);
    const localAppData = paths.join(home, 'relocated-local');
    const legacy = paths.join(localAppData, 'Programs', 'Facet', 'bin', filename);
    const fallbackLegacy = paths.join(home, 'AppData', 'Local', 'Programs', 'Facet', 'bin', filename);
    const cases = [
      { label: 'fails without PATH recursion', files: [] },
      { label: 'uses native user install', files: [native], expected: native },
      { label: 'preserves generic package precedence', files: [native, generic], expected: generic },
      { label: 'preserves platform package precedence', files: [native, generic, bundled], expected: bundled },
      { label: 'uses existing explicit executable first', files: [explicit, bundled, native], env: { FACET_BIN: explicit }, expected: explicit },
      { label: 'ignores missing explicit executable', files: [native], env: { FACET_BIN: explicit }, expected: native },
      { label: 'missing explicit and native fails safely', files: [], env: { FACET_BIN: explicit } },
    ];
    if (platform === 'win32') cases.push(
      { label: 'prefers current home install over stale legacy', files: [native, legacy], env: { LOCALAPPDATA: localAppData }, expected: native },
      { label: 'keeps legacy-only install working', files: [legacy], env: { LOCALAPPDATA: localAppData }, expected: legacy },
      { label: 'keeps legacy fallback without LOCALAPPDATA', files: [fallbackLegacy], expected: fallbackLegacy },
    );
    for (const fixture of cases) {
      test(`${name} ${platform}: ${fixture.label}`, () => {
        const calls = [];
        const errors = [];
        const exit = {};
        const context = {
          __dirname: packageBin,
          console: { error: message => errors.push(message) },
          process: {
            argv: ['node', `${name}-cli.js`, 'version'],
            env: fixture.env || {},
            exit(code) { assert.equal(code, 1); throw exit; }
          },
          require(module) {
            if (module === 'path') return paths;
            if (module === 'os') return { platform: () => platform, arch: () => 'x64', homedir: () => home };
            if (module === 'fs') return { existsSync: candidate => fixture.files.includes(candidate) };
            if (module === 'child_process') return { spawn(...args) { calls.push(args); return { on() {} }; } };
            throw new Error(`Unexpected module ${module}`);
          }
        };
        const run = () => vm.runInNewContext(fs.readFileSync(path.join(__dirname, `${name}-cli.js`), 'utf8'), context);
        if (fixture.expected) {
          run();
          assert.equal(calls.length, 1);
          assert.equal(calls[0][0], fixture.expected);
          assert.equal(calls[0][1][0], 'version');
        } else {
          assert.throws(run, error => error === exit);
          assert.equal(calls.length, 0);
          assert.match(errors[0], /native binary is missing/);
        }
      });
    }
  }
}
