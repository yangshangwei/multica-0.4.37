# skillcat-check report (agent #3) — started 2026-09-20
Task: .trellis/tasks/archive/2026-09/09-19-skill-library-categories
Range: 8053e8d71..a9d034dc6
[progress] read prd/design/implement/check.jsonl + 4 specs + skill-presentation.md spec. Starting item 1 (views diff).
[item1 partial] skill-card.tsx / skill-card-grid.tsx / sidebar / chips / facets / icon lib+tsx read.
  - card: checkbox button stops click+auxclick; kebab delegated to SkillRowActions (verify it stops propagation).
  - card selected state: border-primary/50 (hover does not touch border) => identifiable while hovered. OK.
  - sidebar active: font-medium + text-foreground (weight untouched by hover). OK. chips active: border-foreground/40 + font-medium. OK.
  - grid: ResizeObserver effect deps [] reads ref once, disconnects on cleanup; jsdom guard present. lines memo [rows, columns]. keys: line=vi.key, card=skill.id. OK.
  - sidebar is w-52 (design said w-56) - cosmetic, not a defect.
  - TODO verify: useRowLink events (mousedown/keydown?) vs stopPropagation on checkbox; skills-page memo deps; toolbar circular import; tokens --skill-* exist; `dot` tone usage.
[item1 cont.] skills-page.tsx / toolbar / list-actions / presentation-fields / detail-page / create-dialog / label-picker / tokens read.
  - allRows memo deps [skills, assignments, membersById, runtimesById, currentUserId, myRole, presentSkill]; rows memo deps [allRows, search, filters, sortField, sortDirection, locale]. No stale closure found.
  - detail-page: presentationMeta added to sync-effect deps + dirtySummary deps; save merges via writeSkillPresentationMeta(skill.config, ...). OK.
  - create-dialog: config only sent when non-empty; label_ids only when non-empty; presentation seeded once via useState initializer. OK.
  - label-picker draft mode: attach/detach hooks called unconditionally with "" id, never invoked in draft. TODO verify resourceLabelsOptions disables on empty id.
  - tokens.css: --skill-* light+dark, --color-skill-* in @theme. No hardcoded palette classes seen so far.
  - PRE-EXISTING (not in range): SkillRowActions wrapper span stops only `click`, not `auxclick` (useRowLink doc asks for rowLinkInteractiveProps). Card inherits it: middle-click on kebab opens row in new tab. Judgment call.
  - JUDGMENT: list view inside sidebar layout: list scroller is its own @container; between ~672 and ~880px outer width the sidebar shows but the list collapses to core columns (name+usedBy). Honest to its own width, not a bug.
  - JUDGMENT: users with a persisted hiddenColumns from before this feature will see the new "labels" column by default (their payload lacks "labels"); PRD default-hidden applies only to fresh state.
[item3 partial] core presentation.ts read: reader guards null/undefined/non-object/array; writer destructures into a new object (no mutation). view-store merge: filters deep-merged; viewMode NOT defaulted for old payloads -> inherits previous workspace's in-memory viewMode on switch (candidate fix: spread DEFAULTS before persisted). Checking tests next.
[item3] presentation.test.ts covers undefined/{}/string/[]/null + wrong-typed fields + legacy tags; writer no-mutate test present; schemas.test.ts has labels missing/malformed fallback (per-summary .catch). view-store.test.ts merge test only proves filters backfill; viewMode backfill assertion passes trivially (current already "card").
  DEFECT CANDIDATE (core view-store merge): persisted payload from before `viewMode` existed keeps the PREVIOUS workspace's in-memory viewMode (merge = {...current, ...p}); zustand persist then writes that merged state back, so workspace B silently adopts A's card/list choice. Fix: spread DEFAULTS before persisted. Will fix + test.
[item2 start] server diffs read (skill.go, skill_create.go, skill_template.go, agent_template.go, presentation.go).
  - Create: Validate+Normalize; label_ids parsed by parseUUIDSliceOrBadRequest; GetLabel check runs BEFORE the tx (h.Queries, not qtx); attach happens inside tx. PRD R4 says "在创建事务内校验" -> checking whether moving the check into createSkillWithFilesInTx is cheap.
  - Update config branch: Validate+Normalize. Import finish: seed from frontmatter when absent, Validate (400) then Normalize. Import overwrite: Validate (400 structured) + Normalize. Template materialization: Normalize only (trusted built-in values). ListSkills: Labels default []LabelResponse{} (never null).
