# Research: remaining TC-01/03/04/14 semantics and execution boundaries

- Query: After accepted TC-01 positive creation, TC-04 claimed-machine refusal, TC-06 concurrency, TC-12 revision regression, and S1–S3, is one new exact internal stimulus still justified for TC-01/03/04/14?
- Scope: internal source/contract/accepted-report research for these four cases only; no runtime verification.
- Date: 2026-11-20

## Findings

**NO_NEW_TEST_ONLY_STIMULUS_JUSTIFIED_FOR_THESE_FOUR_CASES.** The current parent ledger is **9 PASS /11 PARTIAL /4 NOT_RUN**, and all four reviewed cases remain PARTIAL. TC-01/03 retain unconfirmed business semantics; TC-04/14 retain actual-model evidence outside this session's allowed execution. A separate packaged-client harness exists, but its present scenario does not settle the remaining real-model clause. Adding another positive creation, assignment notification, machine refusal, fake timeout or output replay would repeat accepted proof without resolving these exact requirements.

This is a bounded research conclusion, not a claim that all remaining PAY-241 work is blocked or that current HEAD passed old tests. Other cases and S4/S5 are outside this four-case review.

| Case | Already accepted / actual owner | Exact remaining need and smallest useful next input |
| --- | --- | --- |
| TC-01 | Latest TC-01 independent check accepts actual production Web and matching renderer/preload positive creation, complete supplied PAY fields, title gate, initial mapping, creator/time and original identity. The real service requires source, resolved handler, nonblank title and request key; description/project/priority/dates are optional. | Decide whether “必填完整” means this PAY scenario's supplied checklist or a server-enforced completeness gate. If enforcement is intended, name the exact fields, affected scope (PAY template/project versus all collaboration), enforcement point (create versus triage/stage), and handling of existing incomplete issues. A new test cannot choose this contract. Do not make generic fields globally mandatory. |
| TC-03 | S2 A01 accepts stable business owner, current handler, designated roles and permission separation. TC-01 adds actual project, High, start/due date and current handler observations. Current enums are `none/low/medium/high/urgent`, with no inspected PAY P1 mapping. Existing assignment/transfer/mention rules have distinct recipients. | Supply the PAY P1-to-enum mapping and the triage notification's precise trigger, recipient identities/role selection, required content/destination, and replay/delivery expectation. Then test that named event through the existing notification owners. A single `assigned` row does not establish the full requirement. |
| TC-04 | Latest TC-04 check accepts actual claimed generic/restricted task credentials plus separate Agent hint against exact M1 grant, revocation and collaboration-close requests, with legitimate human controls and complete unchanged facts. | Real analysis needs selected authorized Provider/model/configuration and finite call budget, PAY source/input evidence, and a rubric/reviewer for correctness of cited basis, risks and confidence. Merely requiring those words in controlled output is not quality evidence. Current user instruction prohibits real-account calls; none were attempted. |
| TC-14 | S3 final report accepts R01–R09, including real test-owned process timeout, both Stop orders, original receipts across runner restart, explicit new retry, current-authority output uniqueness and late-result refusal. | Real-model timeout/recovery needs selected Provider/account scope, finite attempt budget and the allowed failure/timeout injection method, with expected observed failure/recovery identities. Packaged production Main is a separate client/platform scope: select its OS and pinned executable/ASAR/bundled CLI plus a matched API/source candidate and the exact client assertion. Neither model identity nor installed-client correctness can be inferred from renderer or fake-provider results. |

### TC-01: optional fields are the actual contract

