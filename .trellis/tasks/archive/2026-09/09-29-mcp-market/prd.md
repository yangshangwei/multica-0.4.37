# MCP market: B core, then C contextual entry

User approval: 2026-09-29, implement B using the existing three MCP templates, then add C's agent-context entry.

## Requirements
- Shared Web/Desktop MCP market and workspace views, searchable scenario categories, template details, explicit runtime requirements.
- Existing Chrome DevTools, Playwright, Sequential Thinking only. No new dependencies or credential templates.
- Guided template setup: choose template, name/configure, save to workspace, explicitly choose eligible agents, report assignment results or skip. Never preselect all agents.
- Trustworthy template identity survives instance rename; custom config replacement clears identity. Multiple uniquely named instances allowed. Historical servers stay custom/unknown.
- Reuse existing workspace CRUD, write-only summaries, agent assignment and enable/disable semantics. Existing custom form/JSON remains accessible.
- New template creation resolves config on the server from key/version, never trusts client attribution on arbitrary config.
- C: agent MCP tab opens the same catalog/setup, first offers existing workspace instances, carries target agent context, no implicit grant. Non-admin agent owners may assign existing instances but cannot create workspace instances.
- Distinguish saved, assigned, enabled and runtime verification; no claim of connected/running. Changes apply to subsequent execution. Indicate known name override conflicts.
- Loading/error/empty and partial failure recovery, keyboard/focus/narrow-layout support; en and zh-Hans copy.

## Acceptance
- All three templates complete save + explicit assignment; skipping leaves unassigned instance.
- Failed assignment can retry without duplicate create or repeating successful grants.
- Rename retains source; replace custom config clears it; arbitrary client config cannot claim built-in identity; wrong/stale template rejected.
- Members can browse; creation and assignment obey existing human permissions; malformed/old API payloads safe.
- Agent entry reuses same components and supports existing-instance reuse.
- Relevant TS/Go tests, typecheck, lint/static checks and browser happy-path/permission/error evidence pass.

## Out of scope
OAuth, connection probes, version upgrade UI, package pinning/verified badges, external registries, plugin authorization changes, mobile parity. These were expressly later increments in approved options.
