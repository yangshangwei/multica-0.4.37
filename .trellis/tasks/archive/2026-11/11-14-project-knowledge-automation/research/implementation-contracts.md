# Implementation contract refinements

Approved product/design revision: 2026-11-14-v1. These are storage and wire representations of that scope, not new product choices.

## Strict versions and exact knowledge

Legacy knowledge constants and serialization stay at1. Explicit latest-version constants select2 in updated writers after readers are complete. Public requests keep human authority; no public origin/system override.

Exact delivery source fields are `kind=delivery`, `issue_id`, `submission_id`, `delivery_event_seq`, `accepted_event_seq`, and `sha256`. Hash uses the canonical full result/basis structure. Event numbers are exact positive canonical int64 decimal strings.

A v2 runtime manifest entry adds optional `validity={revision, accepted_event_seq, submission_id, validated_close_epoch}` to its existing candidate/version/content hash/source issue/target identity. v1 cannot carry it. Comparison uses field values, not pointer addresses. Same candidate with changed proof cannot be silently replaced in a retained manifest.

Public v2 candidate metadata includes `proposal_origin`, `decision_origin`, truthful nullable human IDs, `automation={job_id,policy_revision,rule_version}`, and `usage_status`. A later human withdrawal of a system-approved version remains a human latest decision while preserving the immutable original approval.

## Durable batches and attempts

The four conceptual domain records in design section4 remain policy/job/evaluation/validity. The job's bounded batches and phase attempts use normalized `collaboration_knowledge_batch` and `collaboration_knowledge_attempt` child records so immutable successful output, uncertain calls and independently committed batch receipts are retained without replacing a nested mutable JSON history.

A job has at most6 revalidation batches (up to3 exact predecessors each) followed by at most1 create batch. Enumerate all approved automated predecessors before batching, including expired records which must remain paused/pending rather than silently omitted. New independent candidates remain limited to0–3 across the accepted event.

Policy operation recovery reuses the retained collaboration operation namespace with a project scope; it must recover older original requests after subsequent settings changes. The accepted human UUID is only the source-reading basis, never a fabricated proposal/approval actor.

## Ordinary retained storage

Outer ordinary `knowledge_context_version=2` may retain original nested v1 claims alongside new v2 claims; each nested preparation, authorization and observation must match its own claim version. Outer v1 accepts only v1. Upgrading the enclosing metadata cannot rewrite old manifest bytes/hashes or original receipts.

Collaboration storage slots stay `knowledge:claim:v1` and `knowledge:prepare:v1`; payloads select their explicit versions. Old consumers must see retained knowledge and reject unsupported semantics rather than lose the slot and take a base path.

## Verification identity

The private package launcher supports focused serial iterations. Final acceptance additionally requires stable source hashes, exact named test inventory and owned environment identity; a successful process exit alone does not prove those conditions. Other sessions may edit this shared checkout, so drift must be retained and resolved with a new attributable verification run.

## HTTP integration names (P2/P4)

Project policy: GET/PUT `/api/projects/{id}/knowledge-policy`; PUT carries `request_id`, `expected_revision`, `is_enabled`, `max_candidates`. Current human project lead or workspace owner/admin changes it. Default revision0 resolves to system-enabled, max3, `project-knowledge-v1`; explicit overrides are versioned. Original-operation recovery is GET `/api/projects/{id}/knowledge-policy/operations/{requestId}` and must not replay a new PUT.

Job status: GET `/api/collaboration/issues/{id}/knowledge-jobs`; explicit retry POST `/api/collaboration/issues/{id}/knowledge-jobs/{jobId}/retry` with original request UUID and expected job revision. Keep these content-free public projections source-authorized, and require target confirmation authority for retry. They do not expose raw model reasons or input text. Raw request-source closures stay private. Policy/job JSON shapes will be published by the owning service implementer before UI activation.
