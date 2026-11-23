# AC38/AC50 same-resource task isolation

Status: the bounded missing backend regression and adjacent verification pass. The complete backend-primary clauses of AC38/AC50 now have current execution evidence; the coordinator owns independent review and matrix promotion. This report does not change the acceptance ledger or claim full 97-item acceptance.

Change boundary: add only `server/internal/service/collaboration_grant_task_isolation_test.go` and this report/evidence. Reuse the actual managed fixture, issue creation/participant/target/consent/control services and canonical state snapshot. Two actual issues share one owner, agent, runtime and installation binding. The affected issue has a different organizer so the resource owner can leave voluntarily. A completed peer run with retained proof makes its history/control/authority snapshot substantive; a later positive peer submission independently verifies that its exact grant remains usable without mixing shared-machine scheduling into the isolation assertion.

The missing clauses are per-issue grant withdrawal and voluntary leave/rejoin preserving the exact same resource's other-task grant, participation, input/control, proof and original operations. Existing single-issue tests already cover invalid-grant Resume, renewed consent retaining holds, and non-restoring participant rejoin. The new test adds full peer snapshots, a positive peer submission through the actual service, and fresh affected consent/Resume without replaying terminal commands. Synthetic service proofs remain explicitly separate from native/kernel proof.

Production and helper files, schemas, providers, browser/native slots, task status and acceptance ledger are read-only. Existing fixtures set/restore only their isolated test database gate under the coordinator's exclusive service_s5 test slot; deployment and production rollout were not changed. This is an evidence extension, not a new plan or product implementation.

## Initial execution

The initial new test passed both `revoke_grant` and `leave_rejoin` on its first guarded serial `service_s5` race run: one top-level/three including subtests, zero fail/skip, package 4.207 s. Exact command/results are `grant-task-isolation-focused-first-20261021*`; raw SHA-256 `6c977653b1c698bdb1d1b9e32035b05a469552d2616c70cf575d91a41250e064`. Its source SHA-256 `21e843d7b812aba696dc4048698a74d670422ccc69f871d88367150bef5a0cde` precedes the final narrow assertion additions. Do not reattribute this earlier run to later hashes.

After the pass, the test was hardened to require the peer control/permission/proof/operation arrays to exist and be nonempty, explicitly retain the participating owner's waiting input after grant-only withdrawal, and retain its cancelled input after fresh rejoin/consent/Resume. Current source SHA-256 is `2c80c3277f3d4d8891f1938f585f7f4bb6eb4cfe83f251bc080f03fdf6ea13fe`; the final adjacent race below verifies these additions. No product/helper source changed. Fresh final gofmt and service vet both exit 0 with empty diagnostics (`grant-task-isolation-static-final-20261021-result.json`, vet 2.197 s).

## Final behavior and actual identities

Canonical test `TestCollaborationGrantWithdrawalAndOwnerRejoinIsolateOtherTask` has two cases, `revoke_grant` and `leave_rejoin`. Each creates a real first collaboration through the managed fixture, then a second real collaboration in a different organizer's source workspace using `Create`/`InviteParticipant`/`RegisterTarget`/`ConsentTargetResource`. The grant on each issue names the same actual resource owner, private agent/runtime and managed installation binding, with distinct issue/session/target/channel/grant identities. No parallel fake owner/resource is substituted for the isolation clause. The affected owner is neither creator nor handler and has no source membership, so its voluntary leave and loss/recovery of private reads are exercised directly.

The peer runs one real server-protocol submit/Resume/Claim/Permit/business-outcome/proof cycle before the affected issue changes. Its completed command/group/run, original proof, reservation, attempts and capability rows make the retained snapshot substantive. The canonical control snapshot is extended only with full issue/collaboration/participants/original-operation/notification rows; 13 required arrays must be nonempty. Whole ordered row snapshots, exact `ResolveIssueAccess` and authorized target catalogue compare unchanged after second-issue creation/consent, withdrawal, leave, reinvite, old affected proof, new consent, replay of refused Resume and fresh Resume. A final actual peer `SubmitCommand` succeeds using its unchanged grant, allocates one fresh waiting input/batch and leaves its prior command successful. This positive writer check is separate from shared machine scheduling.

| Case | Affected issue / grant | Peer issue / grant | Shared owner / agent / runtime |
| --- | --- | --- | --- |
| `revoke_grant` | `01a15e78-277f-7a87-9066-add059722f35` / `f286559f-0630-4c1c-9ded-33e51de9a920` | `01a15e78-25e7-75eb-a048-bd8c3d9c6071` / `f108a142-d4ef-419e-90d1-67426488a6fe` | `286bd692-4b51-459c-9de3-702688742e20` / `9760b100-bf8e-4b79-bc00-3c63831416e8` / `3f57f70a-745c-428e-9028-3cef3561c253` |
| `leave_rejoin` | `01a15e78-2aff-76e5-a0d2-0408bd40e842` / `9cbdc8bb-a64a-4a1c-b438-9aac24af83a0` | `01a15e78-2991-7631-8182-13156e3aa712` / `411882b0-a074-4684-a570-60982f10a772` | `ba8b03b0-9a29-4e8a-8635-7f9b998ea814` / `dce7a3a7-7b3a-4c04-b2c6-26a4ea7cab42` / `5d4feedb-cf59-49e8-a9cb-308b47e1ad0f` |

Safe raw identities, including affected old run, peer proven run and fresh peer command IDs, appear in the passing test output. No tokens, proof payload, credentials or account quota are logged.

