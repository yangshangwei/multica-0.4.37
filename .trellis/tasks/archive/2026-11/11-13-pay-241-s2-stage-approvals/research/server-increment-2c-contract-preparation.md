# Research: S2 increment 2c execution and rollback evidence contract

- Query: define the smallest implementable handoff for release/rollback approval, trusted execution history, original-operation recovery and rollback metrics.
- Scope: internal source/spec research only; recommendations for the existing S2 task. No product, wire, database, task-state or Git changes; no external calls or tests.
- Date: 2026-11-14.
- Status: **proposal, not frozen or served**. The sole server owner must freeze the chosen request/read changes in wire §13 before implementation. S2 2b was still changing during this review; the companion JSON records inspected bytes and symbols.
- Coordinator direction: retain the current-valid-approval gate. Late execution observations may become history, but do not renew approval, authorize another invocation or advance A9. Do not add an existing-deployment endorsement/renewal operation in 2c.

## Findings

### 1. Existing code to reuse, and its limits

| Source | Actual pattern and implication |
| --- | --- |
| `service/collaboration_stage_review_types.go:21` | `CollaborationStageReviewEvidence` has build/environment, deployment ID and rollback target, but no metric reference or reviewed execution-target version. |
| `service/collaboration_stage_review.go:174` | `stageReviewPayload` is the retained evidence/dependency envelope. Add a private resolved execution binding here instead of modifying immutable code-candidate identity with later deployment observations. |
| `service/collaboration_stage_review.go:199` | `DecideStage` already owns human, role, revision, candidate and source checks. Its current `state.task.Stage == p.Step` gate needs a 2c mapping: release decisions occur in `release`; rollback decisions occur in `verification`. `rollback` is not a workflow stage. This is unserved 2c work, not a defect claim against served code-review/QA behavior. |
| `service/collaboration_stage_signer.go:99,208` | `stageSignerCurrent` and `checkStageFinalAuthority` retain original signer/member identity and recheck the last database clock after observers. Reuse the issuer checks for execution admission, but check the real internal integration consumer separately; never manufacture a human context for it. |
| `service/collaboration_workflow.go:455,483` | The real workflow lock prefix and source-member fence precede the final access read. New release/rollback writes must use this boundary; an earlier permission boolean or user-row lock does not fence member deletion. |
| `service/delivery_completion_webhook.go:21,61,187` | `DeliveryWebhookProof` is internal, `json:"-"`, and carries an authentication-only verifier checked against locked current trust. It is a useful proof shape, but its actual contract is PR close/merge for ordinary issues; do not repurpose its accepted states or infer deployment support from a valid PR signature. |
| `handler/agent_artifact.go:343,442,462,484` and `queries/agent_artifact_execution.sql:39` | A definite durable claim precedes the sole side effect; ambiguity grants no invocation. Original nonce/spec finalization can record history without requiring the former human's live account. Reuse this sequencing, not its table or local packager. |
| `migrations/766_agent_artifact_execution.up.sql:3` | The artifact table fixes action to `package_release_candidate`, policy to `artifact-tar-v1`, and environment to `local-synthetic`. It is not a deployment ledger. |
| `service/admin_operation.go:83` and `queries/admin_operation.sql:6` | Administrative cancellation carries organization/platform authority and fixed `task.cancel` predicates. Reusing it for stage deployment would introduce the wrong authority and reconciliation model. |
| `service/admin_alert.go:22,46` and `handler/admin_health.go:17` | Existing observations describe installation liveness, queue age, execution failure and platform health. They contain no PAY deployment/query/window/threshold evidence. They cannot stand in for rollback business metrics. |
| `service/collaboration_stage_review_view.go:122` | R4 already has a single protected source-evidence group, current signer checks and bounded arrays. Extend this projection; do not add a parallel report/approval workflow. |
| `cmd/server/router.go:403` and `handler/collaboration_stage.go:161` | `NewRouterWithOptions` returns the actual Handler; `stageService()` constructs a service for each request. This provides a practical test-only injection point for the new narrow source resolver without a public fixture route. |

### 2. Recommended boundary: narrow ledgers, no external executor

Keep 2c inside the collaboration service and three narrow immutable record types. S4 owns actual provider SDK calls, target configuration and original-operation queries. S2 must not add a scheduler, a generic connector manager, a shell runner or payment logic.

