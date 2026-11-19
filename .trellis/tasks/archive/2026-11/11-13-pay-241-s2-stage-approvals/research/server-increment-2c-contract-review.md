# S2 increment 2c — independent technical contract review

- Reviewer: `/root/s2_server_check`, 2026-11-14.
- Reviewed proposal: `server-increment-2c-contract-preparation.md` / `.json`, design F05/F06 and wire §§8/10/13.
- Verdict: **READY for sole-owner freeze with the mandatory amendments below.** The coordinator accepted the three narrow repairs identified in this review. The original preparation alone is not the complete contract; carry these rules into the owner's wire freeze. No role, manual-child, provider-selection or product-scope discussion needs reopening.
- Scope: technical preparation only. No product, wire, spec, task-status, database, API, model or Git changes. Only this report and its JSON companion were written.

## Findings (fixed)

The coordinator accepted the technical repairs below. This report specifies their final semantics; it does not edit the proposal, implement them or change the frozen wire.

## Mandatory contract amendments

### 2C-R01 — separate the physical target conflict key from the reviewed target digest

**Proposal locations:** lines 53–59, 78 and 138.

The proposal correctly requires unresolved-target serialization, but the only concrete identity tuple includes local integration ID, adapter/config/trust versions and workspace-scoped resolution. Those are suitable inputs to the **reviewed target digest**, which must change when authority/configuration changes. They are unsuitable as the sole key for blocking another invocation of the **same physical target**.

Counterexample: an unresolved deployment under integration/config generation A remains pending. A reconnect, second local integration binding, another issue, or another workspace resolves the same provider tenant/environment under generation B. If the unresolved lookup or lock uses the new target digest or local binding, the new approval can start another operation against the still-uncertain target.

**Minimum repair:** have the trusted resolver return two explicitly different values:

- the versioned `target_digest` used by A7, admission, facts and current-trust checks;
- an internal stable conflict key for the provider's actual concurrency domain, such as its canonical remote instance/tenant/target identity. It must exclude root/issue IDs, friendly environment names, local integration IDs and mutable trust/config generations.

Take one transaction-scoped lock for that conflict key before the unresolved-intent query and insertion. Reuse the existing advisory namespace/try-lock mechanism; no target-management framework or extra mutable registry is required. The unresolved query must cover all local roots/bindings resolving to that conflict domain, while outward errors must not reveal another workspace's operation identity. If a registered adapter cannot provide a stable domain for aliases, that mapping is unsupported rather than presumed independent. Provider-side conditional execution still handles changes outside Multica.

**Required counterexamples:** two roots sharing a target; reconnect/config change with a pending intent; two local workspace bindings to the same target; and unrelated targets with the same display name. Exactly one unresolved admission is allowed for the shared domain.

### 2C-R02 — make the post-commit dispatch permit actually single-use and time-bounded

**Proposal locations:** lines 45, 53 and 63–66.

An unexported, nonserializable value prevents a normal API caller from manufacturing a permit, but it does not define what consumes that permit. The proposal also stores a dispatch expiry without specifying the action when the definite owner reaches invocation after that deadline. A legitimate internal caller can otherwise invoke twice through the same handle or retain an unused handle until its authorization window has passed.

This is a missing execution contract, not an assertion that a trusted server constructor is hostile.

**Selected repair:** use a synchronous service-owned one-shot invocation handoff. The service admits and definitely commits the intent, then owns entry into the registered callback. No cached or serializable permit escapes to a scheduler, caller or worker. The entry guard is consumed atomically once. No receipt, restart, changed request key or second caller reconstructs an entry opportunity.

Define intent commit as the admission/approval-consumption point and check the bounded dispatch deadline immediately before callback entry. A definite invocation receives the original external request key and exact frozen target/build/conditional version. Remote calls remain outside database transactions. The deadline bounds **entry**, not the eventual completion time of an already-entered operation.

Freeze these outcomes explicitly:

