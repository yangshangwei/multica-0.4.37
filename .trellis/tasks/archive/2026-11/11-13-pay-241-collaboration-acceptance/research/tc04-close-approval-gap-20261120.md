# Research: TC-04 exact machine completion and human authorization refusal

- Query: What is the smallest new proof that an authenticated Agent cannot supply or override applicable human completion authority, without repeating accepted TC-13 or prohibiting supported ordinary Agent writes?
- Scope: internal; current production router, collaboration lifecycle and M1 authorization contracts, existing integration-test seams.
- Date: 2026-11-20 (Asia/Shanghai).

## Findings

**Execution correction, 2026-11-20:** the first normal run showed that generic
claimed `mat_` authorization create/revoke requests stop at the registered
`RequireHumanActor` middleware, returning403 with the exact one-field error
`this endpoint is only available to human actors`. The initial research below
identified the downstream M1 human guard but missed this earlier response layer.
JWT+Agent hints reach `delivery_completion_denied`; restricted task credentials
stop at the capability guard. The failed r01 source/raw/database remain retained.
R02 asserts each exact identity-specific boundary, with unchanged bodies, keys,
full-fact comparisons, finite denominator and timeouts. The independent result
is recorded separately in `tc04-independent-check-20261120.md`.

**Add one actual-router test file with two tops, nine refusal leaves and eleven named results per mode.** The concrete gap is fresh server-claimed generic/restricted `mat_` identity plus otherwise admissible human requests and comprehensive retained-fact equality. Existing source already rejects these identities. This research found a missing exact proof, not a demonstrated product defect.

TC-04's original CSV text is “分析包含依据风险置信度，Agent不可关闭任务”; its current status is NOT_RUN. A successful bounded refusal proof can support PARTIAL, not PASS: real-model evidence/risk/confidence quality remains independent. The authoritative parent state before execution is 9 PASS / 10 PARTIAL / 5 NOT_RUN. TC-13 and S1/S2/S3 are accepted and are not reopened by older snapshots.

### Contracts and real write owners

| Boundary | Current owner and behavior |
| --- | --- |
| Machine identity | `server/internal/middleware/auth.go:114` verifies persisted `mat_`, overwrites user/Agent/task/workspace headers, and supplies `CredentialTask` plus verified token. A successful same-token ordinary GET proves authentication before later refusal. |
| Restricted task ceiling | `server/internal/handler/task_capability.go:22` is the closed capability middleware. Its explicit 403 is different from downstream human refusal; it must be reported as such. |
| Human collaboration workflow | `server/internal/collaboration/access.go:23` excludes restricted, AgentActor and non-JWT/PAT principals. `server/internal/service/collaboration_workflow.go:431` rejects machines before transaction/reservation. `ChangeLifecycle` in `collaboration_lifecycle.go:358` delegates to this owner, then validates status/current workflow and invokes M1 preparation in the same transaction. |
| Explicit M1 acceptance | `server/internal/handler/delivery_completion_authorization.go:88` / `:97` requires human before creating a grant; service repeats that invariant in `server/internal/service/delivery_completion_authorization.go:49`. Revoke validates legitimate UUID/key/reason at the handler and the human principal in service `:250`. Expected human-gate error is 403 `delivery_completion_denied`. |
| Actual registered routes | `server/cmd/server/router.go:1645` installs Auth and TaskCapabilityGuard; `:1798` registers collaboration lifecycle; `:2162` registers completion authorizations and revocations. Existing `controlledArtifactRouter` at `agent_artifact_integration_test.go:61` calls actual `NewRouter`, not a custom shortcut. |

`mcb_` is a collaboration runtime capability and is not a valid substitute for the `mat_` credential accepted by these Auth routes. A 401 from an unsupported credential family cannot establish a business refusal. JWT plus `X-Agent-ID` is a separate deny-only hint control and cannot establish a genuinely claimed machine identity.

The “Agent cannot close” statement is scoped to the PAY collaboration human lifecycle and applicable acceptance boundary. Ordinary task consumers with explicit current human grants may complete their designated tasks. `server/cmd/server/delivery_completion_managed_integration_test.go:92` already exercises successful designated Contributor completion, exact task actor and one-time consumed receipt. Do not add a blanket product prohibition or claim that this new file re-executes that historical positive test. General off/observe policy also remains distinct from enforce.

### Existing proof and precise delta

