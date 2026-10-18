# Independent server/runtime continuation review — 2026-10-18

Reviewed the current tracked and untracked Go/runtime changes against
`bd168d18edb37829203cd97c980d3e36895cce64`, the complete saved Trellis context,
CLAUDE.md, security boundaries, and the target, squad, Switch and shared-skill
specifications. The review includes service/query/migration behavior, runtime
business routes, CLI identity selection, and the final daemon/CLI test source.
No commit, global migration, sqlc generation, deployment or rollout change was
performed by this reviewer.

## Findings (fixed)

- Files: `server/internal/handler/collaboration_commands.go` and
  `collaboration_commands_test.go`.
  A departed actor's cancellation recovery returned a zero-valued full
  `CollaborationQueueReceipt` with `redacted:true`. Its extra execution fields
  failed core's strict seven-field redacted schema, so a successful GET could
  not resolve the original unknown operation. The workbench owner fixed the
  HTTP projection using the existing Switch pattern and added
  `TestCollaborationCommandHTTPRedactedRecoveryOmitsExecutionFields`. The service
  continues to authorize the current account and recover only its original key.
  The reviewer independently reran both command and Switch HTTP regressions;
  both pass with race detection. No schema relaxation or private-state recovery
  was introduced.

## Findings (not fixed)

- Documentation handed to the integration owner:
  `builtin_skills/multica-working-on-issues/references/collaboration-workflow.md`
  still described v4 and whole-group-only nonlaunch return when inspected.
  It and the source map must describe v5, the M11 all-reader exception, the four
  admitted CLI commands, required `--request-id` on writes and explicit static
  `cli_executable`. Root owns this documentation pass; the server specifications
  already describe the reader and leader-order contracts. This is a documentation
  follow-up, not a reason to weaken the implemented boundary.

No unresolved high-severity server/runtime defect was found. Full native
daemon/CLI execution, final combined checks and complete product acceptance
remain with the integration owner; this review does not complete all 97 ACs.

## Verified behavior and review boundaries

- Every input freezes its own target/configuration and complete owner/grant
  generations. Claim persists its prompt once. Current replacement consent
  cannot authorize an old input, and later claims retain independent authors.
- M11 cancellation detaches only unread input/dependencies. Once claimed,
  nonlaunch return checks every immutable prompt reader, preserves old
  memberships/prompts/proofs and retains the barrier while any reader may have
  launched. Root batch return still requires whole-group nonlaunch proof.
- Stop, owner/config invalidation and close keep their distinct lane/hold and
  author rules. Proof uses the original termination identity after business
  authority or resources disappear. A parent proof does not settle a child.
- Migration 724 and the service enforce the same return boundary. Migration
  725 derives active historical turns from unique causal ancestry, refuses
  ambiguous active history, and preserves settled results. New turns are
  allocated under the group lock; final settlement uses that order. Migration
  726 and the other new indexes are separate concurrent builds with registered
  invalid-index cleanup; the new migrations add no foreign keys or cascades.
- Switch stages proposal/owner consent separately and commits only after
  current authority and retained proof checks. Waiting input moves to the new
  lane tail under `switch_hold`; old lanes/history remain. Original-key and
  departed-account Switch recovery retain their respective response contracts.
- Actual skill/file/plugin/binding writers advance enabled agent epochs and
  durable invalidation, preserving exact no-op bundles and presentation-only
  changes. File/binding set diffs replace delete/reinsert churn. No dependency
  or alternate execution path was added.
- The CLI selects only the five explicit run-identity variables, rejects
  malformed/unsupported invocation before ordinary profile/client fallback,
  checks the returned run/issue identity, and requires a stable write UUID.
  The controller copies an explicitly configured static CLI through its ELF
  checks and exposes only the admitted business identity.
- The final `TestCollaborationSquadCLIAndDaemonCapacityOneRoutesLinux` source
  drives production `runBatchPoller` with `MaxConcurrentTasks=1`, real
  registration/router/database, test-built CLI commands, an M11 input from a
  second author, per-claim slot observations and three distinct native proof
  ACKs. This reviewer inspected that path; its native execution result belongs
  to root's combined Linux run. Host tests do not substitute for Linux evidence.

## Verification

Environment: Go 1.26.6, repository agent-CLI guard, and the dedicated
`postgres://postgres@127.0.0.1:51974/collaboration_review?sslmode=disable`
database through migration 726. Migration tests use disposable private schemas.
No installed AI CLI or paid account was used.

Evidence directory:
`/var/folders/3n/gbt3p39s5pdc4l55js62gxnm0000gn/T/multica-cwc-resume-20261018-23vtl_pb`.
Counts below distinguish top-level tests from subtests; repeated regression
checks are not additional product acceptance.

