# Independent planning review — completion evidence hints

Reviewed against source at `05de761de`. This is a scoped API foundation for explicit human requests on ordinary issues. It is not a user-visible automatic prompt, lifecycle permission, skip implementation, or enforcement coverage. No business decision blocks this scope; the future skip question remains separate.

## Required technical clarifications

1. **Open the endpoint with `deliveryPolicyTx`, not only `artifactTx`.** `delivery_policy.go:65` validates the exact ready policy/evaluation indexes in addition to the artifact schema. `deliveryEvaluationInTx` assumes its caller has already done so (`delivery_evaluation.go:108` and `:128`); directly invoking that helper does not repeat readiness. All branches, including no-key and not-applicable, must refuse unavailable/incomplete schema rather than describe an unchecked configuration as off. Add a missing/invalid readiness regression for the new endpoint. No new readiness implementation is needed.
2. **Bind the hint timestamp to the actual evaluation check.** For a recovered evaluation, use `evaluation.applicability.checked_at` as the top-level `checked_at`. The existing applicability function uses the live observer's DB timestamp to test expiry (`delivery_evaluation.go:239`). Taking a later clock sample solely for the outer response could imply that an earlier matching result was revalidated at that later instant. No-key/not-applicable responses can take their own DB clock sample. Assert exact outer/inner timestamp equality for evaluated hints and unchanged original `observation.observed_at`/`valid_until`.

These are implementation-level refinements to the written plan, not new product or permission decisions. The coordinator owns edits to PRD/design/implementation manifests.

## Confirmed reuse and boundaries

- `loadIssueForUser` resolves aliases and refuses legacy POST access to independent collaboration (`handler.go:1132`, `collaboration_boundary.go:61`). Reuse that public boundary. Do not create a new generic collaboration eligibility rule or call the observer with an invented cross-account principal.
- The endpoint still needs its transactional `artifactUsers` and `artifactCredential` checks; loading an issue is not the current-identity fence. `deliveryEvaluationInTx(create=false)` then preserves the original evaluator namespace, issue binding, complete artifact authorization, original observation, and no-write behavior.
- Historical request-version drift must not bypass authority. The existing recovery code rewrites only its temporary current version fields before invoking the same observer, then compares old versus current applicability (`delivery_evaluation.go:208`). The new endpoint should project this result, not duplicate its precedence rules.
- `issuestatus.Resolve` is the correct target-status boundary (`issuestatus.go:338`). It normalizes keys, rejects archived custom statuses, distinguishes unknown status from storage errors, and preserves the built-in fallback without inserting catalog rows. Do not call `Ensure` from this read-only API and do not use `Effective`, which deliberately hides lookup errors for background behavior.
- The existing custom-status shared catalog lock plus transaction-local lock timeout bounds a concurrent archive/create conflict. Project SHARE NOWAIT and policy-writer UPDATE NOWAIT retain a consistent policy snapshot. None of these locks survive the response or authorize a later status write.
- For non-done, no-project or off branches, not reading the supplied key avoids an unnecessary private-history lookup. Validate input syntax consistently, return no evaluation body, and preserve `enforced=false` and `usable_for_completion=false`.
- Ordinary completion, bulk update, create-as-done, move, comment-to-child, VCS automation and independent collaboration writers remain untouched. The separate writer inventory and future contract must remain in delivery/spec/handoff documentation.

## Acceptance details to retain

Use the existing real packaging/router fixtures and verify archive content/SHA before evaluating. Compare persisted observation JSON, original expiry, policy/evaluation/execution/queue counts and issue state before/after hints. Prove custom done and normalized built-in keys; archived/custom-missing status refusal; policy off and no-project branches; own versus other actor key; same actor cross-issue conflict; current credential/producer/approver loss; old issue revision; policy/PR drift; original hold/unknown; actual expiry; custom catalog and project contention; failed/uncertain commit. No refusal may expose a saved evaluation body.

Inherited PAT last-used writes and global PR SHARE NOWAIT fences can produce legitimate conflicts. Preserve failed attempts and synchronize only actual positive-fixture prerequisites; do not weaken live locks or add automatic request retry.

This planning pass read all six curated specs and actual helper/route implementations. It changed only this review document; no product code, database, historical environment, model, or external service was touched. Build/lint/tests are deferred to the implementation verification stage because there is no product change in this pass.
