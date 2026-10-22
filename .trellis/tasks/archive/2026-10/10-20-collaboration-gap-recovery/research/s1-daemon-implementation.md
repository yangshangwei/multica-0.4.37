# S1b daemon recovery implementation — 2026-10-21

Scope: AC-I02 and the daemon portion of AC-I01. Server scanning, SQL, native provider containment and rollout acceptance remain with their respective owners. No task status, gate, credential configuration or commit changed here. The final candidate policy is server-owned single-candidate paging over the original `run_ids` wire.

## Implementation

- `server/internal/daemon/collaborationrun/recovery.go`: private `.collaboration-proof-recovery.cursor` and `.lock`, excluded from the run `.json` inventory. Reuses owned-file checks, cross-process locking, atomic file fsync and directory fsync. Each attempt is recorded before the run lock or HTTP request; failed requests and peer `ErrBusy` therefore advance. Full inventory validation precedes even the shared-cursor busy check. Malformed/unreadable cursor or inventory and failed cursor writes refuse recovery.
- `server/internal/daemon/collaboration.go`: at most eight proof attempts share one ten-second round deadline. Caller cancellation propagates; exhausting the internal round budget preserves subsequent polling. Non-proveable records, original proof identity and blocked capacity remain untouched. Candidate dispatch preserves the original first-unblocked `run_ids` selection. The private controller field uses the existing `Launcher` interface so actual Claim/Permit traffic can be tested with a non-forking fake; production still constructs `NamespaceController`.
- `server/internal/daemon/collaborationrun/runner.go`: only retained pre-burn recovery now preserves the caller's deadline; foreground abandonment still performs shutdown-resistant proof completion. Existing permit retry behavior and durable burn/seal logic are preserved.
- Candidate scheduling position belongs to the server's durable, priority-preserving scan. `ListCollaborationRuntimeCandidates` now offers one row per response, matching the daemon's one attempted run per dispatch. The client has no new cursor, metadata field or query flag. The intermediate candidate cursor/helper/tests and optional metadata parser were removed after the composed paging counterexample below; `client.go` is byte-identical to the pre-existing dirty baseline.

## Composed candidate paging contract

A local last-ID cursor cannot recover the unattempted tails of disjoint multi-item pages. With server pages `[1..16]`, `[17,18]`, repeated, the previous ID is absent from each next page, so the client repeatedly chooses 1 and 17. A per-priority cursor alone does not solve that composition. The accepted simplification is for the server to advance exactly one offered candidate per List; no last-recently-attempted map or additional wire protocol is introduced.

`TestCollaborationCandidatePagingAcrossRestart/single_candidate_pages` uses 18 candidates and 17 persistent Claim-409 responses. Its httptest server advances its own page position when List is called, independently of Claim success and of the reconstructed poller. Each List returns one `run_ids` entry. All 18 candidates receive their real HTTP Claim in order, the final one receives Claim/Permit admission, and a fake uncertain Start preserves its slot, blocked identity and update barrier. The legacy-order control verifies unchanged selection for an old static multi-ID response.

This guarantees daemon consumption of the server's offered single item. It does not claim to fix arbitrary multi-item moving-page responses or old servers that keep returning a static failed prefix. Increasing server candidate page size above the daemon's consumption count requires an explicit consumption/progress design and corresponding composed regression; waiting/wake page sizes are independent.

## Failure-first evidence

Commands below ran from `server/` with no database dependency and no real agent CLI.

1. `GOTOOLCHAIN=go1.26.6 bash ../scripts/go-test-with-agent-cli-guard.sh go test ./internal/daemon -run '^TestCollaborationProofRecovery' -count=1 -timeout=45s`
   - Exit 1; daemon package 2.803 s. The ninth proof received zero requests behind eight persistent HTTP failures and behind eight locks held by an actual test child process. Independent ten-second request deadlines and detached pre-burn cancellation also failed.
2. Composed paging diagnostic, before the final test rename: `GOTOOLCHAIN=go1.26.6 bash ../scripts/go-test-with-agent-cli-guard.sh go test ./internal/daemon -run '^TestCollaborationCandidateFailureRotationAcrossRestart/single_candidate_pages$' -count=1 -timeout=45s`.
   - The httptest pager used 16 rows to reproduce the old server consumption mismatch. Exit 1; daemon 1.132 s. Over 36 List/Claim rounds, requests alternated only between candidates 1 and 17, never candidate 18. This reproduced the flaw even with the intermediate client cursor in place. The permanent fixture mirrors S1a's final SQL `LIMIT 1`; the final candidate cursor and metadata cleanup then passed the checks below.

## Final checks

Environment: `go1.26.6`, `darwin/arm64`. The proof and adjacent runtime checks ran before the candidate-only cleanup; their implementation is unchanged by that cleanup:

```sh
GOTOOLCHAIN=go1.26.6 bash ../scripts/go-test-with-agent-cli-guard.sh go test -race ./internal/daemon ./internal/daemon/collaborationrun -run 'Collaboration|Runner|Ledger|Retained|Nonce|Seal|Recovery|Candidate' -count=1 -timeout=120s
```

Exit 0: daemon 8.923 s; collaborationrun 6.910 s. The final HTTP-prefix fixture also drops the ninth proof's first ACK and verifies byte-identical proof/receipt/attempt replay after another restart. Canonical new test files:

- `server/internal/daemon/collaboration_recovery_test.go`: nine retained proofs, persistent HTTP and actual peer-lock failures, ACK loss, common deadline, all three pre-burn phases, caller cancellation, unknown/damaged ledger capacity and update barriers.
- `server/internal/daemon/collaboration_candidates_test.go`: actual managed listing, 18 single-candidate server pages, 17 persistent Claim 409 responses, accepted successor Claim/Permit, reconstructed pollers, legacy order and uncertain-launch slot/blocked/update retention; the fake never forks a provider.
- `server/internal/daemon/collaborationrun/recovery_test.go`: durable selection before any request, eight-attempt bound, wrap after ACK, real cross-process cursor exclusion, malformed/oversized/nonprivate/symlink/hardlink cursor refusal, write failure and corruption discovered despite a busy cursor.

After the final candidate cleanup:

```sh
GOTOOLCHAIN=go1.26.6 bash ../scripts/go-test-with-agent-cli-guard.sh go test -race ./internal/daemon ./internal/daemon/collaborationrun -run 'Candidate|Client' -count=1 -timeout=60s
GOTOOLCHAIN=go1.26.6 go vet ./internal/daemon ./internal/daemon/collaborationrun
```

Both commands exited 0; candidate/client race packages: daemon 2.616 s and collaborationrun 1.683 s. `gofmt -l` on all six remaining changed/new Go files emitted nothing; scoped `git diff --check` passed. Source search found no remaining candidate helper/type references in the daemon; the removed files do not exist. No proof tests were needlessly repeated for the candidate-only cleanup.

The pre-existing dirty baseline is `/tmp/multica-gap-recovery-20261021-h6phxwo9/files/`. Final byte comparisons confirmed that `client.go`, `ledger.go`, `ledger_test.go`, `runner_test.go` and `runner_permit_test.go` remain identical to that baseline. Comparing `runner.go` against it showed only the retained-recovery deadline change; existing dirty work is not attributed to S1b.

## Limits and handoff

No real provider, Linux kernel containment, native packaging, Web/Desktop, DB scheduling or full `make check` was run by this slice. POSIX cursor/ledger evidence on macOS is not Linux execution or general provider support. Parent acceptance and execution gates remain pending. The root session must integrate and verify the server's durable highest-tier `LIMIT 1` scan and its DB-backed fairness/priority cases before claiming all of AC-I01.
