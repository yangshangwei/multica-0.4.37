# Skills Welcome Design

Status: approved for implementation on 2026-11-21 after the user reviewed the final planning summary.

## Scope and Visual Authority

This is a local extension of the existing Skills collection, in Operate mode. The supplied screenshot controls composition; Multica's current typography, semantic colors, primitives, and existing agent/squad welcomes control implementation details. Do not create a replacement product identity, root PRODUCT.md, or global DESIGN.md for this scoped change.

The reference is retained at `research/reference.png`. Its memorable structure is a quiet left introduction and a clipped right carousel showing a complete active card and a faded next card. Use code-native illustration content, rather than a screenshot or a generated raster, so localized text and themes stay readable.

## Recommended Approach and Alternatives

1. **Recommended: domain-local welcome in the existing empty branch.** Matches the reference, shares Web/Desktop, and lets normal Query updates restore the directory. No backend or persistence changes.
2. A centered expanded empty message would be cheaper but would omit the reference's principal composition and explanatory scenes.
3. A separate onboarding route or permanently visible banner would introduce navigation/state or crowd normal skill management. Neither is needed for the supplied brief.

## Component and Ownership Boundaries

- Add `packages/views/skills/components/skill-welcome.tsx`, its CSS module and the local CSS declaration matching neighboring components.
- `SkillWelcome` owns only the selected example, generated accessibility IDs, localized static scenes, and presentational layout. Props supply `onCreate` and a button ref for existing dialog focus recovery.
- `SkillsPage` retains ownership of server queries, collection states, creation session, selection, batch operations, virtualizer and scrolling element. Do not move the list into a wrapper that can detach its scroller without the virtualizer owner rerendering.
- `PageHeaderBar` conditionally omits its primary create action while the welcome owns that action. Preserve the market entry, standard page chrome and populated directory.
- Keep all new copy in the two existing `skills.json` locale namespaces. Reuse create-method labels where appropriate; use dedicated welcome text where a short explanation differs from an action label.
- Do not touch `packages/core`, API handlers, platform routers or generic UI primitives unless evidence reveals a prerequisite and the scope is reviewed first.

## Query and Creation Contract

The intended visibility predicate requires successful list status, a defined result array, and zero raw records. Existing list errors take precedence, including failed refreshes with cached empty data. Unknown/paused states retain a non-empty-state loading/recovery presentation; supporting-query failures keep their existing warning.

Do not inspect filtered rows, selected category, assignment counts, template catalog counts, or origin metadata to decide whether the workspace is empty. A saved built-in-origin skill is an ordinary record. Query errors cannot be turned into zero records by a default array for visibility purposes.

Mount the welcome with `key={wsId}` so a workspace change resets the carousel. Use the existing `setCreation({ triggerRef })` flow and `CreateSkillDialog`. Cancellation restores the live welcome trigger; if it has unmounted following a list transition, preserve the existing header-action fallback. Successful creation continues through the existing awaited creation/navigation logic. Do not change template recovery, draft retention, or picker activation.

## First Viewport and Scenes

- Intro: compact blue Skills badge, display heading “把团队的做事方法写成 skill”, muted explanation, one dark primary create button and quiet book/documentation link. Retain ample whitespace and avoid adding an unrelated checklist.
- Desktop: a balanced two-column grid, intro constrained for readable line length and showcase allowed to preview its next card. The full page must not acquire horizontal scrolling from the track.
- Scene one: a static method chooser showing all five real methods. Caption: “五种方式添加”. Use icons already used by the creation dialog, with one illustrative row emphasis; this is not a second creation form.
- Scene two: a sample `pr-review` skill connected to two sample agents, with role labels. Caption: “分配给智能体”. State that one skill can be explicitly attached to multiple agents; no actual assignment control or live identities.
- Scene three: an illustrative `SKILL.md` edit and shared reuse across subsequent executions. Caption: “维护一份，后续执行复用”. State that workspace edits apply to later execution; runtime imports are snapshots rather than synchronization.
- Every scene carries a visible localized example label. Keep explanatory text available to assistive technology while hiding purely decorative connecting lines/icons.

## Layout, Themes and Motion

Use container queries as the available space includes sidebar and Desktop panes. Stack intro before illustration in narrow containers; use natural content height and vertical scrolling for short windows. Support 320px without compressing copy into clipped rows. Local scene text may wrap; do not rely on screenshot-specific truncation to fit the English translation.

Use the role-named type scale and runtime semantic color, border, shadow and radius tokens. Match the reference's faint blue dotted scene using existing brand tokens; inactive cards may fade but active content must retain contrast. The small dot state and arrow controls follow existing 44px touch targets. Reduced-motion removes carousel and opacity transitions. No timers or autoplay.

Use native buttons with localized names, `aria-controls`, an active position state, disabled endpoints and a polite status announcement. Inactive slides are both `aria-hidden` and `inert`; visual mock controls are non-interactive elements. Do not introduce a tab widget unless its complete keyboard model is implemented.

## Compatibility, Validation and Rollback

No API, schema, routing, persistence or deployment migration is required. The malformed-response-to-empty API fallback is an inherited limit and must be reported honestly rather than hidden behind a claim of authoritative first-use detection.

State and wiring regressions belong in `skills-page.test.tsx`; standalone carousel and accessible content behavior belong in a colocated welcome suite. Existing creation-dialog tests remain the canonical source for actual import/template forms. A focused production-Web browser journey verifies compiled CSS and real navigation; perform a Desktop smoke for actual container scrolling and dialog entry.

The visual pass compares desktop and narrow captures against the supplied reference in one batched inspection, followed by at most one material correction/confirmation batch. Run the Impeccable mechanical detector once after the implementation is complete. Keep independent visual review and its evidence paths with the task.

Rollback consists of restoring only the affected Skills empty-state/header branch and removing newly added domain files/locale keys/tests. No data migration or preference cleanup is needed. Never revert unrelated working-tree changes.
