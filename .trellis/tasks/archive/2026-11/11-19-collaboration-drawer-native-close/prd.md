# Restore native pointer dismissal of collaboration drawers

## Request
Continue the already-authorized direct repair. The user reports that Escape and backdrop dismissal now work, but an actual mouse click on the top-right X still does nothing.

## Acceptance criteria
- A real operating-system pointer click on the visible X closes the full drawer and backdrop in the running Electron client.
- Verify by screen coordinates, not accessibility activation or DOM/Playwright click.
- Reopen and repeat; Escape and backdrop dismissal continue working.
- Retained panel drafts and existing focus return remain unchanged.
- The resource popup opts out of Electron drag hit-testing, matching the existing collaboration creation popup.
- Focused tests, Views typecheck, lint and independent review pass.

## Constraints
No new dependencies, backend changes, global window dragging changes, or shared Dialog redesign. Preserve the first repair for hidden keepMounted nodes. No task data writes, deployment or push. This is a continuation under the user's existing direct-fix authorization.
