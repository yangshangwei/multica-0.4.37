# Client increment 3c: release, rollback and execution evidence

Implemented on 2026-11-14 by `/root/s2_client_resume`. Focused unit, type and lint checks pass. Independent review and joint production Web/Desktop acceptance remain pending. This increment does not enable the project stage policy or close S2.

## Behavior delivered

- R4 parses the frozen targets, execution operations and rollback observations, each bounded to50. Older omission stays unavailable; a partial group, malformed identifiers, oversized UTF-8 values or contradictory overlapping operation/fact projections refuses the response. Native operation IDs remain bounded strings, including non-UUID provider identifiers. The source-denied variant requires every new group to be empty.
- Release approval selects the exact server-returned target digest and environment. The selected current QA approval determines the T4 report read; its full report must match the candidate and the same non-null artifact. The additional `evidenceReport` value is internal form/action state, never an HTTP field.
- Rollback approval is available only in verification. It selects the original successful deployment, its immediately previous observed artifact and exact current, anomalous, unexpired metric observation. Unknown, healthy, ambiguous, superseded or expired observations cannot authorize it. A later metric change disables the open form while preserving its unsent reason.
- Every recorded execution intent consumes its approval for revocation purposes, including pending, unknown, failed and not_started. When the operation page contains50 rows, absence alone does not establish that an older approval is unconsumed. Verification advancement requires the selected release approval's unique matching successful operation and fact, including current candidate build and target identity.
- Shared Web/Desktop UI separates execution requests, execution facts and metric evidence. It renders bounded native identifiers and summaries as text. There is no public execution, metric-producer or simulation control.
- Existing code-review/QA five-field evidence serialization and original-key decoding remain byte-identical in canonical regressions. A lost release write followed by GET404 preserves its exact target, body and key and does not POST again. Reason-only returns remain available under their existing authority rules.
- Release's new T4 read uses the existing source-generation protection. The named Views regression keeps R1 children readable, returns T4's artifact-level404, and verifies cached report and target controls disappear without submitting. C1's protected mention-target authority and the separate global command gate were unchanged.

## Source and ownership

The JSON companion binds16 changed Core/Views/locale files with pre-change and final hashes. The exact incremental source overlay is:

`/Users/artisan/.multica/acceptance/pay241-20261114-resume/s2-client3c/source`

Use `client-increment-3b2-results.json` and `client-increment-3b2-check.json` as the declared reconstruction prerequisites; do not silently substitute live dependency bytes for their frozen source. The pre-change manifest preserves47 scoped files, and the report separately freezes the wire and internal2c control contracts. No server, E2E, platform route, database, dependency or Git write was made by this lane.

Changed groups: `stage-schema.ts`; shared execution fact/metric association in `stage-execution.ts`; code-action/preparation guards; code decision form and separate execution history; labels/locales; canonical Core and shared Views regression fixtures/tests. The existing report Query and source-cache implementation was reused unchanged.

## Verification

| Check | Final result | Evidence ID |
| --- | --- | --- |
| Core stage regressions | 19 suites,400 passed,0 failed/skip | `core-regression-2` |
| Shared Views stage regressions | 4 suites,54 passed,0 failed/skip | `views-final-1` |
| Core and Views typecheck | Both passed | `types-final-1` |
| Changed Core files lint | Passed,0 findings | `core-lint-1` |
| Changed Views files lint | Passed,0 findings | `views-lint-1` |
| Scoped whitespace and30 new bilingual locale-key parity | Passed | `static-final.json` |

All14 raw command logs and direct exit sidecars remain under the same private evidence root. The initial Core red has12 failures/16 passes; initial Views red has4 failures/12 passes. Intermediate failures also remain: a duplicate existing text helper declaration, an old release fixture lacking the newly required target digest, and a recovery test that replaced the API instance after capturing its scope. The latter was corrected only in the fixture; the product account/server scope fence remains intact. Repeated runs are not summed as acceptance cases.

## Remaining work

Independent Trellis review is required before treating this client increment as accepted. Server2c/2d, M1/activity consumption, actual API/CLI/Git journey, production Web and matching Desktop, and complete A01–A12 are separate work. This lane used synthetic unit fixtures and did not execute a real provider, deployment, rollback, payment, model account or browser. Parent acceptance counts and all external NOT_RUN items remain unchanged.
