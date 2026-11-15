# PAY-241 S2 P2 r08 independent check

**NOT_ACCEPTABLE_R08_A326_SOURCE.** The test-only public-DTO correction reached and passed its actual HTTP assertions, but the first fresh canonical normal run then failed on a separate incorrect reopen queue expectation. One top/named/leaf test failed, with zero skips or inventory errors. The other nine permitted commands did not run; no source, database or failed evidence was changed to obtain a pass.

## Scope and binding

This candidate has 4,968 files, manifest SHA256 `a3261a9d92d3e51b01c3d41d933a6005f885624197a6297595df188b0d375414`. Independent byte/mode inspection found exactly one router-test change from r07, with 4,967 protected paths unchanged and zero missing/extra/drifting files. The reviewer inspected the public response, actual authorization/terminal identity checks, P2 cycle, reopen and later original-key/fresh-refusal assertions. No product or SQL changed. ROOT supplied ten exact commands: fresh canonical normal20, canonical race21, then the eight unused race profiles. Only the first Go command executed.

## Actual result

The new public workflow GET returned HTTP200 with the following actual body captured in the new raw test stream:

```json
{"code": "completed", "collaboration_revision": "2", "delivery_confirmation": {"submission_id": "ba24f606-9d5e-4d7f-9f39-ec346cb8e689", "attachment_ids": []}}
```

The status key is absent, as the existing public success contract requires. The original consumed identity assertions subsequently passed. All three real source-issuer P2 enable/disable/re-enable PUTs returned HTTP200; the actual reopen POST and stage GET also returned HTTP200. The new first failure occurred at router test line300, where line299 requires `queue_state='open'` after reopen. The raw SHA256 is `79b48d0b2306c96c0f65fc24c5600dd3baf94e8d0d6ea4af609e72ff8e37c0d2`.

## Finding not fixed by this reviewer

**R08-K06-F02 — reopen queue contract.** `ReopenCollaborationSession` in `server/pkg/db/queries/collaboration_lifecycle.sql:22–25` explicitly sets `queue_state='paused'` and `pause_reason='issue_reopened'`. The existing lifecycle service calls that query and pauses each target at lines543–555. This preserves the established no-automatic-resume rule. The new test's open-queue expectation is incompatible with that source contract. ROOT assigned the existing test-only owner to correct the test on a new candidate; product/SQL changes and an extra Resume to manufacture an open queue are inappropriate.

This run did not log the immediate session row at the failing condition, and teardown removes it; a later empty row is not a runtime-state capture. The corrected test should record and assert the actual paused/reason/version facts while retaining non-done state, no new dispatch and the existing receipt/consumption checks.

A separate **unexecuted source-contract risk** was sent to ROOT before another run: the later fresh request uses reopened revisions/epochs with an old unused M1 authorization, while its test currently requires `stage_incomplete`. The existing Completion branch compares the original M1 collaboration target before the stage guard (`delivery_completion.go:249`, then `:286`), so the exact old-grant mismatch can be `delivery_completion_conflict`. Do not change product ordering or accept several error codes without identifying the exercised path. A separate no-Completion current intent can prove absent stage roles/scope if needed, with its own bounded refusal and complete effect assertions.

## Preserved preflight error and final audit

The first r08 sequence stopped before any Go command because the new read-only retained-history auditor sorted evaluation rows by an invented `id` column. Migration776 defines `evaluation_id`; the auditor reported SQLSTATE42703. Original helper bytes and its error remain preserved. A new auditor and execution directory corrected that exact field without any source/config/database change, then ran the first Go command once.

The corrected actual before/after audits verified all 4,968 source bytes/modes, the 0600 configuration, actual trusted Data HOME/TMP/GOTMPDIR ancestry and absent Git roots. Database20 remains at the exact835/exact864/barehex64 schema, with unchanged logical catalog/OIDs/constraints/ledger and zero other connections. Its immutable M1 history is correctly retained: one evaluation, two policy revisions, two authorizations and one consumed terminal; stage and quota facts in the audited tables are zero. The old generic empty-history postcondition is not applied to this canonical fixture.

## Verification and continuation

- Source/code review and current binding: pass for the single public-DTO test repair; no product delta.
- Original static formatting/vet evidence is preserved; actual new Go compilation reached the canonical test.
- Actual canonical normal20: FAIL; canonical race21 and the eight other race commands: NOT_RUN.
- The old r07 eight passing normal profiles were not rerun. Original r07 and this r08 failure remain separate source-specific results.
- No source integration, LIVE activation, manual API/browser startup, Git commit, new dependency, real model, fund or production operation was performed.

A new exact test candidate and ROOT descriptor are required before continuing. This report does not accept S2, A01–A12, the four actual Web/Desktop scenarios or the parent24-case matrix. Its JSON companion retains commands/raw/source/config/catalog snapshots, the preflight error and the exact captured public body.
