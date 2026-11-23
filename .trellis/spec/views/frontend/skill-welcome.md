# Skills Welcome

## Scope and Entry

`SkillsPage` renders the shared `SkillWelcome` for an empty saved workspace
directory. Web and Desktop consume the same view; this is a collection state,
not a separate route or persisted onboarding completion flag.

```tsx
const showWelcome = isSuccess && skillData !== undefined && skills.length === 0;

<SkillWelcome
  key={wsId}
  buttonRef={emptyCreateButtonRef}
  onCreate={() => setCreation({ triggerRef: emptyCreateButtonRef })}
/>
```

Require a successful, defined result before claiming an empty directory. List
errors take precedence, including a failed refresh with cached empty data.
Pending, paused and idle reads without data retain the loading presentation.
Supporting member, agent or runtime query failures keep their existing warning
without delaying a successfully empty directory.

| Observation | Required collection state |
| --- | --- |
| Successful defined empty list | Welcome with one primary create button |
| Pending/paused/idle without data | Loading, no welcome |
| Initial or cached-data refresh error | Retryable list error, no welcome |
| Any saved skill, including built-in origin | Existing directory |
| Search/category removes populated results | Existing no-match/category recovery |
| Empty to populated Query data | Welcome is replaced by the directory |
| All saved records removed | Welcome can return; this does not prove no history |
| Workspace changes | Local example selection resets via `key={wsId}` |

`ApiClient.listSkills()` already falls back to `EMPTY_SKILL_LIST` for malformed
successful responses. This view cannot distinguish that inherited normalization
from a valid empty result. Do not claim authoritative historical first-use
detection or silently change the API compatibility contract here.

## Creation, Documents and Market

The welcome owns the single `New skill` action. Hide the duplicate header action
while it is visible; keep the header's explicit `From template` market link.
Reuse `CreateSkillDialog`, the opening button ref and existing connected-opener
or header fallback for final focus. Do not reproduce import/template forms or
change their draft and recovery semantics.

Documentation uses `paths.docsPage(DOCS_SLUGS.skills)` through `AppLink`.
Template discovery stays at `paths.marketplaceTab("skills")`. Templates are not
saved workspace instances and do not suppress the welcome; empty workspaces
must not be automatically redirected to the market.

Desktop `AppLink` intentionally exposes an absolute shareable URL, while Web
retains a relative href. Native tests must verify the resolved expected origin
and workspace path, rather than forcing Web's raw href representation.

## Illustrative Content and Interaction

The three scenes show five actual creation methods, explicit assignment to
multiple agents, and subsequent-run reuse. All five methods must remain present:
manual, template, local folder/archive, supported URL and runtime copy. Keep
labels in the existing `skills` locale namespace.

Every scene is explicitly illustrative, uses static localized examples and
performs no server reads or writes. A runtime copy is a snapshot; workspace
edits apply to subsequent executions and do not change work already running.
Do not imply automatic assignment or live runtime synchronization.

Carousel state is local and manually selected. Position buttons expose their
pressed state, direction buttons disable at the ends, and a polite status names
the selected scene. Inactive scenes are both `aria-hidden` and `inert`. There
are no timers, autoplay, new stores or dependencies.

Container queries account for sidebars and Desktop panes. Narrow layouts place
the primary action before the examples and scroll vertically. Use the shared
runtime tokens and role-named type scale; reduced motion disables the track and
opacity transitions. The clipped preview must not cause page overflow.

## Canonical Tests

- `skills/components/skills-page.test.tsx`: successful-empty gates, query error
  precedence, paused/unknown reads, auxiliary failures, data transitions,
  workspace reset, market routing and existing list/filter regressions.
- `skills/components/skill-welcome.test.tsx`: bilingual content, three manual
  positions, keyboard selection, disabled endpoints, hidden/inert scenes, real
  creation callback and embedded docs route.
- Existing creation-dialog/panel suites remain canonical for real import,
  template and recovery behavior; do not duplicate their matrices in welcome
  tests.
- `e2e/skill-welcome.spec.ts`: production-Web creation/cancel/focus, docs/market
  navigation, live directory transitions, filtered-empty recovery, compiled
  themes/locales/narrow CSS and reduced motion.
- `e2e/skill-welcome-desktop.spec.ts`: built Electron renderer, dialog/focus,
  exact shareable docs URL and controls reachable in the actual scroll container.
  Set reduced motion or await transition completion before compositor captures.

Wrong: show the welcome because filtered rows are empty or a query's default
array is empty. Correct: derive it from successful defined raw workspace data
and let the existing collection own loading, errors and filtered recovery.
