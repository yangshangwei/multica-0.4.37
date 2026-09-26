# Skill library template-entry planning evidence

## Task and current authorization

The user asked to create a new branch, refine requirements, and produce a design and implementation plan following a screenshot-based skill-library review. This turn is planning only. Branch: plan/skill-library-template-entry. Base: 14d64257ca11ac10516aa1d2566b3da8944b2aac on local main (already seven commits ahead of origin/main). Preserve the existing untracked .impeccable review artifacts. No application code, network writes, deployments, or runtime workflow is requested.

## Desired outcome

Make workspace skill management the main page task; provide discoverable, compact access to read-only templates; explain how templates relate to existing workspace skills and newly created independent copies. Reuse the current preview and draft flow. Preserve the established dark/light identity and all server-owned instruction content.

## Observed facts and code anchors

- The screenshot shows an expanded catalog. Current common catalog defaults closed: packages/views/common/builtin-template-catalog.tsx:29,54. The list has its own scroll and height cap at :78, and the container is shrink-0 at :52.
- The catalog sits before the management toolbar: packages/views/skills/components/skills-page.tsx:1009,1031. Workspace search/facets are intentionally separate (:825,829,976). Existing tests explicitly enforce scope separation.
- BuiltinSkillCatalog queries all named templates and uses skills.find for one matching instance: packages/views/skills/components/builtin-skill-catalog.tsx:24,39. Matching is official canonical name + verified builtin presentation OR template_source.name equality. Multiple copies are reduced to an unspecified first match. A merely same-name manual skill is not an official instance.
- Template previews already support search, platform/deployment groups, responsive list/preview navigation, and full instructions: packages/views/skills/components/template-skill-create-panel.tsx:31,53,59,123,181,204. Reuse this rather than build another browser.
- CreateSkillDialog presently supports initialTemplateName but cannot directly open unnamed template browsing: packages/views/skills/components/create-skill-dialog.tsx:648,661,687. Initial selection only seeds preview. Root owns template session and discard guard (:680,703); final create is distinct from preview and adoption.
- Root template session holds drafts, unknown-result recovery, and workspace request affinity: packages/views/skills/hooks/use-template-skill-session.ts. Existing tests in create-skill-template-flow.test.tsx and e2e/skill-template-creation.spec.ts cover preview-without-writes, draft customization and independent copies. Do not route existing skill links through recovery openCandidate: it invokes creation completion semantics.
- Official role localization is provenance-sensitive. getBuiltinRoleSkillPresentation is valid for template names; getSkillPresentation requires known name + origin metadata for workspace instances. It preserves customized and renamed content. See packages/views/skills/lib/skill-presentation.ts:36,89 and .trellis/spec/views/frontend/builtin-skill-localization.md.
- Chinese purpose descriptions are source-synced against server SKILL.md frontmatter. Short UI summaries must be a distinct display-only field; do not overwrite description, stored drafts, payloads or search behavior. All four locales en/zh-Hans/ja/ko must remain in parity. Unknown/deployment templates retain supplied descriptions.
- Copies record informational config.template_source {name,version}; they do not inherit official origin or permissions: packages/core/skills/template-draft.ts:79,106. Match data is a UX hint, not a security or content-integrity guarantee. Existing owner/admin/creator rules remain authoritative.
- readOrigin falls back to manual for builtin/copied sources: packages/views/skills/lib/origin.ts:23. Source-facet overhaul would expand scope and should be deferred.
- Cards are fixed-height 172px and virtualized by row; 240px minimum width and no column cap: packages/views/skills/components/skill-card.tsx:22,70,108 and skill-card-grid.tsx:12,56. Density changes need separate measured acceptance and must not break the fixed-height contract.
- Card primary navigation is clickable div with mouse-only rowLink handlers; an indirect keyboard menu exists. This pre-existing card issue can be a separately scoped follow-up; new template interactions must be fully keyboard accessible.
- The shared BuiltinTemplateCatalog serves other domains. Do not alter its behavior to fix skills. Rename or replace only skills-specific wrapper and update every callsite/test.
- Navigation in shared views must use AppLink/NavigationAdapter, never next/* or react-router-dom. AppLink supports native and desktop modifier/middle clicks: packages/views/navigation/app-link.tsx:1.

## Recommended scope to assess

Core: compact discovery entry; explicit available-template/workspace-skill counts; direct open of existing template picker without selecting a magic first template; clear preview vs draft headings/actions; display-only short summaries; reliable zero/one/multiple-match state; preserve full instruction/draft/create/recovery behavior; desktop/web parity and narrow-screen accessibility.
Deferred: broad workspace-card layout change, source-facet taxonomy, generic catalog rewrite, marketplace/search federation, template editing/version upgrade, automatic installation or assignment, mobile app implementation, API/schema changes and new dependencies.

Decide exact count labels in design: catalog can contain deployment templates, so never present total as a platform-builtin count. Prefer a generic template entry with explicit source counts, or a platform label whose count only includes platform templates. Reuse existing groups.

## Evidence limits

Two independent review agents completed the prior UX critique. Deterministic detector ran once on three target components with exit 0 and output []; this is not evidence of usable interactions. Native browser inventory failed with CUA_REPL_ENABLED_SURFACES is required. No live interaction, contrast, responsive screenshots or application tests were run for the review. Visual acceptance must remain a future implementation gate, not a claimed pass.

## Canonical verification and instructions

Read CLAUDE.md; .trellis/spec/views/frontend/builtin-skill-localization.md; .trellis/spec/views/frontend/skill-presentation.md; component-guidelines.md; existing archived task .trellis/tasks/archive/2026-09/09-12-skill-template-creation/contracts.md.
Commands are sourced from package.json, packages/views/package.json, Makefile and scripts/check.sh. Views tests: pnpm --filter @multica/views test <paths relative to packages/views>; source sync and parity beside existing tests; E2E: e2e/skill-template-creation.spec.ts with TestApiClient and an isolated checkout-owned environment. Exact runnable commands and prerequisites belong in implement.md/test-spec.md. No tests need running for this planning-only change except document/link/metadata validation.
