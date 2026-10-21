# Offline handoff final review — 2026-10-21

Review of the existing S6 slice in `.trellis/tasks/10-20-collaboration-gap-recovery`. **Recommend AC59 (P6) and AC77 (P2) as passed; keep AC83 (P6) not verified with partial support.** The authoritative matrix remains untouched at its reviewed 70 passed / 27 not verified state. Neither task nor parent is ready to archive.

Both final-source Web and actual Electron journeys pass twice: implementation run3 **2/2**, then the coordinator current-source rerun **2/2**, with zero unexpected, skipped, flaky or retries. Scoped ESLint, strict TypeScript and discovery pass. No new product issue was found in the reviewed path.

## Per-row conclusions

| Criterion / primary owner | Recommendation | Exact evidence and clauses |
| --- | --- | --- |
| AC59 / P6 | Passed | Both clients transfer H's responsibility back to P, read/archive the original transfer notification, close the actual context/application, perform a fresh password login, and open the same record/comment from participating workbench with current total 0, participating total 1 and zero source workspace memberships. Removal produces totals 0/0, explicit private-detail denial, API404, content-free notices and refusal of the captured old notification URL. Canonical workbench/voluntary-exit tests confirm the same revoked-participant predicate. |
| AC77 / P2 | Passed | Recipient's UI client is closed before public invite/assignment. Invite immediately gives participation while P remains handler; transfer immediately moves handler and current workbench responsibility to H. Real login opens directly with no accept/join step. Actual UI original-key replays preserve issue/notification snapshots and one handoff. Canonical current-role/invalid-target/lock-race/rollback tests cover no partial effects; explicit owner consent tests and empty client targets/runs/commands establish that invitation grants no personal runtime. |
| AC83 / P6 | Pending, partial support | Invite, transfer, new comment, removed notice, bounded operator-self suppression, exact comment anchor, read/archive independence and revoked old links are current in both clients. The full offline `information_requested`, `information_returned`, `acceptance_submitted`, `ai_completed`, `ai_failed`, `ai_stopped` journey and full source inbox/subscriber plus post-commit delivery-failure fanout are still absent. |

Exact unmodified criterion text, assertion locations, source hashes and clause assessments are in [offline-handoff-final-review.json](offline-handoff-final-review.json). The four proposed E2E evidence IDs are `offline-handoff-run3-web`, `offline-handoff-run3-native`, `offline-handoff-coordinator-rerun-web`, `offline-handoff-coordinator-rerun-native`; root still owns catalog registration. Backend evidence IDs already registered are `continuation-service-2` and `continuation-go-other`.

AC59's public dual-client denial is participant removal. Voluntary exit is covered at the canonical backend (`TestCollaborationWorkflowVoluntaryLeaveAndMinimalRecovery`) and the same workbench SQL `p.revoked_at IS NULL` predicate; this review does not claim an additional native leave journey. Likewise, closing the client is the offline condition; synthetic recipient API reads continue solely to inspect committed workbench/notification state.

## Assertions and canonical pass events

`exerciseOfflineHandoff` checks client close at lines193–196, immediate invitation/assignment lists at199–211, real reopening/no acceptance at224–232, transfer/archive/client close at234–243, new login and participating records at244–257, and removal/old-link refusal at259–284. `archiveTransfer` checks exact notification IDs and unchanged issue/list snapshots after each read/archive POST (lines155–176). `uiWorkflow` verifies original-key replay and exactly one transfer activity; a noncreator former handler's old POST correctly becomes403 and only its own original operation GET recovers (lines107–146).

These are completed canonical PASS events, not name matches or new Go execution by this reviewer:

| Identity (all below are current PASS) | Artifact line / seconds |
| --- | --- |
| `internal/service::TestCollaborationWorkflowInviteTransferPreservesExecution` | `go-service-partition-2.jsonl`:3585 / 0.25s |
| `internal/service::TestCollaborationWorkflowRequiredWriteFailuresRollbackEverything` | `go-service-partition-2.jsonl`:3569 / 4.25s |
| `internal/service::TestCollaborationWorkflowConcurrentIdentityAndRevision` | `go-service-partition-2.jsonl`:3489 / 0.32s |
| `internal/service::TestCollaborationWorkflowCurrentAuthorityAndPersistentRefusals` | `go-service-partition-2.jsonl`:3485 / 1.88s |
| `internal/service::TestCollaborationOwnerConsentIsExplicitScopedAndIdempotent` | `go-service-partition-2.jsonl`:253 / 0.22s |
| `internal/service::TestCollaborationWorkflowNotificationsAndWorkbench` | `go-service-partition-2.jsonl`:3441 / 0.22s |
| `internal/service::TestCollaborationWorkflowRechecksAccountsAndRolesAfterLock` | `go-service-partition-2.jsonl`:3501 / 0.57s |
| `internal/service::TestCollaborationWorkflowReplaysExactLargeVersionsAndDisabledRefusal` | `go-service-partition-2.jsonl`:3437 / 0.2s |
| `internal/service::TestCollaborationWorkflowVoluntaryLeaveAndMinimalRecovery` | `go-service-partition-2.jsonl`:3413 / 0.25s |
| `internal/service::TestCollaborationResultNotificationsRequireProvenTerminalGroup` | `go-service-partition-2.jsonl`:2881 / 2.51s |
| `internal/service::TestCollaborationResultNotificationRollbackAndConcurrentReplay` | `go-service-partition-2.jsonl`:2885 / 0.58s |
| `internal/service::TestCollaborationNotificationsLoseContentAndAnchorsWithAccess` | `go-service-partition-2.jsonl`:2083 / 0.17s |
| `internal/handler::TestCollaborationWorkbenchKeepsResponsibilityParticipationAndPaginationSeparate` | `go-other-packages.jsonl`:35468 / 0.03s |
| `internal/handler::TestCollaborationWorkbenchWorksWithoutAnyWorkspaceMembership` | `go-other-packages.jsonl`:35472 / 0.01s |