- `server/internal/service/collaboration_lifecycle_test.go:213` covers creator authority, and at `:248` probes machine kinds with an empty service/request. It proves early identity ordering, not a current fully admissible claimed-token completion.
- `server/cmd/server/delivery_completion_authorization_integration_test.go:146` already refuses task signing and hints. Its producer task token originates from `auth.GenerateAgentTaskToken` plus direct row insertion in `agent_artifact_integration_test.go:85`; it does not claim the new real-daemon issuance path. Assertions compare one source snapshot/count, not a full original human-grant and side-effect map.
- `delivery_completion_authorization_integration_test.go:200` proves designated-task GET and refused revoke of an exact original grant, again using that seeded producer identity.
- `delivery_completion_collaboration_integration_test.go:442` already refuses direct producer-token self-completion against an issued grant. The machine owner is the source issuer, distinct from the collaboration initiator, and comparisons are limited to terminal count and evaluations. The new scope uses actual claimed identity owned by the otherwise authorized human actor, exact submission workflow and stronger unchanged-state evidence.
- TC-13's actual files are `server/internal/service/collaboration_stage_qa_protection_test.go` and `server/cmd/server/collaboration_stage_qa_protection_test.go`. Older research's proposed `*_qa_override*` names are not current files. Their accepted QA opposite/revoke matrix is not repeated. In particular, do not reproduce the TC-13 router's missing-report opposite-approval setup and call it fully admissible; its final report explicitly labels that authentication ordering only.

## Selected slice and finite denominator

Write only `server/cmd/server/collaboration_machine_completion_refusal_test.go` for code. Existing helpers are package-local, so one router file is sufficient; a second service file would duplicate the already traced early principal gate without adding needed provenance. Main owns task/plan/spec updates and environment, implement owns the new file, and independent check decides acceptance.

| Top | Refusal leaves | Positive controls and original facts |
| --- | --- | --- |
| Human M1 authorization protection | `generic_claimed_task/create_authorization`, `generic_claimed_task/revoke_original_authorization`, `restricted_claimed_task/create_authorization`, `restricted_claimed_task/revoke_original_authorization`, `human_with_agent_hint/create_authorization`, `human_with_agent_hint/revoke_original_authorization` | Real artifact packaging, public enforce activation, original human grant and original-key/read receipt; after every refusal full facts/receipt remain unchanged. A human issues the same otherwise-valid new grant and revokes the original after all refusal baselines, proving request admissibility. |
| Human collaboration completion protection | `generic_claimed_task/complete`, `restricted_claimed_task/complete`, `human_with_agent_hint/complete` | Real external submission and original receipt, real source-child artifact, current public enforce policy and actual human completion grant. All three attempts leave submission/decision/status/session/receipt/grant/effects untouched. The legitimate initiator then completes the exact same body and operation successfully. |

The proposed count is **2 tops + 9 leaves = 11 named tests per mode**, 22 executions across separate fresh normal/race databases. When encoding names, use slash-qualified single `t.Run` names if retaining this count; nested actor container `t.Run` calls add named records and must be preregistered before execution. Human controls and supported-read checks are assertions in the two tops, not extra business cases. Freeze final actual names before the run.

### Minimal helper composition

1. First top: `newDeliveryPolicyRouterFixture(t, "legacy", true)` (`delivery_policy_integration_test.go:52`) physically packages through current routes. `activateCompletionPolicy(t, f, 0)` (`delivery_completion_enforcement_integration_test.go:17`) uses actual owner/admin API. Build complete requests with `completionAuthorizationRouterBody(t, f, 1, humanConsumer)` (`delivery_completion_authorization_integration_test.go:47`). Capture original issuance key, response, exact SQL authorization row and GET/by-key view.
2. Second top: `newCompletionCollaborationRouterFixture(t, "submission")` (`delivery_completion_collaboration_integration_test.go:35`) creates a real ordinary source child, approved physical archive, observe→enforce policy, and genuine external acceptance submission (`:200`). `f.authorization(t, "submission")` (`:270`) supplies current issue/project/policy/workflow/status observations and exact submission; `f.grant` (`:333`) signs it and `f.complete` (`:340`) performs the real initiator lifecycle.
3. For the second top, the existing initiator intentionally lacks source membership. To create a task owned by that authorized initiator, explicitly add test-only source membership before forming the final authorization/body, and create claim fixtures through `testutil.New(testPool, testWorkspaceID, f.initiator)`. Record this setup; do not use the source issuer's fixture owner while claiming it proves the initiator's identity.
4. Reuse the actual claim pattern from `collaboration_stage_qa_protection_test.go:175`: own fresh runtime/Agent/ordinary issue/task, POST `/api/daemon/runtimes/{runtime}/tasks/claim`, inspect `handler.AgentTaskResponse.AuthToken`, verify persisted token hash binding and profile, then GET the supported ordinary issue with the exact returned token. Use generic NULL capability and restricted Code Reviewer template version2. The new claims—not the artifact producer's seeded token—must perform refusal requests. Account credentials and bearer bytes stay private.
5. Allocate all setup before the no-effect baseline. Do not revoke/expire token/task, change role/policy, omit evidence, or let stale observations replace the intended identity gate. For lifecycle, body `completion.operation_id`, request `Idempotency-Key` and issued grant's operation must be the same identity for every machine attempt and the later human control. A random key per actor would create an invalid-body confounder.
6. Retain strict full-row comparisons. `tc13RouterQAFacts` (`collaboration_stage_qa_protection_test.go:260`) offers seventeen actual workspace collections; extend the test-local snapshot with exact full M1 grant/terminal, handoff, original operation, private evaluation, artifact approval/execution, policy and relevant notification/iteration facts. Count-only comparisons cannot prove immutable rows. Include archive byte equality and explicit zero new command/run/dispatch/terminal/denied operation where relevant. Snapshot scopes must cover both the root and its real source child.
7. Legitimate human positive mutations occur after the complete refusal block. Original immutable row/receipt comparisons are evaluated before those intentional controls. Capture the final human completed receipt separately; do not demand unchanged full database state across a successful intentional completion/revocation.