[item5] env headers: node only on presentation/view-store/template-draft/facets/icon-map tests (0 window/document refs); component suites jsdom; schemas.test.ts + create-skill-dialog.test.tsx have no header (pre-existing files, default env). OK.
[item6 partial] SKILL.md + source-map describe presentation validate/seed, label_ids 400 text, list labels; no tags residue outside deliberate legacy-ignored tests + spec note.
[item4] key parity: zh-Hans/ja/ko each have exactly en's keys minus 19 `_one` plurals (en-only). zh copy: straight quotes in empty_title, lowercase `skill`, 「你」 in set_category_no_permission. conventions.zh.mdx:295 confirms straight quotes. MET.
[item2 cont.] AttachLabelToSkill SQL guards ws + resource_type='skill' + ON CONFLICT DO NOTHING; GetLabel pre-check outside tx (design.md §1.6 shape). Race = label deleted between check and tx -> silently missing label, never a bad row. JUDGMENT, not fixing. ListSkills parseUUID(workspaceID) on middleware-resolved id = trusted round-trip, OK. Test t.Cleanup at :232 is the mock ClawHub server, not an INSERT/DELETE pair. Error strings name the field (tests assert). Source-map line refs (1895/1944/1813) were stale BEFORE this range (base already had origin at :2383); new rows use symbol names. Role SKILL.md metadata matches design table; builtin_agent_templates_test pins all 8.
[concurrency] view-store.ts changed on disk mid-review: someone added persist `version: 1` + `migrate` (hides `labels` for v0 payloads). Another check agent is live in this tree. Checking git status before touching the file.
[concurrency] git status shows another live agent's UNCOMMITTED edits: schemas.ts (template category/icon .catch), skill-template-client.test.ts, view-store.ts (version:1 + migrate hiddenColumns), view-store.test.ts (jsdom persistence suite), skill-list-actions.tsx (kebab focus-visible). Not mine. Their `migrate` makes hydration call setItem() for every v0 payload, which PERSISTS the viewMode leak from the previous workspace (merge = {...current, ...p}, p lacks viewMode). Planning fix: spread DEFAULTS before p in merge. Coordinating by message first.
[item2 done] Other config-storing paths audited: RefreshSkill passes mergeSkillConfigOrigin(skill.Config, origin) = already-validated stored config (no new presentation input) OK; runtime-local daemon import writes {origin} only (no presentation, no seed) OK/judgment; UpdateSkill replaces whole config (client merges via writeSkillPresentationMeta) OK. PRE-EXISTING: UpdateSkill uses parseUUID(id) from the URL param instead of skill.ID (CLAUDE.md UUID rule) - out of range, not touched.
[zustand 5.0.12 verified] hydrate: migrate -> merge(migratedState, get()) -> set -> setItem() when migrated. Confirms: v0 payload lacking viewMode + another workspace's in-memory viewMode => leak persisted immediately once `version:1` exists. Proceeding with merge fix (DEFAULTS spread before persisted payload).
[UNCOMMITTED FIX] packages/core/skills/stores/view-store.ts merge: `...DEFAULTS` spread between current and persisted payload (comment explains). Tests added in packages/core/skills/stores/view-store.test.ts: pure-merge case "falls back to the default, not the in-memory value" + persistence case "does not carry the previous workspace's view mode into a v0 payload". Running the file now.
[design drift, judgment] design.md §1.1 said server stores presentation as written; shipped code normalizes (drops default icon / empty presentation) on every write and .trellis/spec/views/frontend/skill-presentation.md documents that. TS writer and Go normalizer agree, so no functional drift; only design.md is stale.
[verify] view-store.test.ts 12/12 pass (exit 0); red-state probe on a temp copy without the DEFAULTS spread: exactly the 2 new tests fail (probe files deleted). packages/core tsc --noEmit exit 0; eslint on the 2 edited files exit 0.

## FINAL (agent #3)