All14 listed top-level end events and their32 child end events are pass with no scoped failures/skips. Source files for these tests match the retained initial source manifest exactly. P2 authority, conflict, atomicity and consent matrices stay in service tests; browser evidence proves actual current wiring. Result notification database-write rollback/concurrent proof replay supports AC83 only partially: it proves atomic storage, not external delivery failure after a committed business operation.

## Current command/results and attempts

Coordinator rerun, from the repository root:

```sh
python3 /tmp/multica-gap-recovery-20261021-h6phxwo9/admin-test.py env pnpm exec playwright test e2e/collaboration-offline-handoff.spec.ts --workers=1 --retries=0 --project=chromium --reporter=json --output=/tmp/multica-gap-recovery-20261021-h6phxwo9/offline-handoff-coordinator-rerun-output
```

Exit0; reporter started `2026-10-21T14:40:10.772Z`, total20.000s. Web8625ms and Electron10034ms, both expected/pass at `e2e/collaboration-offline-handoff.spec.ts:12`. Raw SHA256 `142ac84cbb5b0de47f68dbf5129cdb27a22a5fad517844372e2344371d271e7e` matches its result record.

Run3 used the same command with output `offline-handoff-run3-output`; exit0, started `2026-10-21T14:06:53.086Z`, total25.266s, Web11933ms and Electron11735ms. Raw SHA256 `51109a3bfb72ee88aa8c2976c6f1ae307d5bb880f3085ce58d440d78e854a469`. I decoded its two inline reporter attachments and confirmed exact equality with `offline-handoff-run3-identities.json`.

Run1 remains failed2/2, exit1: both clients reached comment setup then received400 because the earlier fixture sent workflow-only `expected_collaboration_revision` to the comment writer. The fixture correction sends accepted comment authority fields and does not weaken assertions or change product behavior. Run2 remains earlier-source pass2/2; its fixture hash differs from current. Their raw reports, command/preflight/stderr and all first attempts remain retained. The only stderr text in these retained attempts is the existing pnpm configuration warning.

Reviewer commands, all exit0 (including the root-authorized native helper dependency):

```sh
pnpm exec eslint --config packages/core/eslint.config.mjs --rule 'import-x/no-extraneous-dependencies: [error, {devDependencies: true, includeTypes: false}]' e2e/collaboration-offline-handoff.spec.ts e2e/fixtures/collaboration-offline-handoff.ts e2e/fixtures/collaboration-native.ts
pnpm exec tsc --noEmit --module ESNext --moduleResolution Bundler --target ES2022 --skipLibCheck --strict --esModuleInterop --types node e2e/collaboration-offline-handoff.spec.ts e2e/fixtures/collaboration-offline-handoff.ts e2e/fixtures/collaboration-native.ts
pnpm exec playwright test e2e/collaboration-offline-handoff.spec.ts --list --project=chromium --reporter=list
```

Discovery lists exactly two tests and starts no browser or database. Initial two-file versions of lint/tsc also passed. Exact command output is retained in the review JSON; no source edits were needed. This reviewer did not launch live Playwright or Go/DB tests; the root exclusively owns those slots.

## Source and environment assessment

Both source files match current bytes, run3 preflight/attachments and coordinator rerun preflight/attachments:

- `e2e/collaboration-offline-handoff.spec.ts`: `c70c973f61c8b0f09b64da4c5c23a0eb34c120dd7525bc89d750a2de934cdb0e`.
- `e2e/fixtures/collaboration-offline-handoff.ts`: `733fb4d46b603752c518ac316de0fa1258623b9b24cb7902b8be9d415be2fd35`.

