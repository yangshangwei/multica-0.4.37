# PAY-241 TC-02 requirements synchronization and history

Continue the existing parent task `11-13-pay-241-collaboration-acceptance` under the approved internal acceptance plan. Baseline HEAD and freshly fetched origin are `e9e97785286f881abc9cb54e2092f5a0f53ec2e8`; the initial index is empty. Preserve unrelated shared-worktree changes. Parent starts at 8 PASS / 9 PARTIAL / 7 NOT_RUN; this plan alone changes no case status.

## Change boundary

The remaining gap is evidence that two deliberate acceptance-criteria edits synchronize across actual product/developer/QA sessions and that all previous description versions remain readable in Activity. Existing description edits already own revision, full before/after history, actor/time and original operation receipts. No separate version table, restore system, business-field requirement or priority mapping is introduced.

The implementation owner may add only `e2e/collaboration-requirements-history.spec.ts` and a narrow `e2e/fixtures/collaboration-requirements-history.ts`. Reuse the existing editing control, password onboarding, recovery sessions, native renderer/preload and Activity disclosure helpers. Main owns private source/config/database/builds, task records/spec synchronization and directed Lore commits/push. Product changes require a demonstrated failure and explicit bounded diagnosis.

## Preregistered denominator

Two top-level journeys, with retries zero:

1. Production Web: three independent password-authenticated product/developer/QA browser contexts.
2. Desktop: three independent native Electron sessions with the matching built renderer/preload; installed production Main is outside this slice.

Each journey creates one PAY-shaped V1 description through the product actor's public setup API, preserving impact, reproduction and acceptance sections. Participant invitations are enumerated setup actions. Exactly two subsequent description writes use the actual Edit issue form: V1→V2→V3. This yields four explicit UI edits, six real actor sessions, eighteen current-version observations and six actual Activity inspections. A creation UI journey is not claimed.

For every version, every session must render the exact description and retain the corresponding real authorized issue-response revision; an unrelated API GET is not sufficient UI synchronization proof. For each session's Activity, inspect complete API pagination with limit 1 through the creation boundary and compare stable event IDs/sequences, actor, UTC timestamp, full before/after texts and original request/operation receipts. Read V1/V2/V3 in the actual drawer; comments are a different history.

Require exactly one event per explicit edit, unchanged original receipt identity after later edits, zero observer business writes and no new command/run/dispatch. Capture failed/unstarted scenarios and original raw results without merging passing fragments across rounds or increasing existing assertion budgets.

## Execution and acceptance

1. Freeze committed product source separately from peer WIP; bind dependencies, tools, private configuration, database catalog/gate and fresh runtime state. Build production Web and Desktop from these bytes and a matching API. Use password authentication explicitly and no real model accounts.
2. Implement and statically check the narrow harness. Independent `trellis-check` examines the assertions and source/environment binding before actual acceptance.
3. Run the complete two-journey denominator on the fresh owned runtime. Diagnose any failures before editing; retain each round and database. A fixture-only correction keeps product assertions and timeouts intact.
4. Stop only owned services; verify complete source/artifact/config identities, gate/catalog and zero database connections. Preserve scenario data and failed history.
5. Independent review decides the exact accepted range. Only then update TC-02, the parent progress/gaps and relevant QA spec. Commit named owned files with Lore records, recheck HEAD/index/remote and push the current branch.

TC-01/03 field/P1 semantics, real-model TC-04/14, S4 I02–I09, final-candidate S5 and business signatures remain separate. The parent remains in progress even if this slice passes.
