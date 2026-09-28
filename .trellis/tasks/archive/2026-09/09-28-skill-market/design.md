# Skill market implementation design

## Approved reference
`.impeccable/design/skill-market/proposal.md` and its three PNGs are the visual and interaction reference. Data comes from the existing template catalog; ASCII deployment names and localized builtin names remain unchanged.

## Boundaries and data flow
- `SkillsPage` keeps the root creation dialog mounted independently of query error/success branches. The catalog wrapper manages the page view, and receives the existing workspace collection as children.
- New `SkillLibraryCatalog` in views owns the catalog query and renders view navigation, a compact deployment shelf or full market. It consumes `getSkillTemplateDiscoveryItems`, `getRelatedWorkspaceSkills`, `readSkillPresentationMeta` and existing icon/navigation primitives.
- Props: `workspaceId: string`, `skills: readonly SkillSummary[] | undefined`, `skillsError: boolean`, `children: ReactNode`, `onPreview: (templateName: string, trigger: HTMLButtonElement) => void`, `onCreate: (trigger: HTMLButtonElement) => void`.
- Undefined skills means the collection is not loaded. On workspace failure, copy status is unknown even with cached rows. Catalog remains usable.
- Catalog query uses existing `skillTemplateListOptions`; foreground market polls, collapsed workspace shelf does not. The dialog retains its own current polling/draft contract. No query changes expected.
- Extend existing core `useSkillsViewStore` with `libraryView: "workspace" | "market" | null`, `marketSource: "all" | "deployment" | "builtin"`, `marketCategory: SkillCategory | null`, `templatesCollapsed: boolean`. Setters: `setLibraryView`, `setMarketSource`, `setMarketCategory`, `setTemplatesCollapsed`. Existing workspace-aware storage/rehydration must reset missing fields. No server data persisted. Search remains component-local and clears across workspace changes.
- Null initial view resolves only after successful workspace data load, then sticks; explicitly selected view always wins. Browser rendering must not initialize from a pending/error empty array.
- Template preview callbacks receive the current button; page keeps the ref for dialog focus restoration. Fallback to a persistent page action if the card is removed.
- Direct named catalog creation remains in context. Only those successful creates stay on page and show a success action linking to the skill; existing normal create and template recovery semantics remain intact.

## Component and accessibility contract
- Template cards use real buttons for preview and independent links/menu for existing copies. Avoid clickable divs and nested buttons. Main actions remain visible without hover.
- Reuse Tabs/Button/Input/DropdownMenu/Skeleton, role-named text sizes, semantic colors and `SkillPresentationIcon`.
- One compact row: responsive 1/2/3/4 visible cards; full market grid uses the same card, content-based heights, at most four columns, no nested catalog scrollbar.
- EN and zh-Hans locale parity. Do not change existing template-picker strings that unrelated tests depend on unless needed for exact approved wording.

## Risks and rollback
- Workspace/global ambient persistence follows incumbent store contract; test workspace rehydration and payloads missing new keys.
- Do not remount the creation dialog on source/view/query state changes. Preserve refresh work present at task start.
- Existing source classification lacks explicit API source; reuse current helper, not a new version/name heuristic.
- Roll back only this task's newly added/modified hunks; baseline snapshot identifies pre-existing uncommitted changes.
