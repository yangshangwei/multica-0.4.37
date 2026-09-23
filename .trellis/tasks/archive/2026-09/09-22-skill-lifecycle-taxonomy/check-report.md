# Check Report: 09-22-skill-lifecycle-taxonomy

Independent check lane. Implementation was complete-but-uncommitted; this audit
verified the working-tree diff (27 modified files + 1 untracked e2e spec)
against `prd.md`, `design.md`, `implement.md`, `.trellis/spec/guides/*`,
root `CLAUDE.md`, and `apps/docs/content/docs/developers/conventions.zh.mdx`.

## Files Checked

All 27 modified files plus the untracked `e2e/skill-category-taxonomy.spec.ts`:

- Contract: `packages/core/skills/presentation.ts`(+test), `server/internal/skill/presentation.go`(+`presentation_test.go`, `presentation_parity_test.go` read)
- Presentation: `packages/ui/styles/tokens.css`, `packages/views/skills/lib/skill-presentation-icon.ts`, `packages/views/skills/hooks/use-skill-category-labels.ts`
- Surfaces (read, unmodified): `skill-category-sidebar.tsx`, `skill-category-chips.tsx`, `skill-presentation-fields.tsx`, `skill-list-toolbar.tsx`, `skills-page.tsx`, `hooks/use-skill-list-facets.ts`, `packages/core/skills/stores/view-store.ts`
- Bulk labels: `packages/views/skills/components/skill-list-actions.tsx`(+test), vs `packages/views/labels/resource-label-picker.tsx`
- i18n: `packages/views/locales/{en,zh-Hans,ja,ko}/skills.json`, `packages/views/locales/parity.test.ts`
- Built-ins: 4 × `server/internal/service/builtin_role_skills/*/SKILL.md`, `builtin_agent_templates_test.go`; `builtin_skills/multica-skill-importing/**` (source-map rule)
- Docs: `apps/docs/content/docs/skills.mdx`, `skills.zh.mdx`

## Issues Found and Fixed

### 1. Dead `_one` plural keys in zh-Hans/ja/ko — locale parity suite FAILING (fixed in this lane)

The implementation added `manage_labels_added_toast_one` /
`manage_labels_removed_toast_one` to zh-Hans, ja and ko. Those locales have no
CLDR `one` category, so the keys are dead weight, and
`packages/views/locales/parity.test.ts:96-112` (dead plural-key guard) fails:

```
FAIL locales/parity.test.ts > dead plural-key guard > zh-Hans ships no dead _one keys
FAIL locales/parity.test.ts > dead plural-key guard > ko ships no dead _one keys
FAIL locales/parity.test.ts > dead plural-key guard > ja ships no dead _one keys
+ [ "skills:actions.manage_labels_added_toast_one",
+   "skills:actions.manage_labels_removed_toast_one" ]
Test Files  1 failed | 442 passed (443) — Tests  3 failed | 5362 passed (5365)
```

This also violates the written convention
(`apps/docs/content/docs/developers/conventions.zh.mdx:223`: "中文不区分语法单复数，只填 `_other`").
The implementing context had reported the locale tests green — they were not.

Fix (landed): removed the four dead `_one` keys, keeping only `_other`, which
matches the existing `set_category_toast_other`-only convention in those files.
Post-fix the full views suite passes (see Verification).

### 2. Stale "the six categories" comment in `skill-category-sidebar.tsx` — fixed by the coordinator, verified here

An earlier session of this check lane claimed to fix the outdated doc comment
at `packages/views/skills/components/skill-category-sidebar.tsx:32-36` but the
edit never landed. The coordinator applied it; I verified only (no edit): the
comment now reads `"All" + every `SKILL_CATEGORIES` entry with counts`, and
`grep -rn "six categor" packages/views/ apps/docs/content/docs/` finds nothing.

## Issues Not Fixed (notes, none blocking)

