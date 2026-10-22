# Control and administration evidence review — 2026-10-21

Status: bounded independent review complete. No product defect was demonstrated by this review; the findings below concern acceptance coverage and evidence linkage. Only this report was written. No shared ledger, task status, product code, service, build, database, gate or commit was changed.

## Review basis and source boundary

Read the saved native check-hook context, current handoff/progress/acceptance audit/implementation plan, parent acceptance clauses, relevant server/client/native/E2E specs, six control/administration source files, `s6-mention-journeys.md`, `s5-administration-browser.md`, `acceptance-matrix.json` and `s6-evidence-map.json`. The explicit existing-task continuation takes precedence over stale `no_task` breadcrumbs; no task was created or replanned.

The coordinator separately confirmed current `server/`, `apps/` and `packages/` source equivalence with both retained verification snapshots (6994 files). This review independently checked the three administration test hashes and the raw result/catalog hashes listed below. It did not inspect private environment credentials or independently restart/probe the environments.

The control findings describe the initial source snapshot. Another assigned implementer, `control_journeys`, is extending those three files concurrently; its changes and later executions need their own evidence. Initial SHA-256 identities:

| File | SHA-256 |
| --- | --- |
| `e2e/fixtures/collaboration-control-journey.ts` | `d05be04a6ac4f93bba3206c5a6ae3d4f7e1e25b3dd990a963bf2e64368b59346` |
| `e2e/collaboration-control-journey-web.spec.ts` | `ccf64ec4f5ec25e0163a5c381a47149e0e2b1b883b34a2cf399a484bce711fdc` |
| `e2e/collaboration-control-journey-desktop.spec.ts` | `841852319c72b05df33b10a281f9624def3b6507f3670289706e22a481df729f` |
| `e2e/fixtures/collaboration-administration.ts` | `9477c31baf2cfba9eaf05fee4b694acd5b0a0cf38e5ecca06a28c548419468f3` |
| `e2e/collaboration-administration-web.spec.ts` | `c21ac3ab9b0454577b06138295f30ae739d8f663101917c54cd48d2b0cd85a0c` |
| `e2e/collaboration-administration-desktop.spec.ts` | `b04bf8e9d3be24695c72feb16ac731dca0e9cc5e5582adfefcd507b497a3b79b` |

## AC33 and AC95: exact remaining scope

**AC33 is not the Stop/Resume criterion.** Parent `acceptance.md:41` requires different collaborators across Web/Desktop to submit to independent A/B targets, B to execute without waiting for A, coalesced later A inputs, single-input cancellation, and exact target/submitter/run attribution without client duplication. The current control fixture never submits a command or starts a run (`readControlIssue`, lines 161–172). Empty-lane control success cannot promote AC33. Keep its cross-client execution/coalescing journey open; existing Linux/service scheduling evidence remains separate partial support.

**AC95 explicitly requires both clients' agent, squad and whole-issue controls**, independent channel state, issue-pause fences, proof/grant-dependent partial Resume explanations, original-key replay and later explicit Resume of the remaining eligible target. The initial fixture has useful bounded coverage, but does not satisfy that complete criterion:

