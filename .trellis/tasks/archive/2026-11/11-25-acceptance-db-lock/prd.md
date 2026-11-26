# Service delivery observation lock investigation

## Goal
Identify the actual PostgreSQL holder responsible for a reproduced delivery-evidence NOWAIT conflict and close the service acceptance gap without weakening authorization or observation fences.

## Requirements
- Preserve the original 2026-11-24 wrong_consumer failure and all old evidence read-only.
- Capture actual pg_locks and pg_stat_activity identity/query/mode/transaction timing for the competing holder in a fresh owned database. A hypothesis or artificial conflict alone does not identify the historical holder.
- Distinguish unrelated writer, leaked transaction, maintenance, and own fixture interference before choosing a minimal fix.
- Keep SHARE NOWAIT, authorization checks, refusal assertions, deadlines and retry counts unchanged.
- Own only delivery-evidence service and service fixture code; no handler, daemon, dependencies, production migrations or shared runtime edits.
- Run the complete service package normally and with race instrumentation after the fix; keep complete pass/fail/skip inventories and original failed attempts.
- Use allocated task DB, private HOME and scratch paths; no personal accounts, paid models, old runners, commits, push or deployment.

## Acceptance criteria
- [ ] Actual conflicting PID, backend type, lock mode, relation and query captured with timestamp at the observed refusal, or cause explicitly remains UNKNOWN.
- [ ] Minimal fix tied to the captured cause, with original safety semantics and meaningful negative controls.
- [ ] Full service normal/race finish with commands, exits and exact named inventories; targeted passes never replace them.
- [ ] Source hashes, tool versions, resource ownership and final live-connection audit recorded in new evidence.
