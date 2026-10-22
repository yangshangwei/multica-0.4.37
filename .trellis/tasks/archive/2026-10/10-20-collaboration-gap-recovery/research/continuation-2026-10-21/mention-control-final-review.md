# Final mixed-mention and control review — 2026-10-21

Status: independent bounded review passed. No concrete product/test-code defect was found in the six finalized E2E files. No code fix was required. This lane wrote only this report; shared acceptance ledgers, product source, services, builds, gates and commits were not changed.

This review supersedes the **initial control-source** gap assessment in `control-admin-review.md`. Its S5 evidence assessment remains applicable. AC33's separate cross-client execution lane and the remaining parent criteria are outside this report.

## Acceptance promotion verdict

| Criterion / assigned primary layer | Verdict | Current evidence supporting the whole scoped criterion |
| --- | --- | --- |
| **AC90 / P3** | **Promote to `passed`.** | Final Web and actual Electron mixed-comment journeys commit one H+A+B+S comment, retain three distinct target/channel/command/batch/input-event associations, deliver exactly one located H notification, and preserve participation, handler, business state and workbench responsibility. Newly current canonical comment preview/partial-commit tests also pass. This is the content/queue association contract, not a claim that the paused offline providers executed. |
| **AC91 / P6** | **Promote to `passed`.** | Both final clients exercise read-only concrete-target/recipient and paused/waiting previews, suppression with preserved text, actual A-owner revocation between the preview and original POST, one comment with B queued/H notified/A refused, exact original-key POST replay with unchanged complete state, and a fresh read-only already-unavailable A preview. The corresponding canonical service tests pass in the final service partition. |
| **AC97 / P3** | **Promote to `passed`.** | H's first client closes before send; actual legacy API login after send preserves H's identity, and a fresh browser context/native profile reads the durable notification and focuses the original comment. Read/archive preserve the workbench and controls; comment/body edits notify only newly effective people, body notifications correlate with exact edit events, actor/nonparticipant negatives hold, typed edit mentions create no AI commands, and @S's outside human member remains uninvited and denied. Final canonical delta/access-loss tests pass. Interactive login-page behavior is not part of this criterion. |
| **AC95 / P5** | **Promote to `passed` by combining current P5 service/Linux evidence with the completed dual-client controls.** | Web and actual Electron now exercise agent/squad/whole-issue Stop/Resume, unselected-lane preservation, issue-pause refusal, grant refusal and explicit fresh Resume after reconsent, genuine server pending-proof refusals/partial receipts, and original-key GET recovery. Final canonical service tests cover descendants, later targets, lifecycle barriers, global guards, removed-channel reservations and revocation; actual Linux route tests supply real execution/proof arrival/fresh Resume evidence. The public withheld-Permit fixture supplies UI explanation evidence only. |

The raw completed canonical results inspected below remove the previously recorded missing-current-test-evidence arrays for these four criteria. No new provider-in-GUI matrix is required: keep the parent-assigned P3/P5/P6 ownership and combine evidence at those boundaries. These four promotion recommendations do not close AC33, authorize provider accounts/quota or rollout, prove packaged full Main startup, or finish/archive the 97-row parent task.

## Final source identities

Independently computed current SHA-256 values equal the two implementation reports. All final platform executions use these identities, including the unavailable-preview extension:

| File | SHA-256 |
| --- | --- |
| `e2e/fixtures/collaboration-mention-journeys.ts` | `aa256fb98809c7fcf55944557bbda44e16b551416db38e32bdf075d6eb76d2c7` |
| `e2e/collaboration-mention-journeys-web.spec.ts` | `67a6cdd7da37d5840dbaca0f0b3396c7f22671e74c4a388554536d40b7d63d47` |
| `e2e/collaboration-mention-journeys-desktop.spec.ts` | `515eb616f099e313002727a9bbf16d33727b9e83993942c570a1999cba4304e8` |
| `e2e/fixtures/collaboration-control-journey.ts` | `1e6b3536b6ba030242320635e9892ee4df86dd63e10f5e81fbd84d63c8e4f512` |
| `e2e/collaboration-control-journey-web.spec.ts` | `97189fd859b7d8a524b6686bb19f23f478600362f9eb9eed46b627acb886dc9b` |
| `e2e/collaboration-control-journey-desktop.spec.ts` | `705b5648f4a675ab43283688e0145dbf6b7f6544eb38fed64e0f2e7214f62c01` |

