# Release Harness Configuration

Project-owned test intent for **facet**:
- `harness.config.json`: Execution controls, timeouts, and port blocks.
- `topology.json`: Service graph, Docker Compose services, and health probes.
- `origins.json`: Served surface definitions (browser apps, APIs, workers).
- `scenarios/`: Declarative Playwright scenarios (smoke, core, full).

Run checks with:
```bash
npx release-harness doctor
npx release-harness check-pr
npx release-harness run-local
```
