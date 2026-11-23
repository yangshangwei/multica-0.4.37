# R20 original four-scenario runtime entry check

Active task: `.trellis/tasks/11-13-pay-241-s2-stage-approvals`

## Verdict

**ACCEPTED for one canonical original four-case entry**, after the leader binds this review into the final live binding and single-command permit. No blocking correctness finding was found in the reviewed current inputs. This is entry readiness, not a passed runtime, S2, A01–A12, external integration or parent acceptance.

The prior independent product/source/output review `2d3751e4…` is reused. Its complete historical source and1,676-output audit was not repeated. This check independently rehashed the current API-start plan's48 references and draft live binding's60 references: zero mismatches.

## Actual recorded runtime and configuration

- API87271 and bridge87268 correspond to `bridge-ready.json`, startup metadata and the recorded live health response. The API is the same accepted R18 harness executable/source; health returns the matching `86fea651…` metadata.
- New owned production Web86953 serves64925, and renderer86966 serves64926. Web starts with production `next start`; renderer provenance names the accepted R20 `out/renderer` path. The recorded probe returns200 for API, Web and renderer, BUILD_ID `PpNZY2bAtTOUVfi6UgJKT` and renderer index `0be23842…`.
- The recorded probe validates password authentication, a fresh login using the retained initialized administrator, `/api/me` identity and `/api/admin/me` with `super_admin`. It does not initialize, change the password or change roles. Credentials remain private.
- Bridge and browser configurations agree on R20 frontend source, renderer URL, Web origin and the R20 control socket. The actual router's `CORS_ALLOWED_ORIGINS` includes64922/64925/64926; frontend API and WebSocket values point to64922.
- The initial incorrect bridge preparation is preserved in its own snapshot and superseded by the explicit source-rebind receipt. Current inputs use the corrected configuration. Database identity, administrator/JWT values, trusted artifact roots/constructor identity and model-key values match the retained R19 configuration; the reviewed model keys remain empty.
- API admission permits one launch command and one PAY database suite, with global maximum two database suites. The captured connection-budget calculation is100 maximum minus3 reserved minus18 other client connections, giving79 headroom for the proposed25 API connections. This is the recorded planning budget, not a newly measured pool size.

The new platform uses separate ports and recorded ownership. The retained peer Web39744/renderer39748 was not signaled. These statements are based on the hash-bound startup/probe receipts; this reviewer issued no fresh API or process query.

## Exact case, helper and deadline preservation

Both original case files and the four relevant host/stage fixtures match their R19 bytes exactly. The R19 metric-TTL fixture repair remains present; there is no additional R20 test-body or assertion change.

The original case inventory is one approval/QA case and three execution/completion cases. Both specs retain `actionTimeout:20_000`; all four tests retain `test.setTimeout(480_000)`. The original banner assertion and Playwright's default five-second expectation are unchanged. Existing explicit30-second poll and20-second response controls also remain unchanged. The top-level config's60-second default is unchanged and is overridden by each test's original480-second setting.

The copied launcher and Playwright config differ from R19 only in owned frontend/runtime path constants. The canonical command names the two original spec files through the existing config, with one worker, zero retries and `maxFailures=1`; it has no case grep or additional test selector. Its existing input hashes, owner/single-command permit checks, no-ambient-environment child launch and exclusive output creation remain intact. `runtime/four-scenarios-r01` was absent when checked.

This review approves that original inventory and its unchanged controls. A failure must still stop the run and retain unstarted cases; it must not cause automatic retries or a reduced denominator.

## Recorded natural history and staging

The live-before snapshot is a repeatable-read, read-only transaction. Relative to the pre-API snapshot, no old queue ID was removed. One old running synthetic producer naturally became `failed/runtime_offline`, one old deferred retry became `failed/runtime_reconnect_timeout`, and one new attempt2 retry row is deferred. The recorded live-before result is therefore12 queue rows:11 failed and1 deferred, with no lease or execution binding. Those rows are preserved and are not claimed permanently inert.

All19 original Host run IDs remain settled. All8 artifact execution IDs remain complete. No reservation is unreleased, one unknown stage intent remains, no rollback metric is present and gate booleans/epoch20 are unchanged. The live snapshot records five other connections after the owned API starts; the earlier pre-API zero-connection snapshot is not misreported as the live connection count.

The narrow filesystem check verifies staging root0500, the eight R20 generation directories empty/0700/owner-matching/non-symlink, and56 old generation inode/mode identities preserved. The configured generation list matches those eight new IDs. No old generation, sealed input, artifact or history row was reused, deleted or rewritten by this review.

Natural background changes do not require an empty historical database. The leader's final before/permit should preserve and classify any later natural changes while retaining the unchanged source, authority, gate and single-run controls.

## Handoff and verification boundary

Bind this accepted entry review with the actual final live binding, current before evidence, environment/configuration, unchanged launcher and Playwright config, and the one-command permit. Then execute the original four cases once. Preserve the entire original result and perform the separate after-state/runtime acceptance review.

Only this report and its JSON companion were written. No lint, type-check, test, DB/API/browser, build, install, process signal or Git command was executed. Current reference hashes, case/helper bytes, configurations and retained startup/probe/snapshot records were independently inspected.