- `server/cmd/server/router.go:1678` routes `POST /api/collaboration/issues` to the existing creation handler; no PAY-specific creation endpoint exists in this route.
- `server/internal/handler/collaboration_creation.go:13` defines optional pointer project/priority/date/handler fields. At line 80 the handler defaults to the actual human principal; at line 34 the handler requires human authority before decoding/creation.
- `server/internal/service/collaboration.go:156` owns the transaction. `CreateInTx` at line 200 validates source, resolved handler, nonblank title and bounded request key, not PAY prose sections or occupational roles.
- `server/internal/service/collaboration_creation_fields.go:74` defaults `todo` and `none`; lines 92–104 accept omitted/empty calendar dates. Optional selected values are validated under the existing catalog transaction.
- `packages/core/collaboration/creation-schema.ts:29` keeps project/priority/dates optional; line 40 requires source/title while description and handler remain optional.
- Existing `TestCollaborationCreationInitialFieldsAreOneRecoverableIssue` (`server/internal/service/collaboration_source_fields_test.go:244`) already covers selected fields, original identity and no AI command. Latest `research/tc01-independent-check-20261120.md:7` supplies the actual UI positive proof and line 47 retains its semantics boundary.

A negative test that insists missing description/project/dates must fail would encode an unapproved rule. A test asserting their current optionality would document existing behavior but would not close the original business-completeness question.

### TC-03: assignment, transfer and mention are distinct events

Current ingress and recipients are concrete:

1. Creation emits `assigned` only when actor and handler differ, to that single handler (`server/internal/service/collaboration.go:358`). Latest TC-01 proof already observes exactly one such initial notification per root before a separately labeled QA invitation.
2. `POST /api/collaboration/issues/{id}/handoffs` (`server/cmd/server/router.go:1705`) invokes `Transfer`. It changes the current handler and finishes with `transferred` (`server/internal/service/collaboration_workflow.go:131`). The shared `finish` path at line 371 sends that event to current valid participants other than the actor. The SQL recipient query (`server/pkg/db/queries/collaboration_workflow.sql:89`) selects non-revoked participants with non-disabled users; `validTarget` at service line 194 also excludes temporarily disabled users.
3. Ordinary metadata edits intentionally send no ordinary notification. Only newly added human body mentions produce `mention_added` (`server/internal/service/collaboration_edit.go:152`; `collaboration_notifications.go:68`). The before/after mention expansion uses the same locked participant set and excludes the actor.
4. Account notification list/read/archive are already exposed at `server/cmd/server/router.go:1672` and handled by `server/internal/handler/collaboration_notifications.go:11`. These are existing delivery/read owners, not evidence of a distinct “triaged” business event.

Existing `TestCollaborationWorkflowNotificationsAndWorkbench` (`collaboration_workflow_test.go:308`) covers participant notifications, actor exclusion, transfer responsibility and source-inbox isolation. `TestCollaborationEditRecordsCurrentActorAndImmutableExecution` (`collaboration_edit_test.go:28`) includes the explicit assertion that ordinary metadata editing does not change notification counts. S2's actual shared role form (`e2e/collaboration-stage-approvals.spec.ts:141`) and original A01 acceptance already distinguish business owner from handler. The current priority config (`packages/core/issues/config/priority.ts:9`) orders enum values but assigns no P1 business label.

Therefore the missing choice is not how to invent another notifier: it is which existing event, or a separately approved new event, the requested triage contract means. Required recipients might be the new handler, all participants or designated business roles; source cannot choose among those for the user. An `assigned`-only UI exercise would be a narrower extra observation and cannot be sold as closing TC-03.

### TC-04/14: actual-model evidence cannot be supplied by another controlled string

The two claimed-machine router tests are already present at `server/cmd/server/collaboration_machine_completion_refusal_test.go:24` and `:102`; their accepted source/runtime bindings remain those of the TC-04 independent check. They cover the authority clause without any real model. Ordinary tasks expressly designated by valid human grants retain their separate lawful execution contract.

For TC-14, `server/internal/daemon/collaborationrun/host_timeout_recovery_test.go:12` and `:106` exercise timeout receipt recovery and both Stop orders using test-owned execution. `e2e/collaboration-timeout-recovery.spec.ts:19` composes an actual 15-second test-owned timeout, retained history and an explicit new instruction/run. `e2e/collaboration-runtime-output.spec.ts:8` composes controlled output identity/receipts/current authority; archived S3's final acceptance preparation explicitly says no additional controlled stimulus was found for its accepted scope (`research/s3-final-acceptance-preparation-20261119.md:16`). These are existing coverage, not fresh executions by this researcher.