Served application provenance remains coordinator-owned: sibling `source-environment-manifest.json`, `native-artifacts.json`, the legacy/admin `*.running.json` records, `managed-control-environment.json` and `native-renderer-restart.json`. The coordinator established application-source equivalence for the retained production snapshots. This reviewer did not start/rebuild services or reinterpret a current E2E test hash as the compiled product build fingerprint.

## Actual completed executions

Raw reports under `.omx/reports/cwc-gap-recovery-20261021/` were independently parsed; every selected test has one `passed` result, `retry: 0`, and expected status. These are ten actual executions, not discovery counts:

| Raw report | Result and total duration | SHA-256 |
| --- | --- | --- |
| `continuation-mentions-web/attempt-6.json` | 3 expected; 0 unexpected/skipped/flaky; 19.954 s | `2d48e6a14ce3864cf138a76485a379d9d768c26b6662a699165843e024fb75dd` |
| `continuation-mentions-desktop/attempt-2.json` | 3 expected; 0 unexpected/skipped/flaky; 24.615 s | `e1ce9d447a426572fdba786c69a1eb7c551fed11115d8d3cadfc4ee763d5a552` |
| `continuation-controls-web-v5.json` | 2 expected; 0 unexpected/skipped/flaky; 15.003 s | `10665857af923f80ccacd62c11833a4491d86663b76c2c107940f809f4f76d40` |
| `continuation-controls-desktop-v2.json` | 2 expected; 0 unexpected/skipped/flaky; 14.465 s | `bacaf0e97be66a59a1513b843fecc468092271c6f6beabc24d173cae871c1a56` |

Exact identities (the Web tuple tests resolve to `collaboration-mention-journeys-web.spec.ts:46`; its offline test is at `:65`; Desktop mention tests are at `:97`, `:102`, `:107`; control tuple tests are at Web `:10` and Desktop `:14`):

- `Web mixed H+A+B+S mentions retain one comment and exact queued target associations`.
- `Desktop mixed H+A+B+S mentions retain one comment and exact queued target associations`.
- `Web owner revocation after preview refuses only A and original-key replay stays deduplicated`.
- `Desktop owner revocation after preview refuses only A and original-key replay stays deduplicated`.
- `Web offline mentions survive a fresh context and edits notify only newly added people`.
- `Desktop offline mentions survive a fresh profile and edits notify only newly added people`.
- `Web controls recover partial Resume, isolate agent/squad lanes and stop the whole issue`.
- `Desktop controls recover partial Resume, isolate agent/squad lanes and stop the whole issue`.
- `Web pending termination proof keeps A paused while whole Resume releases B and squad S`.
- `Desktop pending termination proof keeps A paused while whole Resume releases B and squad S`.

Earlier attempts remain retained. Mention failures were retired-preview checkbox postcondition and unavailable-preview locator defects, repaired before the final full-platform reruns. Control failures were onboarding setup, missing real CSRF cookie issuance, and the recorded fixture/renderer environment issues. The final passes do not erase those attempts or turn them into passing executions. `mention-journeys.md` and `control-journeys.md` record their commands and exact outcomes.

## Source-path review and sufficiency

**Mixed comment and partial refusal.** `collaboration-mention-journeys.ts:40–129` establishes distinct agents A/B and squad S through owner offer/adoption/every-resource consent. Its squad contains outside human X. `:274–299` corroborates receipt identities against actual retained commands, exactly one comment, H's one mention notification, D's ordinary-comment notification, actor exclusion and outsider refusal. The ordinary notification for D is intentional and does not duplicate H's mention.

`exerciseMixedMentionJourney` (`:330–355`) compares all preview epochs/holds with current API state and checks that previewing changes no recorded state. `exercisePartialMentionRefusal` (`:358–444`) preserves text through suppression, barriers the original real POST until A's real owner grant is revoked, verifies B and other resources remain authorized, checks the visible named refusal, and replays the identical key/body with full before/after state equality. The extension at `:418–437` separately proves A's already-unavailable disabled/unchecked preview reason and B's paused/waiting state without another send. No target response is fabricated.