## Files found

- `server/cmd/server/collaboration_stage_qa_protection_test.go` — real daemon-claim helper pattern and comprehensive immutable SQL collection comparisons.
- `server/cmd/server/delivery_completion_authorization_integration_test.go` — exact current authorization body, original-key/read/revoke API and existing limited machine proof.
- `server/cmd/server/delivery_completion_collaboration_integration_test.go` — genuine source-child artifact, exact external submission, current M1 grant and legitimate completion controls.
- `server/cmd/server/delivery_completion_enforcement_integration_test.go` — public policy activation and real writer completion examples.
- `server/cmd/server/delivery_completion_managed_integration_test.go` — supported designated task completion; prevents overbroad denial semantics.
- `server/cmd/server/agent_artifact_integration_test.go` — actual NewRouter and physical packager fixtures; distinguishes producer setup token from newly claimed refusal token.
- `server/internal/service/collaboration_workflow.go`, `collaboration_lifecycle.go`, `delivery_completion_authorization.go` — owning transaction/principal checks.
- `server/internal/handler/delivery_completion_authorization.go`, `task_capability.go`, `server/internal/middleware/auth.go` — public request boundary and actual machine authentication.

## Related specs and evidence

- `.trellis/spec/server/delivery-completion-enforcement.md` — explicit human signature, designated task consumers, exact M1 transactions and collaboration restriction.
- `.trellis/spec/server/collaboration-delivery.md` — immutable exact submissions and initiator decisions.
- `.trellis/spec/server/task-api-capabilities.md` — restricted route scope/current authority and actual claim provenance.
- `.trellis/spec/server/collaboration-stage-approvals.md` — accepted TC-13 boundary, not a new stage matrix here.
- `.trellis/spec/server/database-test-isolation.md` and `verification-environment.md` — source-bound fresh per-package/per-mode databases, no unconfigured TestMain.
- Parent CSV/latest progress, archived S2 final report and `research/tc13-independent-check-20261119.md` — current disposition and accurate reused scope.

No external documentation or network sources were needed. The adjacent JSON fingerprints current inspected files; it is a research-source manifest, not execution evidence. Main reported shared committed HEAD advancing through unrelated navigation/test-environment/E2E work to `ad5affc9a` during intake and froze a new committed candidate. This researcher performed no Git operations; Main must verify exact candidate/product identity before execution.

## Caveats / Not Found

No tests, database requests, TestMain, API/Provider/browser/service processes or product edits were executed by research. Only this research pair was written. Helpers use real production routing and local synthetic filesystem packaging with database-observed PR metadata; they do not call a real model, remote CI/payment service or deployed production system.

The original artifact producer token remains seeded setup in reused helpers; that is acceptable only when explicitly distinguished from the fresh claimed token executing the new refusal proof. Human JWT fixtures represent test actors, not business signatories or new password-login acceptance. The fixture authorization horizon is one minute: form fresh final bodies after setup, preserve current budgets and inspect any failure before changing scope. Restricted-route refusals never establish passage through the downstream human validator.

Independent review must verify final named counts, full admissibility via successful human controls, exact error boundaries, original facts and closure before any TC-04 promotion. If successful, the maximum proposed disposition is TC-04 PARTIAL and parent 9 PASS / 11 PARTIAL / 4 NOT_RUN. Real model quality, S4 I02–I09, final S5 and business signatures remain open.