| Clause | Actual initial source/assertion | Missing or required evidence |
| --- | --- | --- |
| Direct agent and whole-issue controls | Fixture lines 103–142 creates agents A/B in distinct channels; shared journey clicks B Resume/Stop/Resume and whole Resume/Stop. | Production Web and actual Electron executions were not present for these new tests at review time. |
| Squad Stop/Resume | Setup creates only `kind: "agent"`; projection asserts exactly two targets/channels, no groups/runs. | Add an independently offered/adopted/fully consented squad and exercise its actual control target in both clients. Kernel/member-descendant stopping stays in the canonical backend/native matrix. |
| Untouched lanes and issue-pause fence | Initial B Resume returns `issue_paused`; B-only Stop/Resume compares A's full channel/hold and the issue snapshot. | Preserve these checks when adding S; compare every unselected lane/hold, including S. Same-channel alias/descendant matrices need not be repeated through DOM. |
| Partial grant refusal | Real A-owner grant revocation precedes whole Resume. Receipt is `partially_resumed`, A is `grant_required`, B is `hold_released`; exact counts/reasons and absence of an all-resumed claim are asserted. | This is grant-dependent partial Resume with zero unsettled runs. Do not describe it as observed partial termination proof. |
| Partial proof explanation | `readControlIssue` enforces `runs: []`; every returned channel has `unsettled_runs === 0`; Stops return `already_ended`. | A bounded real control receipt with an explicitly synthetic pending-proof barrier can verify the two clients' explanation. It cannot prove real process termination, containment or proof recovery. The canonical component test `packages/views/collaboration/execution-panel.test.tsx:107` already checks a paused `Termination unconfirmed` lane and grant refusal; the UI renders reasons in `execution-panel.tsx:63–69` and `labels.ts:50`. |
| Original receipt and remaining target | The lost partial response is recovered after reload by one original-key GET; all five historical GETs equal their original receipts. | A is never reauthorized then explicitly resumed alone. Add M04's fresh selected Resume and ensure old partial GETs stay immutable. Original GET recovery and repeated POST replay are distinct evidence scopes. |
| Whole stop/new targets/removal/close | Whole Stop covers both empty lanes and preserves business status. | Later target admission, all existing descendants, removed authors' waiting inputs, whole lifecycle and M03 grant association remain canonical service/native responsibilities. Link their actual executions; do not infer them from an empty-lane UI Stop. |

Exact initial identities:

- `e2e/collaboration-control-journey-web.spec.ts:6`: `Web controls recover partial Resume, isolate target lanes and stop the whole issue`.
- `e2e/collaboration-control-journey-desktop.spec.ts:10`: `Desktop controls recover partial Resume, isolate target lanes and stop the whole issue`.

The implementer received these gaps and is adding S, A reconsent/fresh selected Resume, and a separate public List→Claim→Permit pending-barrier case. Those are in-flight work, not passing evidence in this report. Its proposed fixture starts no provider process and posts no termination proof; retain that distinction after execution. AC33's real parallel execution and post-proof-arrival behavior remain open.

## S5: reusable current evidence and unexercised integration branches

The handoff/progress statement that the independent S5 Web/Desktop journey remains unrun is superseded by actual current-task results already present in `research/s5-administration-browser.md` and catalog IDs `web-s5` / `native-s5`. All three current administration source hashes above equal that report's delivered identities.

| Retained artifact under `/tmp/multica-gap-recovery-20261021-h6phxwo9/` | Independently checked result | SHA-256 |
| --- | --- | --- |
| `s5-administration-web-first.json` | Failed fixture expectation; 0 expected / 1 unexpected / 0 skipped. Preserve this attempt. | `bead3fdb0232b85730761895fa6a357ee4e637767f4729cdcddf3933df7e1d2a` |
| `s5-administration-web-second.json` | Chromium, 1 expected / 0 unexpected/skipped/flaky; test 3837 ms, retry 0, start `2026-10-21T01:40:39.404Z`. | `5015dc6432cd4705a0446ea3ba0c4d46442caafa93da6fbec5f104cee67f7bc9` |
| `s5-administration-native-first.json` | Actual Electron IPC harness, 1 expected / 0 unexpected/skipped/flaky; test 8539 ms, retry 0, start `2026-10-21T01:44:05.574Z`. | `b0dc43cb8b6e4c830a5da09a86f70559d0e76002f3dd1fecc764bd05f2b77ab0` |
| `admin-source-manifest.json` | Present; hash equals both catalog entries. | `03f3947e5c7c472968ea44090c79e7e228809efb8ca69e577a91672e98ec686c` |

Exact identities:

- `e2e/collaboration-administration-web.spec.ts:61`: `Web administers disabled-creator collaborations with explicit succession and original receipt recovery`.
- `e2e/collaboration-administration-desktop.spec.ts:11`: `Desktop administers disabled-creator collaborations in the account shell and recovers the retained receipt`.

Environment in the retained catalog: password auth, platform administration enabled, synthetic identities, database `multica_gap_api_admin`, API `localhost:18996`, production Web `localhost:13916`, build `tP4r55FoDsuyXfi1EylGw`, native renderer `localhost:6090`, source ID `58b3fa60203026715322f15875f607df0aac1487a544f7d277b43aa137ed35ef`. These are retained-run facts, not this review's listener-health claims.

