# Independent review: project metadata lock correction

**Approved; no blocking findings.** The change narrowly fixes the demonstrated
description/preparation conflict without weakening the reviewed authority,
revision, association, deletion or squad/resource create-update fences.

This is an independent read-only review of main's correction. I changed no
production code/tests and ran no database suites. Exact inspected hashes,
saved test names and command metadata are in
`project-lock-independent-review.json`.

## Scope and evidence

The correction adds `LockProjectForUpdate` with `FOR NO KEY UPDATE` in
`server/pkg/db/queries/project.sql`, its generated sqlc method, and one production
caller in `Handler.UpdateProject` at `project.go:648`. A repository-wide caller
search found no other use of the new helper. The ID/workspace predicates and
returned project fields match the previous locking read.

The retained red log demonstrates the actual issue: while the real description
handler holds its successful write before commit, actual knowledge preparation
returns `503 knowledge_busy`. Issue-number allocation passes in that same red
run. The test does not substitute a generated lock query for the real writer.

Main's saved guarded `-race` handler run has **266 passing tests/subtests** across
**131 top-level tests**, zero failures, zero skips and empty stderr. The exact
`TestKnowledgePreparationWhileRealOrdinaryWriterIsUncommitted/project_description`
case passes. The command record reports exit 0 and unchanged source; I separately
compared its before/after inventories and the three current correction files.
They match exactly.

| Correction file | SHA-256 |
| --- | --- |
| `server/pkg/db/queries/project.sql` | `dcfed9ecaec29b307f6ebdc038eae3711d4514f346002aedba42051f9fb9758d` |
| `server/pkg/db/generated/project.sql.go` | `961c5ee46187420158a3b1796493f025450db89633fdf35bcdc8a6029d52c97c` |
| `server/internal/handler/project.go` | `9dc839e6ae2942d3df4e5b8f6660ecd193524281f7433535c480040e38abc6a7` |

## Why the fences remain intact

`UpdateProject` changes metadata, status/lead fields, dates and revisions; its SQL
does not assign `id`, `workspace_id` or execution-squad selections. Its
transaction still locks workspace existence, the subscriber revocation fence
and current membership before taking the project lock. Expected revision and
description revision are checked under that lock, omitted fields are merged
from the locked row, and all writes/audit entries commit together. Repeatable
Read and the existing bounded whole-transaction retry remain unchanged.

PostgreSQL `NO KEY UPDATE` conflicts with another `NO KEY UPDATE`, with `SHARE`,
and with `UPDATE`; it is compatible with `KEY SHARE`. Therefore:

- Concurrent metadata edits remain serialized. The saved race run includes the
  one-winner description test, stale-revision/no-op behavior and membership
  revocation after a Repeatable Read fence wait.
- `LockKnowledgeProject` intentionally reads only project identity/existence
  under `KEY SHARE NOWAIT`; this is the newly allowed overlap. It grants no
  project-lead authority and does not protect a mutable base description.
  Independent knowledge revision/source/candidate locks remain unchanged.
- Actual human project-lead decisions use `LockKnowledgeProjectLead`, which
  remains `SHARE NOWAIT`. Metadata lead changes still conflict with it.
  Owner/admin or source-authorized paths do not depend on the lead field.
- Association, project-health and progress/notification readers use the
  unchanged `LockProjectForAssociation[Nowait]` `SHARE` modes. Issue creation,
  issue association, triage, chat, autopilot and iteration callers retain those
  queries, so the correction does not permit their protected reads to race a
  project metadata update.
- `DeleteProject` still takes the exclusive `LockProjectForExecutionSquad`
  `FOR UPDATE` before revision validation and child cleanup. It keeps its
  Read Committed sweep protocol and workspace/iteration/member fences.
  Deletion still conflicts with metadata updates and parent-existence reads.
- Execution-squad configuration, progress writes, and project-resource
  create/update still use `LockProjectForExecutionSquad` `FOR UPDATE`. Their
  selection/materialization and resource ordering protocols are unchanged.

The correction changes a lock mode at the same position in the existing graph;
it removes an unnecessary conflict with identity-only readers and introduces no
new lock acquisition or reversed ordering. The generated implementation matches
the query and preserves both tenant arguments.

## Precision and limits

The separate knowledge-target cleanup in the current `DeleteProject` diff is
outside this lock-mode correction; I confirmed that its exclusive parent lock
remains in place. Statements about exclusive resource writers refer specifically
to resource create/update. `DeleteProjectResource` has an unchanged direct-delete
path, so this review does not imply a newly added lock there.

The saved test command and source evidence are under
`../integration-verification-2026-11-01/go-final/project-lock*`. This approval is
for the bounded correction above, not a new full audit of all project behavior
or a claim that the reviewer reran the database matrix.
