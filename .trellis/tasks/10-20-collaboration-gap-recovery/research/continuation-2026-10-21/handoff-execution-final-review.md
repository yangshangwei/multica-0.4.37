# Independent handoff execution authority review — 2026-10-21

Recommend **AC06 / P5, AC16 / P4, AC28 / P2 and AC30 / P2 passed** at their assigned primary layers. The complete original clauses are supported by the new positive cases, the fresh canonical adjacency, and the separately executed same-resource peer-task isolation. No mechanical issue or product defect requires a fix. This reviewer writes only this report and its [structured evidence](handoff-execution-final-review.json); source, matrix/catalog/progress/specs, git index and runtime environments stay read-only.

## Exact clause assessment

| Criterion / owner | Complete current support |
| --- | --- |
| AC06 / P5 | Actual InviteParticipant then Transfer keeps creator, current handler and former handler active. Each role submits one exact-author input through the same unchanged grant, activates, claims/permits, Stops, receives early Resume refusal, accepts original synthetic proof and explicitly Resumes with a fresh key. The stopped input remains terminal and the old refused key stays refused. Canonical stale/machine/participant fences, selected-target isolation and no-deferred-Resume cases pass. |
| AC16 / P4 | The current handler is a genuinely newly invited password account with no source membership. Grant bytes and one-grant/one-target counts stay unchanged through its submission/control cycle, without per-person reconsent. Existing complete squad/offer/input tests require every actual resource owner's consent, admit only the squad and create no standalone member target. Consent preserves the participant count; registration requires existing owner participation and does not enroll an owner. The separately executed same-resource peer-task regression supplies explicit nonempty isolation snapshots and successful other-task submission. |
| AC28 / P2 | Stage-note Transfer retains todo and creator; the information-return chain restores requester/status rather than done. A real service-issued permit survives acceptance, and its original business token records AI success afterward. A human former-handler stage/test-pass comment is actually persisted without AI targets. Issue remains in_review, creator responsible, lifecycle epoch1 and queue active. Former handler cannot complete; only a fresh explicit creator lifecycle does. Explicit R38 admin lifecycle preserves original creator/real actor and does not enroll the admin. |
| AC30 / P2 | SubmitAcceptance while actually permitted/unsettled leaves session, channel, target, grant, command, batch, group, input/dependency, run, attempt/capability/reservation/termination facts identical. Waiting input remains waiting; no Stop/proof intent is added. Original business capability still works afterward. Canonical paused acceptance preserves paused queue/control1 and former comment/control rights; new active case retains former submit/control and persists its comment. Explicit creator/admin completion remains required. |

The parent acceptance text, requirement references, source lines and P5/P4/P2 ownership were matched directly. The authoritative rows were still pending when read; this is a review recommendation, not an automatic promotion. Individual clause/source/test mappings and exact raw/result/source hashes are in the companion JSON.

## Assertions and real code paths

The new source is `server/internal/service/collaboration_handoff_execution_boundary_test.go`, SHA-256:

`2711e8e6aea0e111f5438834e1bb5f116c2b1ffb7b071d4b9c39e8b434baefb6`

`TestCollaborationTransferredHandlersShareExistingGrantAndControls` executes `creator`, `current_handler` and `former_handler`. It compares input command/author/body/issue/target to the actual Claim response, observes the exact permitted run after Stop with unknown proof and unset settlement, and checks immutable refusal/replay plus fresh Resume. The newly invited handler uses the original complete owner grant, which is compared byte-for-byte after every role cycle. Invitation/transfer preserves all three active participant rows and grants no source membership or personal owner grant.

`TestCollaborationActiveAcceptanceAndResultsRequireExplicitCompletion` issues a real managed Claim/Permit before SubmitAcceptance. Its snapshot includes nonempty active run/attempt/capability/reservation/termination rows and waiting input, so successful acceptance cannot pass by comparing an empty execution fixture. The exact returned business token is passed to RecordRuntimeOutcome after acceptance; the current capability/admission/attempt checks succeed. A real CreateComment stores the former handler's human test-pass record. Assertions retain in_review, creator responsibility, lifecycle1 and active queue through both events. Former handler's ChangeLifecycle is forbidden; only explicit creator completion closes and cancels the waiting command.

