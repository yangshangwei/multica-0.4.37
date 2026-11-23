# Source catalog current session (AC85/AC89 source surface; AC96 note)

Active task: `.trellis/tasks/10-20-collaboration-gap-recovery`. Existing S6 acceptance
execution; no replan. Owned writes: `e2e/collaboration-source-surface.spec.ts`,
`e2e/fixtures/collaboration-source-surface.ts`, this file and `source-catalog-*`.
No product, migration, shared fixture, locale, SQL or dependency change.

## Behavior boundary written before edits

Clauses exercised by the pair through production Web and actual Electron, against
real public password sessions and current source code:

| Clause | Observable assertion | Canonical non-browser owner (not re-run as matrix) |
| --- | --- | --- |
| AC85 identity | List row, assignee board card, Gantt row and comment show handler name, avatar title and `External collaborator`; no `Unknown`; source GET summary has `external:true` and no email/username keys | `TestSourceCollaborationProjectionAndExternalFilter` |
| AC85 external filter | `External collaborators` toggles `aria-pressed`, hides ordinary rows, then restores them | same |
| AC85 assignee drag | Card's column root is unchanged while over target and after drop; no `PUT/PATCH /api/issues/{shared}`; DB snapshot (revision/epoch/assignee/status/events/notifications/session) unchanged; creator sees `Issue actions` dialog with `Transfer` form and recipient input | core `sourceCollaborationAction` + views surface tests |
| AC85 refusal | Web source-only observer gets `Only the current handler or creator can make this change.`; no write | capability service tests |
| AC85 nonterminal status | Drop on `In Progress`: `Change status` form preselects `in_progress`, no ordinary write. Web confirms through the collaboration action API (R47 validated workflow), Electron dismisses | workflow service status-option tests |
| AC85 terminal | Drop on `Done`: lifecycle description `Use Complete, Cancel or Reopen…`; no write, status unchanged | same |
| AC85 ordinary drag | Ordinary control card cross-status drop sends ordinary PUT with `status` and `move_intent`; persists | existing ordinary board suites |
| AC89 board reorder | Same-column drop sends exactly `{position, expected_revision}`, 200, persists after reload, collaboration snapshot unchanged | `TestCollaborationPositionUsesSourceAuthorityWithoutChangingCollaborationVersion` (also machine 403) |
| AC89 CLI reorder | Explicit task-built `multica issue reorder <shared> --before <id>` with isolated config; JSON result; order persisted; snapshot unchanged | same |

Not exercised by the browser pair (canonical service/handler evidence only):
machine-credential reorder denial, stage edit authority and transfer/departure
races, child-completion stage wake. Those stay with the canonical handler tests in
`source-projection-parent-canonical-*` (8 top-level / 31 including subtests passed).

## Harness defects found by static reading (fixed in owned files)

1. Workflow dialog accessible name is `Issue actions` (`collaboration.workflow.title`),
   not `Collaboration workflow`; the shared `DialogTitle` provides it.
2. Electron main window uses `createMemoryRouter({ initialEntries: ["/"] })`, owned by
   the tab coordinator. `page.goto(renderer + workspacePath)` cannot select a
   workspace route; Desktop `multica://account` deep links only open account paths.
   The pair now launches without an account deep link and uses canonical native
   navigation (sidebar `Issues`/`Projects` menu buttons, project `Issues` section,
   list row click), matching `project-p1-desktop.ts::openNativeProject`.
3. `issue reorder` prints nothing on stdout when already in position; the second
   client now reorders relative to the currently-later card so each call is a real move.
4. Status-column drags mutated state used by the later client; ordering now runs reorder
   and CLI before status drops, and a separate ordinary control card owns the
   ordinary cross-status drag.

## AC96

No AC96 file was added. The remaining AC96 gap (all-owner squad, new participant,
old credential, derived runtime and minimal-reference matrix as one combined
current proof) would need a new canonical service test; proposed name for root
coordination: `server/internal/service/collaboration_catalogue_matrix_test.go`,
selector `^TestCollaborationCatalogueAuthorityMatrix$`. Not created.
