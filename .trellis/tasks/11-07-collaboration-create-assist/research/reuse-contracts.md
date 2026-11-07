# Source research: ordinary description assist reuse

- `packages/views/modals/issue-description-assist.tsx`: required four-method editorRef; request reads live editor rather than value. Validated done auto-adopts unchanged/unblocked text, supports Undo and clarification answers; partial stream stays inert. Existing markdown reads trim. Stale completed suggestion uses ReadonlyContent with workspace resource policy, so plain collaboration requires an opt-in inert preview/raw-read path.
- `packages/core/issues/mutations.ts`: useOptimizeIssueDescription(wsId) sets retry false and delegates explicitly scoped API; use selected sourceId even on account route.
- `packages/core/api/client.ts`: workspaceRequestInit sets X-Workspace-ID and clears ambient slug; optimizeIssueDescription supports bounded validated SSE and old JSON.
- `server/internal/handler/issue_description_assist.go`: member-gated server LLM, nonempty body<=20000 Unicodepoints/title<=500,45seconds,8process requests,128KiB request. Only text/title/mode sent; no attachment ingestion/runtime dispatch.
- `server/internal/service/collaboration_creation_options.go`: creation sources require membership, compatible with existing optimize endpoint membership gate.
- `packages/core/collaboration/delivery-access.ts`: protectCollaborationDelivery turns401/403/404/410 into source-loss cleanup. Do not use it for AI route errors, since old servers may have no optimize route; AI failure must preserve draft/files.
- Existing source keyed mount (apiScope/userId/sourceId) aborts old assist; draftId additionally resets undo/questions. Live isCollaborationScopeCurrent guard covers auth/APIepoch changes before callbacks/adoption. UI state flags alone do not establish ownership.
- Collaboration title1000/body1000000 exceed AI limits. Visible AI-only disabled reason; no silent truncation. Independent task AI execution policy does not change manual description-assistance mode.

Research was read-only; no actual model calls or quota consumed.
