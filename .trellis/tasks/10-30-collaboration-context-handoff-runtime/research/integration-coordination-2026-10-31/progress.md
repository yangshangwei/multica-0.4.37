# Runtime integration coordination

This is the coordinator's live handoff, not a completion report.

## Scope and ownership

- User approved the six-step parent plan in session `01a1920d-a340-7dd3-9d25-9191aa128cb0`.
- Main runtime implementation remains owned by session `01a191c6-bf9b-74b3-b80e-8258a46b4b5b` until its bounded handoff.
- Launch injection remains owned by session `01a191c6-6a2a-7313-b674-485c195a5cae`, worktree `/Volumes/artisan/code/2026/mc-handoff-launch-20261031`. It agreed not to integrate into main; this coordinator will apply its owned-only patch and preserve checkpoint hooks.
- Integration worktree: `/Volumes/artisan/code/2026/mc-runtime-integration-20261031`, branch `codex/collaboration-runtime-integration-20261031`, base `e3620721e`.
- Parent PRD/implementation/status bookkeeping belongs to this coordinator. Runtime product sources in main are not edited by this coordinator before owner handoff.

## Captured baseline and setup

- `runtime-source-snapshot.json` and `.tar.gz` preserve 87 changed files from a consistent read of the moving main worktree. This is an unverified source snapshot, not a passing HEAD.
- Clean-base `go test ./pkg/protocol -count=1` passed before overlay.
- `pnpm install --frozen-lockfile` completed with pnpm 10.28.2; no dependency changes.
- Task-owned Go database: `multica_runtime_integrate_20261031_f2478725`. Credentials remain only in `/tmp/multica_runtime_integrate_20261031_f2478725.env`; metadata/logs live in the integration worktree's `research/integration-verification/`. Migration passed. Service and handler suites run sequentially; this database is never served by the API.

## Initial snapshot checks

- Protocol: 16 top-level / 66 including subtests passed, zero skips.
- CLI: 24 top-level / 92 including subtests passed, zero skips.
- Frontend: 117 core tests in 7 files and 17 views tests in 3 files passed; both package typechecks and scoped ESLint passed.
- Router gaps from the earlier audit are now closed in this snapshot: checkpoint/seal, runtime recall/content, handoff snapshot/bind/source, and human handoff routes match callers.
- Daemon/client suite passed 11 top-level / 47 including subtests before a timeout in `TestHostCheckpointBackpressureDoesNotBlockMessageDrain`. The body reached deferred `httptest.Server.Close`, while the fixture handler waits indefinitely for request-context cancellation. Checkpoint owner was asked to add an explicit fixture release. No production failure is inferred from this cleanup stack.
- New service and handler targeted checks are running on the exclusive Go database; full suites wait for final handoff/integration.

## Pending coordination

1. Receive `research/integration-handoff.md` from runtime owner and `research/launch-injection-handoff.md` plus owned-only patch from launch owner.
2. Reconcile changed files incrementally against the saved snapshot; do not replace shared host files wholesale.
3. Runtime owner must check delegation JSON escaping against the 256 KiB handoff envelope: 64 KiB of `<` can expand to roughly 384 KiB under default Go JSON escaping. Client must fail closed; server must preserve admitted assignment and budget contracts.
4. Finish deterministic integration, full-scope review, production Web and native Desktop verification; retain raw failures and exact source/diff metadata.
5. Knowledge-promotion prerequisite research is independent and read-only; candidate writes wait for stable handoff/source/authority contracts.
6. No real Provider calls during coding. Original experiment next slot remains E2a trial 2 with cap 23; new-runtime real evidence requires its own source-bound registration and must not silently consume or rewrite the original series.

## Integration and initial browser progress

