# Human information blocking and recovery

## 1. Scope / trigger

Use the existing collaboration handoff/workflow operation when requesting
information, returning a response or repairing an unavailable request chain.
This contract covers human structured blocks and retained history. Runtime v1
does not synthesize the required metadata; real external systems remain separate.

## 2. Signatures

- `informationMetadata(note, condition, estimate, managed)` and
  `loadInformationState(...)` in `collaboration_information_state.go` own
  metadata validation and current actionability.
- `CollaborationInformationRepairSelection` carries the observed context and
  exact repair basis; `collaboration_information_repair.go` executes within
  the existing workflow transaction and original-key operation.
- `CollaborationInformationWorkflow` exposes `context_version`, current request,
  record state and separate request/return/repair actions.
- Human history is implemented by `collaboration_information_history.go`,
  `handler/collaboration_information.go` and `queries/collaboration_information.sql`.
- Migrations 866/867 add nullable `issue_handoff.unblock_condition`,
  `estimated_resolution_at` and `superseded_by_repair_handoff_id`, with readiness
  and downgrade retention guards. No foreign key, cascade or index is added.

## 3. Contracts

A new managed request needs reason, recipient, unblock condition and estimated
resolution instant. Validate a timezone-bearing RFC3339 timestamp with zero to
six fractional digits, a UTC year in 1–9999 and microsecond precision. Store UTC;
the final database clock validates new writes after original-operation recovery.
An old estimate passing in time does not rewrite its history or dispatch work.
Omitted legacy metadata keeps its old request digest; partial/invalid metadata
is unavailable rather than invented.

Repair atomically records the actual actor, target/status, exact superseded
request links/count, event and original-key receipt. It does not create another
information request, Resume, dispatch or consume completion approval. Pending
submission requires its exact retained submission/event, available creator and
the supported in-review exit. Normal return uses saved previous status and
records the actual restored status/fallback; it cannot be reconstructed later
from a changed status catalog.

History uses real `(created_at,id)` keyset ordering with an upper bound, current
authority and closed request/return/repair variants. Legacy missing event links
stay explicitly null. Do not invent actor/content/provenance for unavailable
rows. Context fingerprints include current actor/member/account/participant
authority and credential facts; the same actor with a new credential may have
a different context version without a business write.

History cursors use a domain-separated HMAC and the verified cached server JWT
signing key. Public development defaults are refused and key rotation invalidates
old cursors. Reads have a five-second transaction budget, limit 1–50, cursor at
most 2,048 bytes and complete JSON response at most 65,536 bytes including
metadata, escaping and newline. An oversized first item returns unavailable;
never silently skip a row or claim a complete page.

Creation options may observe source/project policy, but those GET observations
are not creation POST input or request identity. Creation resolves effective
project inheritance transactionally. Managed initial blocked status remains
unavailable. Core parses strict closed shapes; shared Views keeps the original
pending intent and uses explicit repair actions and current permissions.

## 4. Validation and error matrix

| Condition | Required result |
| --- | --- |
| Managed request lacks both new fields | `information_metadata_required` |
| Partial metadata, invalid timezone/precision/UTC range | `information_metadata_invalid` |
| Original request recovery after ETA passes | Recover original receipt before applying new-write clock rules |
| Stale repair context or changed current authority | Refuse under existing workflow rules; do not renew the original request |
| Pending result lacks exact supported basis | No repair side effect or automatic completion |
| Legacy event linkage missing | Explicit unavailable/null provenance; retain real row identity |
| Cursor key changes or full item cannot fit budget | Refuse; no fabricated empty/complete page |

## 5. Good / base / bad cases

Good: repair records one real blocked-to-unblocked transition and retains every
superseded original request. Base: a legacy request displays its actual limited
metadata. Bad: repair silently sends a replacement request, uses the current
catalog to rewrite an old return or fills absent authority epochs with defaults.

## 6. Required tests

Service information creation/metadata/repair/history/concurrency/runtime suites
own final-clock, atomic rollback, pending-result, immutable provenance and
permission boundaries. Migration tests cover actual column/readiness and retained
downgrade refusal. Core owns malformed transport and original-operation recovery;
Views tests cover forms/history and explicit conflict/denial rendering.

`e2e/collaboration-information-recovery.spec.ts` verifies both platforms, four
block fields and exact request/return/repair history. Accepted R08 retained 48
history items and 70 operations. Its registration-window helper may repeat only
one identical registration after opt-in plus explicit 429/valid Retry-After;
`password-registration-window.node-test.mjs` owns that finite boundary. Do not
retry uncertain business writes or relax product rate limits.

## 7. Wrong versus correct

Wrong: compare authority-derived context fingerprints across different fixture
credentials, or hide a mismatch by deleting context fields. Correct: use one
original credential for before/after comparison and also compare persisted
business fingerprints; report legitimate credential changes separately.

Wrong: infer full S3 or external acceptance from the block suite alone. Correct:
retain each accepted source and denominator, compose only affected recovery
contracts and keep original failures alongside later complete rounds.
