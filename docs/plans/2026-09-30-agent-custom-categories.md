# Custom Agent Categories Implementation Plan

**Goal:** Let people assign their own category when creating or editing an agent and find agents by category.

**Architecture:** Store one optional category name on each agent using the existing create/update/read APIs and workspace permissions. Existing names are suggested from visible workspace agents. Category is independent of immutable role-template provenance. Reuse shared creation drafts, settings autosave, Query caches and directory preferences across web and desktop.

**Tech Stack:** Go, PostgreSQL/sqlc, React, TanStack Query, Zustand, Vitest.

## Contract and decisions

- `category` is a string; empty means uncategorized. Trim surrounding whitespace; reject more than 50 Unicode code points and control characters at write boundaries. Omitted PATCH fields preserve the current value; an empty string clears it.
- Existing agents remain uncategorized. Older/malformed responses normalize the optional category safely. No role-template rewriting, runtime or autonomy effects.
- One category per agent. Users can type a new name or reuse an existing visible name. This avoids adding a separate taxonomy administration surface for a simple grouping feature. Multi-label tagging and hierarchical categories are outside this request.
- Creation (manual, template, builder, duplicate), persisted drafts, settings and list discovery share the contract. Duplicates preserve the category.
- Directory supports category badges, searching, category filtering, and category/role/none grouping. Filters compose with Mine/All/Archived and existing filters. Blank category is displayed as Uncategorized / 未分类.
- Honor existing edit permissions, autosave failure feedback, keyboard/IME behavior and long text constraints.

## Execution

1. Backend: add failing API regression tests; add reversible category-column migration, sqlc query fields and generated code; wire create/template/update/response and validation. Verify clearing, omission, Unicode length, permissions, persistence and unchanged provenance. Update built-in creating-agent documentation.
2. Shared core: add response/request/draft category fields and malformed-response tests; preserve category through stored drafts, duplicates and requests. Add category filter/group preference and pure category helpers/tests. Run targeted core tests and typecheck.
3. Shared views: add reusable category input with existing-name suggestions to configuration and settings; add category labels/filter/group UI and English/Chinese copy. Add meaningful interaction and directory regression tests. Run targeted view tests, lint and typecheck.
4. Integration: review all changed layers, run wider relevant tests and static checks, verify browser flow and responsive screenshots, persist visual verdict, update agent discovery spec, and report evidence and limitations.

## Review and rollback

Keep pre-existing sidebar/layout edits untouched. No new dependencies. The additive migration has a matching down migration; clients defensively parse absent categories. Review generated sqlc diffs for unrelated churn. Do not deploy or push.

## Follow-up: visible category choices and creation guidance

The native datalist does not provide a dependable visible dropdown, especially
when a workspace has no categories. Replace it with the already-installed Base
UI Combobox, using the existing input styling and menu tokens. Keep free typing,
add an explicit arrow trigger, selectable existing names and an Uncategorized
option, and show a "Use new category" option for valid unmatched input. Empty,
loading and failed suggestions must each provide useful guidance without
blocking manual entry. Distinguish automatic settings saves from creation drafts.

Validation: first add failing interaction tests for opening, selecting, keyboard
navigation, new/empty guidance and disabled controls; then implement and run
category/inspector/create regressions, lint and typecheck. Update real-browser
tests to click visible options rather than inspect datalist markup. Verify both
desktop and narrow dropdown screenshots in the isolated production environment.

## Follow-up: default category suggestions

Always put General-purpose agents, Specialists and Coordinators at the start of
the category dropdown, using the existing localized role labels as suggested
category names. Merge visible saved category names after these defaults and
deduplicate exact names. Presets remain available while custom suggestions load
or fail. They are ordinary category strings, not role-template or permission
changes. Preserve typing, new-name actions and Uncategorized; update empty-state
copy to describe the lack of custom categories rather than an empty dropdown.
Verify default ordering, selection, deduplication and coexistence with custom
categories in component and production-browser tests.

## Fix: unify category grouping, filters and counts

Confirmed reproduction: a template-free OpenCode saved as 专业角色 still appears
under 通用智能体 and is excluded by the top 专业角色 filter. The default view
uses template provenance while the edited category is only a badge/separate mode.

Resolution:
- Resolve one effective directory category: nonblank manual category first;
  otherwise use the template's role kind, falling back to general-purpose for
  agents without a recognized template. Preserve provenance and permissions.
- Normalize the English/Chinese preset names to `preset:other`,
  `preset:specialist`, `preset:coordinator`; custom names use `custom:<name>`.
  The API continues storing names. This keeps saved default selections usable
  across UI languages and avoids key collisions with arbitrary custom names.
- Keep only Category/None grouping and one `categories` filter dimension. Top
  preset chips and the category dropdown operate the same keys/counts. Custom
  names create their own group. Template provenance remains unchanged and searchable.
- Migrate persisted version-0 role grouping to category, preserve explicit None,
  sort/scope/columns/squads and other filters. Convert named category filters;
  otherwise convert legacy role filters. Blank category filters are cleared
  because a cleared category now means automatic classification.
- Template metadata is required only for rows needing fallback. Keep explicitly
  categorized rows usable during metadata errors; never assert false fallback
  classification or empty results before required metadata resolves.
- Rename the clear option/helper to use the default category, reflecting actual
  fallback behavior. No database changes or new dependencies.

Validation: tests first for manual preset/custom overrides, unset fallback,
cross-language presets, preference migration, counts/filter/group consistency
and metadata errors. Production browser regression edits OpenCode, returns to
the default directory and verifies its group, top-chip inclusion and counts,
then changes to a custom category and clears it. Preserve unrelated workspace edits.
