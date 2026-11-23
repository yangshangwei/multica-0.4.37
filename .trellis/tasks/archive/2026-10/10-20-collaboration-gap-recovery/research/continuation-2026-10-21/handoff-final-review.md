# Independent handoff/execution review — 2026-10-21

Decision: **approve complete AC06/16/28/30 at their assigned P5/P4/P2 layers**, combining the new positive assertions and exact current canonical adjacency. No product defect or remaining clause gap was found in this bounded review. This reviewer writes only this report; product/tests, staging index, matrix/catalog and environment remain read-only.

## Exact clause assessment

| AC / primary | New evidence plus current canonical clauses |
| --- | --- |
| AC06 / P5 | Actual invite/transfer preserves all three participants. Creator, current handler and former handler each submit a distinct original-author input through the same valid target, Resume, managed Claim/Permit, Stop, early-Resume refusal, original proof and fresh explicit Resume. Old refused keys cannot become intent; stopped input stays stopped. Current role predicates withhold transfer/new acceptance only from former handler. Adjacent stale/machine and selected-target isolation tests pass. |
| AC16 / P4 | The current handler is a genuinely newly invited password account, with no source membership. It uses the exact pre-existing grant with no reconsent; grant bytes/count and one-target count stay unchanged through all role cycles. Current owner/complete-squad/offer/squad-input tests require every actual resource owner consent and create no standalone member target or participant invitation. Existing issue/grant scopes still bound admission; new participant authority does not expand to other tasks/resources. |
| AC28 / P2 | A real persisted active permitted run survives SubmitAcceptance; its valid business capability records success afterward. A real human test-pass/stage-result comment is saved without AI targets. Both leave issue in_review, creator responsible, lifecycle epoch1 and queue active. Former handler cannot complete; only fresh explicit creator complete changes whole lifecycle. Current information-return/stage/paused-acceptance and limited-admin lifecycle tests supply the other explicit-action branches. |
| AC30 / P2 | During active acceptance, session/channel/target/grant/command/batch/group/run/input/dependency/attempt/capability/reservation/termination state is snapshot-identical, with no Stop or proof intent. Waiting input stays waiting; former handler retains submit/comment/control. Existing canonical paused acceptance preserves paused state. New post-acceptance success and final explicit creator complete prove no automatic completion/restart. |

The new `handoffBoundaryExecutionSnapshot` excludes only `session.event_seq`, because human invite/transfer/acceptance/comment legitimately add source events. Queue/control/hold/resource/proof/nonce/attempt/capacity rows remain fully compared. Exact original-key checks use the complete existing `controlStateSnapshot`. This repairs a fixture boundary; it does not weaken product invariants.

Former noncreator handler's old transfer POST is not allowed to reacquire lost transfer authority. The test uses actor-owned original workflow GET recovery, which changes no state; acceptance's original replay has its existing documented special recovery behavior. Existing permission functions and writers remain unchanged.

## Current execution and source

Canonical additions in `server/internal/service/collaboration_handoff_execution_boundary_test.go`:

- `TestCollaborationTransferredHandlersShareExistingGrantAndControls` — all `creator`, `current_handler`, `former_handler` cases pass.
- `TestCollaborationActiveAcceptanceAndResultsRequireExplicitCompletion` — active permit/acceptance/outcome/human-validation/explicit-completion sequence passes.

Current file and isolated copy match SHA-256:

`2711e8e6aea0e111f5438834e1bb5f116c2b1ffb7b071d4b9c39e8b434baefb6`

| Raw artifact in this directory | Actual completed result | SHA-256 |
| --- | --- | --- |
| `handoff-execution-boundary-adjacent-final-20261021.jsonl` | 14 top-level / 36 including cases PASS; no failures/skips, package11.856s | `93e09c491eb014ecb1b3a5ea1c9362fcfbfa46041193d5e8f1933e10f58d5c1d` |
| `handoff-isolated-head-race.jsonl` | 2 top-level / 5 including cases PASS; no failures/skips, package3.624s | `925fea4e06bde111ec9cbce1b8085e121cb892ef5618467634d6df5f176dcc2c` |

