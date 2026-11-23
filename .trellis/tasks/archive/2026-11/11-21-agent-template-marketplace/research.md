# Repository evidence — 2026-11-21

## Inspected facts

| Evidence | Finding | Planning consequence |
| --- | --- | --- |
| `packages/views/marketplace/marketplace-page.tsx:18–42`, `:56–72` | Shared workspace-keyed page; URL type selects MCP/Skills/squad templates; visited panels remain mounted and receive active flags. | Append one type and reuse this lifecycle; no new route shell or global state. |
| `packages/core/paths/paths.ts:105–107` | `marketplaceTab()` currently accepts three literal types. | Add `agent-templates` to the existing helper and path tests. |
| `packages/views/agents/create/template-create-agent-page.tsx:36–85` | Role picker/configure share a route; configuration resolves only for a known template; Back currently replaces with the picker URL. | Market entry can reuse configuration but needs explicit source-aware Back behavior. |
| Same file `:89–145`, `:179–234` | Picker owns an agent-list query, gallery scroll and inline template card rendering. | Extract card presentation only; preserve the picker controller and avoid mounting it as a market panel. |
| Same file `:183–229`; `template-create-agent-page.test.tsx:154–211` | Instances match saved `template_key`, exclude archived, keep renamed instances, open rather than mutate; errors never mean zero instances. | Preserve these existing creation-picker contracts; market cards omit the instance dependency entirely. |
| Same file `:112–137`; tests `:299–368` | Gallery stores a custom scroll memento because configuration shares its pathname; restoring before both queries resolve clamps offsets. | Do not move or reset the creation picker's scroll controller during extraction. Market owns a distinct scroll root. |
| `packages/views/agents/create/use-role-templates.ts:8–25` | Template query is server/locale-scoped, not workspace-scoped, and already supports `enabled`. | Reuse the cache key and enabled option. Do not mutate shared query state when a market observer becomes inactive. |
| `packages/views/agents/create/choose-create-method-page.tsx:16–38` | Blank/template/AI are real links; `squad` passes from chooser to the selected method. | Keep the existing creation link and squad pipeline; the market is an additional discovery entrance. |
| `packages/views/agents/create/use-create-template-agent-submit.ts:51–111` | Server owns template instructions/skills/authority; save is awaited, then optional squad join; join failure warns without recreating; navigation lands on squad or agent. | Keep submit hook/request contract unchanged; source is client navigation data only. |
| Submit tests `:172–218`, `:238–255` | Cache is populated before navigation; squad success/join failure and no-runtime cases already covered. | Re-run canonical tests rather than recreating the matrix through a market DOM suite. |
| `packages/views/navigation/use-back-or-replace.ts:32–40`; `navigation/types.ts:49–57` | App Back uses history only when the platform proves it is safe; otherwise replaces with a fixed fallback. | Use the existing hook for market-origin configuration and a fixed market fallback, not arbitrary return URLs. |
| `apps/web/app/[workspaceSlug]/(dashboard)/agents/new/template/page.tsx:1–5`; `apps/desktop/src/renderer/src/routes.tsx:211` | Both platforms already mount `TemplateCreateAgentPage`. | Shared view changes cover the flow; no new platform routes are planned. |
| `.trellis/spec/views/frontend/agent-discovery.md:3–6`; section “Template picker instances — navigate, never mutate” | Current specification says templates live only in creation, and forbids template inventory inside the members directory. | Update the owner statement to include the market; preserve directory separation and existing-instance navigation rules. |
| `.trellis/spec/views/frontend/capability-market.md` | Missing/invalid type selects MCP; hidden portals/queries must deactivate; cross-route/desktop-tab disposal remains existing behavior. | Do not promise persisted configuration or Skills drafts after leaving the market route. |
| `packages/views/locales/{en,zh-Hans}/layout.json` → `marketplace`; `zh-Hans/agents.json:431–462` | Current market tagline enumerates three types; role copy combines existing-member opening with creation. | Add market-specific role description/action, update both taglines, leave picker-instance copy appropriate to its own surface. |
| `apps/docs/content/docs/developers/conventions.zh.mdx:106–156` | Only en/zh-Hans UI locales; Agent is 智能体, Squad is AI小队, skill labels follow the glossary. | New copy must follow existing terminology; no broad locale renaming in this task. |
| `e2e/agent-role-template.spec.ts:172–245`, `:248–275` | Existing E2E protects no-mutation member opening, explicit creation, responsive captures and picker/config back restoration. | Extend the market journey; keep the original journey as regression coverage. |
| `e2e/capability-market.spec.ts:5–111` | Existing market journey checks URL history, keyboard selection, unrelated query/hash and Skills draft retention. | Add fourth-category assertions without weakening the other domains. |
| `.trellis/spec/web/frontend/e2e-run-environment.md:3–32` | Playwright does not start servers; production Web and owned environment provenance are required. | Record owned API/Web origins and build revision for runtime acceptance; do not use dev cold compilation as product evidence. |

## Context and alternatives

- Related task: `.trellis/tasks/11-21-squad-marketplace/{prd,design,implement}.md`. Its market-category integration exists in the inspected working tree and recent commit `3e1b08d4c`; reuse that result instead of reopening its scope.
- Relevant prior decisions were available in the current thread, task artifacts, specs and regression tests. Following `trellis-session-insight`, no full conversation-history scan was necessary.
- The Impeccable context launcher did not complete in this session. Direct inspection found no root `PRODUCT.md` or `DESIGN.md`; this is an established surface, so existing UI/components and the user screenshot are the visual authority. No design-system bootstrap is planned.
- The checkout contains extensive unrelated staged, unstaged and untracked work. This planning turn writes only the new task directory plus Trellis's session task pointer created by `task.py create`. Implementation must re-read its exact targets before touching them and avoid bulk staging.

## Risks and non-claims

1. Returning from market configuration is a new navigation branch; source must be an exact allowlisted value, and explicit squad creation must retain priority if an unusual URL combines both contexts.
2. Hiding one observer with `enabled=false` does not cancel a shared request already started by another consumer; verify observer gating, not a blanket “no network anywhere” assertion.
3. The market route unmounts when opening configuration. Existing unrelated transient market editors keep their existing disposal behavior; no cross-route draft retention is added.
4. Template metadata is server-defined. Do not fill unavailable fields or invent new permission claims for the catalog.
5. Planning inspection is not product verification. No lint, typecheck, unit, browser or native Desktop run has been performed in this planning turn.
