# Clarify skill library template discovery and workspace copies

Status: planning only; not activated. Branch: `plan/skill-library-template-entry`.
Design: [design.md](design.md). Execution: [implement.md](implement.md). Verification: [test-spec.md](test-spec.md).

## Problem and outcome

The skill library must primarily help people find and manage skills already in the current workspace. Templates are read-only starting points: browsing or previewing them creates nothing; explicitly adopting one starts an editable draft; final creation saves an independent workspace skill.

The current skills-specific catalog occupies a separate expandable collection above the management toolbar and opens an unspecified first matching workspace instance. The existing creation picker already provides search, source groups, preview, draft editing, and recovery. Replace the inline catalog with compact discovery and reuse that picker. Evidence: [current-state.md](research/current-state.md), `skills-page.tsx:1009`, `builtin-skill-catalog.tsx:24`, `template-skill-create-panel.tsx:31` (all component paths under `packages/views/skills/components/`).

## Users and scope

Workspace members can browse available templates, inspect related workspace skills, or prepare a new skill using existing permissions. Owners, admins, creators, and other members retain their current edit/delete/assignment rights. Relatedness is informational metadata, never an authorization or integrity guarantee.

In scope: shared Web/Desktop entry, accurate source counts, existing template browser, concise display summaries, explicit preview/draft phases, zero/one/many related skills, query states, keyboard/focus behavior, and all four locales.

Out of scope: API/schema/backend changes; new dependencies or stores; mobile implementation; template editing/upgrades; automatic installation/assignment; broad card density or source-facet redesign; generic catalog changes; new routes or persistent preferences. Keep the fixed-height card grid contract unchanged.

## Requirements and acceptance criteria

| ID | Requirement | Concrete acceptance criteria |
| --- | --- | --- |
| R1 | Workspace management remains the main page task. | AC1: replace the skills inline catalog with one compact `Skill 模板` entry; no template rows expand on the page. At 1280×720 it occupies at most 64 px above the existing toolbar, excluding the page header. Search, facets, counts, list/card choice, and category-empty behavior retain their existing scope. |
| R2 | Counts describe the correct collection and source. | AC2: workspace header counts only workspace skills. Entry total equals valid platform + deployment templates from the unfiltered catalog; mixed data never labels the total as platform built-ins. Search changes picker result counts only. Loading/error without data never displays a definitive zero. |
| R3 | Browsing has a direct, explicit entry. | AC3: `浏览模板` opens the existing picker without passing a magic first name and without a create request. Existing `新建 skill` still opens the method chooser; its template option reaches the same picker. Initial source/preview derive from loaded visible data. |
| R4 | Preview, adoption, and creation are distinct. | AC4: picker shows `模板预览` and `使用模板`; adoption opens `创建副本` with editable fields and final `创建 skill`. Preview/search/source changes do not seed or replace a draft. Only final creation writes; source content and existing workspace instances remain unchanged. |
| R5 | Related workspace instances are represented honestly. | AC5: preview shows verified zero, one named link, or an exact count plus every named link for many. Match known canonical name + verified built-in presentation, or exact `config.template_source.name`. A manual same-name skill without either signal is excluded; renamed copies with matching source remain included. Do not describe official instances as independent copies. |
| R6 | Short summaries are display copy only. | AC6: known, uncustomized template defaults use a distinct short-summary locale field in picker rows. Full descriptions remain the source for preview purposes, adoption, existing search text, and payloads. Customized/unknown/deployment descriptions fall back to supplied copy. Existing workspace descriptions, source-sync defaults, YAML, instruction bodies, and supporting files are not rewritten. |
| R7 | Navigation cannot bypass creation safeguards. | AC7: all Desktop related-link adapter actions and Web in-place navigation pass through the root busy/dirty/unconfirmed guard. Cancel preserves the draft and focus; acceptance resets/closes, then executes the original snapshotted path/title/push-or-tab intent exactly once. This includes background/middle opens to an already-open Desktop tab, which can activate and unmount the source. No `openCandidate`, `onCreated`, or creation-success toast. Web-native modified links retain current AppLink behavior. |
| R8 | Query transitions preserve the dialog session. | AC8: cold loading, no-data error/retry, cached refresh/error, successful empty catalog, no search matches, deployment-only data, and workspace-skill loading/error remain distinct. One stable dialog host outside conditional page bodies preserves edited draft, unconfirmed submission, and pending discard across same-workspace cached query failure/retry. Workspace change separately resets selection, pending navigation/discard and draft; stale actions and late responses never navigate the new workspace. |
| R9 | New interactions work with keyboard and narrow layouts. | AC9: entry, source controls, rows, related links, back, adopt, and create have accessible names and visible focus. Narrow list→preview→back restores the selected row or search focus. At 375×667 and 360×800, no horizontal page/dialog overflow, clipped actions, or keyboard-inaccessible content; entry remains within 112 px in its normal loaded state. |
| R10 | Shared boundaries and localization remain intact. | AC10: changes stay within shared views/locales and their tests, documented E2E updates, and corresponding localization guidance. No common catalog behavior, core mutation contract, app-specific router import, source instructions, permissions, or dependency change. New keys exist and render with single-locale providers for `en`, `zh-Hans`, `ja`, and `ko`. |

## Product copy examples

- Entry: `Skill 模板` · `17 个模板 · 平台内置 15 · 部署提供 2` · `浏览模板`.
- Entry help: `预览模板，再创建独立的工作区 skill。`
- Preview: `模板预览` · `工作区已有 2 个相关 skill` · `使用模板`.
- Confirmed zero: `工作区暂无相关 skill`.
- Draft: `创建副本` · `基于模板：代码审查` · `创建 skill`.
- Cold failure: `无法加载模板` + `重试`; cached failure: `模板未能刷新` + `重试` while retaining cached content.

These are UI examples, not source-description replacements. Follow `apps/docs/content/docs/developers/conventions{,.zh}.mdx`: lowercase `skill` in Chinese prose, natural action verbs, spaces around English terms, existing punctuation/plural conventions.

## Success evidence and remaining limits

Each AC maps to a canonical test or visual check in [test-spec.md](test-spec.md). Record production Web screenshots, keyboard results, Web/Desktop navigation-adapter evidence, and service/build provenance during implementation. A static screenshot or detector result alone does not prove usability.

The prior review could not run a live browser; no application tests, contrast audit, or responsive interaction checks were performed. This planning deliverable does not claim implementation or test success. Iteration 2 incorporates the Architect/Critic lifecycle findings; see the review-adoption note in design. Broad card accessibility/density and source taxonomy remain explicit follow-ups.
