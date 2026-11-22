# Sidebar start-work entry points

## Goal

Make the three ways of starting work directly discoverable in the shared Web/Desktop sidebar, with clear workspace ownership and no extra New-menu click.

## Approved requirements

The user approved the proposed top-of-sidebar Start work area and explicitly requested implementation. This is consent to create and execute this task.

- Keep New issue, Start collaboration and AI Chat visible together above the existing Collaboration and Workspace directories.
- Present New issue as the primary action, Start collaboration as a secondary action with its Beta maturity marker, and AI Chat as a native navigation destination.
- Show the selected workspace name for ordinary creation and AI Chat; show cross-workspace scope for collaboration.
- Remove the generic New dropdown and the duplicate Chat entry from the workspace personal directory.
- Preserve existing issue-creation mode preference, drafts, collaboration source preselection, dialog focus return, chat unread semantics and selected-route highlighting.
- From an account/collaboration route, New issue and AI Chat use the authoritative workspace already displayed in the sidebar. They must enter that actual workspace before any workspace-owned form or chat operation runs.
- With no available selected workspace, retain collaboration access and visibly explain why workspace-owned actions are unavailable.
- Support both existing locales, themes, compact navigation sheets, and Web/Desktop shared behavior.

## Acceptance criteria

1. All three entry points are visible on the first open sidebar without opening a New menu or another directory.
2. One click on New issue opens the preference-backed existing creation form in an active workspace; the same click from account scope reaches the selected workspace and opens the form once.
3. Start collaboration opens the existing dialog with the displayed workspace preselected; dismissal returns focus to this visible entry.
4. AI Chat is labelled AI 对话 in Chinese / AI Chat in English and links to the displayed workspace's existing chat page. Its selected state and unread count retain existing semantics.
5. Account previews do not install a workspace API header, execute workspace queries/mutations, or open workspace modals before a real route owns the workspace.
6. Workspace switching, identity changes, navigation away and compact sheet closure cannot replay an obsolete ordinary-creation intent.
7. Existing scope restoration, collaboration disclosure/counts, zero-workspace account access, and issue/chat forms remain working.
8. Related component regressions, locale parity, lint/type checks and focused production Web/Desktop browser journeys pass; screenshots verify the approved layout and visible focus.

## Boundaries

This task changes sidebar navigation and its necessary route handoff only. It does not add a global chat data model, chat-to-task conversion, new creation forms, backend endpoints, mobile UI, dependencies, or unrelated cleanup. The future conversation-to-task workflow was a suggestion beyond this approved menu increment.