**Internal source seam, proposed:** a `StageExecutionSources` dependency resolves an exact target and authenticates observations using current locked authority. It is supplied by trusted server construction and is nil/unavailable by default. It has no HTTP decoder, registration endpoint or `verified`/`simulation` input. Existing GitHub/VCS identity and lock helpers may support a concrete adapter, but their mere presence does not mean that adapter supports deployment or metrics.

The target resolver receives the actual `pgx.Tx` for current configuration/trust fences. Any remote preflight happens before those locks. A proof verifier inside the transaction authenticates the already captured immutable response/event and returns scoped identity; it cannot decide stage policy or call arbitrary remote URLs.

Proposed internal signatures (names are handoff suggestions, not existing APIs):

```go
PrepareStageExecution(ctx, request, sourceProof) (admission, error)
RecordStageExecutionObservation(ctx, observation, sourceProof) (receipt, error)
StageExecutionReceipt(ctx, originalIdentity, originalRequestID) (receipt, error)
RecordStageRollbackObservation(ctx, metricObservation, sourceProof) (receipt, error)
```

`admission` distinguishes **new definite admission** from **historical lookup**. Only the former contains an unexported, nonserializable dispatch permit. An original-key lookup never reconstructs that permit. A caller must not enqueue or transfer it to another worker. If the trusted caller does not invoke immediately, the durable unresolved intent remains; it is not reclaimed.

`checkStageFinalAuthority` currently also checks a human caller credential. Extract only its existing retained-signer portion if needed, keeping the final shared clock check. Human A7/A9 still check the actual human caller; internal admission checks the actual registered integration proof. Do not call it with the approver's reconstructed JWT/PAT.

#### Minimum retained identities

| Record | Required binding |
| --- | --- |
| `collaboration_stage_execution_intent` | Server intent UUID; workspace/root; candidate ID/version/digest; lifecycle, role and policy revisions; deployment/rollback kind; exact approval ID and dependency IDs; internal original request UUID/digest; private source/target snapshot and public target digest; intended artifact; original deployment fact and rollback metric reference when applicable; original external request key; original executor/claim identity; database admission time and dispatch expiry. |
| `collaboration_stage_execution_observation` | Server observation/fact UUID; exact intent; canonical provider observation key/version and digest; authenticated source identity; external operation ID when one exists; result; actual before/after artifact observations when known; external observed time and server received time; retained redacted evidence reference/digest. |
| `collaboration_stage_rollback_observation` | Server observation UUID/version/digest; workspace/root/candidate/lifecycle; exact original deployment fact; stable target identity/environment/deployed artifact; current metric-source identity/trust; query/rule identities and digests; actual observation window/time and finite validity deadline; normalized `anomaly | healthy | unknown`; bounded redacted summary; retained source evidence digest; exact predecessor/version when superseding a prior observation. |

Source identity must separate the **code PR integration**, **deployment integration** and **metric integration**. Use a private typed tuple such as adapter name/version, local integration ID, trust/config generation, stable remote instance/tenant/target identity. Do not store bearer credentials or accept source identity from normal stage JSON. The public target digest hashes canonical non-secret identity/version fields, not a raw secret or arbitrary JSONB text.

All references in v1 must resolve within the root's source workspace. Cross-workspace integration delegation is not introduced. Canonical IDs are nonzero UUIDs where server-owned; native IDs remain bounded strings, not coerced UUIDs. Reuse the stage decimal-string wire convention for versions and lowercase SHA-256 digests. Canonical typed records need domain-separated hashes; transport timestamps and generated IDs must not change same-request identity.

### 3. Admission, observation and recovery sequence

