# S4 readiness follow-up after S2 contract freeze

Recorded: 2026-11-14. This supplements `integration-readiness.md`; the original research and its source hashes remain unchanged. No real external target, account, credential, provider API, payment, deployment or metric source was accessed in this follow-up.

**E01–E07 remain NOT_RUN.** S4 still lacks the supplied PAY payment/fulfillment sandbox, real repository/CI and build identities, nonproduction release/rollback target, registered metric rules and designated reviewers. Multica Cloud Stripe billing remains outside the PAY business boundary. S4 is not ready to archive.

## Changes to the internal handoff

| Earlier readiness item | Current evidence | Remaining boundary |
| --- | --- | --- |
| Older GitHub PR delivery can replace a newer head or derive stale links | S2 2a/follow-up source and independent checks now cover stale observations, equal-time conflicting heads, complete check sets and derived-link fencing. | Local synthetic service/handler evidence; E01 still needs real provider/version and replay evidence. |
| Rollback metric reference is missing from the public contract | Frozen S2 wire §8.10 adds `rollback_observation` with exact observation ID/digest, and requires verification-stage rollback of the successful deployment's immediately previous artifact. | Frozen interface only. Actual metric provider, native ordering, freshness, query/rule identity and samples remain absent. |
| Durable intent, unknown outcome and invocation ownership are unresolved | Frozen internal control contract specifies a stable physical-target conflict key, one intent per approval, synchronous service-owned entry only after definite commit, and historical lookup without another invocation. | At this follow-up the server has only 2c DTO/serialization drafts; ledger and consumer implementation is pending. No execution pass is inferred. |
| Equal native metric versions could retain a previously approved anomaly despite contradictory content | Accepted review amendment 2C-R03 requires durable ambiguity for either arrival order; only a strictly newer authenticated observation can resolve it. | Must be implemented and independently tested in S2, then verified against the selected real S4 source contract. |
| Test harness could stand in for a configured production adapter | Frozen contract permits only a test executable using actual router/auth/background services and a private control pipe; normal production construction has no adapter. | Controlled results and test-account signatures must stay labelled synthetic. No public fixture route or production simulation switch is authorized. |

The current S2 priority is S2B-C01: T4/optional QA report references must use the original frozen artifact's current read authority. Its first reproduction has been obtained but the fix has not yet passed independent review. It does not change S4's external execution status.

## Carry-forward requirements

- Release consumes an actually observed build matching the selected QA report; a source-only report with a null artifact cannot establish a build.
- Actual external execution and the human approval remain separate facts. Pending, unknown and not-started records never establish successful deployment or rollback.
- Late authenticated facts may remain in history, while current approval, source, role, lifecycle and expiry requirements still block A9/final completion. No renewal, endorsement, automatic retry or automatic advance is added.
- The same internal source dependency must reach every S2 2d submit/completion/administrative/project-exit transaction. Injecting it only into the stage reader is insufficient.
- The existing I01–I10 intake register remains the source of missing external inputs. I08/I10 now have a frozen internal shape, but no real provider configuration or final executable source has been supplied.

Companion JSON records the exact documents used for this update. This is a documentation reconciliation, not a new environment inventory or an external acceptance run.