1. **Screen-reader tri-state conflation** —
   `skill-list-actions.tsx:960` `aria-pressed={state === "all"}` exposes
   `aria-pressed="false"` for both "some" and "none"; the `Minus` indicator
   (lines 973-974) is visual-only with no text alternative. PRD R3's
   "三种状态可区分" is fully satisfied visually and by the
   `deriveSkillLabelState` unit matrix, but only two states are distinguishable
   to AT. This matches the codebase's existing bar (`AgentPickerRow`,
   lines 130-145, has the same shape), so it is a note, not a regression.
   Suggested follow-up: a visually-hidden state text / `aria-describedby`
   with one new key × 4 locales.

2. **`e2e/skill-category-taxonomy.spec.ts` mirrors taxonomy literals** — the
   untracked spec (outside the dispatched 26-file diff) hardcodes the
   eight-key order plus the EN and ZH display names. A future rename of any
   category label must touch this file too or the visual-acceptance spec
   silently drifts. Left as-is: it belongs to the visual-acceptance step
   `implement.md` marks pending, and it is out of this audit's file scope.

3. **`labelKeys.byResource` staleness after bulk label** —
   `ManageLabelsMenu.handlePick` invalidates only `workspaceKeys.skills`
   (line 892), per the dispatched requirement. The per-skill
   `labelKeys.byResource` cache can go stale until its next refetch (e.g.
   opening the detail picker). The catalog query needs no invalidation —
   attach/detach change no label row. Acceptable; noting for completeness.

## Verified clean (no findings)

- **TS/Go enum parity**: `presentation_parity_test.go:66-84` parses the TS
  literals and `reflect.DeepEqual`s them against the Go slices — order- and
  membership-exact (DeepEqual on slices is order-sensitive), plus a
  `DEFAULT_SKILL_CATEGORY` regex check. Real enforcement. Both sides list the
  eight keys in the PRD order; `go test ./internal/skill/...` passes.
- **API compatibility**: the bulk-label path reuses existing parsed endpoints —
  `listLabels` (`client.ts:3995-4000`, `parseWithFallback` +
  `ListLabelsResponseSchema`) and `attachLabelToResource` /
  `detachLabelFromResource` (`client.ts:4274-4295`, `parseWithFallback` +
  `ResourceLabelsResponseSchema`). No raw casts, no new endpoint. Unknown
  category values degrade via `readSkillPresentationMeta` → `other`
  (`presentation.ts:141`); facet buckets are seeded from `SKILL_CATEGORIES`
  (`use-skill-list-facets.ts:41-43`) so counts are never `undefined`;
  category sort uses `indexOf` arithmetic that stays numeric for any reader
  output (`skills-page.tsx:884-885`); the persisted-filter deep-merge guard in
  `view-store.ts` keeps `.length` reads safe. The category→label/tone/icon
  maps are `Record<SkillCategory, …>` — compile-time exhaustive, stronger than
  a runtime `default` branch. No new server booleans consumed.
- **Package boundaries**: diff-greps found no `react-dom`/`localStorage`/
  `process.env`/UI imports added to core, no `@multica/core` import added to
  ui, no `next/*`/`react-router-dom`/store creation added to views. The e2e
  spec's `process.env` use is app-test territory, allowed.
- **Exhaustiveness / single source**: every surface (sidebar 75, chips 49,
  toolbar 307, presentation-fields 54/78, list-actions 770, facets 42, sort
  884) iterates `SKILL_CATEGORIES`; no hardcoded six-item list survives
  anywhere (`grep '"research"'` over ts/tsx/go hits only the contract files,
  tests, and the e2e spec).
- **Bulk-label semantics**: `deriveSkillLabelState` (797-810) counts only
  `canEdit` rows; `setSkillsLabel` (823-859) derives add/remove
  deterministically, skips locked rows up front, aggregates per-item failure,
  and makes no request for rows already in the target state. No optimistic
  update anywhere in the path; single `workspaceKeys.skills` invalidation.
  Nine new tests cover all/some/none, permission skip and partial failure.
