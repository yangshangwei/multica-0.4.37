# Comment recall and shared handoff delivery

Status: implementation lane complete; integration/platform gates remain with the main session. No commit or real provider invocation was made by this lane.

## Boundary and delivered behavior

The missing behavior was access to comments beyond the legacy newest-200 runtime context, with bounded serialized JSON and recoverable oversized bodies. The change lives in the authorized comment service projection; CLI and shared views consume its typed contract. Existing human reads without `max_bytes` keep full entries, and legacy CLI reads keep their array/table behavior.

- Runtime `GET /api/collaboration/runtime/runs/{runId}/comments` accepts `limit` (1–100, default 50), `max_bytes` (4096–1048576, default 65536), and the existing opaque `(created_at,id)` cursor. It returns protocol 1 with identity, total, returned, bytes, budget, truncation reasons and next cursor. Current business authorization is checked under the existing runtime locks on every read.
- Human `GET /api/collaboration/issues/{id}/comments` enters the budgeted contract only with explicit `max_bytes`. Its total includes tombstones; runtime totals exclude deleted rows.
- Serialization budgets include UTF-8, JSON escaping, the full envelope, the self-referential byte count, and the handler's final newline. An oversized first row becomes an explicit placeholder with exact ID/revision/source reference; it advances the list cursor. A later unselected row is never skipped. A metadata envelope that cannot fit returns `budget_too_small`.
- Runtime `GET .../comments/{commentId}/content?revision=...` returns UTF-8 aligned chunks with issue/run/comment/revision/offset binding. Editing returns `stale_comment_revision`; deletion or current authority loss refuses further reads. JSON chunk budgets include all metadata.
- Comment `trust` is always `data_only`. Batched current account/participant/source-membership facts determine author role and `can_submit_command`, independently of immutable historical command/delegation admission. The author is never installed as the requesting principal.
- Explicit CLI `--limit`, `--max-bytes`, or `--cursor` selects the JSON envelope. `issue comment content <issue> <comment> --revision ...` reads chunks. Scope, malformed cursor/revision, unsupported filters and credential overrides refuse before HTTP. No ordinary-client fallback was added.
- Shared comment queries include projection budgets in cache identity. Schemas preserve missing old-server fields as unknown, reject malformed declared metadata, and keep placeholders noneditable. The existing human detail endpoint restores a placeholder only at the same revision. The detail displays loaded/total comments; source role and current author capability are visible.
- Shared run records accept stable `record:<stream UUID>:<0..255>` item IDs and optional recording-retention metadata, including an explicitly unknown tail. The new human-authorized handoff query/schema/panel displays live versus snapshot context, current handler, run role, prior result/source links, and missing/unknown facts. It does not call business-token routes.
- Built-in working-on-issues skill, collaboration workflow and source map describe pagination, fulltext recovery, stale versions and the trust boundary.

Reuse/simplification: retained the existing cursor ordering, runtime lock gate, human read transaction, single-comment recovery query, capability calculator and shared Web/Desktop views. No new table, migration, dependency, platform page or server-state store was introduced. SQL generation was performed by the main session.

## Files

The exact 40-file lane manifest and SHA-256 values are in [verification/recall-source.json](verification/recall-source.json); the tracked diff is [verification/recall-source.diff](verification/recall-source.diff). This includes the generated recall query file produced by the main session. Router/claim/checkpoint/handoff backend writers and unrelated concurrent edits remain owned by the other lanes.

Main groups:

- `server/pkg/protocol/collaboration_recall*`; `server/pkg/db/{queries,generated}/collaboration_recall*`.
- `server/internal/service/collaboration_comments_recall*`, `collaboration_comments_view.go`; `server/internal/handler/collaboration_comments{,_recall*}`.
- `server/internal/cli/collaboration_recall.go`; `server/cmd/multica/cmd_collaboration{,_recall*}` and comment registration in `cmd_issue.go`.
- `packages/core/collaboration/{content-*,comment-recall-queries*,run-records-schema*,run-handoff-*,index.ts}`, API client and content transport tests.
- `packages/views/collaboration/{comment-list*,collaboration-detail*,labels.ts,run-records.tsx,run-handoff*}`, `en`/`zh-Hans` collaboration locale files.
- The three built-in working-on-issues documents above.