1. Resolve only a configured target/provider. Verify current issuer and full code-review → QA → release or rollback dependency chain, exact source/build/target binding, current policy/roles/lifecycle, source membership and integration trust under existing locks. Recheck the final DB clock after observations. All rejected admission paths invoke the external adapter zero times.
2. Before any external call, atomically insert one immutable intent and consume the approval's execution opportunity. Use uniqueness on the approval and original internal request. A second caller, changed key or process restart cannot create another intent for that same approval.
3. Commit. A commit error/ambiguity returns no dispatch permit and produces zero invocation from that caller. A later lookup can identify an existing intent but cannot authorize calling again. The sole definite owner passes the stored external request key and exact target/build/conditional version to the adapter.
4. Never hold a database transaction across network execution. S4 must use provider-side idempotency and, for rollback, a conditional target/deployment version. Local issue locks cannot stop another system from deploying to the same environment. A provider without the required conditional/query capability remains unsupported for automatic execution; do not infer safety from loopback fixtures.
5. Record observations through the internal authenticated boundary. This is not another human approval and does not change the stage. Lock the original intent and compare its immutable binding. Do not relabel a late result with the current candidate.
6. Exact receipt-key/digest replay returns the same minimal receipt before new admission checks. A different payload under the key conflicts. Canonical provider observation identity prevents repeated delivery attempts from manufacturing multiple facts; delivery-attempt logs stay distinct from execution identity.

#### Pending, unknown and definitive facts

An intent with no observation is **pending confirmation**, not proof that the external call started. An explicit transport/commit/query ambiguity is **unknown**. Neither belongs in R4 `execution_facts` or unlocks A9. No timeout converts either into `failed` or authorizes another call.

An authenticated terminal provider observation with an actual external operation identity may create one `succeeded`/`failed` fact. Success requires the exact intended target and observed resulting artifact. A pre-operation refusal with proven zero execution should be retained as `not_started` in operation history, not fabricated as an executed deployment with a made-up native operation ID. A provider timeout is not `not_started`.

At most one definitive terminal outcome belongs to an intent. Unknown observations can precede it. A terminal failed execution is never overwritten by a later successful retry; a genuine retry requires a new explicit approval/intent and a different external execution identity. Ambiguous or contradictory terminal observations remain a conflict requiring investigation.

Before permitting a new intent for the same target, reject an unresolved earlier intent rather than allowing a new approval/key to evade uncertainty. The target's stable identity, not a friendly environment label or issue ID alone, defines that check. S4 must still enforce external conditional state against deployments performed outside Multica.

#### Late observations and revoked/expired authority

Historical finalization validates the original intent and authenticated producer, not a forged context for its former human approver. A late result can therefore be retained after that approval expires. Failure to authenticate the source remains a refusal; “historical” does not authenticate an old callback by itself. If a changed trust generation cannot verify the original operation, retain the unresolved intent and require a currently authenticated original-operation lookup; do not accept an old secret or renew its authority.

**A9 remains conservative:** its required current approval can be expired, revoked, superseded or otherwise inapplicable even when a historical deployment succeeded. The fact stays visible to authorized readers, while advancement is blocked. This proposal adds no renewal, endorsement or permission to reuse the old execution. Existing explicit return/rework/new-approval paths remain explicit human actions; they are not described as an implemented way to retroactively rebind an old deployment. The user may face a blocked historical deployment until a separately reviewed path exists; do not force or silently repeat deployment to make the UI progress.

Once an approval has admitted an external execution intent, A8 must not pretend that revocation cancels that execution. Extend its existing `stage_decision_not_revocable` rule to the consumed dispatch opportunity. The admitted operation's history remains recordable; external cancellation is not added in this increment.

### 4. Minimal public contract amendments to freeze

No new public write endpoint is required. A7/A9 remain the existing human actions; R4 supplies bounded read evidence. Proposed additions:

```json
"build": {
  "artifact_digest": "<hex64>",
  "environment": "<existing bounded name>",
  "target_digest": "<server-issued exact target/config/trust digest>"
},
"rollback_observation": {
  "observation_id": "<server UUID>",
  "observation_digest": "<hex64>"
}
```

