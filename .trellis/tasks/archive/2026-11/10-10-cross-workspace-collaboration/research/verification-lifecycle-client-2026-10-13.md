# Collaboration lifecycle client verification

Date: 2026-10-13. Scope: P2 status options, manual nonterminal status changes, and creator lifecycle API contracts in `packages/core`.

## Contract and implementation

The lifecycle service implementer confirmed these DTO rules before client implementation:

- `GET /api/collaboration/issues/{id}/status-options?action=change_status|complete|cancel|reopen` returns `action`, lowercase 64-character SHA-256 `status_options_version`, `default_status`, and `options: [{key, name, category}]`.
- A successful options response has at least one option, and its default names a returned option. Complete only offers `done`; cancel only offers `cancelled`; reopen and change_status only offer `backlog`, `todo`, `in_progress`, `in_review`, or `blocked`. Unavailable source categories are an HTTP error, not an empty successful catalogue.
- Lifecycle POST and status PATCH send the four captured expected version strings, the original `Idempotency-Key`, selected `status`, and captured `status_options_version`; only lifecycle POST adds `action`.
- `status_options_changed` is a rejection with fresh currently authorized `status_options`. Operation recovery can include that optional projection; absent/null options do not synthesize a selection or permission.

Changed files:

- `packages/core/collaboration/schema.ts`: strict options schema and redacted parser; optional nullable options on operation receipts; camelCase status/lifecycle input and action types. Options reject unknown categories/actions, duplicate keys, absent critical fields, mismatched action categories, invalid digests, and a default outside the returned choices. Malformed present recovery options make the parsed receipt unavailable.
- `packages/core/collaboration/index.ts`: public exports for the new schema, parser, and types, preserving the previous workflow exports.
- `packages/core/api/client.ts`: `getCollaborationStatusOptions`, `changeCollaborationStatus`, and `changeCollaborationLifecycle`, sharing the existing credential/endpoint/abort guards. The options GET also checks its returned action matches the requested one. The shared writer now takes its HTTP verb internally; public caller options only supply cancellation. GET methods similarly forward only their supported signal, so unrelated transport properties cannot turn reads into writes.
- `packages/core/collaboration/lifecycle-schema.test.ts` and `packages/core/api/collaboration-lifecycle.test.ts`: canonical parsing and endpoint/protection regressions.

HTTP errors remain normal `ApiError` values with an unknown body; consumers can validate supplied fresh options with the exported redacted parser. The client never selects a default, retries an operation, changes its idempotency key, or replaces the originally submitted revision/status/digest with fresher values.

## Verification

Before implementation, the two new suites failed all 66 tests on missing methods/options schemas and stripped recovery options. Two further transport-option regressions independently reproduced an incorrect POST/PATCH/GET override before it was corrected.

Final command results, checked after process completion:

| Command | Result |
| --- | --- |
| `pnpm --filter @multica/core test collaboration` | 5 files, 161 tests passed; exit 0 |
| `pnpm --filter @multica/core typecheck` | Exit 0 |
| `pnpm --filter @multica/core lint` | Exit 0 |

Tests use synthetic fetch responses, without a running service, database, browser, or AI provider. The commands printed the existing root pnpm configuration-placement warning for `pnpm.onlyBuiltDependencies` / `pnpm.overrides`; no configuration was changed.

The suites cover all three new methods and lifecycle actions, exact route/body/header mapping, large positive version strings, strict status-options parsing, stored recovery options, literal stale key/digest/status preservation after `409 status_options_changed`, malformed JSON and missing critical fields, private-log sentinels, delayed account changes, and cancellation during body parsing. Prior workflow/read/ordinary-revision regressions run in the same collaboration test filter.

## Remaining integration work

This evidence verifies the core client boundary, not full P2 or Web/Desktop acceptance. Root and the lifecycle service implementer own HTTP authorization, database transactions, role/state checks, and status catalogue locks. Query/hooks, persisted recovery, cache invalidation, UI selection/review, creation enablement, and AI execution remain outside this assigned slice. No commits or dependency changes were made.