### (a) Acceptance criteria
| # | Criterion | Status | Evidence |
|---|---|---|---|
| 1 | enum / whitelist / parser unit tests incl. missing / mistyped / invalid fallback | MET | packages/core/skills/presentation.test.ts (undefined, {}, string, [], null, category 3, icon 1, legacy tags) |
| 2 | server 400 on invalid config.presentation, valid stored, table-driven | MET | server/internal/skill/presentation_test.go; server/internal/handler/skill_presentation_test.go (Create/Update tables assert error text names the field) |
| 3 | built-in role templates materialize with preset category+icon | MET | 8 SKILL.md metadata blocks; service/builtin_agent_templates_test.go pins all 8; handler/agent_template.go writes presentation via NormalizePresentation; handler test reads config from DB |
| 4 | import with metadata.category/icon seeds config.presentation | MET | handler/skill.go finishSkillImport seed (only when absent) + Validate + Normalize; TestImportSkill_SeedsPresentationFromFrontmatterMetadata (invalid icon dropped, tags ignored). Archive import goes through the same finishSkillImport |
| 5 | list embeds labels; invalid label_ids 400; valid attached | MET | SkillSummaryResponse.Labels default []LabelResponse{}; labelsBySkill batched ListLabelsForSkills; CreateSkill GetLabel + resource_type check; AttachLabelToSkill inside createSkillWithFilesInTx; handler tests for malformed / other-ws / agent-scope / happy path |
| 6 | wide: sidebar with counts, click narrows; narrow: chips | MET | skill-category-sidebar.tsx / skill-category-chips.tsx driven by useSkillListFacets(allRows); @container @2xl switch; sidebar + chips + page tests; main-session smoke |
| 7 | view mode survives reload; per-workspace isolation | MET after fix | view-store partialize viewMode; workspace-aware storage; UNCOMMITTED FIX in merge (see b) closes the v0-payload leak; persistence tests now cover both |
| 8 | card view content, select, batch, kebab parity | MET | skill-card.tsx (tile, name, lock, desc, LabelChip+N, avatars+N, origin icon, checkbox stopPropagation, SkillRowActions same ctx); skill-card.test.tsx |
| 9 | list: icon before name, category+labels columns toggleable | MET | NameCell SkillPresentationIcon; CategoryCell/LabelsCell; GRID_COLS + COLUMN_WIDTHS + FIXED_TRACKS_WIDTH 11 gaps; toolbar COLUMN_KEYS |
| 10 | create dialog category/icon + draft labels; detail edits persist; batch set-category | MET | create-skill-dialog.tsx (config only when non-default, label_ids only when chosen); skill-detail-page.tsx (presentation in draft/dirty/save, merge via writeSkillPresentationMeta); setSkillsCategory keeps icon+origin; tests in all three suites |
| 11 | label filter + search by label name | MET | skills-page.tsx rows memo: labels filter (any-of) + searchText OR label.name; page tests |
| 12 | category empty state with prefilled create | MET | CategoryEmptyState only when facets.categoryCounts[c]===0; setCreation({category}) -> initialPresentation; page test |
| 13 | four skills.json complete, parity | MET | node key diff: zh-Hans/ja/ko == en minus 19 en-only `_one` keys; zh copy uses straight quotes, lowercase skill, 「你」 |
| 14 | typecheck / TS tests / make test | MET (per main session) | I re-ran only core tsc + eslint on edited files + view-store test file |

### (b) Defects fixed (UNCOMMITTED)
1. packages/core/skills/stores/view-store.ts:198 — `merge` now spreads `...DEFAULTS` between `current` and the persisted payload. Before: a payload lacking `viewMode` (any pre-feature v0 payload) kept the PREVIOUS workspace's in-memory viewMode; with the other agent's new `version: 1` + `migrate`, zustand's hydrate calls setItem() right after merge, so workspace B's payload would be permanently stamped with workspace A's card/list choice. Verified in zustand 5.0.12 middleware.mjs (migrate -> merge -> set -> setItem when migrated).
2. packages/core/skills/stores/view-store.test.ts — two new cases: pure `merge` fallback ("falls back to the default, not the in-memory value...") and end-to-end persistence ("does not carry the previous workspace's view mode into a v0 payload..."). Red-state probe (temp copy without the spread) fails exactly these two; probe files removed.

