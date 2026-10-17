# Skill template discovery

## Scope

`SkillsPage` manages workspace instances. `SkillMarketPanel` owns template
discovery and copy creation under `/{slug}/marketplace?type=skills`; its
`SkillTemplateCatalog` renders built-in and deployment templates. Web and Desktop
share these components, queries and the copy editor. See [Capability market](./capability-market.md)
for the page shell and route lifecycle.

## Signatures

```ts
interface SkillMarketPanelProps {
  workspaceId: string;
  active?: boolean;
}

interface SkillTemplateCatalogProps {
  workspaceId: string;
  skills: readonly SkillSummary[] | undefined;
  skillsError: boolean;
  active?: boolean;
  poll?: boolean;
  searchRef?: RefObject<HTMLInputElement | null>;
  onPreview: (templateName: string, trigger: HTMLButtonElement) => void;
  onCreate: (trigger: HTMLButtonElement) => void;
}
```

`useSkillsViewStore` owns `marketSource: "all" | "deployment" | "builtin"`,
`marketCategory: SkillCategory | null`, with matching setters. Internal `market`
identifiers preserve existing preferences; user-facing copy says templates.
There is no library-view, shelf or collapse preference. Strip retired `libraryView`
and shelf fields from both migrated and merged payloads; old persisted values
must never decide the current route. Workspace-aware persistence resets missing
fields on rehydration; search stays local and resets when workspace identity changes.

`CreateSkillDialog.onTemplateCreated?: (skill: Skill) => void` handles a newly
created catalog copy in place. `onCreated` retains ordinary creation and explicit
recovery-result navigation. Do not confuse opening a recovered result with a new
successful write or emit a creation toast for it.

## Contracts

- The workspace page never renders templates or catalog loading/error/empty-source
  messages. Its compact From template link opens `paths.marketplaceTab("skills")`.
  Keep workspace search and this action reachable in narrow containers.
- Counts belong to the corresponding collection: workspace instances in the
  library and templates in the market. Do not add a workspace count to the market heading.
- The template panel begins with search and source filters. Do not repeat the
  tab context in an introductory heading; independent-copy guidance belongs in
  the creation preview, where it informs adoption.
- An empty workspace stays in the library and offers explicit market navigation.
  Do not auto-redirect based on instance counts or persisted former tab state.
- Source totals use the unfiltered, valid named template catalog. Result count
  uses source + category + search. Category availability uses source only; a
  search must not make the selected category disappear.
- Source changes clear a category only if it is unavailable in the new source.
  Use `readSkillPresentationMeta` for optional/unknown category/icon values.
- Reuse `getSkillTemplateDiscoveryItems` for source and bilingual display/search,
  and `getRelatedWorkspaceSkills` for renamed or multiple related workspace skills.
  Template labels are not workspace labels; do not invent author/rating/date fields.
- The catalog polls only in the active market view. While its creation dialog is
  open, the picker owns foreground polling of the shared key. Inactive panels
  disable queries and hide portaled dialogs/confirmation layers without replacing
  the editor DOM or draft. Cached data stays usable.
- `SkillsPage` owns its list virtualizer and scrolling element directly. Do not
  introduce a child wrapper that can detach the scroller without its owner rerendering.
- Cards are preview buttons with independent sibling links/menu for existing
  skills. Preserve opener DOM identity through query changes.
- Before closing after `onTemplateCreated`, mark `closed.current = true` so the
  dialog's intentional-close focus resolver runs. Use the still-connected opener,
  otherwise the market search input.
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
| Confirmed empty workspace | Keep library empty state and offer From template navigation |
| Templates pending | Skeletons, no false zero source counts |
| Templates fail without cache | Error + retry, no empty-market claim |
| Templates fail with cache | Keep cards/counts and show refresh error |
| Deployment source is truly empty | Hint and builtin discovery action in templates only |
| Populated workspace | Existing collection and From template action, no template shelf |
| Search removes all results | Preserve source count; clear-search/category action |
| Source/category refresh | Retain valid selection, never adopt/reseed an editor |
| Successful direct catalog copy | Stay in market, toast action, restore opener focus |
| Explicitly opened recovery candidate | Navigate using the original recovery path |
| Switch market type | Keep draft/editor mounted; hide portals and stop polling until active |

## Good, base and bad cases

- Good: a renamed template copy is linked by `config.template_source.name`; its
  market card remains available for another preview and independent copy.
- Base: a fresh workspace has an explicit From template entry; the market starts
  with the current source preference and shows a deployment-zero hint when applicable.
- Bad: auto-opening the market based on an empty or failed workspace query;
  hiding the catalog on workspace failure blocks recovery/discovery.

## Tests

- `core/skills/stores/view-store.test.ts`: preferences, old payloads, workspace reset.
- `views/skills/lib/skill-market.test.ts`: source/category/search composition.
- `views/skills/components/skill-template-catalog.test.tsx`: loading/error,
  active polling, cache and copy links, locale-only resource loading.
- `views/skills/components/skill-market-panel.test.tsx`: real query lifetime,
  draft guards, market creation, hidden portals and opener focus through failures.
- `e2e/skill-market.spec.ts`: production browser, wide/narrow discovery, shelf absence,
  From template navigation, real copy creation and persisted
  preferences; only the template GET uses synthetic fixtures.
- `e2e/skill-template-creation.spec.ts`: real builtin catalog and existing editing,
  navigation/dirty guards through the template catalog and ordinary creation entry.
- `e2e/localized-template-defaults.spec.ts`: English/Chinese template entry,
  source counts and From template visibility at 375px/360px. Use distinct
  workspace/catalog capture names so equal viewport sizes do not overwrite evidence.
- `e2e/workspace-defaults.spec.ts`: a fresh workspace can open Skill templates
  from the library without requiring a runtime.

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