| Event | Retained state and allowed action |
| --- | --- |
| Intent commit failed or is ambiguous | Zero callback entries from this invocation; no entry opportunity returned. Any retained intent is unresolved until original-key lookup/reconciliation. |
| The live sole guard detects expiry or cancellation before callback entry | Retire entry permanently; zero external calls. The service may append an internal-only `not_started` observation bound to that original intent/executor and proof that its sole entry never occurred. Native operation ID is null; this is operation history, not an execution fact. Normal HTTP/runtime input cannot assert this origin. |
| Recording that pre-entry `not_started` fails or has ambiguous commit | Preserve unresolved/unknown history and reconcile by the original identity; do not recover entry, call the adapter or infer successful persistence. |
| Process dies after intent commit without a conclusive retained observation | Pending/unknown; a missing in-memory guard is not proof of `not_started`. No retry or permit reconstruction. |
| Cancellation, deadline, panic or transport error after callback entry | Unknown unless authenticated provider evidence proves a definite terminal result or a definite zero-execution refusal. The local error alone is insufficient. |
| Authenticated original-operation query later proves the outcome | Append/recover the exact historical result; never invoke again. A provider “not found” answer is conclusive only if its registered S4 query/idempotency contract actually proves zero execution. |

The pre-entry `not_started` classification needs no extra table or public toggle: it is a narrow internal origin in the existing observation ledger, writable only by the sole service-owned guard before entry. Persist it with a bounded finalization context even when the caller cancelled. If its persistence cannot be established, retain uncertainty. A definite `not_started` may release the unresolved-target block for a **fresh explicit approval** after all normal checks; it never restores the consumed approval or dispatch opportunity.

**Required counterexamples:** calling the same handle twice, concurrent use of one handle, expiry between intent commit and invocation, ambiguous commit, and original-key recovery after restart. The adapter call count is at most one; all failed/ambiguous admission paths call it zero times.

### 2C-R03 — equal-version contradictory metrics must also invalidate an existing anomaly

**Proposal locations:** line 114 and the source-ordering test row.

The stated rules cover older observations and a newer healthy/unknown observation superseding an anomaly. The equal-version test only says ambiguity cannot replace healthy evidence or revive an old approval. It does not specify the converse: an already-current anomalous observation followed by a different authenticated payload for the same native version.

Keeping the first anomaly current while merely rejecting the contradictory payload would still allow rollback based on evidence the system now knows is ambiguous.

**Minimum repair:** define a metric stream key that includes the exact deployment/target and registered query/rule identity, then make equal native version plus different canonical digest an explicit ambiguous/unavailable stream state. Retain the conflicting observations; do not choose by arrival time. Both new rollback approval and consumption of a previously signed rollback approval must fail until a strictly newer, authenticated, well-ordered observation resolves the stream. Apply the registered freshness deadline at the final database clock; a transport timestamp must not renew it.

**Required counterexamples:** current anomaly followed by equal-version healthy/unknown/different anomaly, and the reverse order. Neither order can consume the ambiguous version. Older delayed observations cannot clear the ambiguity or restore authority. Actual native ordering and freshness rules remain S4 provider contracts, not invented PAY thresholds.

## Choices ready to carry into the freeze

These are clarifications of the approved approach, not additional product decisions:

1. **Trusted source boundary:** accept the narrow server-constructed resolver/verifier and three ledgers. Its nil default means unavailable. Keep code, deployment and metric trust identities separate. Authenticate captured observations against locked current trust; do not reinterpret a GitHub PR signature as deployment proof or substitute the approver's JWT for the internal consumer. No public simulation flag or generic connector framework is needed.
2. **Exact build and rollback target:** release must consume a currently valid, server-observed build bound to its candidate and selected QA report. A source-only report's null artifact is not verified build evidence. Freeze rollback v1 to the selected successful deployment fact's **immediately previous observed artifact**; an unknown previous artifact is unavailable. `rollback_target_digest` must equal that retained artifact, and the provider's conditional target/version must still identify the original deployment. Do not add arbitrary historical artifact selection.
3. **A9 fact selection:** the existing A9 body has `decision_ids`, not a fact ID. With one intent per approval and at most one definitive terminal fact, derive the exact successful deployment fact from the selected release approval. State that lookup explicitly; never choose the latest fact by candidate, environment label or target. A new approval cannot endorse an older deployment fact. Late authenticated facts remain history while current-valid approval/evidence checks can still block A9.
4. **A8 consumption:** the chosen proposal intentionally extends current wire §8.4, which presently says “consumed by an irreversible fact.” Amend it explicitly to refuse revocation after the durable external-execution intent consumes the approval's one dispatch opportunity, including pending/unknown history and a later `not_started` result. Explain that this is irrevocable admission, not proof of deployment and not external cancellation. A definite failure or proven `not_started` may permit a new explicit approval/intent after target-state checks; it never restores or makes the consumed approval revocable. Keep the original reason/receipt history.
5. **Terminal identity:** repeated delivery/query observations for the same external execution and equivalent canonical terminal result return its existing fact. A changed payload under an existing identity conflicts; a failed execution followed by a genuine successful retry requires a different approval, intent and external execution identity. No timeout becomes a terminal result.
6. **Source-denied R4:** preserve one anonymous withheld group. The new targets, metric observations and operation arrays are empty with the existing null baseline/unavailable observations, `blocked_evidence:1` and identifier-free blockers. Missing adapters or unsupported targets do not become synthetic successes.