Both raw package PASS records agree with companion returncode0 and exact selectors. Commands use Go1.26.6, guarded task-owned `service_s5` migration737, `-race -p 1 -parallel 1 -count=1 -timeout=30m`; no ambient provider is resolved. The 11 source hashes in the final current result all match current bytes.

Actual current canonical neighbors were inspected and parsed as PASS: owner explicit scoped/idempotent consent; complete squad roster and no standalone member target; offered squad every-owner consent; every squad input resource revalidation; invite/transfer preservation; stale/machine submission; selected-target isolation; no deferred Resume; nested information return; paused acceptance retaining rights/in_review; explicit creator lifecycle; limited-admin creator/real-actor retention. Their source assertions cover the remaining rows above, not merely matching names.

## Exact HEAD single-file commit isolation

`handoff-isolated-head-source.json` records HEAD `5baf8659296a4a04666e6eaaa60e5965ffc70725` at `/tmp/multica-gap-recovery-20261021-h6phxwo9/handoff-execution-head-yvnid2bq`. Independent read-only `git archive` byte comparison confirms **all 4061 tracked server files** are unchanged from that HEAD. The only added Go source is the one new handoff test; it supplies its own private snapshot helper and uses tracked managed/workflow/status/comment fixtures. Current and isolated copies match the SHA above.

The recorded isolated argument vector runs only the two new identities and passes. No dirty product, generated SQL, history helper or sibling test was imported into this isolated checkout. Thus root may stage one verified test file plus its task evidence/spec record as an atomic Lore commit without incorporating the broad dirty implementation. Staging index was empty when inspected; this reviewer did not stage or commit.

Companion hashes:

- `handoff-isolated-head-source.json`: `8a089002d0b52146277eb7eb0152f168263faec91a1827e1da6ebcb9cb1d5b1a`.
- `handoff-isolated-head-race-result.json`: `b33e71e58a11918979a0dcae00b848b0d257342d755ba4f6ab95bc2670eaa258`.
- `handoff-execution-boundary-adjacent-final-20261021-result.json`: `b18f9799be0b3f033ead4626f1ad8745b76ac04607de7e37729d0bb685e0a534`.

## Retained failures / proof limits

The first focused attempt incorrectly compared human-event sequence within an execution-only invariant. The second incorrectly re-POSTed transfer after current-handler authority was lost. Both logs remain; only new test expectations were corrected, using legitimate event ordering and the existing original GET recovery. Final fixed/focused/adjacent/isolated runs are reported by their exact result. No product authorization/replay rule was relaxed.

Claim/Permit/Stop/outcome/proof are real DB-backed service operations. **Outcome and sealed proof are test-created protocol inputs**, not observation of an actual provider/kernel process; an “active permitted run” here means persisted valid capability and unsettled barrier. Successful outcome after acceptance proves the service still accepts its original business capability. Natural Linux, full dual-client, paid provider/version/account, installed Main/installer and other-OS/storage/fault acceptance stay separate. No quota, gate, configuration, deployment or task archive action occurred.

## Reviewer verification

```sh
gofmt -d server/internal/service/collaboration_handoff_execution_boundary_test.go
# From server/:
GOTOOLCHAIN=go1.26.6 go vet -p 1 ./internal/service
```

Both exit0 with empty diagnostics/diff. Read-only Python verifies actual current/isolated JSONL counts, all source/result hashes and exact single-file archive isolation. Trellis context/report whitespace validation follows writing. No DB suite, browser/Electron or frontend lint/typecheck was rerun in this Go/report-only review.

Fixed findings: none. Unfixed product findings: none. Changed file: this report only. Simplification: none; no source/dependency/authority edits. Recommendation: root may promote the four primary criteria and commit the verified narrow test/evidence slice. Parent/child stay in progress; overall 97-item completion and rollout are not inferred.
