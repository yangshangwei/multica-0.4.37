# Research: F2 source collection and immutable persistence

- Query: Find the smallest reusable source/persistence implementation for an explicitly human-triggered report snapshot and exact Reporter GET replay.
- Scope: internal source research only; no code edits, database access, tests, git operations, Provider calls, or task lifecycle changes.
- Date: 2026-11-13
- Authorization context: the parent received “F2 授权，请推进”. This research recommends an implementation boundary; the parent owns the final contract and work allocation.

## Findings

### 1. Reuse projecthealth inputs, not just its result

`server/internal/projecthealth/snapshot.go:53–93` already separates the pure inputs into `Project`, `Issue`, `Status`, `Reference`, and `Input`. `Compute(Input)` at line 123 calculates deterministic counts/risk/health; `CanonicalJSON` at line 102 preserves JSON integer precision. This is the best existing reducer boundary.

`server/internal/projecthealth/collect.go:12–57` assembles the input using only its caller-supplied q; it does not borrow a pool or create a transaction. It loads workspace timezone, statuses including archived categories, current formal project issues, member/agent/squad/runtime validity, and latest project update. At lines 124–141 it reduces the fully assembled input and returns only `Collection`; the input itself is discarded.

Recommended small extraction:

- Add an input-returning collection function or an additive input field that preserves the exact assembled input. Keep existing `Collect` behavior, including its `UnavailableError` distinction, and have it call the same pure reducer.
- Freeze the actual minimal input payload, not only `StatisticsSnapshot`. F2 also needs native issue/project revisions, creation time, origin/admission classification and source manifests, which the current `projecthealth.Issue` does not contain. Carry these in F2-specific source structs rather than changing the meaning of the old health output.
- Reuse `Compute` for stock parity with the same fixed `Now`/timezone/input. F2's report exclusion and unknown partitioning belong in its own explicit reducer/projection. Preserve old `open` including unknown-status behavior for parity, then explain the F2 difference.
- A safe package split is a pure `internal/reportsnapshot` model/reducer/validation layer, a collector using q, and thin handlers. An implementation may keep the collector in the same package if it imports generated DB only; do not build a second workflow engine.

The stored replay input is **not automatically an authorized public DTO**. `collect.go:81–87` puts runtime IDs in `Reference.Facts`; lines 105–120 put a squad's leader and member-agent facts in it. Existing project-health responses do not expose these internal directory/roster details. Freeze only the necessary boolean availability facts and/or keep replay-only internal data separate from the returned projection. Reporter GET must not dump the entire collected input, all members, agents, runtime rows, private agent names, or invocation permissions. Public source issue ID/status/date/assignee fields may be returned only under the frozen project/report contract; arbitrary activity details, comments and protected evidence should not be added for convenience.

Files found:

| File | Purpose |
| --- | --- |
| `server/internal/projecthealth/snapshot.go` | Pure types, reducer, canonical hashing |
| `server/internal/projecthealth/collect.go` | Current q-bound input assembly and validity/readiness projection |
| `server/internal/projecthealth/counts.go` | Formal status partition and legacy open semantics |
| `server/pkg/db/queries/project_health.sql` | Formal project issue source and latest published update |
| `server/internal/projecthealth/unavailable.go` | Source failure versus globally unusable transaction behavior |

### 2. Report provenance: reliable positives, no universal negative

The strongest existing report identity chain is:

`issue.origin_type = autopilot` + `issue.origin_id` → `autopilot` in the same workspace + template key/version → an `autopilot_run` that links the same issue and autopilot.

Evidence:

- `server/internal/service/autopilot.go:790–814` sets origin_type=autopilot and origin_id=ap.ID server-side.
- `autopilot.go:838–854` links `autopilot_run.issue_id` in the same transaction as issue creation. This is stronger than a title or assignment heuristic.
- `server/internal/handler/autopilot_template.go:245–265` uses server-loaded template.Key and template.Version when creating the autopilot.
- `server/internal/handler/autopilot.go:50–55` documents provenance; `server/pkg/db/queries/autopilot.sql:56–64` writes it. Ordinary `UpdateAutopilot` does not change template_key/version.
- `server/internal/handler/issue.go:3140–3165` accepts only quick_create as public origin input; callers cannot stamp autopilot origin through the ordinary create endpoint.
- `server/pkg/db/queries/issue.sql:310–327` already demonstrates validating origin alongside a linked autopilot_run instead of trusting origin_id alone.