Coordinator preflight compares5138 non-test application files with the owned `verification-admin-source` snapshot and records no differences; identity SHA256 `534a2079cf16ea3821c3f3b11a4be40186226f427a2ced763096adc0fa7d49a9`. My narrower supplementary comparison of4652 non-test files from the initial manifest also finds no differences, normalizing only Next's generated route-type import. This is not a replacement for the coordinator current-file comparison.

The API/production Web/renderer are task-owned: API `localhost:18996`, listenerPID68800, source ID `3315c0bc363e1fa5a6b62b8316fabaaa494e9dbb179fa981dd467d1562c19265`, configuration ID `805fc993af4daf895e4de11a22cd372244d7845eed807b7eaa56a50f30b39cd3`; Web `localhost:13916` uses `next start`, modeproduction and build `tP4r55FoDsuyXfi1EylGw`; renderer `localhost:6090` is200 with exact snapshot index `337c08a228217fb574b77bb5fb6942bcbc40f2f1ce42587ceec5264204f3b35c`. Retained running records, run3 and fresh rerun agree. The old API commit label `8ecb98883` is not used as a current-source substitute; fingerprints plus file identity establish current product bytes.

The runner explicitly supplies `COLLABORATION_ELECTRON_SOURCE_ROOT=.../verification-admin-source`. The harness/index/preload are from that snapshot, not the different generated outputs in the dirty root checkout. Harness SHA `f9021ab07635f2f48d0f90613fcd3c8558cb7262b3bdd2aca98edb31654475d2`, tracked Main SHA `2765bc28a321d3f9770a4e7f43170ae0fc0abfd9cffa039ff890d7aaf6fc802c`, preload SHA `e5b95b95037375b8996a84c7e7650b7c13839700b6f3fad7d90f2358e9ae803e`. Raw/configuration IDs were checked as safe retained records and public configuration; I did not read or recompute private environment/credentials.

Fresh login is an actual public `/auth/login` request: Web obtains real HttpOnly auth and CSRF cookies in a new context; Electron calls `loginPassword` and places the freshly returned token in a new own profile before startup. Three recipient logins are asserted in each run. This satisfies AC59/77's login-after-offline state clause. Interactive login-page UX, installed full Main/installer and other OS/provider acceptance remain separate.

Task-only acceptance gates are already enabled in its synthetic database; final execution remains paused/empty. No production gate/rollout/provider-account change is made or inferred.

## Commit dependency review

A two-file offline test commit alone is incomplete because it imports the currently untracked `e2e/fixtures/collaboration-native.ts`. Existing HEAD mention/selection specs already reference this missing module. Root explicitly took ownership of including the existing helper **unchanged**, SHA256 `91d23deacfe20f57f85d7533dc2fd7bdd942d0a1eedbfdb49ae5f200dc7b7e94`, in the offline Lore slice. No duplicated extraction is needed.

The helper has only Node and Playwright imports. It launches the already tracked, HEAD-identical `collaboration-account-electron.mjs`; its direct native Main/shared dependencies are also HEAD-identical. `packages/core/paths/account.ts` differs only in administration routes, which offline view/issue/comment links do not use. HEAD's `e2e/fixtures.ts` already supplies all called password/login/request helpers; its dirty dispatch-count row guard is unused here.

I reviewed loopback validation (fixture/spec plus tracked native harness), token-in-private-profile handling, real routing/IPC/preload usage, first-window failure cleanup, page errors, ambient account headers and the outer owns-and-closes Sets/finally cleanup. No source issue was found. Geometry/screenshot helpers in `collaboration-native.ts` pass static checks but are not called by this offline journey; their previously separate zoom/capture evidence is not newly claimed. Root retains final staging/source ownership and clean-HEAD runtime verification; this reviewer does not touch git index.

## Findings and limits

No mechanical/local source fixes were necessary. AC83's missing full fanout and delivery clauses remain explicit findings outside this slice; they are not product failures. Existing specs already express no acceptance, atomic current-authority writes, independent consent, fresh offline login, read/archive independence and the native harness/packaging boundary; root may link these receipts while preserving limits.

Changed files by this reviewer: this report and its JSON only. Simplifications: none. Remaining risks/untested boundaries: AC83 above, actual login-page UX, clean-HEAD product build, full installed Main/installer, other OS containment, provider-account/quota and monolithic/full-suite verification. Production rollout stays unchanged; child and parent remain in_progress.

Root's new Tests Required entry in `.trellis/spec/desktop/frontend/account-navigation.md` was also reviewed: it accurately captures the observed archive/restart/revoked-entry contract, current-authority POST versus original GET recovery, and fresh API login/actual client versus login-page/full Main limits. It does not expand AC83 coverage.

Post-write verification passes: task context validation (15 implement /16 check entries), `git diff --check`, JSON recommendations, all25 referenced artifact hashes, two current source hashes, native helper hash, and14 canonical top-level/32 child PASS end records.
