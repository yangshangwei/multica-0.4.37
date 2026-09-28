# 小阿孚 product rename design

## Boundaries and content

The display name is product content; `system_key=mika` is stable identity. Keep the bootstrap endpoint, locking keys, code symbols, UUID references and client identity checks intact. Use existing constants, locale values and prompt bodies without a new naming abstraction or dependency.

Web and desktop share `packages/views`; mobile already renders `agent.name`. Work in `/Volumes/artisan/code/2026/multica-afu-rename` to isolate active skill-market edits in the main checkout.

- `MikaDefaultName` becomes `小阿孚` in both creation languages.
- Default descriptions become name-neutral so later owner renames remain coherent. English: `Your workspace Chief of Staff. Turns goals into issues, coordinates agents, and helps build reusable workflows.` Chinese: `你的工作区 Chief of Staff。把目标转化为任务、协调智能体，并帮你建立可复用的工作流。`
- Keep `{{AGENT_NAME}}` in the system prompt; remove the parenthesized legacy display name. The onboarding skill refers to the built-in assistant or itself rather than a literal name for CLI resolution.
- Keep localization keys. Update four locale files, the two existing onboarding templates and the bootstrap error. Default titles become `Getting started with 小阿孚` and `开始使用小阿孚`.
- Historical release notes and references to internal code names remain historical. Current product instructions use the new display name or the agent's current name.

## Stored defaults

Add data-only migration `457_builtin_agent_display_name` with up/down files, using existing `agent` and `chat_session` columns and no new schema objects.

1. Consider only `system_key='mika' AND name='Mika'` for renaming.
2. Rename only when no other agent, including archived rows, owns `小阿孚` in that workspace. Report skipped agent/workspace IDs with a PostgreSQL notice; the migration CLI forwards notices to its structured log through its connection configuration. Catch concurrent unique-name conflicts per row so other candidates continue.
3. For successfully renamed or already `小阿孚` built-ins, replace only exact old English/Chinese default descriptions with the new name-neutral text. Preserve custom-named and conflicted agents completely, including descriptions and timestamps; preserve all custom descriptions and instructions.
4. For built-in agents now named `小阿孚`, update only the two exact old onboarding session titles: `Getting started with Mika` and `和 Mika 开始`. Preserve custom titles, messages, hidden kickoff rows and old issues. The similar `开始使用 Mika` text in old tests is an arbitrary session title, not the current shipped default.
5. Update timestamps only when content changes; replay must leave already updated rows untouched. Use an atomic operation where practical.

The down migration deliberately retains editable content changes (`SELECT 1` with an explanation). A blind reverse rename cannot distinguish migrated rows from later user choices. Old code already supports arbitrary display names, so code rollback remains compatible. Restore a selected name through the ordinary rename interface after inspecting its current state if needed; do not introduce a permanent journal table for this content change.

Resolve skipped collisions explicitly and rerun the idempotent update after review. The schema ledger proves migration ordering, not that every conflicting row was renamed.

## Compatibility and verification

Retain `/api/agents/mika` and UUID relations. External scripts using `--agent Mika` or `--leader Mika` by name must adopt the new name or a UUID. Desktop clients require a new build for bundled copy. Historical messages may still contain Mika.

Use current-name, idempotency and custom-content handler tests. Add migration behavior tests in an isolated PostgreSQL schema for old defaults, custom content, ordinary Mika agents, active/archived collisions, default/custom titles, history preservation, repeated execution and safe code rollback. Update existing meaningful frontend assertions instead of testing every literal string. Run affected tests, lint, type checks and Go vet. Do not invoke ambient authenticated agent CLIs.
