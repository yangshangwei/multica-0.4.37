# P1 foundation review

2026-11-14. Independent native `trellis-check` review of the implemented P1
storage/protocol/Core readers and the separately prepared P3 typed LLM helper.
Approved product revision: `2026-11-14-v1`.

## Verdict and boundary

The reviewed foundation can proceed to the remaining public projections and
P2/P3 implementation. One concrete downgrade defect was found, reproduced and
fixed with a fresh-database regression. No other blocking defect was found in
this bounded review.

This is not completion of P1 as a whole or product acceptance. Public HTTP
negotiation/projections, accepted-delivery domain reads, automatic validity,
durable worker orchestration, rule settlement, daemon v2 activation and shared
Views are still implementation work. No real model, account or deployment was
used by this reviewer.

Changes were compared with the task-start source backup, not with the branch's
large unrelated Git diff:

`/var/folders/3n/gbt3p39s5pdc4l55js62gxnm0000gn/T/knowledge-automation-baseline-pe7zjzso/source`

## Finding fixed: downgrade omitted human v2 and retained runtime history

`server/migrations/844_project_knowledge_automation.up.sql` initially refused
automation rows/origins, project operations and ordinary task storage version2.
It did not refuse either of these states when all those predicates were empty:

- Human candidates with a `delivery` source or a delivery source in evidence,
  and human knowledge operation receipts with protocol2.
- Protocol2 claims/preparations/authorizations retained under the stable
  `collaboration_run_event` knowledge slots.

Thus an operator could run a new down migration while retaining records that
the older knowledge readers could not represent or recover. Missing/unknown
runtime versions also passed the original guard. This contradicted the approved
retained-history and downgrade contract.

The guard now also locks the runtime-event table and refuses nonlegacy source
kinds, non-v1/malformed knowledge receipts, and unsupported knowledge runtime
envelopes. Runtime checks are scoped to knowledge kinds/producer keys, inspect
the correct nested preparation version and manifest, reject new manifest
validity fields under v1, and preserve canonical v1 slot shapes. Unrelated
business payloads declaring protocol2 do not block downgrade. Runtime status is
derived from the preparation slot and has no separate retained status record;
the public `knowledge_changed` event stores candidate/action/version/revision,
not a protocol receipt.

Added `server/internal/migrations/collaboration_knowledge_automation_rollback_test.go`
with retained-source/receipt/runtime refusal cases and an actual legacy-only
downgrade that compares retained event/operation bytes before and after. The
existing automation migration fixture now clones the runtime-event table; its
previous assertions remain unchanged.

The first correction run caught two local SQL mistakes: an evidence alias
resolved to the outer evidence column, and identity JSON extraction required
parentheses before subtraction. Both were fixed without weakening assertions.
The failed evidence remains alongside the passing replay.

## Other checks

- New schema relationships introduce no FK/cascade or inline unique/primary
  indexes. All15 new index migrations contain exactly one concurrent index
  statement. Candidate expiry CHECK5 and immutable original proposal/approval
  facts remain; latest human decisions can follow automatic approval.
- Normalized batch/attempt records retain bounded slots, immutable successful
  or uncertain model attempts, exact source/target/policy inputs and append-only
  evaluations. Source-read basis is separate from human proposal/approval IDs.
- Ordinary stored versions match their database envelope. Outer2 can retain
  original nested1 claims, but each claim's preparation, authorization and
  observation must keep its version. Same-candidate changed proofs are refused
  by manifest union; comparison uses values, not pointer identity.
- Fixed collaboration claim/prepare slots remain discoverable. Existing
  ordinary prepare/authorize/observe writes persist the actual delivery version;
  version mismatch and v1-after-v2 history are refused before metadata/pointer
  writes or a fresh-session fallback.
- Core keeps separate strict v1/v2 projections and preserves original pending
  operation versions. V1 receives no synthetic system fields. Model reasons
  cannot become public automatic decision prose.
- The typed LLM helper preserves the shared request/parameter retry path,
  actual-zero versus missing usage and returned model facts on rejected output.
  Its configuration fingerprint exposes only SHA256 and documents the custom
  HTTP transport limitation. No new consumer was enabled in this helper slice.
- Root's policy cleanup calls stay inside existing project/workspace deletion
  transactions; retained job/evaluation data is not deleted. The actual handler
  replay confirms these behaviors while preserving refusal with live links.

## Verification

Reviewer-executed checks passed:

- Scoped ESLint on the five changed Core knowledge files.
- `pnpm --filter @multica/core typecheck`.
- `go -C server vet ./pkg/protocol ./pkg/llm` and `go -C server vet ./internal/migrations`.
- Scoped `git diff --check`, migration index/FK/cascade inspection.

Fresh owned database runs executed by this reviewer:

| Evidence | Result |
| --- | --- |
| `p1-rollback-red-4968f933.json` | Original defect:13 FAIL/1 PASS, source stable, zero owned connections |
| `p1-foundation-rollback-green-ac839724.json` | First correction:19 PASS/3 FAIL, source stable, zero owned connections |
| `p1-foundation-rollback-green2-966db641.json` | **22/22 PASS**, zero skips, source stable, zero owned connections |

The final run contains15 negative retained-shape cases, their parent, the
legacy-only actual downgrade, and five existing automation schema/query tests.
Every invocation used the recorded private launcher, a new database with current
migrations, Git-external HOME/TMP and no inherited real model credentials.

Also inspected actual completed peer evidence: Core87 tests, LLM56 named events,
stored-version/service25 tests, storage9 tests and corrected handler3 tests all
passed. The three database reports were source-stable with zero remaining owned
connections. Earlier failed or source-drifting peer runs remain preserved and
are not substituted for these passing identities.

## Findings not fixed / remaining integration

Concurrent collaboration-stage changes and the previously reported workspace
deletion manifest drift for `report_snapshot`/`report_snapshot_control` belong to
other work. They were preserved, excluded from task attribution and not changed
by this reviewer. The full repository check still needs root's final integration
review after all task code is complete; this scoped pass does not override that
unrelated failed inventory or any historical product/model acceptance evidence.
