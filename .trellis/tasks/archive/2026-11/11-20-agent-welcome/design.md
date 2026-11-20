# Agent welcome design

## Change boundary

The gap is the generic empty message versus the requested agent introduction. It lives in shared `AgentsPage`. Add domain-local `AgentWelcome` and CSS module; modify only the empty branch, localized agent copy and focused tests. Preserve the populated directory, auxiliary gates and approval entry. Do not modify other welcome pages or introduce a cross-domain carousel abstraction.

## State and navigation

Use agent-query successful status and raw count, not filtered rows or active count. Existing errors take precedence. Successful-empty is independent of auxiliary readiness. Pass runtime readiness from the page's existing runtime query, requiring success and an online runtime. Reset local carousel state on workspace change with `key={wsId}`.

Use `AppLink` with `paths.newAgent()`, `paths.docsPage(DOCS_SLUGS.agents)` and `paths.runtimes()`. React Query remains the source of existence; existing invalidations handle creation. Avoid duplicate primary header CTAs when the hero is visible, while retaining `ApprovalQueueAction`.

## Visual composition

Follow the screenshot within Multica's tokens and typography: blue agent badge, dark display headline, muted introduction, primary button, quiet book link, outlined checklist. The right column has a softly elevated card on a pale dotted blue scene and white caption area, with a faded next-card preview inside a clipped viewport. Distinguish real controls from inert illustrative content. Reuse provider logos/icons and localize sample prompt/task text, labeled as examples.

Local CSS owns container-aware stacking, carousel transforms, backdrop, dimensions and reduced motion. Use semantic color, type, shadow and radius tokens. No raster assets, font dependencies or global CSS. Content wraps in both languages; short/narrow screens scroll with the primary action before the illustration.

## Alternatives

- Enhance the generic centered state: smaller diff but misses the requested composition.
- New onboarding route or persisted dismissal: extra state and conflicts with data-driven visibility.
- Domain-local welcome in the existing empty branch (selected): shares across platforms and automatically returns to the list.

## Validation and rollback

Colocated tests own state gates, navigation and carousel semantics. Focused production-Web Playwright owns interactions, responsive screenshots and list transition. Run typecheck, focused lint and locale parity, then independent code/screenshot review. Rollback restores the empty branch and removes local component/styles/keys; no persisted state changes.
