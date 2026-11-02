# Research: Protected collaboration source-task browsing and identifier search

- Query: Which existing headless queries/API endpoints should the creation source-task picker use to browse on focus and search by title or identifier?
- Scope: internal; protected collaboration source options, backend query semantics, existing regression surfaces
- Date: 2026-11-02

## Findings

### Reuse the specialized protected source-option queries

- `packages/core/collaboration/creation.ts:21` owns `collaborationCreationOptions(client, scope, sourceId, parentSearch)`. A selected source workspace gets a Query key beneath its Server/viewer/creation/source identity with the search text appended. The query sends `AbortSignal`, uses `protectCollaborationDelivery`, disables automatic retries, refreshes on mount/focus and periodically, and reconciles current source memberships.
- `packages/core/collaboration/field-queries.ts:8` owns `collaborationFieldOptions(client, scope, issue, parentSearch)`. Its query stays beneath the issue's protected account identity. It submits parent search only when `edit_source_structure === true`; an external editor must never receive source catalogs.
- `packages/core/api/client.ts:1036` implements `GET /api/collaboration/creation-options?source_workspace_id=...&parent_search=...`. Empty search omits `parent_search` and is valid browsing. The existing client strictly parses options and checks the source identity before returning them.
- `packages/core/api/client.ts:1851` implements `GET /api/collaboration/issues/{id}/field-options?parent_search=...`, with strict parsing and an echoed issue-ID check.
- `packages/core/collaboration/delivery-access.ts:19` clears denied source-query data and drafts; `:52` guards late responses against changed account/source epochs. Replacing these reads with ambient ordinary issue queries would bypass established source-option access and cleanup behavior.
- `packages/core/collaboration/schema.ts:169` validates `parents`, `parents_truncated`, and the rest of the source-option projection, rejecting duplicate IDs within each catalog.

### Empty search already browses the latest eligible tasks

- `server/internal/handler/collaboration_creation.go:114` passes optional `parent_search` to the service after validating the source UUID and preparing the collaboration request.
- `server/internal/handler/collaboration_source_fields.go:31` accepts an omitted query, or one query value of at most 200 Unicode characters. `server/internal/service/collaboration_source_fields.go:132` trims it while preserving the same limit.
- `server/internal/service/collaboration_creation_options.go:27` proves a human principal and actual source membership inside one repeatable-read, read-only transaction before returning selected-source fields. `server/internal/service/collaboration_fields.go:52` proves current issue editing authority; only source-structure editors receive source catalogs.
- `server/pkg/db/queries/collaboration_source_fields.sql:3` owns `ListCollaborationSourceParents`: workspace-scoped, formal admission (`not_required` or `accepted`), excludes the supplied current issue, ordered by `number DESC, id`, with `LIMIT 201`.
- `server/internal/service/collaboration_source_fields.go:59` publishes at most 200 references. A 201st raw row sets `parents_truncated=true`. There is no cursor/offset or full-list pagination on this specialized endpoint. Browsing is a bounded latest-task catalog; narrowing search can find older tasks outside the initial window.
- Current eligibility includes completed/cancelled formal tasks and collaboration tasks: there is no status filter and no `issue_collaboration` exclusion in this SQL. Preserve this established behavior; adding exclusions is a separate product change.

### Root cause for full displayed identifiers

The current SQL search predicates are:

1. empty string matches all eligible rows;
2. case-insensitive literal substring in the title;
3. exact decimal text of `number` (e.g. `123`);
4. exact UUID text (case-insensitive).

It does **not** match the displayed identifier such as `MUL-123`. That full input returns no task unless the title itself contains the text. The identifier shown in the source reference is generated at `server/internal/service/collaboration_source_fields.go:69` with `util.ResolveIssuePrefix(ws.IssuePrefix, ws.Name)` plus `number`.

### Minimal backend repair recommendation

