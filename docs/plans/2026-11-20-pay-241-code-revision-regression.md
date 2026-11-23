# PAY-241 TC-12 local code revision regression proof

Continue `.trellis/tasks/11-13-pay-241-collaboration-acceptance`, already in progress. Intake HEAD is `efdb597d6b80f930b5956de30bc8e4780e4023c7`; the parent has 9 PASS /11 PARTIAL /4 NOT_RUN. Preserve peer work and all accepted S1–S3/TC-02/13 evidence. No duplicate task or parent archive.

## Gap and boundary

S2 R33 proves failure/run/case/log association, human QA return and an independent retry. Both runs use the same code and differ by a fault switch. That does not prove regression after code repair. The new slice follows [the exact gap research](../../.trellis/tasks/11-13-pay-241-collaboration-acceptance/research/tc12-code-revision-gap-20261120.md).

Add only `server/internal/service/collaboration_code_revision_regression_test.go`. Reuse existing stage, managed-runtime, preparation, report and original-request owners. No product logic, schema, dependencies, new workflow or broad denial matrix. The implementation agent owns the new file in a private committed-source candidate; Main owns environment, source integration, specification, records and Git. A demonstrated product defect requires diagnosis before widening this boundary.

This is a local synthetic code repair and service version/report association proof. It does not execute a real payment repository, daemon checkout, remote CI, model, browser or deployment. Synthetic PR/CI observations and managed preparation/consent/settlement fixtures stay explicit. In particular, `refreshPreparationConsent` uses the test-only retired `ReconfigureTarget` fixture, not the production configuration Switch. TC-12 remains PARTIAL even after this slice is accepted.

## Finite proof

Preregister one top-level test, `TestStageCodeRevisionRegressionPreservesFailedHistory`, with no subtests. Normal and race each execute that name once: one distinct named test, two mode executions, supporting one bounded supplement. Expected A-program failure is an internal stimulus, separate from the Go suite result.

1. Create an isolated Git repository with a defective synthetic business function, fixed regression runner and fixed input. Commit A, retain exact head/tree/blob/bundle identities, and actually execute the runner. Require the precise failing exit and diagnostic. Disable inherited Git configuration/hooks/signing, use a synthetic local identity and minimal credential-free subprocess environment; no remote exists.
2. Register the real commit/bundle through current preparation. Use the existing typed comment, claim, permit and frozen test subject. Record observed case outcome and actual log bytes through `RecordRuntimeTestReport`, settle the failed execution, then obtain a real service human QA return and transition to development. Retain original operation identities and full immutable A histories.
3. Change only the business function, commit B with A as its direct parent, and prove unchanged runner/input/command versus changed implementation/tree/head. Create a superseding B candidate and fresh code review. Register B preparation, renew explicit consent and resume the target when the original failure paused it.
4. Execute the same runner/input/command for B, require exact success, and bind it to a distinct typed command/run/attempt/report. Reject an otherwise-current B QA request that refers to A's report with the exact existing error and its defined denial receipt; then approve B using its own report and fresh code-review dependency.
5. Recover A's original report ACK and QA receipt after B. Compare complete immutable A rows and Git/log bytes before/after B and recovery, including cases, logs, source versions and human decisions. Old applicability may become stale; it must not rewrite stored failure history. No duplicate report or hidden new run is allowed.

## Execution and closure

Use a source snapshot from committed HEAD, excluding shared WIP. Main preregisters exact test/configuration/tool/command hashes, creates separate new service normal/race databases and private HOME/TMP paths, runs compile-only (`go test -c`), gofmt, vet and both actual modes. Never execute an unconfigured TestMain or use `-run '^$'` as compilation. No assertion or timeout is weakened to obtain a pass; each failed round retains its source and complete raw result.

The historical template connection was unavailable during intake (shared PostgreSQL still starting). No write or restart was attempted there. Use a separate task-owned container from the already-installed pgvector image and current migrations. Keep its named data volume and final database histories; stop the owned container only after connection/resource audit. This environment change does not change the test denominator.

Retain the local A/B Git repositories, bundles, raw execution outputs, source/command linkage and pre-cleanup SQL history snapshots. Existing fixture cleanup may remove transient rows; report actual final SQL counts separately. Verify migration/index/gate state, zero connections, unchanged frozen source/configuration/tools and termination of all owned commands. Restore only explicitly recorded fixture-modified gate flags, preserving the monotonic epoch.

An independent native `trellis-check` reviews the full slice, finite raw inventory, source association, reports/history and resource closure. Main updates only TC-12 current evidence plus parent progress/gaps and task-local session notes, records the precise remaining S4 boundary, makes directed Lore work and record commits, rechecks HEAD/index/remote, and pushes the existing origin branch. Parent and business signatures remain open.