AC38: `RevokeGrant` leaves the affected owner's participant epoch and full capability set unchanged and retains both authors' waiting inputs. Only its per-issue grant is revoked; exact old group/run is forced stopped with unknown proof, a retained physical reservation and no fabricated termination proof. Accepting the original synthetic sealed proof terminates only that execution and leaves its first input `stopped`. Target Resume without current consent returns `grant_required`. Explicit owner reconsent creates a new grant but retains the hold and never allocates a new run. The refused original Resume remains immutable; only a fresh authorized Resume releases the hold, and the original stopped command and revoked grant remain terminal.

AC50: actual voluntary `Leave` cancels only the resource owner's waiting input and revokes only its issue participant/grants. The other collaborator's waiting input remains. The owner loses the affected private read, then `InviteParticipant` restores a new participant epoch (+2) without restoring the revoked grant, cancelled command or execution hold. The old issued-permit run remains unknown/unsettled and retains its original termination path. Old proof plus fresh consent and fresh authorized Resume are required; rejoin/reconsent/old-refusal replay alone cannot launch or revive old commands. The same owner's exact peer grant/current permissions/history/proof remain unchanged at every step.

## Final command and results

Executed from `/Volumes/artisan/code/2026/multica-0.4.37/server` with Go 1.26.6, task-owned `service_s5`, schema 737 and ambient agent CLI guard. The coordinator serialized the DB slot; it was released immediately after the final race result.

```sh
GOTOOLCHAIN=go1.26.6 python3 /tmp/multica-gap-recovery-20261021-h6phxwo9/with-db.py service_s5 \
  bash ../scripts/go-test-with-agent-cli-guard.sh go test -json -race ./internal/service \
  -run '^(TestCollaborationGrantWithdrawalAndOwnerRejoinIsolateOtherTask|TestCollaborationTargetResumeInvalidGrantIsAtomic|TestCollaborationReconsentHoldRequiresFreshResumeBeforeLaunch|TestCollaborationPermissionRestoreCannotReviveConsent|TestCollaborationWorkflowRemovalRevokesAuthorAndResourceWithoutInventingProof|TestCollaborationResourceRevocationPreservesAllWaitingAuthors|TestCollaborationResourceHoldDoesNotPauseChannel|TestCollaborationTargetResumeForcedActiveGroupRefusesWithoutWrites|TestCollaborationTargetConsentRetainsInitialHold|TestCollaborationWorkflowDepartureOnlyStopsAffectedExecution)$' \
  -p 1 -parallel 1 -count=1 -timeout=30m
```

Result: exit 0, **10 top-level identities / 22 including subtests passed / 0 failures / 0 skips**, package 9.203 s, command elapsed 14.493 s, empty stderr. Start `2026-10-21T12:38:07.851183+00:00` is recorded in `grant-task-isolation-adjacent-final-20261021-command.json`; the result artifact remains authoritative for all timestamps. Raw output SHA-256 `6a514344838ae566151eda183bb732c460cb2ecd2312f104846636400b9c2866`. All ten expected identities appear as actual passes and all eleven recorded source files match before/after hashes. No red assertion or retry occurred; the earlier narrower pass is preserved as the pre-hardening run.

The adjacent identities cover invalid-grant atomic refusal, renewed consent retaining explicit Resume, permission-value restoration not reviving consent, withdrawal preserving waiting authors/channel controls, initial consent hold, pending proof refusing Resume without Stop, and voluntary/removal author-resource isolation. Static final checks (`grant-task-isolation-static-final-20261021*`) pass gofmt/vet with empty streams. This Go-only test has no TypeScript scope; no TS lint/typecheck or full `make check` was repeated.

The exact per-issue production paths were read, not changed: `RevokeGrant` locks owner grants by `IssueID` and revokes only the requested grant/issue; `depart` uses issue+owner grant revocation and session+author waiting cancellation; invalidation stops only matching frozen dependencies; reconsent stores a fresh participant-bound grant and advances hold observation; Resume separately requires current grants, proof and explicit request identity. The new test locks these already implemented contracts without introducing a new production abstraction or dependency.

## Handoff and limits

Changed files: new `server/internal/service/collaboration_grant_task_isolation_test.go`, this report and unique `grant-task-isolation-*` command/result/raw/static evidence. Production/helper source, SQL generation, schemas, matrix/spec/task state and existing work remained unchanged. Reused canonical managed, control, catalogue, submit and fixture helpers; no duplicated production logic or open-coded setup insert chain was added.

This is current backend service evidence for full AC38/AC50 primary-layer clauses, subject to the coordinator's independent review. It does not prove kernel/process termination: the managed cycle accepts test-created sealed proof and no provider executable is launched. Broader Web/Electron owner-exit journeys, paid provider/version/account smoke, native packaging and other OS/S3/Redis scenarios remain independent scopes. Task-only fixture gates do not authorize production rollout; task/parent must remain active until the complete 97-row ledger is verified.

For a Lore test-only commit, root may verify exactly HEAD plus this new file outside the dirty workspace. The new file uses tracked managed/control/catalogue helpers and core production primitives, avoiding untracked S3b history DTO/query dependencies. That commit-isolation check is coordinator-owned and is not claimed from current-tree suite results alone.

## Coordinator independent HEAD validation

The coordinator ran the new grant-isolation identity in the same recorded
exact-HEAD snapshot with only the listed test additions, preserving every
product file at HEAD. Both revoke_grant and leave_rejoin pass, **1 top-level /3
total, zero failures/skips**, exit0. `grant-isolated-head-race-result.json` lists
HEAD, exact command and all added test files; raw JSONL/stderr are retained.
The coordinator also reviewed exact shared owner/agent/runtime/binding identity,
13 nonempty peer relationship/control/history/proof arrays, current catalogue
and access snapshot equality at each affected action, and the positive peer
submission. This is independent backend criterion evidence; synthetic sealed
proof is not a provider process or natural kernel observation.
