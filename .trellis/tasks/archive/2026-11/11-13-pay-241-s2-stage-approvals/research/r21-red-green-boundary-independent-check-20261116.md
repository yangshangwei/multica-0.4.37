# R21 corrected RED and minimal GREEN independent check

Active task: `.trellis/tasks/11-13-pay-241-s2-stage-approvals`

## Corrected RED — ACCEPTED

The fresh r02 original-product RED matches the expected4 top-level /6 named /5 leaf inventory. Its only two failed leaves are receipt-first R4 PostgreSQL55P03 and R4-first receipt recovery's original five-second timeout. The real original-writer55P03/zero projection/final returned control and both authority controls pass. There are no skips, parser errors, source drift or outstanding fixture connections.

The raw/command/consumed binding hashes match. The first r01 RED with the incorrect writer needle remains preserved and unaccepted. The r02 run consumed the independently recorded early boundary and its original binding; the later formal review is referenced separately without rewriting that consumed record.

## Minimal GREEN source — boundary PASS

The exact private source diff changes only `GetWorkflowOperation:708` from `LockCollaborationUsers` to the existing `LockArtifactUsers`. The former query selects the actor UUID set from `user`, ordered by ID, with `FOR UPDATE`; the latter selects the identical rows/fields/predicate/order with `FOR SHARE NOWAIT`.

The shared fence permits concurrent authorized readers while still conflicting with user updates, no-key updates and exclusive writer locks. A lock refusal still returns a zero receipt before account authorization or operation projection. The one-user cardinality check, `authorizeWorkflowAccount`, issue/actor/request lookup, operation-kind allowlist, decoding and minimal historical return remain unchanged. No source membership or participant prerequisite is added to original-key recovery.

All other workflow functions/writers and source byte/mode identities are preserved. The5,045-file manifest changes only `collaboration_workflow.go`; the corrected214-line test remains `0dfec7c3…`. SQL/generated code, API schema, dependencies, frontend and ROOT files were not changed in this private step.

## Actual normal regression — PASS

The recorded normal GREEN is exit0 with4 top-level /6 named /5 leaf PASS, zero FAIL/SKIP/parser errors, source unchanged and required fixture tables/other connections clean. Raw and consumed binding references match. This reviewer did not rerun it.

This proves the bounded pure-GET coexistence repair and the new writer/current-account controls. It does not prove the actual R20 lock holder or a new Web/Desktop runtime result.

## Selected original controls and combined race boundary — PASS

The static plan `d9ea880d…` selects three preserved original controls; each has no nested subtest:

- `TestCollaborationWorkflowVoluntaryLeaveAndMinimalRecovery`: a departed actor recovers only its own minimal receipt without private content or object access; another actor is refused.
- `TestCollaborationWorkflowLostLeaveReplyRecoversWithoutObjectAccess`: an actual lost commit reply is recovered by the original key after object access is lost, with one departure event and no retry mutation.
- `TestStageCodeReviewTransitionsAndOriginalKey`: exact candidate/manual approval, original-key recovery, changed-payload refusal and reopen invalidation.

Their source hashes match the frozen GREEN. Adding them to the new4/6/5 suite yields the exact7 top-level /9 named /8 leaf combined race inventory. The reviewed draft names only that anchored selector, `-race`, the unchanged five-minute package budget and fresh unused race_r21r02 profile. Its nine bound references match. This is sufficient scoped follow-up; this review does not require an unrelated116-test rerun.

The actual combined race completed exit0 with7 top-level /9 named /8 leaf PASS and no FAIL/SKIP/parser errors. The raw run and terminal sets match the exact consumed nine-name inventory. Source before/after remains bound and required fixture tables/other connections are clean. Its consumed binding and all referenced raw/source/profile inputs match.

The normal six-name set is a subset of the combined race nine-name set. These are separate normal/race observations, not fifteen distinct tests.

## Final scoped code quality — ACCEPTED

Actual `go vet ./internal/service` completed exit0 with empty stdout/stderr and no source drift. The5,046-file runtime build source carries the same accepted workflow bytes `51e7eb2b…` and test bytes `0dfec7c3…`. Its two existing cmd/server test-hook differences and extra private tracer retain the original R18 runtime bytes; the new test's private runtime mode is recorded separately. Those test-hook differences are not a claim of whole canonical/ROOT equivalence. Vet's source/environment/log references match, and it executed no runtime DB test.

The immutable-receipt read repair is **CODE_ACCEPTED_SCOPED**: corrected RED, actual normal and combined race, exact one-line source boundary, current-authority/original-writer/original-key controls and service vet are independently supported. No blocking scoped finding remains. This verdict does not establish the original R20 lock holder or a passed new four-scenario runtime.

## Scope and handoff

Only the two declared research reports were written by this checker. No Go, test, API/DB/browser, build, install, signal or Git command was executed. Use this accepted private source with the leader's separately recorded exact ROOT hunk/test integration and matching new runtime products. Original bad-control RED, corrected RED, normal/race results and R20 failures all remain separate.

The actual R20 lock holder, full S2/A01–A12 and parent acceptance remain open.