The private `handoffBoundaryExecutionSnapshot` at line195 excludes only `collaboration_session.event_seq` from its session object. The actual `finishWith` workflow code invokes `AdvanceCollaborationEventSequence`, whose SQL changes exactly that ordering field; human invite/transfer/acceptance/comment events legitimately advance it. All queue/control/hold/grant/run/fence/prompt/attempt/capability/proof facts remain fully compared. Human issue responsibility/status and event/operation records are intentionally assessed by their own assertions. Read-only workflow GET, old refused Resume and acceptance replay use the full existing `controlStateSnapshot`, including session/event state.

The original actor-owned workflow GET is the correct minimal recovery after Transfer. A former noncreator handler is not permitted to acquire current-handler authority by re-POSTing Transfer. The retained failed attempt demonstrates that refusal; final tests use the existing GET and do not relax authorization.

For AC16's other-task/resource clause, `TestCollaborationGrantWithdrawalAndOwnerRejoinIsolateOtherTask` shares exact owner/agent/runtime/binding across distinct issue/target/grant scopes. It first creates a genuinely persisted completed peer execution and asserts required snapshot relations are nonempty. Full peer control/proof/issue/participants/operations/notifications, current access and authorized catalogue remain identical through affected create/consent, revoke or leave/rejoin, old proof, fresh consent and Resume. A final actual peer SubmitCommand succeeds under its unchanged grant. This supplementary evidence is tracked in HEAD and remains a separate executed slice; it is not part of the new handoff file's dependency claim.

## Current verification evidence

All service commands use the guarded isolated migration737 `service_s5`, serial race, `-p 1 -parallel 1 -count=1 -timeout=30m`. No DB test was executed by this reviewer. Raw JSONL was parsed independently: actual PASS identities, package PASS, counts, result command vectors, source equality and zero fail/skip were checked rather than inferred from names.

| Raw artifact | Actual PASS identities / package elapsed | SHA-256 |
| --- | --- | --- |
| `handoff-authority-coordinator-rerun-20261021.jsonl` | **14 top-level /36 including cases**, 12.355s | `d09907ff550ae311b7d72927405649932af8aad9727d4bd6660e41a821feac87` |
| `handoff-execution-boundary-adjacent-final-20261021.jsonl` | 14 /36, 11.856s | `93e09c491eb014ecb1b3a5ea1c9362fcfbfa46041193d5e8f1933e10f58d5c1d` |
| `handoff-execution-boundary-focused-recovery-20261021.jsonl` | 2 /5, 4.569s | `61fdfdc895616be19c55fef6c30ac37663c02a721cbc0dbb0f3131e26fca79c2` |
| `handoff-isolated-head-race.jsonl` | 2 /5, 3.624s | `925fea4e06bde111ec9cbce1b8085e121cb892ef5618467634d6df5f176dcc2c` |
| `grant-task-isolation-adjacent-final-20261021.jsonl` | 10 /22, 9.203s | `6a514344838ae566151eda183bb732c460cb2ecd2312f104846636400b9c2866` |
| `grant-isolated-head-race.jsonl` | 1 /3, 3.869s | `da657a0af9fc30320ec4d9d2bb526a3f406a10b34163b64e9fce88a8917eb419` |

Each result is exit0 with zero fail/skip. The fresh coordinator rerun starts `2026-10-21T14:37:41.948939+00:00`, finishes `14:37:58.933902+00:00`, elapsed16.984s, empty stderr. All its four before/after source hashes and all11 original adjacent source hashes match current bytes. Grant adjacency's all11 source hashes also match current bytes. The companion JSON retains the full selectors and every actual PASS, including subgroup identities.

The exact14 fresh top-level identities are:

- TestCollaborationTransferredHandlersShareExistingGrantAndControls
- TestCollaborationActiveAcceptanceAndResultsRequireExplicitCompletion
- TestCollaborationWorkflowInviteTransferPreservesExecution
- TestCollaborationQueueRejectsStaleIdentityAndMachine
- TestCollaborationTargetStopLeavesOtherTargetAndIssueActive
- TestCollaborationQueueResumeRejectionNeverTurnsIntoDeferredIntent
- TestCollaborationOwnerConsentIsExplicitScopedAndIdempotent
- TestCollaborationConsentUsesCompleteSquadRoster
- TestCollaborationResourceOfferSquadStillRequiresEveryOwnerConsent
- TestCollaborationSquadInputRechecksEveryResourceConsent
- TestCollaborationAcceptanceSubmissionReturnsOpenIssueToCreator
- TestCollaborationInformationChainReturnsLayerByLayer
- TestCollaborationLifecycleAuthorityAndExplicitReopen
- TestAdminCollaborationLifecyclePreservesCreatorAndRealActor