- Launch owner is READY. Owned patch SHA-256 `40772c20f0848e48e96c4eb2e340c1d3ed5441ad1849177c29d933c8d7ea8927` applied only to the integration worktree; all 8 resulting file hashes match its manifest. Main product tree remains owned by the runtime workflow. Handoff is `research/launch-injection-handoff.md`.
- Design now specifies one run-bound source snapshot/memory across retries and outer fresh fallback, with separate actual-input hash/attempt binding for every launch. It does not reanchor to newer source facts during fallback.
- Combined `host_handoff_checkpoint_test.go` is owned by native `resume_check` in the integration worktree. It is red for bind refusal/fallback: recording not sealed; lost checkpoint ACK not replayed. Original proof/termination succeeds. Runtime owner is fixing the missing recovery/seal production calls. External launch owner was told not to duplicate this file.
- Frontend initial review found a red retention page-chain repro: individually valid pages with different recording streams are combined. Core owner is fixing it and checking handoff query recovery.
- New E2E ownership: native `experiment_resume` owns only `e2e/fixtures/collaboration-runtime-handoff.ts`, `e2e/collaboration-runtime-handoff-web.spec.ts`, and `e2e/collaboration-runtime-handoff-desktop.spec.ts` in integration.
- Browser API: `http://localhost:18080`, password mode, browser-only database `multica_runtime_browser_32d8f014`. Registry: integration `.omx/runtime-dev`; use `ENV_FILE=.env.worktree MULTICA_DEV_HOME=<integration>/.omx/runtime-dev` with make up/down/env-exec. Source deployment and other task databases are untouched.
- Initial production Web: `http://localhost:13000`, build ID `oIbqTqiNWd3_eOixPDKgv`. API/Web `source_id=d84c14b...` identifies the initial snapshot, before launch delta. Both HTTP probes passed.
- Initial Electron renderer build passed; static renderer `http://localhost:6274` matches built index SHA-256 `d04be0a533f16b0819c6b86c2a4e3708126836e486a326f445f60419e1bcaaab`. Static server tool session is `6091` and is coordinator-owned.
- Initial native E2E passed once, 5.5s, zero skips/retries: deep link, handoff source, unknown recording tail, full-comment recovery.
- Initial Web proves current handler/source, typed prior result API, checkpoint retained 2/observed 5, placeholder/fulltext, 55 comments in 3 pages, receiver removal clearing/404, source continued read and no runtime-token route use. One failing assertion remains: correct source-run href becomes `/account?view=current` on click and drops issue/run. Runtime frontend owner has exact URL evidence.
- Initial browser evidence is exploratory and MUST be rerun after final code delta and rebuild. Raw reports/screenshots are under integration `.omx/runtime-dev/runtime-handoff-*`.

## Remaining independent owners

- `resume_check`: combined launch/checkpoint test and integrated launch/race/vet validation; no product source edits.
- `frontend_integration_check`: initial frontend review completed, no source edits.
- `knowledge_preflight`: completed prerequisite research; drafting planning-only knowledge design/implementation files under runtime research. No candidate writes until runtime contracts stabilize.
- External launch thread: read-only source-purge/snapshot/bind security audit, output `research/security-source-audit.md`; no overlap with combined test or knowledge drafting.

## Later verification and review handoff

- Full initial `pnpm lint`: 6/6 tasks successful (3 executed, 3 cached), warnings only. Full `pnpm typecheck`: 9/9 (4 executed, 5 cached). Normalized hash of 3,586 source files unchanged. Evidence `integration-verification/full-static-initial/`.
- Full initial `go vet ./...` and `go build ./...` passed; server source hash/file set unchanged. Evidence `integration-verification/full-go-static-initial/`. Full TS tests and broad service/handler suites still wait for final source fixes.
- `host_handoff_checkpoint_lifecycle_test.go` is frozen/released for runtime owner, SHA-256 `bdcfd02210d871918a3e16537b6605bbca9c93381b9067effe40a466857830c9`. With race: blocked-recording proof-before-release passes; a fresh Runner reopening ledger fails to replay pending original checkpoint. No source file except this new test was edited by that pass.
- Runtime owner is adding a bounded total telemetry-recovery deadline, preserving phaseProof priority and proof completion despite telemetry errors; monotonic seal sequence/SQL bound is being finalized. New SQL bound observed by owner is LIMIT 290; final sqlc and hash handoff pending.
- Source audit is complete (`research/security-source-audit.{md,json}`). Three P2 integrity defects routed to runtime owner: runtime access omitted EventSequence/InformationRequestID and request provenance used hardcoded zero; untrusted cursor missing run_id inherits a trusted default; handoff source kind reflects irrelevant caller run/version metadata. No new content permission or purged-body resurrection was proven. Regressions must use real runtime entry, literally omitted run_id, and valid but inapplicable run/version values.
- Provider blocker clarified without a new call: all original E2a 429s have `gateway_concurrency_limit` / account concurrency exceeded. See `provider-blocker-analysis.json`; this does not prove current recovery or identify other account users. Original series waits for an independent serial window after coding/review.
- Knowledge draft files now exist under runtime research. Native `knowledge_plan_review` is independently checking them before promotion; findings so far concern suppressing retained candidate body after source purge, session-wide rather than last-task resume eligibility, and ordinary pre-launch failure lifecycle (cannot reuse a dispatched-only requeue primitive after task running).
- Main runtime source is explicitly NOT RELEASED yet. Do not overwrite shared main product files. On release, reconcile each file against `runtime-source-snapshot.tar.gz`, preserving integrated launch changes, new E2E and two combined regression files. Then rerun fixed red cases, full quality gate, rebuild/restart only this coordinator's API/Web/static renderer, and rerun browser suites.
