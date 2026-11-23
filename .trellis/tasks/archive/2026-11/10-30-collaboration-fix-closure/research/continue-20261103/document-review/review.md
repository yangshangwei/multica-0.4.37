# Bounded closure document/spec consistency review

Verdict: **changes-required for four mechanical document/metadata corrections**. No current runtime/source blocker was found; this is not final task acceptance and does not interrupt the root-owned complete check.

## Findings (not fixed)

1. **Field aliases, spec lines85–88.** The finished run uses `owned_bindings_sha256`; clearance and independent review use `bindings_sha256`. The development identity is `run.dev_environment_source_id`, compared with `binding.source.checkout_source_id`, while the review records it under `source_identities.development_HEAD_overlay`. Clarify these mappings in the signatures; do not change the actual schema.
2. **Explicit actual audit checklist, plan lines19/28.** Line19 refers to16 private auditor regressions, not an actual final audit. Label it accordingly and add an unchecked post-Make actual auditor step. Require raw exit0, actual reviewed final audit pass and complete named inventory/provenance together. The spec and task metadata already keep this gate open.
3. **Current generic counters/reviews, task.json line71.** `current_fullcheck_TS_pass=13082` is stale beside `v11_TS_pass=13109`. Generic preflight/latest-review/fake-count fields still describe v9. Update current values/pointers and retain old facts under explicit historical names.
4. **Current continuation pointers, task.json lines64/72.** Generic original registration/password preparation point to20261102. Current20261103 original registration and password preparation/readiness exist for this owner/candidate. Record their current pointers; keep old records and launch holds unchanged.

No plan/task/spec/manifest file was edited under this read-only scope. Root owns the mechanical corrections.

## Verified consistent

- Task remains `in_progress` with no completion date. Actual candidate304 complete-check record is **running**, process14257, with no final exit code or pass claim.
- All **68** scoped Markdown links, curated JSONL references and task evidence paths resolve. Existence does not mean current evidence; stale semantic pointers are listed above.
- Failed/unaccepted v9/v10 records remain historical. Canonical product, development overlay and tracked snapshot IDs keep separate meanings. Root clearance binds the corrected launcher and required final auditor/review hashes.
- Password7 remains an independent actual7pass/0skip/0retry gate. Original preregistration retains **13 consumed /14 remaining**, new calls0, and stays held for current wholecheck/password/exclusive-window. It does not consume RK1 or knowledge budgets.

Machine findings and snapshots: [review.json](review.json). Reference checks: [reference-checks.json](reference-checks.json). No tests, services, database/account/Provider operations, candidate changes or commits were performed.
