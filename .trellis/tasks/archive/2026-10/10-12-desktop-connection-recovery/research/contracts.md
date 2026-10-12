# Source evidence
- Core AuthInitializer owns boot config/identity retries; auth store owns teardown/manual retry. Preserve transient credentials, 401 device handoff, epoch and StrictMode protections.
- ApiClient combines endpoint/caller signals; preserve deferred-response-body isolation.
- Desktop AppContent and IssueWindowContent both have early recovery returns. Persistent editing must live above them.
- Login already embeds DesktopEndpointSetupPage; reuse URL validation and configuration writer.
- main/server-switch.ts probes before mutation. main/index.ts save is main-window-only; probeServer already bounds /health and /api/config to 5 seconds.
- useWorkspaceList default retry is once; focus retries disabled. WSProvider needs an active workspace slug, so initial-list recovery cannot depend on WebSocket reconnection.
- Baseline: 208 core auth/store/API and desktop login/daemon-auth tests passed before changes.
- Pre-existing unrelated changes: cross-workspace task planning files and admin-auto-initialization archive movement. Exclude from task commit.
