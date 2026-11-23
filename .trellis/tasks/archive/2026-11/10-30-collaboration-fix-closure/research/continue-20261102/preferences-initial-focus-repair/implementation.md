# Preferences initial-focus fixture repair

The original v9 manual hidden-priority test is unexpected/timedOut. Root's reader traced the intended Title(required) deferred autofocus occurring after More opened, with the menu subsequently detaching during the shell transition. The existing failing trace/error-context and inline failure screenshot are retained in the original report and referenced by before-and-failure-identity.json.

This patch adds three assertions: the Manual dialog is visible, its real Title(required) field has natural focus, and the existing Set priority... menu item is visible before choosing it. It does not call focus(), force an interaction, add a sleep or increase a timeout. All original creation, preference persistence, Quick/Manual hidden-field distinctions, HTTP201/priority and cleanup assertions remain unchanged. The prior memory patch is preserved, and main-worktree files and product UI are untouched.

Installed TypeScript5.9.3 parses the full target file with zero syntax diagnostics. Exact-diff verification proves only these three statements were added, and git diff --check passes. No browser/API/database/Provider run or project-wide lint/typecheck was executed by the implementer. Independent review and root's modified-case trace replay remain required.
