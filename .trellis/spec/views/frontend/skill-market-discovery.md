# Skill template discovery

## Scope

`SkillLibraryCatalog` separates workspace instances from reusable templates on the
shared skills page. Workspace skills are managed in one tab; all reusable
templates, including deployment-provided ones, live in the Skill templates tab.
Web and Desktop use the same component, query cache and copy editor.

## Signatures

```ts
interface SkillLibraryCatalogProps {
  workspaceId: string;
  skills: readonly SkillSummary[] | undefined;
  skillsError: boolean;
  children: ReactNode;
  onPreview: (templateName: string, trigger: HTMLButtonElement) => void;
  onCreate: (trigger: HTMLButtonElement) => void;
}
```

`useSkillsViewStore` owns `libraryView: "workspace" | "market" | null`,
`marketSource: "all" | "deployment" | "builtin"`,
`marketCategory: SkillCategory | null`, with matching setters. Internal `market`
identifiers preserve existing preferences; user-facing copy says templates.
There is no shelf or collapse preference. Workspace-aware persistence resets missing
fields on rehydration; search stays local and resets when workspace identity changes.

`CreateSkillDialog.onTemplateCreated?: (skill: Skill) => void` handles a newly
created catalog copy in place. `onCreated` retains ordinary creation and explicit
recovery-result navigation. Do not confuse opening a recovered result with a new
successful write or emit a creation toast for it.

## Contracts

- The workspace panel never renders deployment templates or catalog loading/error/
  empty-source messages. A compact From template action opens the template tab
  and moves keyboard focus to its persistent tab trigger. Keep the action outside
  the tablist and allow its surrounding row to wrap in narrow containers.
- Counts belong to the respective tabs: workspace instances versus templates.
  The shared page heading has no redundant workspace count above the template view.
- The template panel begins with search and source filters. Do not repeat the
  tab context in an introductory heading; independent-copy guidance belongs in
  the creation preview, where it informs adoption.
- Only a successfully loaded empty workspace can initialize the market view.
  Explicit selection wins, including clicking the already-active Workspace tab
  while its data is still pending. Persist that gesture, not only a changed tab value.
- Source totals use the unfiltered, valid named template catalog. Result count
  uses source + category + search. Category availability uses source only; a
  search must not make the selected category disappear.
- Source changes clear a category only if it is unavailable in the new source.
  Use `readSkillPresentationMeta` for optional/unknown category/icon values.
- Reuse `getSkillTemplateDiscoveryItems` for source and bilingual display/search,
  and `getRelatedWorkspaceSkills` for renamed or multiple related workspace skills.
  Template labels are not workspace labels; do not invent author/rating/date fields.
- The catalog has one query owner and polls only in the active market view.
  Existing picker foreground polling remains separate; cached data stays usable.
- Keep the workspace tab panel mounted and explicitly hidden/inert while showing
  the market. `SkillsPage` owns the list virtualizer: unmounting its scroller from
  a child without rerendering the virtualizer can strand it on a detached element.
- Cards are preview buttons with independent sibling links/menu for existing
  skills. Preserve opener DOM identity through query changes.
- Before closing after `onTemplateCreated`, mark `closed.current = true` so the
  dialog's intentional-close focus resolver runs. Use the still-connected opener,
  otherwise the persistent page creation action.
- A successfully displayed preview identity is retained across catalog refreshes. If it is removed, show an unavailable message and disable adoption until another template is explicitly selected. An invalid initial seed may still use the established fallback. Adopted drafts stay editable.
- Workspace search remains visible in narrow containers and shrinks beside the toolbar actions.
- Named preview seeds resolve the narrow detail panel after catalog loading.
  The initial narrow preference is null; explicit Back/source selection is false
  and must not be overwritten by refreshes of the same preview seed.

## Validation and error matrix

| Input/state | Required outcome |
| --- | --- |
| Workspace pending | No empty default; hide unknown count |
| Workspace error | Keep catalog accessible; related state unknown |
| Confirmed empty workspace, no explicit view | Initialize market once |
| Templates pending | Skeletons, no false zero source counts |
| Templates fail without cache | Error + retry, no empty-market claim |
| Templates fail with cache | Keep cards/counts and show refresh error |
| Deployment source is truly empty | Hint and builtin discovery action in templates only |
| Populated workspace | Existing collection and From template action, no template shelf |
| Search removes all results | Preserve source count; clear-search/category action |
| Source/category refresh | Retain valid selection, never adopt/reseed an editor |
| Successful direct catalog copy | Stay in market, toast action, restore opener focus |
| Explicitly opened recovery candidate | Navigate using the original recovery path |

## Good, base and bad cases

- Good: a renamed template copy is linked by `config.template_source.name`; its
  market card remains available for another preview and independent copy.
- Base: a fresh workspace with only builtin templates starts in All, shows the
  deployment-zero hint, and retains the user's later Workspace choice.
- Bad: deriving initial view from `skills ?? []` while loading treats unknown as
  empty; hiding the entire catalog on workspace failure blocks recovery/discovery.

## Tests

- `core/skills/stores/view-store.test.ts`: preferences, old payloads, workspace reset.
- `views/skills/lib/skill-market.test.ts`: source/category/search composition.
- `views/skills/components/skill-library-catalog.test.tsx`: loading/error/default
  choice, active-tab intent, cache and copy links, locale-only resource loading.
- `views/skills/components/skills-page-template-session.test.tsx`: real page/query
  lifetime, draft guards, market creation and opener focus through failures.
- `e2e/skill-market.spec.ts`: production browser, wide/narrow discovery, shelf absence,
  From template keyboard focus, distinct counts, real copy creation and persisted
  preferences; only the template GET uses synthetic fixtures.
- `e2e/skill-template-creation.spec.ts`: real builtin catalog and existing editing,
  navigation/dirty guards through the template catalog and ordinary creation entry.
- `e2e/localized-template-defaults.spec.ts`: English/Chinese template entry,
  separate tab counts and From template visibility at 375px/360px. Use distinct
  workspace/catalog capture names so equal viewport sizes do not overwrite evidence.
- `e2e/workspace-defaults.spec.ts`: a fresh workspace exposes Skill templates
  without requiring a runtime; keep the tab name aligned with the locale contract.

## Wrong vs correct

```ts
// Wrong: success remains on the page but suppresses the final-focus resolver.
onTemplateCreated(skill);
onClose();

// Correct: this close is intentional; the caller can resolve the live opener.
closed.current = true;
onTemplateCreated(skill);
onClose();
```