A minimal classification result should retain `classification`, `reason`, and safe provenance refs/version:

1. Exact current report ID: report, even without project.
2. Another issue already established as a snapshot's report root: report if that root relation is server-owned and in the current authorized scope. Do not use a global list from the client.
3. Valid built-in daily-progress-report/weekly-progress-report chain: report. Freeze the source key/version/link identity; later autopilot changes or deletion must not mutate old classifications.
4. Autopilot-origin issue with missing source, missing linked run, empty/legacy template key, or custom automation: unknown unless an explicit final v1 taxonomy chooses and documents a narrower definition. No title matching.
5. Non-autopilot ordinary issue has no universal proof that a human did not use it as an ad-hoc report. The parent must choose the contract explicitly: either (a) v1 excludes only verified report sources and treats other ordinary issues as business under that declared taxonomy, documenting that ad-hoc reports cannot be recognized, or (b) retain the prior strict “unresolved classification is unknown” rule and provide authoritative human classification input/provenance or accept unknown coverage. Do not silently claim absence of template metadata proves the business purpose.

The scope choice is material to usefulness: making every unclassified ordinary issue unknown is honest but makes many stock totals unknown; treating all missing metadata as ordinary silently changes the earlier strict proposal. This requires an explicit final implementation-contract decision by the parent.

### 3. Retained activity and temporal truth

`server/pkg/db/queries/activity.sql:1–23` reads only the newest requested activities and sorts them `(created_at,id)`. Public `handler/activity.go:47` caps activities at 2,000. A new private collector query may page the same current authorized cohort directly and must not call the public Timeline handler.

Proposed source query shape: `workspace_id`, exact cohort issue IDs (or an exact project join under the same RR), action whitelist, optional lower/upper event time and `(created_at,id) > last`, ascending order, `LIMIT page_size+1`. Include an explicit source issue-ID dimension if keyset order is `(issue_id,created_at,id)`. Count/check pages under the same q and cut. Cursor is ordering evidence only.

The current history cannot prove complete window facts:

- `server/cmd/server/activity_listeners.go:68–89` records status from/to after an event, through standalone queries. Failure is logged. This is not the original status writer's transaction.
- `server/internal/events/bus.go:57–61` dispatches synchronously, but synchronicity does not make the separate DB writes atomic or recover omitted history.
- `server/internal/handler/issue.go:4465–4483` publishes `project_changed`, but the activity listener has no corresponding project-history branch.
- `server/migrations/001_init.up.sql:156–164` makes created_at default now(); it is neither source effective_at nor commit time. Row IDs may be random. Do not sort by UUID to invent causal order.
- `server/internal/iteration/record.go:113–115` returns no recorder for non-iteration issues; line 138 persists facts/events for covered iteration rows only. Its sequence does not cover a project globally.
- Delivery project-change/completion receipts cover bounded paths, not a demonstrated full history baseline.

Therefore the first implementation can honestly preserve observed activity refs and classify window metrics as unknown with reasons such as `history_not_transactional`, `project_history_unavailable`, `status_category_history_unavailable`, and `producer_watermark_unavailable`. Complete DB pagination proves only complete retained-visible rows, not complete historical events. For window.created, created_at alone cannot prove project-at-creation. Verified partial lower bounds require proof of the event's project/time/category, not just that the issue is in the project today.

For all sources, max ID/time remains an ordering cursor; actual MVCC RR visibility is the cut. A source_limit result must not fabricate EOF. With proposed 10k candidates/100k activity caps, use limit+1 detection. Returning explicit source-limit unavailable is the simplest safe choice; a sealed partial is valid only if the contract precisely records the unenumerated extent/affected metrics and the transaction remains usable. 16 MiB limit applies to actual canonical persisted bytes, before insert; reject instead of truncating JSON/facts.

