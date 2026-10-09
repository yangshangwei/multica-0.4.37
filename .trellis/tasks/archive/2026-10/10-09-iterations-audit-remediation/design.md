# Technical design

## Ownership

React Query continues owning server state. Views own search/filter/paging/disclosure state. UI primitives remain free of business logic. Platform adapters continue owning routing; no next/* or react-router-dom is introduced into views.

## Readable status and focus

Add warning-foreground and info-foreground semantic text colors in both themes, including @theme mappings. Preserve fill/chart colors. Verify actual tinted badge backgrounds, dialog surfaces and page canvas at the WCAG AA text threshold. Fix shared TabsContent using a visible focus-visible outline/ring that is not clipped by a scroll container.

## Platform-owned main landmark

Overview and detail use div roots. Web keeps the single main landmark supplied by SidebarInset. Desktop's MainCanvas remains a div because other routed views already supply their own main; changing the shared canvas would create nested landmarks outside this task. IterationsPage accepts an optional mainLandmark boolean, default false, to render its outer wrapper as main. Only the two existing Desktop iteration routes opt in. The wrapper contains recovery and initial loading/error states as well as the loaded page, so the route retains one landmark throughout the interaction.

## Operations and controls

Use existing Select/SelectTrigger/SelectValue/SelectContent/SelectItem, Textarea and Checkbox primitives. Keep Select items label maps authoritative. Label controls explicitly, expose required reasons and explain disabled preview/start actions. Group previews into named summary and task sections, use definition lists, preserve stored identities and show operation-specific/destructive confirmation actions. Scope text must wrap. Use pointer-coarse:min-h-11 consistently, including controls rendered in portals; use min-w-11 for icon targets.

## Compact filter metadata

ListIterationIssues already obtains the complete current and original historical projections. Derive optional filter_options from the unfiltered selected projection before search/filter/pagination. Include distinct stored status keys, project identities/names, typed assignee identities/names and labels; never infer them from live metadata or one visible task page. No new query parameter or extra endpoint is necessary. Extend the existing response schema so old installed clients ignore the field and newer clients tolerate its absence. A backend without metadata must not trigger the previous full task traversal; communicate unavailable advanced options and preserve search/scope/paging. Explicit grouping still uses the complete grouped read and retains its coherence checks.

## Pagination and retained state

Add previous-page cursor history for task and project lists. Reset it on search/filter/scope/group/identity changes; a stale-cursor retry returns to the first page. Preserve mounted tab/filter state and existing authorization/error distinctions. Overview local reveal-more is labeled Load more.

## Visual evidence and rollback

Use fresh real renderer/browser captures, paired with incumbent screenshots only as layout references. Batch the first inspection and follow-up confirmation. Each lane owns disjoint files, with the main session owning locales and integrated verification. A pre-edit source snapshot is retained outside the repository so this task's delta can be reviewed separately from the dirty baseline. Revert only task-owned deltas if needed; never reset the shared worktree.
