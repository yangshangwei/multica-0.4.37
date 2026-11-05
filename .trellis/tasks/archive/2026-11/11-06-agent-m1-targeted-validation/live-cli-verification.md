# Actual compiled CLI to full API verification

**20 targeted CLI cases passed,0 failed.** All requests used the actual compiled Multica binary, real current role-template adoption/assignment/claim/start and a transparent trace proxy to the actual isolated server. No provider was called by this fixture lane.

## New evidence
- Reporter and reviewer each: issue get by UUID and canonical identifier, comments list, timeline, current-issue comment add/readback.
- Reporter: exact full-project UUID list and pagination return the complete expected scoped ID set; short project prefix is refused because it requires the forbidden project inventory.
- Reviewer: project list refused; both roles' outside-issue reads and current title writes refused by real API403.
- Each accepted deterministic comment persisted the exact agent author, source_task_id and dispatch_eligible=false. SQL before/after confirms no business issue fields or task inventory/status changed.
- CLI JSON outputs parsed successfully, actual target IDs compared rather than accepting exit0 alone. No /api/me or other broad authorization grant was added for compatibility.

`evidence/actual-cli-20.json` records command arguments, statuses, token-free real request paths and stdout/stderr hashes. Private raw stdout/stderr and before/after SQL states live in `cli-live-01/` under the recorded private run root; `live-harness-manifest.json` binds them. Synthetic identifiers are intentionally retained; credentials are not.

## Model fixture handoff
Reporter and reviewer prompts retain actual server-claimed agent instructions plus only their relevant role skill (14,371 /10,455 UTF-8 bytes at handoff). Both include untrusted quoted scope/status overrides and independently frozen expected conditions. Reporter current-state facts are evidence, not a natural-day completion count. Reviewer supplies the concrete `<100` vs inclusive100 defect. Leader uses an actual squad claim/roster with one permitted worker and one private worker owned by a synthetic second person; no real worker consumes queued work.

Root owns and records actual Codex invocations, semantic judgments, budget, remaining failures and cleanup. Fixture oracles compare persisted rows, actual request traces and working-directory hashes; they do not trust model self-report. Only source/diagnostic harness and sanitized summaries were copied into the task.

## Scope / limits
No product source or dependencies changed. No database creation/migration/drop, account secret access, remote push or production work by this lane. Existing templates/current source were used; this is not full-daemon/provider or broad E2E acceptance. Private harness setup failures remain classified separately. Parent M1–M3 remains incomplete.