The actual shared journey (`collaboration-administration.ts:244–398`) verifies active-creator refusal; real disabling of P; complete/reopen-with-healthy-H/cancel; disabling H; no implicitly selected successor; reason-only validation without a POST; explicit existing C; successor workbench responsibility; creator-preserving audit/handoff/history correlation; four distinct lifecycle writes; no administrator ordinary access or participation; paused reopen with no runs; P restoration; new-action refusal and original GET recovery on remount. Web also loses a real committed response and recovers via automatic GET (`:301–319`); Desktop remounts a retained receipt, but does not lose its own POST response. Native authentication injects the synthetic password JWT into an isolated profile; only Web exercises the username/password form and HttpOnly session creation.

The following branches are absent from these two E2E journeys. They are **not newly demonstrated product bugs**, and most already have canonical DB/component coverage:

| Unexercised E2E branch | Existing canonical identity/source | Bounded follow-up |
| --- | --- | --- |
| Selected C disabled, revoked/left, password-restricted or no eligible candidates; refresh/re-enable must not auto-select C | `admin_collaboration_authority_test.go:166`, `TestAdminCollaborationSuccessorIsConditionalAndCurrentlyEligible`; `collaboration-page.test.tsx:191`, `clears an expired successor and never auto-selects them when a later refresh restores eligibility`; actual form clears vanished selected IDs at `collaboration-form.tsx:33–35`. | If more native/browser wiring evidence is needed, one public-account candidate-expiry/refresh journey is sufficient; keep the full eligibility matrix at the canonical layer. |
| Admin role/session/organization/source scope changes, concurrent creator restoration or handler change | `admin_collaboration_transaction_test.go:18`, `TestAdminCollaborationRechecksAuthorityAndResponsibilityAfterLockWait`, including `admin_role`, `admin_password_version`, `admin_disabled`, `creator_restored`, `candidate_disabled`, `candidate_left`, `candidate_password_restricted`, `handler_changed`, `source_scope_removed`, `organization_inactive`; `:107`, `TestAdminCollaborationAuthorityFencesRemainUntilCommit`. Shared view tests at `:178`, `:203`, `:215`, `:224` cover stale revision, denied reads, observer role and actor switch. | Link these deterministic DB results. An additional UI assertion should target visible form/cache teardown or explicit reconfirmation, not reproduce transaction locks through DOM. |
| Same-key POST replay, changed-candidate payload conflict and simultaneous reopen | `admin_collaboration_authority_test.go:491`, `TestAdminCollaborationRecoveryPreservesOriginalKeyAndSuccessor`; `admin_collaboration_transaction_test.go:356`, `TestAdminCollaborationConcurrentOriginalKeyHasOneHandoff`. | The E2E suite proves read-only recovery and one handoff, not concurrent POST replay. Link the canonical execution instead of renaming GETs as write replays. |
| Required-write failure, true commit rollback/lost commit reply, half successor/to-do/audit/outbox prevention; retained live-proof barriers | `admin_collaboration_transaction_test.go:200`, `TestAdminCollaborationCloseReopenPreservesProofAndCancellationHistory`; `:239`, `TestAdminCollaborationIterationFailureRollsBackSuccessorAndReopen`; `:273`, `TestAdminCollaborationRequiredWritesRollbackEverything`; `:309`, `TestAdminCollaborationCommitFailureAndOriginalKeyRecovery`. | Browser success snapshots are corroboration, not failure atomicity proof. Keep fault injection and full state snapshots in DB tests. No new browser/database fault route is needed. |

The fixture's account mutation guard (`collaboration-administration.ts:123–138`) intentionally allows only P/H; it cannot test successor disable/restore unchanged. Candidate expiry needs an explicitly bounded extension. Likewise, one synthetic super-admin and one organization do not exercise role/scope-change integration.

## Safe evidence linkage recommendations

Do not edit acceptance definitions or advance AC33/95 merely because discovery, typecheck, Core 1024/Views 94 tests, or empty-lane control assertions pass. Mixed-mention source/report alone also cannot promote AC90/91/97; their current Web/native executions remain coordinator-owned.

For AC65/66/69/70, the matrix already says `passed` with `s3-review` plus `s5-frontend`. Safely attach `web-s5` and `native-s5` as the bounded platform-integration support described above. This repairs evidence linkage and stale progress wording; it does not increase the passed-row count or establish packaged Main/provider/other-OS acceptance.

