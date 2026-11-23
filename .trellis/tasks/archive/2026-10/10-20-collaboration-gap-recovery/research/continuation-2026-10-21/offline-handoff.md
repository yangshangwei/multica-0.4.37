# Offline invitation, transfer and retained workbench — 2026-10-21

Scope: only new `e2e/collaboration-offline-handoff.spec.ts`, its fixture and this report/evidence. Existing shared product/helpers, configuration, matrix/catalog and runtime settings remain read-only. Public password accounts and API/UI collaboration writes drive the Web/Electron journey; no database seeding, fake response, provider/account quota, gate change, task creation, replanning, commit or archive.

The fixture closes the recipient's actual client before public invitation/assignment, requires an actual fresh password login before reopening, checks immediate workbench participation/responsibility without an acceptance action, transfers to creator, archives the original transfer notification, closes/reopens with another real login and verifies participating versus current lists and the comment anchor. Removal revokes the old workbench/notification/private-detail entry. **Final current-source execution passes Web and actual Electron, 2/2**. Recommend independent AC77/59 review/promotion; AC83 remains partial.

## Current command and result

From the repository root:

```sh
python3 /tmp/multica-gap-recovery-20261021-h6phxwo9/admin-test.py env pnpm exec playwright test e2e/collaboration-offline-handoff.spec.ts --workers=1 --retries=0 --project=chromium --reporter=json --output=/tmp/multica-gap-recovery-20261021-h6phxwo9/offline-handoff-run3-output
```

Exit 0; **2 expected / 0 unexpected / 0 skipped / 0 flaky**, retry 0. Started `2026-10-21T14:06:53.086Z`, total 25.266s. Exact identities at `e2e/collaboration-offline-handoff.spec.ts:12`:

- `Web offline invite and assignment need no acceptance; transferred-away work survives archive and fresh login until removal` — 11933ms.
- `Electron offline invite and assignment need no acceptance; transferred-away work survives archive and fresh login until removal` — 11735ms.

| Artifact in this directory | SHA-256 |
| --- | --- |
| `offline-handoff-run3.json` | `51109a3bfb72ee88aa8c2976c6f1ae307d5bb880f3085ce58d440d78e854a469` |
| `offline-handoff-run3-identities.json` | `0a5b5f237fe783975dc21df863a25f45a57a57fa9cfaeb29e4f5fe94ed0929e5` |
| `offline-handoff-run3-preflight.json` | `e841c5abf5692ad3f59d694ca10d915ec0f3a1db9a7e59913d1a0a4b18b0fbcb` |
| `offline-handoff-run3-command.json` | `8bf22e635a2185a5217b06c32666786a6d6acd61dfb37fc8d32bca4f455ef8bc` |
| `offline-handoff-run3.stderr` | `1a585257c58d9c0b02938c759c56c62971c922be22bdf610a77bfbaa9528539f` |

Decoded identities are exactly the reporter's two inline evidence attachments. They contain public task-owned IDs, exact request/outcome bodies, workbench/notification snapshots and source hashes; passwords/JWT/cookies are not retained. Tracing is off. Stderr contains only the existing pnpm settings warning.

## Proven behavior and snapshots

Both clients run the same complete public flow with separate synthetic accounts/issues. The recipient client/context/application is closed before invite and transfer. Opening each recipient client performs an actual password login: one before the offline period, one after offline assignment and one after archive/restart; no earlier token establishes the fresh-login step. Electron then loads the freshly obtained JWT into a new isolated profile through the established actual native harness.

| Stage | Creator current / participating | Recipient current / participating |
| --- | --- | --- |
| Invite committed while recipient offline | 1 / 1 | 0 / 1 |
| Assign/transfer committed while recipient offline | 0 / 1 | 1 / 1 |
| Recipient transfers back to creator | 1 / 1 | 0 / 1 |
| Read/archive, close, fresh login and reopen | 1 / 1 | 0 / 1 |
| Creator removes former handler | 1 / 1 | 0 / 0 |

The exact lists/IDs/counts for each stage are retained in `workbench_snapshots`, not inferred from UI text. Creator invite and assignment run through the actual shared forms, using exact-recipient lookup and explicit Confirm. Replaying their original POST keys returns original outcomes without issue/notification/handoff duplication. Freshly logged-in recipient has immediate list/detail/transfer capability and no accept/join step; the recipient has **zero source workspace memberships** throughout. Targets/commands/runs remain empty: participation does not grant a personal runtime or consent.

Recipient's transfer-back runs through the actual shared UI. Current role revalidation then refuses that noncreator former handler's old POST with 403; the original operation GET recovers its prior transfer outcome without changing responsibility or creating another handoff. The creator's permission is preserved. This is the actual contract, not a relaxed replay assertion.

The original located transfer notice is marked read and archived through UI POSTs; each confirms the exact notification identity and leaves business/queue/authority snapshots unchanged. After actual client close and fresh login, current list count is zero, participating list still opens the same private issue/history, and the retained comment notification opens/focuses its exact comment anchor. Archive remains recorded without losing business work.

After public participant removal, both clients refresh the old detail and see the explicit access denial; the title/private comment disappear. Workbench lists are empty, all retained notifications become `can_open:false`, null title/source ID, and only a content-free `removed` notice is added. Notification UI contains no link to the former private title. Navigating the captured old notification href still refuses; direct API detail is 404. The independent creator retains the issue, handler identity and paused/empty execution. Both clients have zero page errors; native account requests have no ambient workspace headers and no daemon start.

