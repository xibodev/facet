import { test } from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { configureRun } from './uat-run.mjs';
import { validateModelConfig } from './uat-config.mjs';

const root = fileURLToPath(new URL('../', import.meta.url));
const evidence = path.join(os.tmpdir(), 'facet-uat-options-test');
const free = JSON.parse(fs.readFileSync(new URL('../.release-harness/docker/opencode-free.json', import.meta.url), 'utf8'));

test('free mode pins both models, selects only built-in provider, and enables real open-network UAT', () => {
  assert.deepEqual(free, { $schema: 'https://opencode.ai/config.json', model: 'opencode/big-pickle', small_model: 'opencode/big-pickle', enabled_providers: ['opencode'], provider: { opencode: {} } });
  validateModelConfig(free);
  for (const args of [[evidence, '--free-model'], ['--free-model', evidence]]) {
    const { env, certify } = configureRun(args, { UAT_REAL_AGENT: '0', UAT_NETWORK_INTERNAL: 'true' });
    assert.equal(certify, false);
    assert.equal(env.UAT_REAL_AGENT, '1');
    assert.equal(env.UAT_NETWORK_INTERNAL, 'false');
    assert.equal(env.UAT_CERTIFY, '0');
    assert.equal(env.UAT_OPENCODE_CONFIG_FILE, './.release-harness/docker/opencode-free.json');
    assert.equal(env.UAT_TURN_TIMEOUT_MS, '600000');
  }
});

test('default stays offline and drops inherited credentials and runtime overrides', () => {
  const { env } = configureRun([evidence], {
    PATH: 'kept', EXAMPLE_API_KEY: 'synthetic', GITHUB_TOKEN: 'synthetic',
    OPENCODE_CONFIG: '/host-home/config', OPENCODE_CONFIG_CONTENT: '{}',
    COMPOSE_FILE: 'host.yml', UAT_NETWORK_INTERNAL: 'false', UAT_CERTIFY: '1',
    UAT_OPENCODE_CONFIG_FILE: '/not-read', UAT_TURN_TIMEOUT_MS: '1'
  });
  assert.deepEqual(env, { PATH: 'kept', UAT_PORT: '31000', UAT_APP_URL: 'http://127.0.0.1:31000', UAT_REAL_AGENT: '0', UAT_NETWORK_INTERNAL: 'true', UAT_CERTIFY: '0' });
  const freeEnv = configureRun([evidence, '--free-model'], { OPENCODE_CONFIG_CONTENT: '{"model":"paid/model"}', PROVIDER_API_KEY: 'synthetic' }).env;
  assert.equal(freeEnv.OPENCODE_CONFIG_CONTENT, undefined);
  assert.equal(freeEnv.PROVIDER_API_KEY, undefined);
});

test('certification, conflicting config, duplicate flags and unsafe arguments fail before effects', () => {
  for (const args of [[evidence, '--free-model', '--certify'], ['--certify', '--free-model', evidence]]) {
    assert.throws(() => configureRun(args, { UAT_OPENCODE_CONFIG_FILE: '/must-not-read' }), /CERTIFICATION_REFUSED/);
  }
  assert.throws(() => configureRun([evidence, '--certify'], { UAT_NETWORK_INTERNAL: 'false' }), /CERTIFICATION_REFUSED/);
  assert.throws(() => configureRun([evidence, '--free-model'], { UAT_OPENCODE_CONFIG_FILE: '/must-not-read' }), /cannot be combined/);
  for (const args of [[], [evidence, '--unknown'], [evidence, '--free-model', '--free-model'], [evidence, '--certify', '--certify'], [evidence, 'extra']]) assert.throws(() => configureRun(args, {}), /Usage/);
  assert.throws(() => configureRun([root], {}), /outside/);
  assert.throws(() => configureRun([path.join(root, 'new-evidence')], {}), /outside/);
  assert.throws(() => configureRun([evidence], { UAT_REAL_AGENT: '1' }), /explicit absolute/);
  assert.throws(() => configureRun([evidence], { UAT_REAL_AGENT: '1', UAT_OPENCODE_CONFIG_FILE: path.join(root, '.release-harness/docker/opencode-free.json') }), /outside/);
});

test('external config remains supported with separate network opt-in', () => {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'facet-uat-config-test-'));
  try {
    const file = path.join(dir, 'config.json');
    fs.writeFileSync(file, JSON.stringify({ model: 'custom/example', provider: { custom: { npm: '@ai-sdk/openai-compatible', options: { baseURL: 'http://example.invalid/v1' }, models: { example: {} } } } }));
    for (const network of ['true', 'false']) {
      const { env } = configureRun([evidence], { UAT_REAL_AGENT: '1', UAT_OPENCODE_CONFIG_FILE: file, UAT_NETWORK_INTERNAL: network });
      assert.equal(env.UAT_OPENCODE_CONFIG_FILE, file);
      assert.equal(env.UAT_REAL_AGENT, '1');
      assert.equal(env.UAT_NETWORK_INTERNAL, network);
    }
  } finally { fs.rmSync(dir, { recursive: true, force: true }); }
});

test('browser config gate accepts built-in empty settings but rejects incomplete or disabled selection', () => {
  for (const config of [{}, { model: 'missing-slash', provider: {} }, { model: 'opencode/big-pickle', provider: {} }, { ...free, provider: { opencode: null } }, { ...free, enabled_providers: ['other'] }]) assert.throws(() => validateModelConfig(config), /REAL_AGENT_UNCONFIGURED/);
});
