# Branch coverage design

Preserve the baseline suite and add focused specs rather than splitting existing scenarios to inflate counts. Each case checks a distinct contract and persists or reloads server state where applicable.

Independent lanes: password browser validation/recovery; triage stale revisions/idempotency/invalid actions; issue mutation rollback/retry and workspace authorization. Root owns environment setup, the device-auth gap, execution, inventory reconciliation and final report. Tests use local production Web/API instances already isolated by the prior task, after checking process/build identity. Device auth is executed separately from legacy login.

Fault injection is restricted to the target browser request and removed before retry. Assertions include database/API state or mutation counts, never just a toast. HTTP-only cases are labeled API integration. Pure exhaustive helper matrices remain at the canonical unit layer.

No edits to production code without a reproducible defect and narrow regression. Existing uncommitted source repairs remain intact. No commits or shared process termination during parallel test-authoring.
