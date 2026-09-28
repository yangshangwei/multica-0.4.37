# MCP sidebar entry

## Requirements

Expose the workspace MCP interface in the main AI Team sidebar, immediately after Skills and before Runtimes. Preserve the existing Settings MCP entry. Work in a new isolated worktree.

## Acceptance criteria

- Web and desktop open a workspace-scoped MCP page directly from the sidebar.
- The new entry uses the existing Server icon, reads MCP in English and Chinese, and stays selected on its page.
- Both entry points share the same MCP library, management permissions, and existing actions.
- Compact layouts retain access to the sidebar and the page scrolls independently.
- Existing MCP and navigation checks pass; run relevant typecheck, lint, and static analysis.

## Scope

No backend, database, mobile app, or MCP behavior changes. Keep all implementation in the new worktree based on the current committed HEAD.