Earlier focused failures remain retained: first SHA `2e18bf1d1b4fe4462fb8fbe4f0313fb641a6fb12a4491eadbe8a64e7d50ee858` compared legitimate human event_seq; second SHA `aa331b0a9d26ba48fb01f98034f3bc2d32245b264cf9f0c7a70f6c4fd53a2232` re-POSTed Transfer after losing role. Those test expectation failures are superseded by corrected final source and successful focused/adjacent/isolated results. They are not current product failures, and no product permission or replay rule was changed.

## Exact HEAD and single-file commit boundary

`handoff-isolated-head-source.json` records HEAD `5baf8659296a4a04666e6eaaa60e5965ffc70725` at `/tmp/multica-gap-recovery-20261021-h6phxwo9/handoff-execution-head-yvnid2bq`. The retained `head-server.tar` matches its recorded SHA exactly:

`bd31d1fad1f975020c06e78af7cd5622eda7283bb7a1d82c5aba2ebb62be9998`

Independent archive-member comparison verifies all **4061 tracked server files** plus the two HEAD guard support files (`scripts/agent-cli-command-names.txt`, `scripts/go-test-with-agent-cli-guard.sh`) against both recorded HEAD and extracted bytes. Only added server source is the new handoff test, with current/isolated SHA equality. The extra `head-server.tar` is a retained archive, not a source dependency. This is an isolated backend snapshot, not a claim about a complete whole-repo snapshot or a physically two-file checkout.

The selected isolated two-test race run passes with that exact HEAD product/fixtures and the new file's own private snapshot helper. Dirty current workflow/runtime/lifecycle/fixture modifications and untracked admin/offer neighbor suites belong to current integration evidence; the new one-file commit does not need them included. The already tracked peer-task isolation source supplies supplementary clause evidence independently. This avoids converting shared history/preparation/runtime code into new slice ownership.

Key manifests:

- `handoff-isolated-head-source.json`: SHA `8a089002d0b52146277eb7eb0152f168263faec91a1827e1da6ebcb9cb1d5b1a`.
- `handoff-isolated-head-race-result.json`: SHA `b33e71e58a11918979a0dcae00b848b0d257342d755ba4f6ab95bc2670eaa258`.
- `handoff-authority-coordinator-rerun-20261021-result.json`: SHA `3938676a4bc35a88d60ab0afc36c4729bf4af5c13d3093d5ee71095d8e98ebe7`.

## Reviewer checks, findings and limits

Fresh reviewer checks (exact stdout/stderr/returncode/command/time in companion JSON):

```sh
# Repository root:
gofmt -d server/internal/service/collaboration_handoff_execution_boundary_test.go
# server/:
GOTOOLCHAIN=go1.26.6 go vet -p 1 ./internal/service
```

Both pass with zero output/diff; Go1.26.6 darwin/arm64 compilation and vet cover this Go-only source. Frontend lint/typecheck is not applicable and no frontend/native suite was rerun here. Current report JSON/source/parent clause links and whitespace are verified after writing.

Fixed findings: none. Unfixed product findings: none. No public behavior/interface change requires a product edit; root may append this exact evidence scope and human-event ordering note to the existing collaboration specs. Changed files are only `handoff-execution-final-review.md` and `.json`. Simplification: existing managed/workflow/status/comment/control primitives are reused; no new product abstraction, dependency, migration or generated SQL is added.

Claim/Permit/Stop/outcome/proof are real persisted service operations. **AI outcome and sealed proof are synthetic protocol inputs**, with no provider process or kernel observation in this slice. Successful outcome after acceptance proves the original service capability remains valid. This review does not supply a new Web/Electron journey, natural Linux proof, real provider version/account/quota, installed Main/installer, unsupported OS or storage/runtime fault matrix. Those boundaries remain separately attributed and authorized; production creation/execution rollout stays closed.

Root may update the four exact primary criteria and stage the verified single test plus owned evidence under Lore. This reviewer does not promote acceptance or stage/commit. Child and parent remain in_progress until all97 full clauses have current evidence.