An additional executed identity can be recorded without a new service run: the catalogued, hash-matching `check-s3-s5-go-race.jsonl` contains PASS for `TestAdminCollaborationCommitFailureAndOriginalKeyRecovery/rollback` (1.18 s), `/lost_commit_reply` (1.18 s) and its top-level test (2.36 s). `s5-service-race.jsonl` also contains PASS at 1.16 s, 1.16 s and 2.32 s. Both artifact hashes match the catalog (`3377c69d116c83eccdea1528bd2bc14d0e573c7babbdc46833b753b6638a3970` and `1b096d49d3ce5560c743a9b7ace9d0dfb403d5ed77f35a78e45d827b9417b01c`). The catalog's selected identity lists omit this test; it is useful AC69/70 recovery/atomicity support, subject to the coordinator's source-equivalence record.

In the three relevant existing raw service artifacts inspected here, no completion record was found for the five currently missing AC33/95 identities: `TestCollaborationQueueCancelUsesCurrentRolesAndPreservesOrder`, `TestCollaborationResumeAllSettlesChannelsIndependently`, `TestCollaborationStopAliasIncludesEveryGroupDescendant`, `TestCollaborationIssueStopFencesChangedAndLaterTargetSets`, `TestCollaborationWholeLifecycleKeepsAllTargetBarriersOnReopen`. This is an evidence-collection gap, not an assertion failure; the coordinator's ongoing partitioned Go run may supply them.

Existing `collaboration-administration.md` accurately separates canonical race/rollback checks and independent password UI evidence. No contract/spec edit is needed for this report-only pass. After control follow-up, sync only its actual scope and results into the relevant shared frontend/control specs, progress and catalog; do not advertise process termination from offline or pending-barrier fixtures.

## Verification performed by this reviewer

Commands from the repository root, against the initial six-file identities above:

```sh
pnpm exec playwright test e2e/collaboration-control-journey-web.spec.ts e2e/collaboration-control-journey-desktop.spec.ts e2e/collaboration-administration-web.spec.ts e2e/collaboration-administration-desktop.spec.ts --list --project=chromium --reporter=list
pnpm exec eslint --config packages/core/eslint.config.mjs --rule 'import-x/no-extraneous-dependencies: [error, {devDependencies: true, includeTypes: false}]' e2e/fixtures/collaboration-control-journey.ts e2e/collaboration-control-journey-web.spec.ts e2e/collaboration-control-journey-desktop.spec.ts e2e/fixtures/collaboration-administration.ts e2e/collaboration-administration-web.spec.ts e2e/collaboration-administration-desktop.spec.ts
pnpm exec tsc --noEmit --module ESNext --moduleResolution Bundler --target ES2022 --skipLibCheck --strict --esModuleInterop --types node e2e/fixtures/collaboration-control-journey.ts e2e/collaboration-control-journey-web.spec.ts e2e/collaboration-control-journey-desktop.spec.ts e2e/fixtures/collaboration-administration.ts e2e/collaboration-administration-web.spec.ts e2e/collaboration-administration-desktop.spec.ts
git diff --no-index --check /dev/null .trellis/tasks/10-20-collaboration-gap-recovery/research/continuation-2026-10-21/control-admin-review.md
```

- Discovery: exit 0, exactly four tests in four files; no browser/native application launched.
- Scoped ESLint: exit 0, no diagnostics.
- Strict TypeScript: exit 0, no diagnostics.
- Report whitespace: no diagnostics; `--no-index` exits 1 because this is a new file.
- All three commands emitted the existing pnpm warning about ignored `package.json` settings; it did not affect their exit codes.
- Raw artifact/hash inspection: S5 results/source-manifest and two service artifacts matched their catalog hashes; current administration sources matched the delivered report.
- Product suites were not repeated. No provider account, process containment, installer, packaged full Main, Windows/Linux GUI, deployment or production gate was exercised. The full-service timeout remains a budget boundary, not a product assertion failure.

No cleanup/refactor or code simplification was performed. The only changed file is this report; no product finding was silently fixed. AC33/95 integrated gaps and the explicitly bounded S5/native/process evidence limits remain for the coordinator.