- `build.target_digest` is required for a new release approval. R4 offers at most 50 currently readable `execution_targets` with environment, target digest and `source_kind:"integration"`; there is no target-management UI or editable integration registry in S2. Missing, ambiguous or unsupported mappings are unavailable. Re-resolve and compare the target at signing and dispatch; a changed mapping/trust version invalidates the old binding.
- `rollback_observation` is required for rollback approval and nil for other approval steps. Reason-only returns may omit it; an optional supplied reference must still be same-candidate/deployment/target and currently readable. Store the resolved private source/target binding alongside `stageReviewPayload`, not in the original immutable code candidate.
- Define `evidence.deployment_id` as the **server UUID of a successful deployment fact**. It is neither the internal intent ID nor the provider's native operation ID.
- For rollback, require that deployment's release approval, the exact metric reference, and an independently resolved allowed rollback artifact. Never default to a “latest” artifact. If the only supported source is the deployment's recorded previous artifact, explicitly freeze that v1 restriction; unknown previous artifact remains unavailable. This research does not silently choose an arbitrary historical target policy.
- R4 adds bounded `rollback_observations` containing the ID/version/digest, deployment fact ID, environment/target digest, observation window/deadline, normalized condition and bounded summary. Raw samples/credentials stay outside the projection. A free-text reason or manual report is not a metric observation.
- Keep `execution_facts` restricted to confirmed execution outcomes. Add bounded `execution_operations` for intent ID, kind, approval ID, environment/target digest, state `pending | unknown | not_started | succeeded | failed`, native operation ID nullable until observed, and confirmed fact ID nullable. These are read projections, not new action capabilities. Do not manufacture a fact to fill an unresolved operation row.
- A successful fact's artifact must match the admitted target. For failed facts, existing `artifact_digest` must be documented as the intended artifact, not evidence that it is live; retain actual before/after internally and never infer success from that field alone.
- Source denial keeps the existing single anonymous withheld group: baseline null, all private report/decision/execution/target/metric arrays empty, `blocked_evidence:1`, identifier-free blocker. Do not disclose hidden row counts or whether an external operation exists.
- Release decisions use the `release` stage. Rollback decisions use `verification`. A9 release → verification requires a selected successful deployment fact tied to the selected current-valid release approval; a failed, unknown, wrong-target, wrong-artifact or merely approved operation cannot unlock it.

Metric freshness must be explicit. A new trusted observation supersedes an exact previous observation/version under a root/stream lock; stale native observations cannot become current merely because they arrived later. A newer healthy/unknown observation or expired validity horizon invalidates an earlier rollback-trigger observation. Use a finite deadline supplied by the registered rule contract, and bound approval expiry by it. Do not invent a PAY metric threshold or a default freshness window before S4 supplies the real rule/version.

Suggested new refusal codes to agree before client work: `stage_execution_unavailable`, `stage_execution_target_changed`, `stage_execution_pending`, `stage_execution_conflict`, `stage_rollback_observation_unavailable`, `stage_rollback_observation_changed`. Reuse existing signer/policy/evidence/candidate codes where they already express the condition. These names are proposed, not implemented. Unknown codes must remain blocking in old clients.

### 5. Practical positive proof through the actual router and UI

| Approach | What it proves | Decision |
| --- | --- | --- |
| Production-shaped target/intent/fact rows seeded into an owned DB, production API reads and UI actions | Public consumer/projection/completion behavior for those rows; does not prove source authentication, pre-call consumption or one invocation. It also needs an actual resolver able to read the target binding. | Useful only as explicitly limited consumer coverage. Never label it the entire 2c positive chain. |
| Test-only API harness using actual router/auth/services, controlled source/verifier and an out-of-band private control pipe | Real public target discovery, human UI A7/A9, production internal admission/finalization, fake adapter invocation counts, and public R4 reads end to end. | **Recommended complete isolated 2c proof**, plus a separate production-default negative smoke. |

The recommended harness is concrete:

