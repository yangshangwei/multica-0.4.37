# Execution plan

- [x] Read repository instructions, recent history and E2E environment contract.
- [x] Record user-authorized scope and evidence requirements.
- [x] Run the `make check` pipeline with retained reporting; after its initial fixture failure, complete the remaining stages and rerun the corrected Desktop package.
- [x] Audit the complete E2E inventory and conditional environment requirements.
- [x] Run applicable supplemental collaboration, password, native Electron and packaged suites sequentially.
- [x] Reproduce failures without weakening assertions and retain screenshots/traces.
- [x] Summarize executed/passed/failed/skipped coverage and environment identity.
- [x] Stop only task-owned services, preserve unrelated changes, update task verification notes.

The user is reviewing existing commits: do not change application code to make tests pass. Fix reversible test-environment prerequisites, and report product/test defects with evidence. Never run real-account agent smoke tests or production writes.

Two obsolete Electron test doubles were corrected after the 18 failures were reproduced. No application code or test assertions were weakened.
