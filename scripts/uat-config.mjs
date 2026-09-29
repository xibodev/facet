import assert from 'node:assert/strict';

export function validateModelConfig(config) {
  assert.ok(typeof config?.model === 'string' && /^[^/\s]+\/[^\s]+$/.test(config.model), 'REAL_AGENT_UNCONFIGURED: explicit provider/model required');
  const provider = config.model.split('/')[0];
  assert.ok(config.provider && typeof config.provider === 'object' && !Array.isArray(config.provider) && Object.hasOwn(config.provider, provider), 'REAL_AGENT_UNCONFIGURED: selected provider configuration required');
  const settings = config.provider[provider];
  assert.ok(settings && typeof settings === 'object' && !Array.isArray(settings), 'REAL_AGENT_UNCONFIGURED: provider settings must be an object (built-in providers may use {})');
  if (config.enabled_providers !== undefined) assert.ok(Array.isArray(config.enabled_providers) && config.enabled_providers.includes(provider), 'REAL_AGENT_UNCONFIGURED: selected provider is not enabled');
}