| Check | Result | Log |
| --- | --- | --- |
| Service collaboration race suite | 274 top-level + 463 subtests pass | `runtime-check-focused-race.jsonl` |
| Handler collaboration race suite | 52 + 69 pass | same |
| Migration boundary race suite | 17 + 16 pass | same |
| Migration runner/recovery race suite | 3 + 4 pass | same |
| CLI identity race suite | 2 + 14 pass | same |
| CLI command race suite | 6 + 14 pass | same |
| Host containment refusal, ledger and client race suite | 15 + 9 pass | `runtime-check-host-router-race.jsonl` |
| Real router business/Switch/projection/handoff race tests | 5 pass | same |
| Corrected cancellation and Switch redacted HTTP regressions | 2 pass | `runtime-check-redaction-fix-race.jsonl` |
| Go type-check/build via `go test -p 2 -run '^$' ./...` | All 82 packages pass | `runtime-check-typecheck.log` |
| Go vet across affected server/CLI/daemon packages | Pass, including final handler/router/daemon rerun | `runtime-check-vet.log`, `runtime-check-vet-final.log` |
| gofmt and `git diff --check HEAD -- server` | Pass; 91 changed/new Go files checked | `runtime-check-format-final.log` |

The completed review suites above have zero failures and zero skips. The first focused
command selected no collaborationrun tests; only the explicit host suite above
is credited for that package. TypeScript and native Web/Desktop verification
remain with their assigned reviewers. Creation/execution rollout remains closed.

## Follow-up: broad-suite failures

The subsequent full affected-package run exposed configuration fixture
contamination and a missing workspace-deletion classification. Both were
reproduced serially on `collaboration_review` before repair; these were not
dismissed as concurrent-run noise.

### Fixed

- `server/internal/service/collaboration_config_invalidation_test.go`:
  the fixture used the public deployment outbox with a limit of one. An older
  unrelated resource tombstone could complete without reaching the intended
  source lock or injected event/cursor failure, returning `1, 1, nil`. A
  controlled older-orphan regression reproduced this before repair. The fixture
  now shadows only the configuration outbox and execution gate in a disposable
  private schema, retaining the actual public resource/source tables and their
  production writer triggers. The fairness test explicitly shares that scope
  between its blocked and healthy resources. The final isolation regression
  locks the public outbox and requires the fixture's own source to progress;
  it does not assert ownership over another package's public work. Existing
  three-attempt NOWAIT, event/cursor rollback, unknown-commit, proof and restart
  assertions remain unchanged. Fixture teardown drops its private schema instead
  of removing configuration intents from the shared public outbox.
- `server/internal/handler/workspace_delete_manifest_test.go`:
  all 32 collaboration tables were absent from the manifest. They are explicitly
  retained: account/deployment state is independent of the workspace, source
  workspace deletion is refused while collaboration records exist, and
  execution-workspace removal cannot delete source history or original run and
  termination identities. No deletion query, foreign key or cascade was added.
- `server/internal/handler/collaboration_boundary_test.go`:
  the source-deletion regression now covers `todo`, `done` and `cancelled`,
  requires the exact `workspace_has_collaboration_records` refusal, and compares
  complete workspace/issue/collaboration/session/participant/comment/event
  snapshots before and after the request. Existing runtime teardown tests also
  recheck retained reservations, original termination identity and later proof.

Production reconciliation, authentication, migration and deletion code did not
change in this follow-up. Its fixes are confined to the three test/manifest
files above and this report.

### Verification

All checks again use Go 1.26.6, the agent-CLI guard and the same isolated review
database. The final run deliberately used `-race -p 2` for service and handler
packages against the same database, matching the broad run's package concurrency.

| Check | Result | Evidence |
| --- | --- | --- |
| Original failures, serial reproduction | Reproduced source-busy, event/cursor and manifest failures | `runtime-check-followup-reproduce.jsonl` |
| Controlled older public tombstone | Failed before fixture isolation | `runtime-check-followup-outbox-red.jsonl` |
| Initial configuration and source-guard checks | Pass | `runtime-check-followup-config-race.jsonl`, `runtime-check-followup-retention-race.jsonl` |
| Final configuration/squad/shared-skill/runtime-teardown service tests | 18 top-level + 29 subtests pass | `runtime-check-followup-parallel-race.jsonl` |
| Final handler config/shared-skill/manifest/source-retention tests | 9 top-level + 9 subtests pass | same |
| Go vet / affected-package type compilation | Pass | `runtime-check-followup-vet.log`; final race build |
| gofmt / diff whitespace | Pass | `runtime-check-followup-format.log` |

The final 65 test/subtest executions have no failures or skips. The main session
owns integration with the separate ordinary-grant concurrency repair and its
final broader result; this follow-up does not claim the entire earlier broad
run was green.
