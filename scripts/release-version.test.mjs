import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { test } from 'node:test';

const root = new URL('../', import.meta.url);
const read = (path) => readFileSync(new URL(path, root), 'utf8');

test('shipping version surfaces identify Facet v1.1.0', () => {
  const exact = {
    'package.json': /"version": "1\.1\.0"/,
    'package-lock.json': /"version": "1\.1\.0"/,
    'cmd/facet/main.go': /const Version = "1\.1\.0"/,
    'cmd/facet-ui/main.go': /const Version = "1\.1\.0"/,
    'cmd/facet-module/main.go': /module\.Describe\("1\.1\.0"\)/,
    'installer/manifest.tsv': /^release\tfacet\t1\.1\.0\t/m,
    '.release-harness/docker/Dockerfile': /facet-1\.1\.0-linux-amd64\.zip/,
    'docs/install.ps1': /\$version = '1\.1\.0'/,
    'docs/install.sh': /version=1\.1\.0/,
  };
  for (const [path, pattern] of Object.entries(exact)) {
    assert.match(read(path), pattern, `${path} does not identify v1.1.0`);
  }
});

test('public release documentation points to v1.1.0', () => {
  for (const path of ['README.md', 'docs/index.html', 'docs/docs.html']) {
    const source = read(path);
    assert.match(source, /v1\.1\.0/, `${path} does not mention v1.1.0`);
    assert.doesNotMatch(source, /releases\/(?:download|tag)\/v1\.0\.4/, `${path} still links to v1.0.4`);
  }
  assert.match(read('CHANGELOG.md'), /^## 1\.1\.0$/m);
});

test('published installer verification runs only as an explicit post-release gate', () => {
  const workflow = read('.github/workflows/bootstrap.yml');
  assert.match(workflow, /name: Verify pinned published installer and handoff\s+if: github\.event_name == 'workflow_dispatch'/);
});