### 4. Smallest immutable storage and GET contract

Latest migration observed: `793_delivery_vcs_trust.up.sql`. The parent should reserve numbers before parallel migration work; suggested range begins at 794 but is not allocated by this research.

One immutable `report_snapshot` table is enough for a first implementation:

- ID, workspace_id, report_issue_id, nullable project_id, report_revision_at_capture, monotonically assigned report-local sequence.
- Capturing human actor_user_id, original request_key, canonical request_digest.
- Cycle identity/digest, optional correction_of, protocol/schema/semantics/coverage versions.
- Timezone/period/window/as_of/cut provenance.
- Actual frozen replay inputs/source manifest, safe public result payload, coverage/gaps, facts/result/snapshot digests, byte size, sealed state, created_at.
- No foreign keys, references, inline primary key, inline unique constraint, or trigger that writes business rows. Do not copy old legacy FK migrations.

Separate single-statement `CREATE UNIQUE INDEX CONCURRENTLY` files for ID, `(workspace_id,actor_user_id,request_key)`, `(workspace_id,report_issue_id,sequence)`; additional cycle-root uniqueness is needed if “same period” must be unique independent of request keys. A nonunique exact report/current read index can often be avoided when the sequence index serves it. Every index, including one on a new table, follows the concurrent rule. Add down files and schema-readiness verification according to existing migration conventions; `789–791` are a concrete no-FK/table/concurrent-index/readiness example.

`current` may be `ORDER BY sequence DESC LIMIT 1` under exact workspace/report/project scope, so there need not be a separate mutable current-pointer table. Seal is one completed row insert; do not publish preparing/incomplete bytes. Exact snapshot GET reads that row without live reaggregation. A correction inserts a new immutable row and becomes current; old IDs remain unchanged. `current` is an alias, never replay identity.

Cycle uniqueness and request idempotency differ. Original request key recovery returns the same snapshot even if sources changed. A new key for the same period must not silently masquerade as a correction: final contract should require correction_of/expected current identity, or define a separate unique root cycle and conflict for duplicate roots. The request digest must bind all scope/window/timezone/version/correction intent fields plus the authorizing actor; same key different payload returns409.

### 5. Human capture transaction and race handling

Recommended API boundary: explicit human `POST /api/issues/{canonical_report_uuid}/report-snapshots`, with a strict small request containing original request key, expected report revision, explicit window/timezone/period and optional correction binding. This is a separate permission surface from the two Reporter GET routes; Reporter/task/legacy agent tokens cannot invoke it.

Reusable patterns:

- `handler/project_update.go:271–293` and `projectHumanActor` establish a human-only actor; `handler/delivery_completion_preview.go:55–64` goes further by using verified credential provenance and requiring credential.Actor()==human. Do not rely on a caller's X-Agent-ID absence or agent display name.
- `handler/delivery_completion_authorization.go:56–65` parses one canonical Idempotency-Key, and lines70–87 reject conflicting workspace selectors. Reuse a neutral helper only if it preserves those exact checks, rather than coupling F2 to delivery semantics.
- `handler/handler.go:1132–1188` normalizes/loads actual issue and rejects independent collaboration access; any initial loader result must be re-read in the capture transaction. Proposed exact route may reject noncanonical UUID before this loader.
- `handler/project_write_fence.go:74–141` runs an entire RR unit, locks workspace/member fence, and retries serialization failures without carrying earlier-read input forward.
- `handler/project_update.go:615–629` checks a durable original request before preparing new state; lines679–683 turn a known concurrent idempotency unique conflict into whole-RR retry. Preserve request identity across retries.

Capture must lock/validate report and current project, then collect through the same q. Coordinate lock order with existing project association/deletion paths: current project before report-row locks, or use a reviewed NOWAIT/retry pattern; never keep an old initial project after a move. Membership/current human must be rechecked in the same transaction. The report revision is an input CAS binding, not a field to increment merely because a report snapshot was generated.

