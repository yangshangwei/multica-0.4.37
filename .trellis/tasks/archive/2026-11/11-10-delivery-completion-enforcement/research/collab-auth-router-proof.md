# Real-auth collaboration completion route proof

Date: 2026-11-11. Owner: native trellis-implement / implement_batch_client.
Scope: only `server/cmd/server/delivery_completion_collaboration_integration_test.go` and this report. No production, schema, dependency, route registration, task state or spec was changed by this lane.

## What was actually exercised

All requests go through the actual `NewRouter` middleware. No authenticated principal/password-session context is injected. Source issuer and outside initiator are distinct test accounts; the initiator has no source-workspace membership. Legacy users use valid fixture-signed JWTs that the real middleware verifies. The R38 administrator obtains its credential from actual `/auth/login` with a valid hashed synthetic password, current super_admin role and active source-organization mapping.

The parent collaboration/account/session rows are controlled test fixtures. The ordinary source child is created through real `POST /api/issues`; its returned parent and inherited project are asserted. A fixture ordinary agent/runtime/task is bound **only to that child**, never to the shared parent. Real task-token authentication drives the existing artifact create/approve/execute routes. The existing archive helper verifies the published tar entry, actual bytes, SHA and EOF; no successful artifact or receipt is seeded. A real observe-policy/evaluation call also creates an issuer-private evaluation before the test installs the internal enforce revision.

The three positive paths are external submission, self-result, and R38 administrative completion. Each creates its grant through the Auth-only endpoint and invokes its actual lifecycle route. Assertions bind the committed issue revision/status, closed queue, single consumed terminal, actual original collaboration operation, exact generated/submitted result and actual actor. R38 additionally binds the real admin operation and proves no fabricated initiator accepted result or source membership. Repeated original POST plus original-key GET create no extra event/terminal, retain real tar bytes, and return only minimal consumer facts.

The private-evaluation test verifies an outside initiator cannot read the issuer's original evaluation; existing and nonexistent issue/key requests have the same refusal status/body and disclose no private evidence. Missing credentials and a real task credential with forged human headers fail through middleware/workflow, without consumption. Current disabled initiator, revoked participant, detached child (through actual ordinary PUT), and revoked source issuer all produce definite 4xx refusals and preserve status/handoff/events/terminal. Unknown service 5xx is not counted as a valid negative proof.

The self-result attachment test uploads actual multipart bytes through NewRouter. Missing grant refuses before binding any draft or creating a handoff; grant issuance is also read-only for the draft. An explicitly new authorized completion binds that exact attachment and consumes once in the same workflow.

## Verification and limits

Final ordinary run **11 named pass / 0 fail / 0 skip**, consisting of 9 leaf cases plus 2 parent tests, in the fresh root-allocated database `enf_p3_collab_auth04`. Actual filter: `^TestDeliveryCompletionCollaborationRouter`. Runs use compiled binaries through `go tool test2json -t -p github.com/multica-ai/multica/server/cmd/server`, with the repository agent-CLI guard. No zero-selection or TestMain bypass is claimed.

`go vet ./cmd/server` passed; the owned test file is gofmt-clean and the owned diff has no whitespace errors. This lane did not run race; root owns integrated final normal/race and public policy activation. The internal enforce row is an explicitly labelled **pre-activation authentication/writer proof**, not evidence of public enforce rollout, browser acceptance, provider execution, full M1 or six-business completion.

The parent allocated and migrated every listed database. This lane only ran the exact approved binary/filter against each named fresh router database, never created/migrated/dropped/switched or reused a historical database, and made zero real-provider calls. Logs and failed databases remain retained.

## Failure history