## Actual router and final-consumer feasibility

The proposed test-only harness is feasible with the existing code. `NewRouterWithOptions` (`server/cmd/server/router.go:403`) returns the actual Handler, and `handler.stageService()` (`server/internal/handler/collaboration_stage.go:161`) constructs the per-request service. Installing a trusted dependency before serving can therefore reach public R4 and A7/A9 without adding a public fixture route. The private controller can exercise the real internal Prepare → definite commit → one-shot call → Record path. Human approvals must come from actual public authenticated actions, not seeded approval rows.

There is one concrete wiring point to include in the 2d handoff: `PrepareDeliveryMutationInTx` is a **free function** receiving `DeliveryMutationIntent`, not a method on the stage service. Its current intent has no execution-source dependency. Its production callers include ordinary handler mutation, `IssueService` status writes, collaboration lifecycle and R38 administration. Merely setting a field on the Handler or stage reader will not inject the same source into these consumers.

A minimal explicit path is an internal typed dependency in the common construction/intent path (for example, an internal execution-source field on `DeliveryMutationIntent`, fed from the same service configuration and retained by the private stage guard). Forward it through every real writer and the submit guard. This dependency is server-constructed, never decoded from request JSON or selected through a public flag. Record that propagation in 2d's existing all-writer table; preserve the final before/after transaction and database-clock checks. No global mutable resolver or hidden context/header override is needed.

`cmd/server` TestMain (`integration_test.go:43`) creates its own database fixture, realtime hub and default httptest listener before the selected test runs. The long-lived harness needs a fresh, recorded router-harness/API database and must account for both listeners. Its scenario identities must avoid the baseline fixture's cleanup slug/email. Start and stop the actual outbox/realtime/background pieces needed for the exercised scenario; a router alone proves neither production-main lifecycle nor WebSocket delivery. Keep private HOME/temp paths, actual source/build/configuration identity and process cleanup evidence.

The complete production-Web/matching-Desktop positive journey must wait for legitimate 2d policy activation. Separately verify the normal production constructor's unavailable/no-invocation behavior with no adapter. This test-only executable is useful product-path evidence, while provider idempotency, conditional writes, metrics and real deployment/rollback remain **S4 NOT_RUN**.

## Verification and source identity

- The preparation is a 36-file **research capture**, not an executable frozen product candidate. At the first identity check, 35 prepared hashes matched; at the final capture, 34 matched and both `collaboration_stage_review.go` and `collaboration_stage_review_view.go` had moved under the active 2b owner. The JSON records prepared and separately observed hashes. No current 2b change is mislabeled as part of the prepared capture.
- Checked the real router return/injection seam, TestMain initialization/cleanup, per-request stage service, internal webhook-proof shape, durable artifact claim-before-call sequence, and the free completion function/current caller topology.
- No implementation exists for the proposed 2c interfaces in this review. Code lint/typecheck/tests were therefore not run for this document-only contract review. Markdown/JSON structure, local references and report hashes were checked.
- No database, runtime, network or account operation was performed. No provider documentation or external environment was validated.

**Freeze handoff: READY.** The sole server owner should incorporate accepted amendments 2C-R01–R03, the explicit dispatch-outcome table, immediate-previous-artifact rollback, A8 consumption wording, exact A9 fact derivation and all-writer source injection into the 2c freeze. This approves the technical handoff only. Implementation, independent tests, provider semantics and S2/S4 acceptance remain separate; no additional role/product approval or broader integration framework is required.
