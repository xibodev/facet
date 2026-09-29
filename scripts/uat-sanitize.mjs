import { readFileSync } from 'node:fs';

// Only the explicitly mounted configuration is consulted; never read host auth stores.
export function sanitizer() {
  const secrets = new Set();
  function collect(value, key = '') {
    if (typeof value === 'string' && /key|secret|token|password|authorization/i.test(key) && value.length > 3) secrets.add(value);
    else if (value && typeof value === 'object') for (const [k, v] of Object.entries(value)) collect(v, k);
  }
  if (process.env.OPENCODE_CONFIG) {
    try { collect(JSON.parse(readFileSync(process.env.OPENCODE_CONFIG, 'utf8'))); } catch { /* Configuration validation happens in the journey. */ }
  }
  const redact = value => {
    let text = String(value);
    const forms = new Set();
    for (const secret of secrets) {
      for (let form of [secret, encodeURIComponent(secret)]) {
        // JSON embedded in JSON (trace resources and tool stdout) adds another
        // escaping layer. Bound expansion by the input length, not a guessed depth.
        while (form.length <= text.length && !forms.has(form)) {
          forms.add(form);
          const escaped = JSON.stringify(form).slice(1, -1);
          if (escaped === form) break;
          form = escaped;
        }
      }
    }
    for (const form of [...forms].sort((a, b) => b.length - a.length)) text = text.split(form).join('[REDACTED]');
    return text.replace(/([?&](?:token|api_key|key)=)[^&\s"<>]+/gi, '$1[REDACTED]')
      .replace(/(Bearer\s+)[\w.+/~=-]+/gi, '$1[REDACTED]')
      .replace(/("(?:token|apiKey|api_key|authorization|password|secret)"\s*:\s*)"(?:\\.|[^"\\])*"/gi, '$1"[REDACTED]"');
  };
  return { redact, add: value => { if (typeof value === 'string' && value.length > 3) secrets.add(value); } };
}