Real entrypoints exist, but they do not already implement the PAY quality oracle:

- `server/internal/daemon/collaborationrun/host_agentintegration_test.go:31`, `TestHostProviderAccountSmoke`, checks `MULTICA_RUN_REAL_AGENT_SMOKE=1` before executable discovery/account use and then uses an exact `HOST_SMOKE_OK` smoke prompt. It proves its named account/adapter scope, not PAY basis/risk/confidence or forced model timeout.
- `server/cmd/server/ac43_real_provider_test.go:30`, `TestAC43RealProviderProvenance`, requires explicit real-provider gate and allocated database, then measures runtime/author/grant provenance. Its exact marker replies are not analysis quality.
- `.trellis/spec/server/collaboration-context-experiments.md:17` requires preregistered source/model/scenario/trials and durable evidence, and separates a test process exit from a semantic verdict. Existing unrelated experiment budgets/approvals do not transfer into PAY.

The current user prohibits real model account calls. No installed executable, version/account probe or credential-store inspection was performed.

### Installed Main is an available but separate client scope

`e2e/collaboration-host-installed.spec.ts:36` already tests a packaged full Main with its preload/renderer. Current code supports an explicitly selected local Linux image **or** native Windows/macOS executable. Its opt-in at line 42 precedes package mounts, credentials, API fixtures and runtime enrollment; explicit missing images fail without pulling. The code pins ASAR/source/tool identities and records client OS separately from the native controlled Provider runtime.

Its stimulus is actual password login, controlled host launch, Stop, packaged-app restart while paused, and explicit Resume of mixed-author inputs (`:129`, `:159`, `:171`). It is not a PAY timeout-specific experiment and explicitly records `realProviderAccounts:false`. Re-running or adapting it can only support a separately selected packaged-client claim with matched source/artifacts; it cannot replace TC-14's real-model remainder or be counted as a novel PAY internal business stimulus merely because the parent retains an installed-Main limitation. This research neither declares that platform scope accepted nor changes its owner.

## Files found / source record

The adjacent JSON records SHA-256/size for each cited source, spec and accepted report and an observed branch-ref identity. The shared HEAD advanced during research; hashes identify the exact inspected bytes. No historical execution is relabeled as current-HEAD passing. The root role retains final source-integration and independent-check decisions.

## Related specs

- `.trellis/spec/server/collaboration-foundation.md`: human creation, current authority, source/handler and original key.
- `.trellis/spec/server/collaboration-source-editing.md`: metadata history, frozen inputs and notification boundaries.
- `.trellis/spec/core/frontend/collaboration-workbench.md`: account notifications and assignment wording.
- `.trellis/spec/web/frontend/collaboration-closure-qa.md`: controlled versus actual Provider and renderer evidence.
- `.trellis/spec/server/collaboration-host-execution.md`: controlled lifecycle, real-provider opt-in and separate packaged Main.
- `.trellis/spec/server/collaboration-context-experiments.md`: preregistration and semantic-result boundaries.

## External references

None fetched. The conclusion uses repository contracts, exact accepted reports and current source. No new SDK, dependency or product rule is proposed.

## Caveats / Not Found

This role writes only this research pair. It ran no test, build, TestMain, database/API/browser/native/provider operation or Git command, changed no product/spec/parent ledger, and started no service or background work. No credentials were read. Initial guessed filenames were corrected with `rg --files`; those lookup failures are research-tool path mistakes, not product/test failures.

The main session supplied initial HEAD `fc51c498c9270758974c5bafd0cbca6c391a9891`; later shared refs advanced. The manifest records the final observed ref by reading `.git/HEAD` and its loose ref, without a Git operation. The four cases stay PARTIAL, parent remains open, and no old result is invalidated or automatically upgraded. S4 external inputs, other cases, final S5 and business signatures require separate review.