## Verification

All commands below exited 0. DB service and handler suites were run sequentially against the exclusive migrated DB loaded with `source /tmp/multica_handoff_4d82a74dea_recall.env`; zero DB tests skipped. No credential values are included in the evidence. The source manifest identifies the final source/diff; earlier probes before coordinated SQL generation failed to compile and were not counted as passing evidence. Initial TS fixture-type errors were fixed before the final successful runs.

| Check | Exact command | Evidence/result |
| --- | --- | --- |
| Comment service regressions | `go -C server test ./internal/service -run 'TestCollaborationComment\|TestCollaborationRecall' -count=1 -timeout 15m -v` | `verification/recall-service-regressions.log`; 22 top-level tests passed, 0 skipped |
| Comment HTTP regressions | `go -C server test ./internal/handler -run 'TestCollaborationComment\|TestCollaborationRecall' -count=1 -timeout 15m -v` | `verification/recall-handler-regressions.log`; 8 top-level tests passed, 0 skipped |
| Protocol and CLI | `go -C server test ./pkg/protocol ./internal/cli ./cmd/multica -run 'TestCollaborationCLI\|TestCollaborationRecall' -count=1 -v` | `verification/recall-cli.log`; 14 top-level tests passed |
| Core schemas/transport/query identity | `pnpm --filter @multica/core test collaboration/content-schema.test.ts collaboration/comment-recall-queries.test.ts collaboration/run-records-schema.test.ts collaboration/run-handoff-schema.test.ts collaboration/handoff-schema.test.ts api/collaboration-content.test.ts` | `verification/recall-core.log`; 92 tests passed |
| Shared views | `pnpm --filter @multica/views test collaboration/comment-list.test.tsx collaboration/collaboration-detail.test.tsx collaboration/run-memory-records.test.tsx collaboration/run-handoff.test.tsx` | `verification/recall-views.log`; 18 tests passed |
| Core types | `pnpm --filter @multica/core typecheck` | `verification/recall-core-typecheck.log`; passed |
| Shared view types | `pnpm --filter @multica/views typecheck` | `verification/recall-views-typecheck.log`; passed |
| Go static checks | `go -C server vet ./pkg/protocol ./internal/cli ./cmd/multica` | `verification/recall-vet.log`; passed |
| Core changed-file ESLint | `pnpm --filter @multica/core exec eslint collaboration/content-queries.ts collaboration/content-schema.ts collaboration/content-schema.test.ts collaboration/comment-recall-queries.test.ts collaboration/run-records-schema.ts collaboration/run-records-schema.test.ts collaboration/run-handoff-schema.ts collaboration/run-handoff-queries.ts collaboration/run-handoff-schema.test.ts collaboration/index.ts api/client.ts api/collaboration-content.test.ts` | `verification/recall-core-lint.log`; passed |
| Shared-view changed-file ESLint | `pnpm --filter @multica/views exec eslint collaboration/comment-list.tsx collaboration/labels.ts collaboration/run-handoff.tsx` | `verification/recall-views-lint.log`; passed; the preceding view lint also included component tests, detail, run records and handoff tests |
| Whitespace/static diff | `git diff --check` | passed |

Named regression evidence includes 206 runtime comments at tied timestamps, an oversized newest comment, all-page progress, exact legacy newest-200 behavior, independent human tombstone count, full Unicode/escaped body reconstruction, edited/deleted content refusal, Stop revocation, foreign credentials, malformed metadata, and placeholder editing protection.

## Remaining integration gates

- Production Web and native Desktop visual/e2e acceptance, including visual-verdict evidence, are assigned to the main session. Component tests do not substitute for them.
- Independent Trellis review, full server/package checks, real model experiments and final acceptance mapping remain task-level gates.
- Handoff source/schema changes made later by the handoff lane must be mirrored in `run-handoff-schema.ts`. The panel currently implements the protocol 1 source kinds `handoff`, `delegation`, `provider_result`, `squad_activity`.
- The shared handoff panel is a bounded human projection with links to retained source runs. It distinguishes current context from a prepared snapshot and does not claim that a provider consumed or understood it.
