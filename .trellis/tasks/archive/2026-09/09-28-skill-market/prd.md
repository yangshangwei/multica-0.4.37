# Discover deployment templates in the skill library

The user approved `.impeccable/design/skill-market/proposal.md` and requested a new Trellis task and implementation on 2026-09-28.

## Requirements
- The workspace skills page directly exposes one compact row of deployment template cards, with collapse and browse-all actions.
- Workspace and market views have separate counts, searches and filters. The market supports all/deployment/builtin sources plus available categories.
- Confirmed empty workspaces initially show the market; explicit view choices remain stable after refresh and creation, scoped per workspace.
- Deployment source zero defaults to useful builtin content and a compact explanation. Loading, failure, search-empty and actual-empty remain distinct.
- Cards show existing template names, summaries, categories/icons, sources and related workspace copies. Support multiple renamed copies, unknown workspace status and keyboard access.
- Clicking a template opens its selected existing preview; reuse the editable independent-copy flow. Preserve draft and focus through query failures and refreshes.
- Creation from the market keeps browsing context and offers a link to the created skill. Ordinary new-skill flow retains existing navigation.
- Follow existing semantic theme tokens and role typography; narrow containers keep search/actions visible and avoid horizontal overflow.
- Preserve current template freshness changes already present in the working tree. No dependencies, backend APIs, publishing controls or invented metadata.

## Acceptance
- Test source/category/search composition and workspace-scoped preferences/reset.
- Test empty workspace default and explicit view persistence; actual empty vs query failure.
- Test direct named preview, copy links and session/focus preservation.
- Relevant existing skills tests, changed-package lint/typecheck, app typecheck and static checks pass.
- Inspect rendered wide and narrow layouts, compare with approved design, and record evidence.