1. Add a narrow server-constructed dependency on Handler, forwarded by `stageService()`. Production defaults to nil/unavailable; a real S4 adapter is supplied only by trusted server wiring. No `MULTICA_*_SIMULATION` switch, request flag or public verifier-registration route is introduced.
2. In a test-only `cmd/server/*_test.go` fixture, call the existing `NewRouterWithOptions`, install the controlled source/verifier on the returned Handler **before serving**, and run the actual router. Its source catalog may expose only this run's synthetic workspace/target identities and must use real database trust/member fences. A valid GitHub fixture row alone does not activate the adapter—the test-only construction does.
3. After 2d makes policy activation legitimately available, serve production Web and the source-matched Desktop renderer against that loopback API. Use public registration/login, real P2/role/candidate/QA/release/rollback actions and original receipts. The fixture must not override the activation constant or seed human approvals or execution facts. Earlier 2c unit/router work is not this complete browser journey.
4. A private stdin/pipe controller sends a closed set of test commands to invoke the production internal admission and observation methods. The controlled adapter records exact call count, request key, target/build/conditional version; it can withhold a result, return a definite outcome, or answer the original-operation query. It performs no real deployment. This control surface exists only in the test executable, never in the public router or production binary.
5. Start the real collaboration outbox/realtime/background components needed by the scenario, using the existing startup code. A router alone is not a claim that production-main lifecycle or WS propagation was exercised. Record exactly which startup components ran. The same execution-source dependency must reach the actual 2d completion consumers as well as `stageService()`; a test-only stage reader injection must not leave the real completion writer with a different authority source or an unavailable fallback.
6. `cmd/server` has a `TestMain` that opens a DB, creates its baseline fixture and starts a default httptest listener (`integration_test.go:43`). Give this harness a **fresh dedicated router-harness/API database**, not a running API or unit-suite DB. Account for and close both owned listeners. Its cleanup targets only the known integration-test slug/email; scenario histories are retained. A task-specific alternate test entry point, if selected, must explicitly preserve these ownership guarantees rather than silently skipping TestMain.
7. Bind harness/product source, test-only additions, API/CLI/Web/Desktop binaries, configuration, database and all observations. Separately launch the normal production API constructor without an adapter and assert that execution target/admission is unavailable, with zero external calls. A successful synthetic harness is not proof that production has a configured deployment provider.

This gives usable real API/UI positive paths without making production callers able to select simulated success. All adapter outcomes and test-account signatures remain explicitly synthetic. Real S4 E01–E07 stay NOT_RUN until actual accounts, targets, provider versions, reviewers and evidence are supplied.

### 6. Persistence, locks, readiness and tests

Use the existing workflow lock prefix for human decisions and a deliberate borrowed-transaction variant for the internal consumer. Lock current source/target trust and every required original member/credential before the final read. Do not add child→root blocking inversions. Intent admission, revocation exclusion, unresolved-target checks and unique insertion must serialize in the same transaction; use deterministic NOWAIT locks and the existing bounded contention semantics.

The three proposed record tables have no foreign keys or cascading actions. Protect immutable columns/rows. Every identity, original-key, per-approval, source-observation and bounded-history index is a separate concurrent-index migration with a registered retry cleanup hook. A partial unique index can enforce one definitive terminal observation per intent. Validate actual index shape/readiness, not only names or migration ledger entries. Do not reserve migration numbers before 2b is finished.

The highest down migration must acquire deterministic `ACCESS EXCLUSIVE NOWAIT` fences and refuse any retained intent, observation or metric history before an index can be removed. Repeat protection before table drop and stop all admission across a multi-file downgrade. Add the tables to workspace deletion classification without deleting audit history to permit downgrade. Keep `stagePolicyActivationAvailable=false` until 2d covers every real writer.

| Required test | Assertion points |
| --- | --- |
| Public boundary | Human, Agent, runtime and plugin HTTP inputs cannot supply a trusted proof or register a fact; no public fact writer or simulation flag exists. |
| Target binding | Unknown/ambiguous resolver, changed target digest/config/trust, or wrong workspace invokes zero times; registered synthetic target is reachable through R4 and actual A7 UI. |
| Role and dependency | QA cannot release; release/rollback holder and exact QA/release dependencies are required; rollback maps only to verification. |
| Metric reference | Missing/foreign/wrong-deployment/wrong-target/changed/expired/non-anomalous observation cannot authorize rollback. Reason-only return remains available under its existing rule. |
| Source ordering | Older native metric observation and equal-version/different-digest ambiguity cannot replace current healthy evidence or revive an old approval. |
| Final clock and source races | Both orders for membership/trust/role revoke versus dispatch; expiry during slow observers; use the actual source-member fence and final DB clock. |
| Concurrent admission | Same original intent creates one record and one invocation; changed payload conflicts; a new key cannot reuse an approval or evade an unresolved operation. |
| Ambiguous commit | Inject unknown intent commit; caller invokes zero times. Receipt lookup returns history but never a usable dispatch permit. |
| External uncertainty | Timeout/crash/unknown query creates no success/failure fact and no retry; original-operation query later records the exact result. |
| Terminal identity | Duplicate callback/query returns one fact; changed terminal payload or failed→successful retry under the same execution cannot overwrite history. |
| Late result | Original admitted result records after signer expiry without issuing authority; A9 still rejects expired current approval and performs no new execution. |
| Revocation after admission | A8 cannot imply cancellation of an already admitted external operation; original outcome remains recordable. |
| Target drift | Original deployment changed externally before rollback: provider conditional request refuses; a different deployment is never rolled back. Real provider proof remains S4. |
| R4 privacy | Source denied hides target/metric/operation/fact arrays and counts; current source denial also fences client caches and pending late reads. |
| Actual isolated journey | Production Web + matched Desktop, actual router/auth, internal producer path, one controlled invocation, failure/unknown/recovery, human rollback and unchanged original evidence. |
| Migration/race | Exact up/down/readiness, interrupted concurrent-index retry, retained-history refusal before index drop; corresponding normal/race suites on assigned databases. |

