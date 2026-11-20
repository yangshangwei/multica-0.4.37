# Restore collaboration drawer dismissal

## Request
The user confirmed the reported collaboration drawer bug and requested direct repair after reviewing the root cause and proposed focused fix.

## Problem
Closing the collaboration resources drawer leaves its header, tabs and dimmed backdrop visible while its body disappears. Further close clicks and Escape cannot clear the shell.

## Acceptance criteria
- Closing with the top-right button hides the complete drawer and backdrop and returns focus to the opener.
- Escape still closes the drawer; reopening displays the selected panel.
- Closing and reopening preserves mounted panel identity and unsaved local drafts.
- Before first open the retained drawer and backdrop are hidden.
- Hiding an enclosing retained surface still suppresses its open popup and prevents stale reopening.
- Existing collaboration drawer, detail and affected popup lifecycle tests pass; scoped lint/type checks pass; actual running Electron UI confirms close and reopen.

## Constraints
Preserve shared Dialog APIs, keepMounted lifetime, Base UI ownership, permission boundaries and existing style. Add no dependencies. Do not change task data, backend, other popup families or unrelated work. No deployment or push.
