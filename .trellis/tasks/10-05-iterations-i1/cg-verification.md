# CG integration acceptance — 2026-10-07

CG passed on the current working-tree implementation based on `115b4cd28`.
UG, FCG and VG remain open. No push, merge, deployment or rollout enablement.

The final parent integration command used the exclusive migrated
`multica_i1_vg_20261006_2310` database and the agent CLI guard:

```sh
bash scripts/go-test-with-agent-cli-guard.sh -- go -C server test -race -p 1 -parallel 2 \
  ./internal/iteration ./internal/service ./internal/handler ./cmd/server \
  -run '^(TestIteration|TestIssueIterationAssignment|TestTriage|TestHistorySnapshot)' -count=1 -json
```

All four packages passed: 422 passing test events, four explicit skips (two
opt-in P95 harnesses and the two existing opt-in triage tests). Counts include
subtests; they are not unique scenarios. Raw log hashes and package results:
[CG integration evidence](../10-05-iterations-i1-closure/cg-integration-evidence.json).

The independently implemented handler matrix adds 28 leaf cases covering real
issue/project deletion, ordinary status and membership changes, member revoke,
start versus disable, guarded status-catalog maintenance, and HTTP recovery
after a real successful commit with lost response. Both lock-queue orders are
observed in PostgreSQL. Status-category edits have no production handler;
that matrix explicitly tests the lower-level catalog fence, not a nonexistent
API. See [writer evidence](../10-05-iterations-i1-closure/closure-writers-verification.md).

Notification evidence separately covers committed outbox, stable inbox IDs,
rollback between insert and delivered marking, concurrent retries, recipient
revocation/rejoin, local-date/DST deduplication, coordinator fallback, postponed
end dates, dead-letter retry limits and after-commit realtime publication.
See [notification evidence](../10-05-iterations-i1-closure/notifications-verification.md).

Review fixes preserve original clock samples, stage frozen data before release
and insert the snapshot only once at finalization, reauthorize date recipients,
compare stale hashes before history projection, and avoid mutating request
pointers during canonical decoding. The batched SQL preserves source-before-
target order and checks affected rows; failures roll back all prior phases.

After CG acceptance, `atomic_handoff` reflects the existing rollout capability.
The feature flag still defaults to false and is not a public frontend flag.
A new RED/GREEN test proves handoff discovery remains false under the closed
rollout and true only with the isolated test provider enabled.

Performance discovery is not acceptance. The explicit warm P95 targets and
remaining measurement protocol are owned by
[VG](../10-05-iterations-i1-verification/performance-protocol.md). Full Go/TS,
production Web/Electron, migration recovery and final source-state checks are
still required before declaring I1 complete.