The test-only producer must exercise real `Prepare` → definite commit → controlled call → `Record` code. SQL-seeded facts, passing unit parsers or a visible approval label do not replace this proof.

### 7. Good/base/bad cases and implementation handoff

- Good: a human reviews a server-resolved target digest and metric observation; the actual internal consumer admits one exact intent; a controlled adapter is called once; unknown stays unresolved until original-operation lookup; public reads remain permission checked.
- Base: no configured adapter/target means unavailable execution while existing discussion, return, Stop and original receipt paths remain usable. No synthetic fallback appears in production.
- Bad: reuse `admin_operation` or `agent_artifact_execution` with new meanings, parse human JSON into a trusted proof, treat a PR signature or release approval as a deployment, or reconstruct a dispatch permit from a retained intent after restart.

Wrong: `approved → call → write success`, retrying on timeout. Correct: `validate current authority and exact target → commit one intent → definite owner calls once → append authenticated observation`; lookup never repeats the call.

Before coding 2c, the sole owner should freeze: target digest/discovery; metric reference and bounded read projection; operation states versus confirmed facts; rollback's verification-stage mapping; consumed-revocation behavior; the test-only router-harness seam; and explicit conservative late-result behavior. Then implement the internal ledgers and A7/A9/R4 consumers sequentially. No role or manual-child product decision needs to be reopened.

## Related specs and external references

- `.trellis/spec/server/security-boundaries.md`: real principal identity, current member/credential fences, retained security history and rollback locking.
- `.trellis/spec/server/controlled-artifact-execution.md`: durable claim before side effect, original nonce finalization and unknown/no-retry policy.
- `.trellis/spec/server/delivery-completion-enforcement.md`: borrowed transactions, actual consumer versus original issuer, exact source/trust binding, all-writer final guards.
- S2 `design.md` F05/F06, `research/server-increment-2-brief.md`, current `research/wire-contract.md` §§8/10/13; S4 `research/integration-readiness.md` input register I07–I10 and E05–E07.
- No external reference was fetched. The inspected Go module declares Go 1.26.6. Actual release/rollback/metric provider, API version, conditional-write semantics, idempotency retention and original-query guarantees remain unknown and require first-party documentation in S4.

## Caveats / Not Found

- No production release adapter, execution-target catalog or business-metric source was found in this bounded review. `NewRouterWithOptions` and Handler injection exist; the proposed execution dependency and API harness do not yet exist.
- The report proposes names, fields and storage roles; it does not freeze a public contract, allocate migration numbers, select a provider or authorize an external call.
- The current-valid A9 rule can leave a late successful deployment blocked after approval expiry. No renewal or existing-deployment endorsement is implemented or recommended in this increment.
- Actual rollback artifact policy must be made explicit if only the immediately previous artifact is supported. Do not claim arbitrary historical rollback-target selection from the existing fields alone.
- `verification` stage entry does not itself prove external health or payment correctness. S4 still supplies real post-deploy/rollback checks and business sign-off; 2d must not equate stage name, approval or unknown operation with completed evidence.
- Source files were actively changing under the 2b owner. The companion JSON hashes the inspected state and records recommendations as non-executable preparation. No tests, API/DB/account inventory or external validation were performed here.
