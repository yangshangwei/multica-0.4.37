# Verification design

Use the existing ordered `scripts/check.sh` pipeline as the baseline: static checks, bounded package tests, script regressions, isolated Go database, fresh API, production Web, then serial Playwright with zero retries. The runner owns cleanup and preserves provenance in its registry. Only one Web build may run in this checkout at a time.

A parallel read-only coverage review maps recent changes and conditional skips to follow-up suites. Native and password acceptance run sequentially after the baseline against task-owned services; they must not race shared catalogs or gates. Fixture providers demonstrate application/runtime behavior, not vendor-account compatibility. The main session owns browser inspection, environment configuration, failure triage and final reporting.

No product code changes are planned. Existing working-tree changes are excluded. Evidence lives under `.omx/reports/recent-commits-e2e-20261027/`; task documents contain durable summaries and paths without secrets.
