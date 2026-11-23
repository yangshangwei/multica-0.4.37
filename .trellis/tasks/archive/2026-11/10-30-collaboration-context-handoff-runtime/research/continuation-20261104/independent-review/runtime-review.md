# Runtime fake acceptance review

The current normal execution passes independent evidence review. The race
execution fails because an actual measured run has no recording seal. The paid
runtime gate must remain closed.

## Findings fixed by the executor

The first private runner searched for flat public JSON files, but the existing
framework writes `proof/<case>/<trial>/public.json`. Its original result retained
exit 0 / 62 passes / `accepted=false`. A separate reconciliation counted the
actual files without rerunning tests or rewriting that result. The separately
versioned race runner uses the correct path.

Independent normal review checked 910 facts, including all 19 fixture cases,
22 public trial records, 32 FakeProof launches, exact expected negative verdicts,
raw trace/Provider-input bindings, and current source/tool/database identities.
All checks passed. H6 remains `manual_review_required`; this is a fake proof.

## Finding not fixed

Race `wrong_continues/RK1-H3b-t2` reports
`incomplete/measured:handoff_or_recording_event_missing`. The Provider exited 0,
the business outcome succeeded, and the proof settled, but the recording seal
is absent. Its checkpoint requests include repeated 503 responses taking about
one second; a later checkpoint succeeded before another timed-out checkpoint,
followed by successful outcome and proof writes. No seal request appears in the
retained trace. The exact performance or deadline cause requires diagnosis;
these observations alone do not prove database load or a particular code fix.

Actual race result: 62 named runs, 60 passes, 2 failures (the affected subtest and
its parent), zero skips, process exit 1. Independent review passes 901 checks
and fails six acceptance checks consequent on this incomplete trial. The
source, seventeen framework files, Go, guard and allocated database identities
are stable. All 19 cases and 22 public records remain available. No race-detector
diagnostic or real Provider use was observed. No retry was performed.

## Verification scope

- Reports: `normal-review.json` and `race-review.json`.
- Read-only verification script: `verify_fake.py`; it reuses the existing
  reviewed raw-evidence checks and records their hash.
- No product changes, test execution, services or database queries by this
  reviewer. Lint/typecheck were not rerun for this evidence-only review.
- Password7 has a separate static review and still needs actual-result review.