**Durable notifications and edits.** The Web wrapper (`:74–96`) and native wrapper (`:107–117`) close H's existing client before the send, perform a new legacy authentication API login after it, verify the same H ID and open a fresh context/profile. The shared journey (`:510–579`) clicks exact comment/body links and verifies focus; marks the original notification read then archived through UI; checks human-workflow invariants; edits comment/body through real forms; compares notification ID deltas for every fixture actor; and correlates body anchors with actual `issue_edited` history. Pure wording edits add none. X remains outside participation with no private response/notification; squad mentions do not invite X. The login is an actual API authentication followed by authenticated UI use, with no assertion that an interactive login screen was exercised.

**Controls.** `collaboration-control-journey.ts:168–207` creates a distinct two-agent squad and verifies three separate target channels with complete managed consent. The first journey (`:342–467`) checks issue-pause refusal, real grant-dependent partial Resume, exact per-target reasons, B/S selected Stop and Resume, unchanged unselected channels/holds, A reconsent that retains its hold until a fresh explicit Resume, whole Stop, and historical result recovery without state changes. It issues exactly eight UI writes with distinct keys and one correlated original-key UI GET after the committed response is dropped.

The pending journey (`:473–585`) creates its command/group/run through public comment/List/Claim/Permit; no database row or control response is invented. It observes `launch_authorized`, unknown proof, Stop accepted without confirmed termination, selected Resume refused, whole Resume releasing B/S while A remains paused, truthful lane totals/reason, unselected isolation and unchanged historical receipts. Its eight writes and single recovery GET are asserted. Only the initial enrollment/daemon protocol participant is synthetic. No provider or supervisor process starts, no outcome/proof is posted, and runtime deregistration deliberately preserves the unknown barrier. This is valid real server/UI pending-barrier evidence; **it is not kernel containment, proof arrival or proof recovery evidence**.

## Canonical evidence combined with the UI runs

The final `go-service-partition-2.jsonl` now has actual PASS records for all ten previously missing identities relevant here, followed by service-package PASS in 302.457 s:

| Criterion | Actual current canonical test identities |
| --- | --- |
| AC90/91 | `TestCollaborationCommentCommitPreservesValidTargetAndHumanWhenAnotherTargetWasRevoked` (0.26 s), `TestCollaborationCommentPreviewIsReadOnlyAndKeepsIndependentPeopleAndTargets` (0.18 s), `TestCollaborationCommentPreviewGroupsTypedTargetsAndSplitsSecondSquad` (0.41 s) |
| AC97 | `TestCollaborationBodyEditNotifiesOnlyAddedHumanMentions` (0.25 s), `TestCollaborationCommentEditMentionDeltaUsesCurrentParticipants` (0.18 s), `TestCollaborationNotificationsLoseContentAndAnchorsWithAccess` (0.17 s) |
| AC95 | `TestCollaborationResumeAllSettlesChannelsIndependently` (0.54 s), `TestCollaborationStopAliasIncludesEveryGroupDescendant` (0.34 s), `TestCollaborationIssueStopFencesChangedAndLaterTargetSets` (0.33 s), `TestCollaborationWholeLifecycleKeepsAllTargetBarriersOnReopen` (0.40 s) |

The same current partition also records PASS for `TestCollaborationResumeAllGlobalGuardsPreserveState`, `TestCollaborationResumeAllRetainsRemovedChannelReservation`, `TestCollaborationResumeAllMaterializesDiscoveredHold`, `TestCollaborationResumeAllEmptyScopeReleasesGlobalPause`, `TestCollaborationResumeAllowsOfflineAuthorizedWaiting` and `TestCollaborationResourceRevocationPreservesAllWaitingAuthors`. These remain canonical lower-layer checks; their matrices were not duplicated through DOM.

`linux-router.jsonl` records six completed PASS identities and no top-level skips: `TestCollaborationPartialResumeRoutesLinux`, `TestCollaborationSingleAgentRuntimeRoutesLinux`, `TestCollaborationSquadCLIAndDaemonCapacityOneRoutesLinux`, `TestCollaborationHeldSquadAllowsDirectAgentRoutesLinux`, `TestCollaborationRuntimeProofSurvivesOwnerDisableLinux`, `TestCollaborationSquadCapacityOneRoutesLinux`. In particular, the partial-Resume and single-agent route tests are the real Linux execution/proof/fresh-Resume evidence; they run against their own test router/resources and are not relabeled as the GUI fixture's process run.

