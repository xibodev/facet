#!/usr/local/bin/node
import { activeReceiptDirectory, forward, realFacet } from './uat-facet-recorder.mjs';

// TEST ONLY: forwards the actual installed executable; never makes media/results.
try {
  const result = await forward(realFacet, process.argv.slice(2), activeReceiptDirectory());
  if (result.signal) process.kill(process.pid, result.signal);
  else process.exitCode = result.code ?? 74;
} catch {
  process.stderr.write('UAT Facet recorder failed; execution evidence unavailable\n');
  process.exitCode = 74;
}
