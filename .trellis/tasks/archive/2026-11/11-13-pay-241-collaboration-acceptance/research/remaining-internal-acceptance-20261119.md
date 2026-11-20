# Research: remaining internal PAY-241 acceptance

- Query: Find a concrete internal next slice after S1/S2/S3, especially TC-01, TC-02 and TC-13, without treating missing S4 inputs as a reason to stop all work.
- Scope: internal requirements, accepted reports and current source
- Date: 2026-11-19

## Findings

**Main selected TC-13 as the next test-only slice in this existing parent task.** Its missing stimulus is precise and does not require a real model or external system: after a valid human QA judgment, submit an authenticated Agent request attempting the opposite QA decision and an explicit revocation of that exact human decision. Assert rejection and unchanged authoritative judgment/history/projection. Existing source already has the human-only guard; this research found a missing proof, not an implementation defect.

Current parent ledger remains **7 PASS / 10 PARTIAL / 7 NOT_RUN**. TC-01/02 remain NOT_RUN and TC-13 PARTIAL. No row is promoted by this report. The parent PRD opening 4/13/7 summary and older S3-open wording are stale history; the CSV and archived S3 final report carry the current disposition. S2 is now archived, so all evidence links below use archive paths.

### What accepted evidence does and does not cover

| Case | Existing proof / actual source | Exact remaining condition |
| --- | --- | --- |
| TC-01 | S1 proves identity/closed-loop scope; generic creation and source-field tests already cover one recoverable issue, optional initial status/priority/project/dates/iteration and zero implicit Agent work. | A current PAY-shaped complete creation with each original required item checked, actual initial status mapping, creator/time and same-key original identity. Existing creation tests do not establish that every PAY business field is globally mandatory. |
| TC-02 | Metadata editing writes full before/after description and actor/time; Activity returns and renders these versions. S3 proves comment recovery, not description acceptance-criteria history. | Two deliberate acceptance-criteria edits, three actual product/developer/QA sessions observing the same version, and explicit prior-version回看 through Activity/complete page identity. Matching current reads alone is insufficient. |
| TC-13 | R33 accepts human QA return, later runtime report preserving that return, later human approval, and separate service machine/AgentActor refusals. | Exact valid machine override request **after** a current human QA return/approval; actual auth/router path, no malformed/stale/expired masking, original judgment and projection unchanged. |
| TC-03 | S2 stable business-owner/handler and roles are accepted; source creation/edit/date and assignment-notice owners exist. | P1 has no documented enum mapping found: current values are urgent/high/medium/low/none. Do not silently equate P1 with the old urgent fixture. Due date, explicit roles and actual notification still need the case-specific check. |

## Selected next slice: TC-13 exact post-QA override refusal

### Write boundary and reusable helpers

Proposed new files only:

```text
server/internal/service/collaboration_stage_qa_override_test.go
server/cmd/server/collaboration_stage_qa_override_integration_test.go
```

No product/schema/migration/client change is indicated. Main allocates a current private server source and distinct service/router normal/race databases; tests use existing `testutil` cleanup and real application methods. A failure first needs diagnosis before widening ownership.

Service helpers live in the same package and can be reused directly:

- `server/internal/service/collaboration_test_report_test.go:27` — `newStageRuntimeFixture` builds real managed runtime/source/policy/role/candidate context through existing service owners; `input`, prepared claim/permit, `RecordRuntimeTestReport` and `finishSquadRun` supply a genuine recorded report and settled run.
- `server/internal/service/collaboration_stage_review_test.go:26` — `newStageReviewFixture`; `stageReviewFixture.req`, `reviewParams` and `evidence` preserve actual current observations, candidate and report identity.
- `server/internal/service/collaboration_stage_execution_fixture_test.go` — accepted QA/execution fixture can be reused if it leaves the required valid QA phase; do not fabricate a decision row to avoid the human write.

Router/token helpers require a package distinction:

- `server/internal/handler/task_capability_test.go:26` — `newTaskCapabilityFixture` creates owned rows and calls the actual claim handler; its returned `response.AuthToken` is server-minted `mat_`. This unexported helper is **handler-package-only**. Use its exact claim pattern in a small local cmd/server helper; do not import or duplicate an entire test framework.
- `server/internal/handler/task_capability_test.go:61` — `taskCapabilityRouter` is a restricted custom router. It is a useful authentication pattern, but cannot be called “full production routing” or substitute for the selected actual-router lane.
- `server/cmd/server/collaboration_stage_routes_test.go:20`/`:42` — existing real-router fixture sends requests through `testServer.Config.Handler.ServeHTTP`, with `testutil.Call/Want/JSON`, actual Auth and task guard. Reuse that route and fixture pattern for new QA requests.
- `server/internal/handler/collaboration_stage_test.go:34`/`:69`/`:92` — `stageHTTPFixture`, `observations` and `body` show exact public setup/current-version bodies, but are also handler-package-only. A cmd/server test must use the real router and its own bounded setup calls, not injected request principals presented as HTTP authentication.

**Setup uncertainty:** this bounded read did not find one ready cross-package helper that both creates the exact two QA states and returns a freshly claimed `mat_` for the production router. The implementation owner must first compose existing cmd/server public setup/claim/QA calls with current provider/source configuration, without exporting/refactoring the service test framework. If that composition is materially larger than the two-file slice, report it before widening. A smaller handler-package test can directly reuse `newTaskCapabilityFixture` and actual Auth/task middleware for an owning-layer proof, but must be labelled as such and retain a concrete production route trace; it cannot be silently counted as a full `NewRouterWithOptions` execution. This uncertainty affects the test seam, not the already identified TC-13 requirement or the human-only product contract.

### Two finite phases and stimuli

Use separate fixture phases under each owning top: **QA returned** and **QA approved**. Keep the same exact root/candidate/report/role revisions valid during each refusal block.

1. Create an exact passing, settled runtime report. In the return phase, a real designated QA actor corrects it with `step=qa, decision=return`, a concrete reason and `expires_at=null`; retain that committed decision. In the approval phase, a real designated QA actor approves the exact report with current dependency and future bounded expiry. Do not advance or revoke upstream facts before the attempted override.
2. Obtain a real server-issued Agent `mat_` credential through an actual owned claim. Prefer a token whose bound owner is the QA person, so the test proves machine identity cannot borrow its owner's human role. Before the baseline, use that same token on its supported ordinary issue GET and require success with server-resolved Agent/task/user/workspace binding. Count claim/setup effects separately.
3. Build syntactically valid, current-observation stage requests. Use a new request key for an opposite `POST /api/collaboration/issues/{id}/stage/decisions` and a new key for `POST .../stage/decisions/{exact-human-decision}/revocations`. Preserve exact candidate version/digest, policy/role/stage/write epochs and nonempty reason; a return has no expiry, an approval has valid exact report/dependency/expiry evidence. Do not let 400 decoding, stale versions, expiry or missing roles become the alleged Agent denial.
4. Require the actual machine request refusal. General unrestricted task identity should reach the existing `forbidden` human gate; a restricted profile may correctly stop at `TaskCapabilityGuard` with its explicit 403. Label that boundary honestly. A restricted-token-only test does not prove the downstream human validator; the service machine-context test and a distinct actual generic task token close that distinction.
5. Separately repeat with the legitimate human QA JWT plus `X-Agent-ID` as a **deny-only hint control**. It is not proof of a real Agent identity and must not replace the server-minted token. Record the exact code/status for each path.
6. Before and after each attempted write compare the original QA decision row/ID/actor/reason/candidate/digest/creation time/expiry/dependencies, review-head and revocation sets, current signable/decision projection and original human operation receipt. Keep expected mutable observation clocks out of byte equality, but preserve all immutable business fields. Require no new decision/revocation/stage advance, command/run, grant/consumption or notification/outbox business event.
7. One legitimate fresh human QA action on an independent control may demonstrate the endpoint remains usable; it must not alter the human judgment used for refusal equality. If existing setup already proves the same well-formed human write, retain it rather than adding a redundant mutation.

The intended minimal denominator is one new service top and one new actual-router top, each with the two phases. Each phase owns explicit Decide/revoke machine and AgentActor controls with complete before/after facts; name the exact children in implementation before running. Normal/race use distinct fresh package databases, source/config binding and original gate/connection cleanup. No real Provider process or model account is necessary for this API authorization proof. **No new browser run is mandatory for this exact API guard** when final acceptance explicitly combines it with the already accepted S2 shared UI/report history. Do not label that combination a fresh browser execution.

### Critical credential distinction