- **Reuse**: `ManageLabelsMenu` shares the catalog query key with
  `ResourceLabelPicker` (`labelListOptions(wsId, "skill")`, lazy via
  `enabled: open`), and copies its Popover/search/color-dot/row/empty-state
  markup. It deliberately calls the api functions directly instead of
  `useAttachResourceLabel`/`useDetachResourceLabel` — those invalidate
  per-call and cannot aggregate failures across a loop; `design.md` §4.2 and
  the `setSkillsCategory` precedent (line 679) define exactly this shape.
  Batch permission gating (`editable.length === 0` → disabled + tooltip)
  mirrors `SetCategoryMenu` (755-764), the correct batch-bar convention.
- **UI rules**: no `text-sm/base/lg` in the diff (only `text-body`/
  `text-caption`); no hardcoded Tailwind colors (the label dot uses
  `label.color` server data, same as the existing picker); no new
  persistent-selected state that hover could downgrade.
- **Built-ins & source-map rule**: ADR→design, code-review/security-review/
  test-report→quality in frontmatter; `TestRoleSkillTemplates_PresentationDefaults`
  covers all 8 role skills. `builtin_skills/multica-skill-importing/**` says
  only "a fixed category set" — no enumeration, so no source-map sync owed.
- **i18n & Chinese voice**: 4-locale key parity and (post-fix) no dead plurals;
  `skill` stays lowercase English with single spaces in zh; straight quotes
  around `\"{{name}}\"` match the doc rule and existing agents/runtimes
  strings; `…` in the search placeholder matches every sibling zh search
  placeholder; docs tables match the locale display names in both languages;
  zh docs use the Chinese label vocabulary per PRD.
- **Tokens**: `--skill-design` (330) and `--skill-quality` (150) added to
  theme alias + light + dark blocks, hues distinct from all six existing
  categories in both modes.
- **Test placement**: pure-helper matrices live beside their exports in
  `skill-list-actions.test.tsx` (jsdom file, needs DOM for the rest); the
  component tests keep wiring/happy-path only; only `@multica/core/api` and
  `sonner` are mocked; no store involved.

## Verification Results

| Command | Result |
| --- | --- |
| `pnpm --filter @multica/views test` (full suite, post-fix) | `Test Files 443 passed (443)`, `Tests 5365 passed (5365)`, EXIT=0 — includes the 172 locale-parity tests and the 3 previously failing dead-plural guards |
| `pnpm --filter @multica/views test -- locales/parity` (pre-fix, evidence of the finding) | `Tests 3 failed | 5362 passed (5365)`, EXIT=1 |
| `pnpm --filter @multica/core test -- skills` | `Test Files 159 passed (159)`, `Tests 2005 passed (2005)`, EXIT=0 |
| `pnpm typecheck` | `Tasks: 9 successful, 9 total`, EXIT=0 |
| `(cd server && go test ./internal/skill/...)` | `ok github.com/multica-ai/multica/server/internal/skill`, EXIT=0 |
| `(cd server && go test ./internal/service/... -run 'Presentation\|RoleSkill\|AgentRoleTemplate')` | `ok ... internal/skill`, `ok ... internal/service 0.883s`, EXIT=0 |

Not run in this lane (per dispatch scope): `pnpm lint`, `pnpm test` (full),
`make test`, Playwright — the targeted suites above cover the changed code;
the e2e visual acceptance remains open on `implement.md` by design.

## Summary

Checked 28 files against the PRD/design/spec set. Found 2 issues: one real
test-suite failure (dead `_one` plural keys in 3 locales, also a documented
convention violation — fixed here and re-verified green) and one stale comment
(fixed by the coordinator; verified here, not edited by this lane). Three
non-blocking notes recorded (AT tri-state conflation matching existing repo
precedent, e2e literal mirror, `byResource` cache staleness). All contract,
compatibility, boundary, exhaustiveness, reuse and i18n checks are clean.

**Verdict: pass-with-notes.**
