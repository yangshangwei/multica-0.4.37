# Design

Mode: Operate. Preserve existing neutral palette, semantic typography and shared Base UI primitives.

## Interaction and hierarchy
- Header: title; compact status/current handler/due date; one primary workflow action. More actions use the existing portaled DropdownMenu. Participants/role detail are one deliberate disclosure.
- Main: requirements → result/decision → discussion; preserve factual AI execution visibility.
- Secondary area below main: collapsed provenance/fields, files, context, knowledge, activity and children. Keep panels full width so complex editors remain usable. Preserve enclosing-disclosure recovery effects and anchors.
- Comment composer: text and Send remain visible; mentions and attachments are optional tools. Do not change draft/preflight/send semantics.
- Workflow dialog: reuse Dialog; action heading and issue context; bounded scrollable content. Keep the existing form and its mutation owner. Render mutation feedback/recovery inside the open dialog, outside when closed. Reject dismissal only during an active mutation/upload as necessary; unresolved receipts must remain recoverable and dismissible.
- Source-workspace entry already owns an outer dialog: render workflow in embedded mode there. Avoid stacked dialog traps and duplicated close controls.

## Boundaries and risks
No Core/Server state duplication. Existing result storage remains authoritative; ordinary local form cancellation retains its current semantics. No store additions. Keep explicit recovery rather than automatic replay. Maintain role/capability checks, exact baseline pinning and source access boundaries. Dropdown focus must return to a mounted trigger after menu actions.

## Rollback
Revert only this task's scoped patch against the pre-task snapshot, not entire files against HEAD: the worktree contains prior edits.