1. In `collaborationSourceOptions`, resolve the selected source workspace's prefix once using `util.ResolveIssuePrefix(ws.IssuePrefix, ws.Name)` and reuse it when formatting returned references.
2. Pass that exact resolved prefix into `ListCollaborationSourceParents` as an SQL argument.
3. Keep the original search text and all existing title/number/UUID predicates. Add an exact case-insensitive identifier predicate comparing the raw query to `resolved_prefix || '-' || number::text`.
4. Regenerate `server/pkg/db/generated/collaboration_source_fields.sql.go` with the repository's existing sqlc command.

This is preferable to stripping the prefix in UI or service code: stripping a wrong prefix could resolve a task from the selected workspace by an unrelated number, and replacing raw text with the suffix broadens title matches. Direct identifier comparison needs no guessed regex and automatically honors numeric-first prefixes and legacy fallback. `server/internal/util/issue_prefix.go:14` freezes the legacy fallback (first three ASCII letters from workspace name, or `WS`); do not replace it with the new-workspace slug rule.

Expected owner files: the SQL file, its generated Go file, `collaboration_source_fields.go`, and focused source-field backend tests. No API schema, database migration, generic issue query, or dependency change is needed.

### Existing tests and focused validation

- `server/internal/service/collaboration_source_fields_test.go:14`: `TestCollaborationParentSearchCharacterLimit` owns the 200-Unicode-character request boundary.
- `server/internal/handler/collaboration_source_fields_test.go:15`: `TestCollaborationCreationParentSearchAcceptsMultibyteCharacters` exercises the real creation-options endpoint and legal Chinese search.
- `server/internal/service/collaboration_source_fields_test.go:43`: `TestCollaborationSourceStructureAuthorityAndAtomicEdit` verifies source catalogs are available to a source editor and withheld from an external handler.
- `packages/core/collaboration/field-queries.test.ts:23`: external editors never submit a private source-catalog search; the remaining tests distinguish edit refusal from actual issue access loss.
- `packages/core/collaboration/fields-schema.test.ts:18`: validates source-option shape and required `parents_truncated`.
- `packages/core/collaboration/delivery-api.test.ts:22`: existing creation-options response parsing/source-identity behavior.

Add backend regressions for a full displayed identifier, case-insensitive full identifier, a wrong prefix (same number), numeric prefix and legacy fallback if they use separate code paths. Retain bare-number, UUID and title searches. Include empty browsing, another workspace exclusion, pending-admission exclusion, and a match older than 200 latest references so server filtering happens before truncation. Do not infer no matching task from an unfiltered 200-row client catalog.

Useful targeted commands (database environment must be configured through repository tooling):

```bash
make sqlc
go test ./internal/service -run 'TestCollaboration(ParentSearch|SourceStructure)' -count=1
go test ./internal/handler -run TestCollaborationCreationParentSearch -count=1
```

Run Go commands from `server/`. Shared UI tests should own focus-open results, selection/clear, keyboard behavior, visible loading/empty/error states, and stale-query suppression; backend tests own the identity/filter matrix.

### Related specs and reusable interaction precedent

- `.trellis/spec/server/collaboration-source-editing.md:18`: optional parent search, source-member authority, bounded references/truncation, and source structure semantics.
- `.trellis/spec/core/frontend/collaboration-workbench.md`: protected account/issue reads, epoch guards, scope ownership, and denial cleanup.
- `.trellis/spec/views/frontend/triage-duplicate-picker.md:45`: existing focus-open task browsing with loading/debounce, clear-to-browse, selection, keyboard and explicit empty/error states. Reuse its interaction expectations, not its generic workspace endpoint.

## Caveats / Not Found

- No existing regression for complete displayed identifiers or specialized parent-search truncation was found. The query currently handles only title substring, bare number and UUID.
- The specialized endpoint provides no pagination; preserve its 200-row/truncation contract and explain truncation in the UI rather than presenting it as exhaustive.
- Collaboration parent tasks are currently eligible and intentionally remain eligible in this task.
- No external references or new SDKs/dependencies are needed.
- This is read-only research. No tests were executed and no product/spec/task-planning files were edited.
