# ENV-1 canonical regression entry

This plan is now implemented in source commit `1ecabb964` and passed in the root-owned `runtime-password-3` run. `ENV1-facts.json` records persisted/API facts and `verification-runs.json` records the command, exit and hashes. It is a bounded regression result; parent release acceptance remains open.

Reuse `seedRuntimeHandoff()` in the canonical runtime-handoff Web spec. Its execution history is deliberately seeded, so evidence must continue to distinguish UI/API reads from actual runtime ingestion.

1. For the specific seeded run only, set `memory_mode='supported'` and `memory_base_version=0` using the existing task fixture database convention. Preserve the affected row count.
2. Perform a real `PUT /api/collaboration/issues/:id/memory` with `expected_version='0'` and a new idempotency key. Require the returned version to be `1`.
3. Independently read the persisted run and the current handoff DTO. Require persisted base version `0` and separate DTO `memory.base_version='0'`, `memory.version='1'`.
4. Open production Web through the recipient's password-authenticated session, reuse `assertRuntimeHandoff`, and require both positive labels: `Started from an empty document` and `Latest is v1`. Require the latest-version link to select memory version `1` for this issue.
5. Require no `Started from v1` attribution. This negative check alone is insufficient because removing all attribution would incorrectly pass.
6. Retain exact source/build/service identity, command, exit code and the persisted/API/UI facts. Provider calls stay `0`.

The runtime-handoff specs' password guard must not hide missing API/DB/renderer/session setup when `E2E_PASSWORD_AUTH=1`. This entry requires a distinct root permit and must not run through the legacy runner.
