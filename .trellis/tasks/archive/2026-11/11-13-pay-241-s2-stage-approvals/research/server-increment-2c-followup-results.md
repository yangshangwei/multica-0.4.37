# S2 increment 2c — independent-check follow-up

Status: **both reported P2 findings corrected; focused verification passed; independent recheck pending**. No2d implementation or policy activation is included.

## Corrections

**S2C-C01:** authenticated Record and its unknown fallback now use one five-second context created after the callback. The fallback receives only that context's remaining budget. Failure or expiry cannot open another window, claim unconfirmed persistence or invoke the adapter again. The mutually exclusive pre-entry `not_started` path retains its own bounded cleanup; caller cancellation, panic, original receipts and historical observation authority are unchanged.

**S2C-C02:** R4's release/rollback evaluator checks the full returned dependency chain through the existing `lockStageFinalSigners` boundary before reporting `valid`. This includes each original issuer's current M1 artifact-read authority. Current-reader source denial remains anonymous. Actual Prepare already rejected the demoted issuer and still does; this correction aligns the read state and hints, not a newly discovered execution bypass. Historical Record still does not require the former human's live credential.

The change touches only:

- `server/internal/service/collaboration_stage_execution.go` — share the post-callback finalization context.
- `server/internal/service/collaboration_stage_execution_view.go` — validate all original issuers before a valid transitive approval projection.
- `server/internal/service/collaboration_stage_execution_followup_test.go` — canonical regression coverage based on the preserved independent diagnostics.

## Red and green evidence

The reviewer's original two failures and original4,938 product files remain unchanged. Canonical `canonical-red-1` reproduces the two findings in the new test file against the unchanged product base:

- Unknown fallback, caller cancellation and unavailable fallback each renewed the deadline by9.7–13.5ms. Absolute deadline comparison proves renewal without sleeping five seconds.
- Both release and rollback read chains retained a `valid` decision after original code-review artifact authority was removed. The rollback hint also remained allowed. The current release viewer's own artifact access was preserved, and real execution still refused with no additional callback.

After the correction, the same7 named regressions pass. They additionally verify that failed bounded finalization returns an error and leaves original pending history, instead of reporting a persisted unknown/success; both readable history and the existing real execution refusal remain intact.

| Final verification scope | Named pass / fail / skip | Exit |
| --- | ---: | ---: |
| Canonical C01/C02 regression |7 /0 /0|0|
| Execution, report ACL, QA and final-expiry service selection |49 /0 /0|0|
| Same service selection, final `-race` run |49 /0 /0|0|
| Existing affected router selection |4 /0 /0|0|
| Service/handler/router `go vet`; full backend build |static|0|
| Three-file `gofmt -l` |empty output|0|

Selections overlap and are not additive acceptance cases. All checks use the new private S2-only reconstruction and root's exact-package identity-checking profiles. No database lifecycle action, new dependency, timeout/pool change, test retry loop or Git write was performed.

The first broad race run is preserved:48 named passes and one existing test failure at a `github_installation` SHARE NOWAIT admission lock (`55P03`). A separately numbered single-test race recheck and the subsequent complete49-test race run passed with identical source and test parameters. Root's later read-only audit found no active backend/lock/vacuum; its later autovacuum timestamps cannot establish the original blocker. **The first lock conflict remains unattributed**, not relabeled as an autovacuum or product cause. No assertion or lock behavior was weakened to obtain the later pass.

## Freeze and handoff

The3-file overlay and full4,939-file source are under:

`/Users/artisan/.multica/acceptance/pay241-20261114-resume/s2-server-inc2c-followup/source`

`manifest.json` binds this overlay to the original S2-only4,938-file2c source, itself reconstructed from the4,894-file ACL base plus63 original2c files. `full-source-manifest.json` records all final bytes. All3 exclusive workspace files match. No current whole-workspace copy, peer knowledge-automation source, shared generated model or844+ migration was used.

The JSON companion binds raw logs/direct exits, original diagnostic references, root's retrospective lock audit and source hashes. All test processes have ended; the implementation's service-normal, service-race and router-normal profiles are released, with the other three profiles untouched and available. The same owner awaits independent recheck before2d. Full A01–A12, long-lived harness, production Web/Desktop and real S4 integrations remain pending.