`mcb_` is a collaboration runtime capability for run-bound endpoints. The generic Auth middleware does not accept it as the human/Agent stage credential; sending it to a stage route would usually prove only an authentication failure. Do not use 401 from that wrong credential family as TC-13's authenticated Agent refusal. `mat_` is verified in `middleware/auth.go:114`, overwrites Agent/task/workspace headers from the stored token and assigns `CredentialTask`; `Principal.Human()` then rejects machine identity. A forged `X-Agent-ID` alone is likewise insufficient evidence of an issued Agent credential.

### Copyable bounded dispatch brief

> Active task: .trellis/tasks/11-13-pay-241-collaboration-acceptance
>
> You are already trellis-implement; no recursive agents. Own only the two new QA-override test files listed in `research/remaining-internal-acceptance-20261119.md`. Preserve all peer WIP. Add a test-only TC-13 proof for fresh current human QA return and approval, then exact opposite machine Decide/revoke requests through service and actual production Auth/router. Use a server-issued `mat_` from an owned actual claim with a successful supported read before the refusal baseline; test JWT+AgentActor separately. Preserve complete original decision/projection/receipt and zero new business effects. No product/schema/client edits without a demonstrated failure and Main scope decision. Use only Main's allocated private source/config/package/mode databases. Do not call installed Agent CLIs, real accounts, models or external systems. Retain complete named inventory, raw pass/fail history and cleanup evidence; independent check owns acceptance. Existing R33 UI evidence may be source-qualified rather than replayed.

## Following slice: TC-02 acceptance-criteria synchronization and history

Use `description` as the existing requirements document; there is no separate acceptance-criteria column. Preserve content sections for impact, reproduction and acceptance rules, and use unique explicit V1/V2/V3 text. Product creates V1, makes two real Edit issue actions V1→V2→V3, and developer/QA sessions observe each exact description and collaboration revision. A paused third session can exercise current revalidation, but do not turn this into another 55-comment test.

Open the actual Activity drawer in all three authorized sessions and fetch complete API pages (e.g. `limit=1` to prove both edit entries and creation boundary). Match the two persisted event IDs/sequences, actor, UTC time, before/after complete text and the original edit operation/receipt. The DOM must show V1/V2/V3 where expected; an ordinary comment history panel is not the issue-description history. Require one accepted event per explicit edit and no observer mutation/Agent dispatch. Current source already supplies this, so begin with a test-only harness addition, not a new revision table.

Reuse `e2e/fixtures/collaboration-editing.ts` for real creation/edit controls and the existing password/native session helpers from the archived S3 field work. Its legacy `TestApiClient.login` setup cannot be copied unchanged into a password run; keep the current authentication mode explicit. Proposed later harness: `e2e/collaboration-requirements-history.spec.ts` plus a narrow fixture, one three-session Web composition and one matching renderer composition if the planned platform scope requires it. Main should register the finite denominator before execution. TC-02 can be assessed independently of P1/global creation-field semantics.

## TC-01 and TC-03: record the actual contract before adding mandatory fields

The generic create DTO has source/title, optional description, optional handler (defaults to the actual principal), project/priority/date fields and current-source catalogs. `CollaborationService.Create` requires a valid source, resolved handler, nonblank title and idempotency key; it does not require business prose sections, a project, dates or occupational roles at initial creation. Role configuration is a separate S2 action. Creation defaults to `todo`, priority `none`, paused execution; a selected nonterminal status is mapped through the actual source catalog.

A bounded positive PAY fixture can include impact, reproduction and acceptance in description; project; explicit handler; dates; then S2 business-owner/implementer/QA roles. It must retain actual generated issue ID/identifier, creator/time and same-key recovery, not force the number PAY-241. Check real technical required-field refusals and creation atomicity. This is immediately testable, but **cannot establish that every PAY business item is globally mandatory**. If the original requirement means enforced completeness before creation/triage, that is a separate product-contract choice; do not silently change generic creation semantics.

The code exposes only urgent/high/medium/low/none. No authoritative P1 mapping was found in the supplied conventions/config. The older urgent fixture cannot settle this. Keep TC-03's P1 item open until an explicit business-to-product mapping is recorded. Also note `EditIssue` deliberately emits no ordinary metadata notification except newly added mentions; creation-to-different-handler emits `assigned`, and transfer uses its own notification. Decide which real existing event the original “分诊通知” means before accepting it; do not infer notification from a role label.