Not mine but present uncommitted in the tree (another live check agent): schemas.ts SkillTemplateSchema category/icon `.catch(undefined)`, skill-template-client.test.ts, view-store.ts version/migrate for hiddenColumns, view-store.test.ts jsdom persistence suite, skill-list-actions.tsx kebab `focus-visible:opacity-100`, new untracked resource-label-picker.test.tsx and skill-category-chips.test.tsx. I messaged skillcat-check and skillcat-check-2 before editing view-store; no reply received.

### (c) Judgment calls needing a decision
1. label_ids validation runs BEFORE the create tx (h.Queries.GetLabel), attach runs inside it; AttachLabelToSkill SQL re-checks workspace + resource_type='skill' and ON CONFLICT DO NOTHING. A label deleted between check and tx silently drops that one attachment (never a bad row, never a 500). PRD R4 literally says "在创建事务内校验"; design.md §1.6 describes the shipped shape. Decide whether the literal PRD wording matters; moving GetLabel onto qtx is a ~10-line change.
2. Pre-existing, outside the range: SkillRowActions wrapper span stops only `click`, not `auxclick`; a middle click on the kebab background-tabs the row (useRowLink's doc asks for rowLinkInteractiveProps). The card inherits it. Not touched.
3. Pre-existing, outside the range: UpdateSkill builds UpdateSkillParams with parseUUID(id) from the URL param instead of the loaded skill.ID (CLAUDE.md UUID rule). Not touched.
4. List view under the new sidebar is its own @container: between ~672 and ~880px outer width the sidebar shows while the table collapses to the core column set. Honest to its own width; a product call, not a bug.
5. design.md §1.1 says the server stores presentation "as written"; shipped Go normalizes on every write (drops default icon / empty block) and the new spec file documents that. TS writer and Go normalizer agree. Only design.md is stale.
6. Runtime-local (daemon) skill import writes {origin} only — no frontmatter seeding on that path. PRD only names create/update/import(zip, URL)/template, so this is consistent, but it is the one import channel that ignores metadata.category/icon.
7. Sidebar is w-52; design said w-56. Cosmetic.
8. Users with a v0 hiddenColumns payload would see the new `labels` column by default; the other agent's uncommitted `version:1` migrate addresses this. If that edit is dropped, criterion 9's "default hidden" only holds for fresh state.

### (d) Commands run
- git diff --stat / git log 8053e8d71..a9d034dc6; git diff on each package path
- node one-liner: flatten + diff keys of the four skills.json
- grep sweeps: next/* & react-router-dom in views diff (none), hardcoded palette / text-sm ramp in added lines (only test fixtures' label.color hex), core importing UI libs (none), tags residue in tree (none outside deliberate legacy tests + spec note)
- cd packages/core && pnpm exec vitest run skills/stores/view-store.test.ts  -> exit 0, 12 passed
- red-state probe: vitest run on temp copy without DEFAULTS spread -> exit 1, 2 failed (expected), temp files removed
- cd packages/core && pnpm exec tsc --noEmit -> exit 0
- cd packages/core && pnpm exec eslint skills/stores/view-store.ts skills/stores/view-store.test.ts -> exit 0
- Not re-run (main session already green): pnpm typecheck (root), pnpm test (all), make test, Playwright

[coordination] team-lead: agent #3 is READ-ONLY from here; skillcat-check-2 owns all edits and has confirmed my merge fix + both tests are on disk and verified. skillcat-check (blue) reports zero edits and plans (1) add group/row to skill-card root, (2) card-view no-matches + tighten category empty state, (3) toolbar view-toggle hover. Checking those against the committed tree before replying.
[coordination] Verified skillcat-check's three planned fixes against the committed tree: (1) group/row already on the card root (skill-card.tsx:61); (2) card grid already renders page.no_matches.title (skill-card-grid.tsx:73-77) and the category empty state is already gated on facets.categoryCounts[c]===0 (skills-page.tsx:985), both covered by skills-page.test.tsx; (3) active view toggle keeps bg-background + shadow-sm + text-foreground under ghost hover (ghost = hover:bg-muted hover:text-foreground, overridden by explicit hover:bg-background), so the selected look survives hover fully. None of the three needs an edit. Replied to skillcat-check.
