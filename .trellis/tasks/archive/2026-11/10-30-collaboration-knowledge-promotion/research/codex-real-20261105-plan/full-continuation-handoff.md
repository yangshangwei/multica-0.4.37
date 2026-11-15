# Full continuation after two failed E0b attempts

User explicitly increased the total cap to17. New test-only entry `TestRK1KnowledgeRealContinuation` consumes a `full_continuation` registration and reuses the unchanged full `rk1kRunProof`: E0b1/K1four/K2two/K3eight, plus3 labelled fake seeds.

## Registration and ledger interface

No new JSON property names are needed. Current registration: `mode:"full_continuation"`, `physical_start_cap:17`, `attempt_limit:15`, `prior_actual_starts:2`, `prior_debits:2`. `prior_registration`/SHA and `prior_ledger`/SHA anchor the immediate stopped retry02 artifacts. Its existing references anchor the initial stopped full attempt. Native executable/hash, version, model and config hash must match this scope's unchanged authenticated namespace.

Current ledger: `cap:17`, `mode:"full_continuation"`, `attempt_limit:15`, `prior_ledger_sha256:<retry02 ledger hash>`, `prior_actual_starts:2`, `prior_debits:2`, `starts:[]`, `stopped:false`.

The finite validator checks retry02 mode before calling existing retry history validation, so it does not recurse over arbitrary history. Each old ledger contributes one unique actual-start fact; nested `prior_actual_starts` is a reference, not another debit. Both old ledgers must be stopped with one unfinished E0b task, with ordered timestamps, distinct physical identities and output namespaces. Missing/drifted/corrupt/aliased/cyclic references refuse. Historical expiry and historical binary bytes are not executed or revalidated as current artifacts. Old ledgers remain unchanged.

Every new reservation revalidates both immutable old artifacts, the new ledger's history namespace and cap. It requires a fully completed previous current sample and bounds current starts to15 and total debits to17. Original full/retry modes remain cap15 and are rejected by the new entry; both original entries reject continuation mode. The wrapper's existing registration reader accepts the new reviewed mode after the same source/native/config checks.

Final full-continuation summary preserves measured15+seed3 and separately adds prior2/new15/total17/remaining0. Fake summaries remain paid0 and do not claim real historical consumption.

## Implementation and controls

Changed existing harness files: registration, retry mode dispatch and full-proof summary call. New files: `rk1k_codex_continuation_test.go`, `rk1k_codex_continuation_controls_test.go`. No product/agent/provider code, dependencies or migrations changed.

The task-local fake launcher accepts `--full-continuation` and selects the original8top-level tests plus existing E0bRetryControls and new ContinuationControls, preserving the full15+3fake scenario. Default/full and E0b-only launcher selections remain explicit and unchanged.

Controls cover two unique prior starts plus exactly15 new starts; 16th refusal after successful completion; historical immutability; missing/drifted/corrupt prior files; cycles/aliases/duplicate identities; forged actual start/count/stopped state; third old launch; namespace/account/model changes; mode isolation; incomplete current predecessor; and distinct fake vs real summaries.

## Current verification status

Gofmt and `git diff --check` passed. Source is frozen for independent static review. No build, test, database or model command has run for this change; the coordinator owns the shared window, fresh database allocations, false execution package and eventual real admission after gateway unblock evidence. Existing real total remains2/17.
