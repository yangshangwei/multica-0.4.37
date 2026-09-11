# Localize built-in role skill presentation

## User request

Localize the seven built-in role skills with Chinese names and purpose descriptions, consistent presentation in lists, detail pages, and selectors, and Chinese/English search. The user approved this scope after reviewing the alternatives.

## Acceptance criteria

- Chinese UI displays 发布检查, 文档更新, 安全审查, 架构决策记录, 代码审查, 需求澄清, and 测试报告 for the corresponding built-in role skills.
- Each built-in skill has a concise Chinese purpose description.
- Workspace lists, detail headings, agent assignment lists, and skill selectors resolve the same presentation.
- Search matches Chinese names/descriptions and English identifiers/descriptions, independent of the current UI language.
- Switching UI language updates presentation and search results without mutating server data.
- Built-in provenance and unchanged canonical names are required for localization; unrelated skills with the same name and user-renamed skills retain their own presentation.
- Custom descriptions remain visible. Editable name/description fields, SKILL.md, assignment UUIDs, invocation identifiers, and runtime behavior remain unchanged.
- Existing workspace copies benefit immediately from the presentation change; no database migration or content replacement is required.

## Constraints

- No new dependencies.
- Shared web/desktop UI belongs in packages/views.
- Use the existing i18n resources and preserve locale key parity.
- Preserve the current layout, navigation, permission checks, and editor semantics.

## Out of scope

Translating executable skill bodies, upgrading workspace copies, changing role instructions, or renaming stored skills.
