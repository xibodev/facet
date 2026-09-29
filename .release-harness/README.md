# Release Harness Configuration

Project-owned test intent for **facet**:
- `harness.config.json`: Execution controls, timeouts, and port blocks.
- `topology.json`: Service graph, Docker Compose services, and health probes.
- `origins.json`: Served surface definitions (browser apps, APIs, workers).
- `scenarios/`: Declarative Playwright scenarios (smoke, core, full).

Docker UAT is implemented by root `docker-compose.test.yml` and the project-owned
Node probes. See `docs/operations/uat-docker.md` for exact installation, isolation,
model opt-in, evidence and cleanup semantics. No model credentials means a clear
real-agent gate failure, never a mock substitute or automatic skip.

Run the bounded development gate with an external evidence root:

```powershell
$evidence = Join-Path ([System.IO.Path]::GetTempPath()) 'facet-uat'
node scripts/uat-run.mjs $evidence
# Clean-source certification attempt (no --allow-dirty):
node scripts/uat-run.mjs $evidence --certify
```

Other harness commands:
```bash
npx release-harness doctor
npx release-harness check-pr
npx release-harness run-local
```
