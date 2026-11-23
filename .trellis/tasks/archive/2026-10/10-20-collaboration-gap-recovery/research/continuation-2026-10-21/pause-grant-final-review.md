# Independent AC10/38/50 review — 2026-10-21

Decision: **approve complete AC10 at P5 and AC38/AC50 at P4**, combining the two new test identities with their existing current canonical control/proof evidence. No product defect or remaining clause gap was identified in this bounded slice. Only this report is written here; source, index, matrix/catalog, other proposals and environment remain read-only.

## AC10 — paused additions and empty target

`TestCollaborationPausedFollowupRequiresProofFreshResumeAndNeverRetriesStoppedInput` uses the actual managed service Submit/Resume/List/Claim/Permit/Stop/proof/recovery writers with an original persisted permitted attempt and unreleased reservation. It covers two distinct cases:

- `paused_inputs` adds inputs 2 and 3 **after target Stop**. Both receipts stay paused and share one new batch, exact channel sequences and separate authors. Repeated Poll/current-v5 reads cannot consume them or change business state. Original sealed proof settles only input 1 and leaves the target paused. Old refused Resume POST/GET remains the exact earlier refusal; old Claim stays stopped. A fresh explicit Resume followed by one new Claim contains exactly 2/3, with fresh run/group/nonce, the original typed target/channel, one attempt and one final success. Claim replay and the old Stop key cannot create another batch or reach the new run.
- `empty_target` has the same original attempted input, proof barrier and fresh Resume but no follow-up input. Repeated Poll/current reads allocate no work; only the original stopped command/group/run/attempt remains and the target is active/idle with no active group.

Both keep the original group-input membership, unrelated managed target and whole-issue control snapshot unchanged. The final count requires exactly one original run plus at most one two-input follow-up; input 1 stays stopped with one original attempt. These directly close both AC10 gaps formerly recorded by `ledger-independent-review.md`.

## AC38/50 — same-resource other-task authorization

`TestCollaborationGrantWithdrawalAndOwnerRejoinIsolateOtherTask` creates two distinct issue/target/grant scopes sharing the exact owner, agent, runtime and installation binding. The affected organizer is a different account/source workspace; the resource owner is an external participant. A peer run is first completed through persisted Claim/Permit/outcome/proof, so its snapshot contains actual nonempty commands, groups/runs/grants/attempts/capabilities/reservations/termination/events/participants/operations. The test asserts each required array is nonempty; a vacuous empty snapshot cannot pass.

The peer's full control/proof snapshot, issue/collaboration/participants/operations/notifications, current access and authorized target catalog remain identical after affected creation/consent, grant withdrawal or owner leave/rejoin, original affected proof, new consent, old refused Resume replay and fresh Resume.

- `revoke_grant` revokes only the affected grant, leaves the owner's participant epoch/full capabilities and own waiting input unchanged, stops only that exact old permitted group/run and retains unknown proof/reservation/termination intent. After original proof, Resume without consent refuses. New consent creates a fresh per-issue grant and retains the hold; the earlier refused key remains refused and changes no business state. Only a fresh authorized Resume releases the hold. The revoked original grant and stopped command remain terminal.
- `leave_rejoin` performs actual `Leave`, cancels only the owner's waiting input and revokes its affected issue access/grants. A remaining collaborator's waiting input survives. Reinvite restores participant epoch +2 without reviving the old grant/command/hold; original affected proof plus new owner consent plus fresh Resume are required. The peer's same-resource grant/current access and all historical proof/operation/participant state remain identical at every stage.

A final actual peer `SubmitCommand` succeeds using its unchanged grant, produces a fresh waiting command/batch and preserves its earlier successful command. This establishes positive other-task use without conflating authorization with shared physical capacity. AC38's no automatic participant removal and AC50's leave-isolation clauses now have execution snapshots, not only source review.

## Current results and source identities

Both new files are test-only and reuse tracked fixtures, actual writers and existing snapshots; product code, dependencies and SQL are unchanged.

| New canonical test file | SHA-256 |
| --- | --- |
| `server/internal/service/collaboration_paused_followup_test.go` | `85ba407981009a381aed0c6e408032cf999891f6cba6b4b645ff70fb02e99793` |
| `server/internal/service/collaboration_grant_task_isolation_test.go` | `2c80c3277f3d4d8891f1938f585f7f4bb6eb4cfe83f251bc080f03fdf6ea13fe` |

Current files, recorded result hashes and isolated copies agree. Parsed raw results:

