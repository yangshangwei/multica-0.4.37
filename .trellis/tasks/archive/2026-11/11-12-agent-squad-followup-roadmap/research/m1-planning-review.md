# Research: M1-S1 planning contract review

- Query: Does the first-package plan reuse the current backend correctly and define safe evidence selection, explicit acceptance, completion and recovery without expanding scope?
- Scope: Internal source/spec review of this task's planning documents. No code, tests, database, Provider, task activation or git operations.
- Date: 2026-11-12

## Findings

The plan is suitable for final user review. All findings and wording clarifications below were incorporated and re-read in the working copy. This review does not provide implementation approval or runtime acceptance. No remaining architectural reason was found to replace the existing completion authorization service, create a second completion writer, or broaden the first package.

### Corrections identified and incorporated during review

| ID | Finding | Resolution observed |
| --- | --- | --- |
| P1 | A last-scanned pagination cursor may identify an unauthorized approval, and hidden rows producing empty pages/has_more contradict the no-hidden-ID/count requirement. A signed/base64 cursor does not hide its payload. | `design.md:26` now derives pagination from fully authorized candidates, only exposes already-returned candidate anchors, and returns generic unavailable when a bounded scan cannot establish a complete page. `implement.md:32` and `:96` include privacy/cursor/scan-limit tests. |
| P2 | The original blanket rule that every unknown state seeing issued/not_checked stays unknown contradicted creation recovery: a by-key read can prove the original authorization exists even though it does not prove current consumability. | `design.md:122` now distinguishes authorization_unknown from completion_unknown/revoke_unknown. An exact unexpired issued grant can restore the authorized state only when completion was never dispatched; an in-flight/uncertain completion still cannot be retried based on issued. |
| P3 | Calling the preview a read-only transaction could cause implementation to select PostgreSQL READ ONLY, which cannot perform the retained row locks used by the observer. | `design.md:36` now specifies a transaction with no business writes while retaining FOR UPDATE/SHARE locks, explicitly excluding SQL READ ONLY. |
| P4 | Exact API names must remain current: creation recovery uses query `workspace_id`, not body field `source_workspace_id`; the reusable observer is `EvaluateDeliveryEvidenceInTx`. | Main session independently corrected `design.md:36` and `:92`; checked against `server/internal/handler/delivery_completion_authorization.go:154` and `server/internal/service/delivery_evidence.go:78`. |

### Final wording clarifications incorporated and verified

1. Limit S1-05's changed-evidence refusal/zero-side-effect assertion to **new admission that has not already consumed**. Original consumed-operation recovery must retain the historical result even if the issue was subsequently reopened or source facts changed. This is already intended by S1-08 and design section 3.3; make that exception explicit in the requirement.
2. In the CLI revoke row, say **the revocation request has already been dispatched**, rather than the operation file has been dispatched. Every authorized file has already dispatched creation, which must not prevent a first explicit revoke. Keep stage-specific markers rather than one generic sent Boolean.
3. Pair the core recovery tests: creation_unknown + original-key issued can confirm issuance and permit a first explicit submit; completion_unknown/revoke_unknown + issued remains unknown and sends no write. This verifies both sides of P2 rather than only the refusal branch.

Verified updates: `requirements.md:33` now limits S1-05 to unconsumed new admission and preserves S1-08 historical recovery; `design.md:148` names the revocation-specific dispatched phase; `implement.md:51` tests the opposite issued recovery branches and records three separate dispatched phases. These are wording/test-shape corrections, not additional scope or a request to add retries. No unresolved review finding remains.

### Boundaries verified

- Current authorization create/request recovery/ID recovery/revocation, ordinary `completion` binding, immutable receipt and actual writer transaction are reused. `target_position:null` is explicitly retained in the ordinary authorization while the first-package PUT is status-only.
- Preview-derived binding is neither a grant nor another person's evaluation. Authorization creation still performs a new current observation; private evaluation keys are excluded from new requests and local recovery files.
- Candidate visibility follows `CheckArtifactAuthorityInTx`, not ordinary approval-list membership. Current original participants/principals and evidence authority remain relevant; lookup after lost access is allowed to refuse instead of widening read privileges.
- Commit ambiguity is not retried automatically. GET 404, lock conflict, issued/not_checked or expired cannot prove an original request will never commit. The plan honestly preserves this current backend limitation.
- Historical receipt and current issue state are separate; old consumed facts do not overwrite a reopened issue or trigger a second status write, notification or runtime action.
- Bound completion avoids optimistic done, protects original intent before writes and fences identity changes/unmount. Session cleanup remains the existing privacy boundary rather than persisting cross-user recovery data.
- CLI uses a dedicated explicit command family, unchanged existing status/update semantics, phase-specific durable files, identity binding and machine-readable outcomes. It does not silently generate human acceptance from the existing hint confirmation.
- Drag, batch, create-as-done, mixed/project changes and collaboration/R38 are separately listed. `skip_supported=false`, real Provider restrictions, knowledge injection, Hosted CI, release/deployment and excluded Windows/billing scope remain intact.
- `implement.md` assigns canonical test layers, actual handler/router database ownership, normal/race evidence, Web/Desktop runtime proof, compatibility coverage, source-map updates and final Trellis approval. No product verification is falsely marked executed.

## Files Found

- `prd.md`, `requirements.md`, `design.md`, `implement.md`: first-package proposal, route boundaries, wire/state contracts and ordered acceptance plan.
- `research/m1-backend-current-contract.md`: backend fields, authority, evidence gaps, receipt and uncertainty semantics.
- `research/m1-clients-current-contract.md`: shared UI/CLI gaps, mutation and original-intent patterns.
- `server/internal/service/delivery_completion_types.go`: immutable ordinary/human request and binding variants.
- `server/internal/handler/delivery_completion_authorization.go:154`: original-key `workspace_id` parsing and currently authorized read/revoke path.
- `server/internal/service/delivery_completion_authorization.go:327`: current issuer/consumer read authority; `:395` minimal state/applicability view; `:417` receipt validation.
- `server/internal/service/delivery_evidence.go:78`: borrowed transaction observer; `:87` onward issue/credential/project and PR locks.
- `server/internal/service/artifact_authority.go:244`: retained artifact authority; `:364` narrower human artifact-manager condition.
- `server/internal/handler/delivery_completion_authorization_test.go:360`: current consume/revoke/evidence-lock regression source (not executed).

## Related Specs

- `.trellis/workflow.md`: planning artifacts, manifest curation and activation phase.
- `.trellis/spec/server/delivery-completion-enforcement.md`: existing authorization/receipt, transaction, skip and writer contracts.
- `.trellis/spec/server/delivery-completion-hints.md`: non-authorizing hints and ordinary UI/CLI scope.
- `.trellis/spec/server/database-test-isolation.md`: no accidental TestMain/database execution during review.

## External References

None. This review uses local current contracts and code; no external research or SDK change was needed.

## Caveats / Not Found

- This is a planning review, not test evidence. No unit, integration, race, browser, Desktop, database or Provider tests ran.
- Lack of a durable pending/failure terminal means lookup-only recovery can remain unknown after a crash before actual network dispatch. The plan deliberately does not add an automatic retransmission protocol; future requirements to guarantee unattended resolution need separate design.
- Visible-candidate pagination may return unavailable under bounded scan exhaustion. It must not be softened to a false empty/complete list; implementation should validate the practical limit on representative task-owned data.
- Line anchors reflect the reviewed working copy and may move as the coordinator performs the final wording pass. No proposal here changes the user's authorization or marks the task in_progress.