## Existing canonical clauses

Current actual PASS records support P2 atomicity, current authority, invalid-target refusal and separate consent without repeating their matrix through the browser:

| Canonical identity | Current artifact / elapsed |
| --- | --- |
| `internal/service::TestCollaborationWorkflowInviteTransferPreservesExecution` | `go-service-partition-2.jsonl` / 0.25s |
| `internal/service::TestCollaborationWorkflowRequiredWriteFailuresRollbackEverything` | `go-service-partition-2.jsonl` / 4.25s |
| `internal/service::TestCollaborationWorkflowConcurrentIdentityAndRevision` | `go-service-partition-2.jsonl` / 0.32s |
| `internal/service::TestCollaborationWorkflowCurrentAuthorityAndPersistentRefusals` | `go-service-partition-2.jsonl` / 1.88s |
| `internal/service::TestCollaborationOwnerConsentIsExplicitScopedAndIdempotent` | `go-service-partition-2.jsonl` / 0.22s |
| `internal/service::TestCollaborationWorkflowNotificationsAndWorkbench` | `go-service-partition-2.jsonl` / 0.22s |
| `internal/handler::TestCollaborationWorkbenchKeepsResponsibilityParticipationAndPaginationSeparate` | `go-other-packages.jsonl` / 0.03s |
| `internal/handler::TestCollaborationWorkbenchWorksWithoutAnyWorkspaceMembership` | `go-other-packages.jsonl` / 0.01s |

These are parsed completed current-task executions, not new Go runs by this lane. AC77 combines P2 current-role/target/failure/consent evidence with public offline no-acceptance UI; AC59 adds the complete P6 actual close/archive/login/workbench/revoked-entry flow.

AC83 remains **partial**: invitation, transfer, new comment, removed redaction, operator self-notification avoidance and read/archive independence are current. Missing full dual-client kinds are `information_requested`, `information_returned`, `acceptance_submitted`, `ai_completed`, `ai_failed`, `ai_stopped`; broader source-member inbox/subscriber dedup and delivery-failure fanout are also not newly demonstrated by this journey. No full AC83 promotion is recommended.

## Source / runtime provenance

Final current-source hashes:

- `e2e/collaboration-offline-handoff.spec.ts`: `c70c973f61c8b0f09b64da4c5c23a0eb34c120dd7525bc89d750a2de934cdb0e`.
- `e2e/fixtures/collaboration-offline-handoff.ts`: `733fb4d46b603752c518ac316de0fa1258623b9b24cb7902b8be9d415be2fd35`.

Owned task API `localhost:18996`, health PID 68800, commit label `8ecb98883`, API source fingerprint `3315c0bc363e1fa5a6b62b8316fabaaa494e9dbb179fa981dd467d1562c19265`, configuration fingerprint `805fc993af4daf895e4de11a22cd372244d7845eed807b7eaa56a50f30b39cd3`. Production Web `localhost:13916`, build `tP4r55FoDsuyXfi1EylGw`; native renderer `localhost:6090` responds 200 and matches built index SHA `337c08a228217fb574b77bb5fb6942bcbc40f2f1ce42587ceec5264204f3b35c` before each invocation. Root's explicit native slot release precedes execution. No environment process/configuration/build change was made here. All native applications/windows/temporary profiles closed; the slot was released after run3.

## Retained attempts and checks

- Run1: both actual clients completed offline invitation/assignment, then the plain-comment fixture returned 400 because it sent workflow-only `expected_collaboration_revision`. The fixture now sends only accepted comment authority fields. No product change or assertion removal.
- Run2: 2/2 passed. Retained as successful earlier-source evidence; run3 repeats after adding the complete workbench snapshots to its attachment.
- Run3: final current-source 2/2 pass above, zero retries/skips/flaky. All run1/run2 reports/command/preflight/stderr remain alongside run3.

Final scoped checks:

```sh
pnpm exec eslint --config packages/core/eslint.config.mjs --rule 'import-x/no-extraneous-dependencies: [error, {devDependencies: true, includeTypes: false}]' e2e/collaboration-offline-handoff.spec.ts e2e/fixtures/collaboration-offline-handoff.ts
pnpm exec tsc --noEmit --module ESNext --moduleResolution Bundler --target ES2022 --skipLibCheck --strict --esModuleInterop --types node e2e/collaboration-offline-handoff.spec.ts e2e/fixtures/collaboration-offline-handoff.ts
pnpm exec playwright test e2e/collaboration-offline-handoff.spec.ts --list --project=chromium --reporter=list
```

All exit 0; discovery lists two tests. No ESLint/type diagnostics beyond the existing pnpm settings warning. Report/raw JSON/source hash, Trellis and whitespace checks follow final writing. Product/backend/full unified/native packaging suites are not repeated.

Changed files: the new spec, fixture, this report and named offline-handoff evidence artifacts only. Simplification: none; no product/dependency/authority changes. Remaining limits: AC83 above, interactive password-login-page UX (actual API login plus fresh authenticated client is used), installed full Main/installer and other OS/provider/fault acceptance. Task/parent remain in progress; no commit, archive or production rollout action.
