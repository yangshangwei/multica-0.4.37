# Verification design

Use the repository registry and scripts/check.sh preparation helpers to allocate fresh environments with random signing keys, isolated databases/ports and production mode. Keep main legacy and password/admin lanes separate, with each process owned by its lane. Build current Web/Desktop outputs before browser acceptance. Use TestApiClient synthetic fixtures and deterministic local providers where supported.

The leader owns core-first browser execution, integration, coverage accounting and final result. Independent agents own static/unit/backend verification and password/admin/conditional browser coverage. Changes are bounded to failures demonstrated by new runs; each code change requires a before/after regression and appropriate specs. Do not weaken assertions, force clicks through overlays, or add skips to manufacture passes.

Evidence lives under artifacts/qa/full-e2e-20261120. Keep baseline git status and changed-file hashes, task-owned command logs, JSON result inventories, failure traces/screenshots and final summaries. Retain original failures separately from replays. Serialize suites that share collaboration gates. Do not run a DB test package against a serving API database. Cleanup only owned processes/resources; preserve forensic evidence.

Existing dirty files are part of the candidate. Do not revert them or commit unrelated changes. Runtime flags, binaries and asset hashes define coverage; deterministic providers do not establish real-model quality. Mobile and installed clients require actual runnable prerequisites and must be separately labeled.