Both canonical artifacts are in this report's directory. Independently computed SHA-256:

- `go-service-partition-2.jsonl`: `1a1edb5507cf4a6b4259194d2d8abfe5c1d2ebb9f56672f52353d999ef07d99d`.
- `linux-router.jsonl`: `411f23c1e244550118dcbf7a09d4fdb6062be74e9b0bda627b827257f9acaf0b`.

These are reviewed coordinator executions, not service/native tests run anew by this reviewer. The coordinator owns complete suite/source/environment manifests and acceptance-ledger integration. The earlier full-suite ten-minute timeout and first targeted hold-fault attempt remain historical evidence; this report does not classify the timeout as a product failure or conceal the mechanical test-fault repair.

## Artifact integrity and visual evidence

The credential-free mention export at `.omx/reports/cwc-gap-recovery-20261021/continuation-mentions-safe-evidence/` was independently checked:

- All eight retained run report hashes match `runs-final.json`.
- All three final source entries match current files.
- All 21 final PNGs match their recorded byte counts, SHA-256 and actual PNG dimensions.
- All six final exported identity JSONs equal the decoded corresponding raw report attachments.
- `runs-final.json`, `source-hashes-final.json` and `png-hashes-final.json` hashes equal the implementer report: `53354dcc63cca4b9a6bbdb6760e3b71e5b10a73e32604cee57c006c74fe947c8`, `3c81bd1312f4ad651dc7cdc45d12021c10f24d92693873bbe9c12d4dd2b21b2a`, `f6ff5566fa394f2897e2e42742c4211b86b180597a06d662c89fb2ebc49bfb48`.

The four final control identity attachments were also decoded: agent/squad IDs, real control receipts and explicit no-process/containment scope are present; native attachments record `native_containment_claim: false`. Actual native tests use production account routing/IPC with isolated profiles and assert no renderer errors, ambient workspace headers or daemon starts.

This reviewer visually inspected final Web unavailable-preview narrow PNG and native pending-proof partial-Resume 200% PNG. The former shows disabled/unavailable A and checked/paused B with wrapped text; the latter visibly says `2 lanes resumed; 1 remain paused` and names the unconfirmed lane without clipping. No product visual edit was made.

## Fresh static verification and remaining boundaries

Commands actually run by this reviewer from the repository root:

```sh
pnpm exec playwright test e2e/collaboration-mention-journeys-web.spec.ts e2e/collaboration-mention-journeys-desktop.spec.ts e2e/collaboration-control-journey-web.spec.ts e2e/collaboration-control-journey-desktop.spec.ts --list --project=chromium --reporter=list
pnpm exec eslint --config packages/core/eslint.config.mjs --rule 'import-x/no-extraneous-dependencies: [error, {devDependencies: true, includeTypes: false}]' e2e/fixtures/collaboration-mention-journeys.ts e2e/collaboration-mention-journeys-web.spec.ts e2e/collaboration-mention-journeys-desktop.spec.ts e2e/fixtures/collaboration-control-journey.ts e2e/collaboration-control-journey-web.spec.ts e2e/collaboration-control-journey-desktop.spec.ts
pnpm exec tsc --noEmit --module ESNext --moduleResolution Bundler --target ES2022 --skipLibCheck --strict --noUncheckedIndexedAccess --esModuleInterop --types node e2e/fixtures/collaboration-mention-journeys.ts e2e/collaboration-mention-journeys-web.spec.ts e2e/collaboration-mention-journeys-desktop.spec.ts e2e/fixtures/collaboration-control-journey.ts e2e/collaboration-control-journey-web.spec.ts e2e/collaboration-control-journey-desktop.spec.ts
```

Discovery: exit 0, ten tests in four files. Scoped lint: exit 0, no diagnostics. Strict TypeScript including `noUncheckedIndexedAccess`: exit 0, no diagnostics. Only the existing pnpm package-field warning appeared. Discovery launched no browser/native application. The report received an untracked-file whitespace check separately.

No new native/service/browser run, environment or gate change, provider account access, dependency, product simplification or commit was made. Changed file: this report only. Kernel proof belongs to the actual Linux tests; interactive login UI, packaged full Main/installers, other operating systems, real provider/version/account/quota support, the separate AC33 journey and the other parent criteria retain their own acceptance requirements.