## Internal versus external remaining work

| Case/family | Actionable internal proof | Separate boundary |
| --- | --- | --- |
| TC-04 | Actual machine close/approval refusal can be tested with real issued credentials. | Analysis quality (evidence/risk/confidence) needs separately authorized real-model output; controlled text is insufficient. |
| TC-06/07/17–19 | Existing source/version/permission/stage gate contracts are accepted; new exact gaps may be checked locally. | Real remote PR/CI, deploy/publish/rollback/metrics and business confirmation remain S4. |
| TC-09–11 | No new platform simulator is needed. | Real payment/order/fulfillment, repeated callback, actual 45-second delay and MQ compensation need supplied targets. |
| TC-12 | A changed local fixture-code revision and regression can be authored if specifically scoped. | R33 changed a same-code fault switch, not real business-code repair; do not promote from that existing evidence. |
| TC-14/20 | Archived S3 now accepts output/recovery/provenance; stale gap text can be reconciled without inventing a new PASS. | Real-model timeout and wider real S4 action audit remain distinct. |
| TC-24 | Final-candidate tooling/source preparation can continue. | Actual 100-observer/100-result final sampling is S5; old pre-S2 rehearsal is not final PASS. |

## Files found and code patterns

- `server/internal/service/collaboration_stage_actions.go:181`, `collaboration_stage_review.go:201`/`:345` — human-only Decide/revoke guard before service request mutation.
- `server/internal/service/collaboration_stage_review_test.go:333` — current machine/AgentActor denial exists before a QA-specific post-decision stimulus.
- `server/internal/service/collaboration_test_report_test.go:182` — settled-report QA acceptance setup and source-bound lifecycle facts.
- `server/internal/middleware/auth.go:114`; `server/internal/handler/task_capability.go:22`; `server/internal/collaboration/access.go:23` — actual machine identity, closed restricted-route guard and human classification.
- `server/internal/service/collaboration_edit.go:137` — full before/after description, timestamp, receipt and original request ID stored with the exact event.
- `server/internal/service/collaboration_activity.go:210`; `packages/views/collaboration/activity.tsx:59` — typed edit projection and actual before/after description rendering.
- `server/internal/service/collaboration_source_fields_test.go:244` — recoverable initial field/source/date/priority/iteration proof, not PAY required-field enforcement.
- `server/internal/service/collaboration_creation_fields.go:74`; `packages/core/collaboration/creation-schema.ts:45`; `packages/views/collaboration/creation-form.tsx:287` — actual defaults, optional business fields and title-based submit gate.
- `packages/core/issues/config/priority.ts` — five named priorities, no P1 business mapping.
- `e2e/collaboration-stage-approvals.spec.ts:522`/`:555` — accepted human QA return/approval and retained report history; exact after-QA machine override absent.

## Accepted references and current snapshot

- [S1 report](../../archive/2026-11/11-13-collaboration-agent-identity-baseline/report.md) — author identity and bounded three-account closure.
- [Archived S2 final R33 review](../../archive/2026-11/11-13-pay-241-s2-stage-approvals/research/r33-full-s2-a01-a12-independent-check-20261118.md) — explicitly retains TC-13's exact post-QA request gap.
- [Archived S2 report](../../archive/2026-11/11-13-pay-241-s2-stage-approvals/report.md) — accepted S2 lifecycle/source disposition.
- [Archived S3 report](../../archive/2026-11/11-13-pay-241-s3-recovery/report.md) — completed R01–R09 and current seven accepted parent cases, with external scope retained.
- [Parent matrix](../test-matrix.csv) and [original current-baseline analysis](current-baseline.md) — current statuses and why old historical PASS labels are insufficient.

Snapshot HEAD is `f9e273584f87079a16b5db1bb5bf4ace7867fa4e`; source/reference fingerprints are in the adjacent JSON. S2 archive happened during this read and all final report links use its archive location. No external documentation was needed.

## Caveats / Not Found

No code, task, matrix or account was changed; no tests/build/API/DB/services/real-model call ran. Only this research pair was written. Existing service tests and source reachability are not a newly executed TC-13 pass. Main owns the fresh private source/databases, exact test inventory, independent check and any parent-case promotion. P1 and global PAY-field enforcement remain unconfirmed semantics and do not block the selected TC-13 test-only slice.
