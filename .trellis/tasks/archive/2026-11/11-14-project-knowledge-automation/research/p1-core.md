# P1 Core dual-version readers

2026-11-14; native `trellis-implement` child `p0_core_contracts`.

## Changes

- `knowledge-schema.ts`: separate frozen v1 and strict v2 envelopes. V2 adds
  exact accepted-delivery sources, human/automation provenance, nullable system
  actor fields and independent active/paused/inactive usage. V1 output receives
  no synthetic origin fields or upgraded version. Human proposal requests cannot
  select automation. Unknown versions remain explicit protocol failures.
- V2 audit checks preserve original automatic approval after a later human
  withdrawal; unavailable/restricted rows still reject private material. Automatic
  decisions expose no free-text reason. Exact positive int64 delivery sequences
  require acceptance after delivery, matching Go.
- `knowledge-actions.ts` and `knowledge-recovery.ts`: optional client-generated
  `protocolVersion` survives storage/recovery; absence means retained v1. Receipt
  and serialized request versions must match that original identity. Prepare
  helpers accept an explicit selected version but still default to 1. Stored v1
  bytes and original request JSON remain unchanged; no source or receipt is
  persisted.
- Tests in `knowledge-automation-schema.test.ts` and `knowledge-actions.test.ts`
  cover these boundaries. P0 fixture consistency was corrected to root version 1
  and target revision equal to its published revision; original P0 red evidence
  remains untouched.

## Verification

- Initial P1 red: 11 failing intended cases; `p1-core-red.log` preserves output.
  A subsequent focused recovery run, after schema support but before recovery
  changes, showed all four recovery cases failing for their intended behavior
  (version dropped, mismatched receipt accepted, explicit selection ignored,
  mismatched serialized version dispatched).
- `pnpm --filter @multica/core exec vitest run knowledge`: **9 files / 87 tests
  passed**, including existing v1, API, leases, cache/source boundaries and actual
  WSClient provider invalidation. Raw `p1-core-green.log`; machine-readable
  `p1-core-tests.json`; completed 2026-11-14 19:28 Asia/Shanghai.
- `pnpm --filter @multica/core typecheck`: passed.
- Scoped ESLint across the three changed production files and two test files:
  passed.

## Next layer contracts

API request headers/defaults are deliberately unchanged until service v2
projections are available. P4 must pass the selected version on writes and use
the original identity's version on operation GET recovery; no automatic upgrade
of unresolved v1 requests. Existing unversioned recovery identities still mean 1.

Views remain unchanged. They must branch nullable system identities and delivery
sources before consuming v2, and select v2 explicitly for decisions (restricted
rows derive their version from the containing projection). V2 human candidates
carry explicit origins, `automation:null` and their independent usage status;
pending decisions remain null with `decision_origin:human`.

This verifies Core readers/recovery only. Server authorization, runtime validity,
model behavior, database races and Web/Desktop journeys remain later gates.
No real model calls, commits, PRs or deployment were performed.