- `collab-auth-router01`: 0 pass / 11 fail. Fixture setup omitted the built-in status catalog when seeding a collaboration directly; actual StatusOptions returned409 status_unavailable. R38 also found no internal organization in its fresh DB. These failed before completion admission, so they are fixture failures, not product red. Added the same production SeedIssueStatusEntries query used by existing collaboration fixtures and a testutil-created active internal organization. No assertion or product rule was relaxed.
- `collab-auth-router02`: 9 pass / 2 fail (R38 and its parent test). The synthetic admin username exceeded NormalizeUsername's32-character maximum, so actual password login returned400 invalid_request. Changed the fixture username to a32-character UUID string; kept real password login and all role/session checks. This is a fixture input failure.
- `collab-auth-router03`: 9 pass / 2 fail (revoked issuer and parent). After tightening negatives to reject5xx, the actual missing issuer authority surfaced as503: completionCollaborationAccess returned pgx.ErrNoRows, which was not translated before the collaboration error adapter. This is an error-protocol defect, not a pass and not a demonstrated authorization bypass. The root/checker fixed the production classification to content-free404. The original third-run assertion stopped at the unexpected status; it is not retroactively presented as having checked all rollback assertions.
- Before run04, diagnostics were changed to record the actual status/body and continue atomicity assertions even when classification is wrong. Positive cases also gained exact operation/submission/actor linkage assertions. Run04 passed those stronger assertions against the corrected source.
- All four compile commands succeeded. Compile01's before/after manifest observed a concurrent change only in `service/delivery_completion_receipt_test.go`, which is not compiled into the cmd/server test binary; compile02/03/04 had no source-manifest drift. The final proof binds compile04 below.
- Read-only lookup commands initially guessed a nonexistent queries/platform_admin.sql and a delivery_completion_request.go candidate; `rg --files`/symbol reads found actual organization.sql and delivery_completion_types.go. These were source-inspection errors/no-results, not product tests or DB actions.

| Run | Exact DB | Exit | Named pass / fail / skip |
| --- | --- | ---: | --- |
| collab-auth-compile01 | none | 0 | 0 / 0 / 0 |
| collab-auth-router01 | enf_p3_collab_auth01 | 1 | 0 / 11 / 0 |
| collab-auth-compile02 | none | 0 | 0 / 0 / 0 |
| collab-auth-router02 | enf_p3_collab_auth02 | 1 | 9 / 2 / 0 |
| collab-auth-compile03 | none | 0 | 0 / 0 / 0 |
| collab-auth-router03 | enf_p3_collab_auth03 | 1 | 9 / 2 / 0 |
| collab-auth-compile04 | none | 0 | 0 / 0 / 0 |
| collab-auth-router04 | enf_p3_collab_auth04 | 0 | 11 / 0 / 0 |
| collab-auth-vet01 | none | 0 | 0 / 0 / 0 |

## Frozen final identity

- Owned test SHA-256: `8614d602e5ab82097d871315cf795a4d48e11d5a0c5777f0ee167c5f7c75fe96`.
- Binary: `/private/tmp/multica-completion-enforcement-20261111-y9c0v8ro/binaries/collab-auth-router04`.
- Binary SHA-256: `eed208ba2f41e04132f338ea9b7213ef7bbe0cd9d897109e6a5dfb36a6b9f6e3`.
- Compile record: `research/runs/collab-auth-compile04.json`; source_before and source_after are equal, and the test hash matches current bytes.
- Test record: `research/runs/collab-auth-router04.json`; raw log SHA rechecked, all started names have one terminal, no duplicate or missing terminal.
- Vet record: `research/runs/collab-auth-vet01.json`, exit0.

The parent must compare later final source to this compile manifest before treating this binary as evidence for subsequent edits. This report does not claim later P4 changes were executed by this binary.

## Public policy fixture upgrade (2026-11-11)

After P4 opened public enforce admission, root assigned a test-only upgrade of this lane's owned fixture. It now calls the actual policy PUT with the original source-owner JWT, expected revision1 and mode=enforce after the prior observe revision/private evaluation. The response must report enforce/revision2 and the actual issuer as created_by. The owner membership is checked unchanged; no temporary role escalation or consumer source membership is introduced. All eleven named completion/authentication/privacy/atomicity cases remain.

The test header now describes the public transition. No production code, other test fixture, migration, dependency or lockfile changed. Historical auth04 above remains an internal-policy pre-activation proof; it is not relabelled as having exercised this new policy PUT.

- Compile-only record: `research/runs/collab-auth-public-compile01.json`, exit0; no tests or database connection executed by this command.
- Updated owned test SHA-256: `1b37f836ac441fb1c81a3d535810a7dc866d9dd555795cf254d1efc745ddc6ad`.
- New binary: `/private/tmp/multica-completion-enforcement-20261111-y9c0v8ro/binaries/collab-auth-public-router01`.
- New binary SHA-256: `03ad42ec57763667c681f619c5dd1aff9711a5746b18c8c22855f7088dc355b6`.
- Source changes during compile: [].
- Gofmt and owned diff whitespace checks passed. Root owns the fresh final ordinary/race execution of this public-policy fixture; this supplement makes no unrun success claim.
- One initial read guessed `delivery_completion_activation_integration_test.go`; rg --files found the actual `delivery_completion_enforcement_integration_test.go`. A subsequent progress glob had no match. Both are read-only inspection failures with no product or database side effect.