| Result | Actual PASS / package elapsed | Raw SHA-256 |
| --- | --- | --- |
| `paused-followup-coordinator-focused-20261021.jsonl` | 1 top-level / 3 including cases, 3.974s | `428189173c33f29c9fb5272e62be3e96af2153f0105982109429e3afcf9c53df` |
| `paused-followup-isolated-adjacent-race.jsonl` | 5 top-level / 7 including cases, 4.972s | `d5656a4b9db1b2547ff64078c233e4f51f290c2ccad8cf0c210e6288cd199717` |
| `grant-task-isolation-adjacent-final-20261021.jsonl` | 10 top-level / 22 including cases, 9.203s | `6a514344838ae566151eda183bb732c460cb2ecd2312f104846636400b9c2866` |
| `grant-isolated-head-race.jsonl` | 1 top-level / 3 including cases, 3.869s | `da657a0af9fc30320ec4d9d2bb526a3f406a10b34163b64e9fce88a8917eb419` |

All four have package PASS, returncode 0, **zero failures/skips**. The exact command vectors/working directories and timestamps are in the neighboring `-result.json` files. Each runs Go 1.26.6, isolated migration-737 `service_s5`, ambient agent CLI guard and `-race -p 1 -parallel 1 -count=1 -timeout=30m`. No database suite was rerun by this reviewer.

The paused adjacent scope includes current `TestCollaborationManagedStopProofPreservesFreshResumeIntent`, `TestCollaborationResumeAllEmptyScopeReleasesGlobalPause`, `TestCollaborationBatchKeepsOneFollowupBehindRunningWork` and `TestCollaborationQueueStaleStopCannotReachNextRun`. Grant neighbors cover invalid-grant atomicity, reconsent/fresh Resume, configuration restore not reviving consent, initial holds, resource-only invalidation, forced-group proof refusal and voluntary/removal author isolation. Their existing whole-criterion role/proof coverage is preserved.

## Isolation and staging safety

Both isolated results use exact recorded HEAD `aef77e9ce62b2f665f6a99676a79244b49fde497` under `/tmp/multica-gap-recovery-20261021-h6phxwo9/content-acceptance-head-bjg3a90e`. Read-only comparison reconfirmed all **4056 tracked server files** match that HEAD byte-for-byte. The only current extra Go files are the four declared test additions: the paused and grant tests plus the separately reviewed content test pair.

These are accurately described as exact HEAD with declared test-only siblings; they are not labeled a physically two-file-only checkout. The content siblings each declare only their own Test function, no shared helper or initialization. The paused candidate declares only its Test; the grant candidate's two private helpers are contained in that same file. Neither candidate references the other new tests, and both exact selected isolated executions pass using the HEAD product/fixture implementation. Thus a commit containing only these two new test files does not require adding broad dirty product source or generated SQL. At inspection the staging index was empty; this reviewer did not stage or commit.

## Proof and platform scope

The new service tests use real persisted managed permits/reservations, but **test-created sealed proof** and synthetic result. They do not launch a provider or observe natural namespace termination. Actual natural process/proof behavior remains the separately catalogued six-test Linux router run (`continuation-linux-router`, raw SHA `411f23c1e244550118dcbf7a09d4fdb6062be74e9b0bda627b827257f9acaf0b`), including single-agent Stop/proof/fresh Resume and two independent lanes. Those raw hashes match the current catalog; the scopes are combined honestly, not relabeled as a new natural-proof run of the paused/grant tests.

P4/P5 primary acceptance is distinct from unrelated full Web/Electron/provider/OS/install/rollout criteria. Real accounts/quota, new dual-client owner-exit journey, stock provider versions, S3/Redis fault scenarios, installed full Main and Windows/macOS containment remain outside this report. Production gates remain closed and neither task nor parent is archived.

## Verification and findings

Reviewer commands:

```sh
gofmt -d server/internal/service/collaboration_paused_followup_test.go server/internal/service/collaboration_grant_task_isolation_test.go
# From server/:
GOTOOLCHAIN=go1.26.6 go vet -p 1 ./internal/service
```

Both exit 0 with empty diagnostics/diff. Read-only Python parsed all current and isolated JSONL, verified counts/source hashes/exact tracked-file isolation and existing catalog provenance. Trellis context/report whitespace validation follows writing. This Go/test/report slice has no frontend TypeScript changes; no frontend lint/typecheck rerun is claimed.

Fixed findings: none; tests and product paths are sound. Unfixed product findings: none. Minor existing note boundaries (prepared/static-before-final-source and originally pending isolated verification) are superseded only by the explicit current results above, preserving older reports as historical records.

Changed file: this independent report only. Simplification: none; no implementation/dependency/environment changes. Recommendation: root may promote AC10/38/50 with these exact identities/artifacts and commit the verified two-test slice under Lore. This report itself does not mutate authoritative acceptance or staging state.