For one-table sequences, `MAX(sequence)+1` under a locked report is not sufficient to prevent an old RR view after lock wait: the report tuple is not modified by snapshot creation, so a waiting transaction can retain an old sequence view. Rely on the named sequence unique constraint and restart the entire RR on its race, or use an explicit internal state row. Do not “repair” with a pool query/new cut or modify issue revision just to serialize snapshots.

Response loss after commit: a retry of the same original POST/key returns the original stored row after current human authority checks, without recollecting. Idempotency conflict, source limit, permission failure, SQL failure and canceled context must not insert a half snapshot or advance current. No report comment, issue/project status, dispatch or Provider side effect belongs in capture.

A DB source query failure can abort the transaction; an error catch cannot seal partial input in an aborted tx. Use narrowly reviewed savepoints for recoverable source reads if partial capture is supported, otherwise rollback and return unavailable. Missing/retained-but-incomplete source facts that do not produce SQL errors can be represented as gaps without savepoints.

### 6. Retention and cleanup integration

The proposed default “retain until report/workspace deletion, no automatic purge” needs no scheduler and avoids accidental time-based destruction. Source issue deletion/move must not erase frozen facts from another report; report deletion itself is different.

- `server/pkg/db/queries/issue.sql:329–358` has a workspace-checked target CTE, then explicitly cleans non-FK children before deleting the issue. Add report-owned snapshots to this target-scoped cleanup, not `WHERE source_issue_id=...` and not a bare foreign issue ID.
- `server/internal/handler/issue.go:4831–4978` performs actual issue/batch deletion in one transaction after fences and row locks. Updating the shared `DeleteIssue` SQL preserves single and batch callers; new collector locks must serialize against this path.
- `server/pkg/db/queries/workspace_delete.sql:556–565` deletes workspace issue roots; `handler/workspace.go:1296` calls it during coordinated cleanup. Add snapshot deletion before roots, scoped by workspace; it is also acceptable to use a dedicated named step in the same existing transaction.
- `handler/project.go:1009–1015` explicitly cleans project progress then deletes the project; `project.sql:121–124` detaches surviving issues. If F2 retention is report/workspace-based, do not silently purge snapshots on project deletion. Their frozen project remains recorded and current Reporter scope denies them when the report is detached/moved. No new automatic cross-project replay permission follows.

The parent must state the project-deletion choice and any explicit future legal-delete policy. The existing project-delete impact response need not claim it deleted retained snapshots if it did not. No true indefinite-content promise should be inferred from absence of an automatic purge.

## Related specs

- `.trellis/workflow.md`: role-isolated research, preserve artifacts and explicit implementation contract.
- `CLAUDE.md`: UUID origin, package/API compatibility, no FK/cascade, concurrent single-statement indexes, test ownership and explicit DB tests.
- `.trellis/spec/server/task-api-capabilities.md`: exact scope, live identity, RR reads and frozen versus live projections.
- `.trellis/spec/server/database-test-isolation.md`: dedicated database and expected test inventory before handler/router execution.
- `.trellis/spec/server/verification-environment.md`: parent owns test environment/source identity and bounded package permits.
- Task `design.md §8` and F2 planning research: frozen input, coverage, metric units and unknown semantics.

## External references

None used. Current repository source and archived task design are the primary evidence. No new library/SDK or external service is recommended.

## Caveats / Not Found

- No current universal immutable project-membership history, status-category-at-event history, producer completeness watermark, or universal business-versus-report classification exists in the inspected surfaces.
- No existing project-health API exposes a complete replay input manifest. Extracting it must preserve the old collector's source-error and return behavior.
- No source read was treated as an authorization to disclose protected runtime/roster/private evidence fields.
- This file does not reserve migration numbers, implement an endpoint, validate a database, or claim any F2 acceptance case passed. The parent must settle the classification taxonomy and cycle/correction identities before parallel code lands.
