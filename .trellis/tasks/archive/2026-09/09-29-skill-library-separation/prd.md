# Separate workspace skills from templates

## Goal and approval
Distinguish workspace-owned skills from reusable templates. The user selected option 1, requested a Trellis task and execution, then reiterated "接着干". No product decision remains open.

## Background
`SkillLibraryCatalog` renders the same deployment templates above workspace skills and in the market. Deployment templates are operator-mounted starting content, not workspace skills or runtime built-ins.

## Requirements and acceptance criteria
- AC1: Retain two tabs, "Workspace skills" / "Skill templates" (Chinese: "工作区 skills" / "skill 模板"). Counts belong to their respective tabs; remove the redundant workspace count from the shared heading.
- AC2: The workspace panel contains its collection, filters and management actions, with no deployment shelf, template cards, or empty-deployment message.
- AC3: Provide a visible "From template" / "从模板创建" workspace action opening the template catalog. Keep ordinary creation. Preserve template-first initialization for successfully loaded empty workspaces and explicit tab choices.
- AC4: Retain catalog search, category filtering and all/deployment/builtin source filters and counts. Chinese deployment source is "部署提供". Loading counts stay unknown; failures are not empty results.
- AC5: Clarify template preview and related-workspace copy wording. Preserve named previews, multiple related links, independent-copy editor, drafts, creation feedback and focus restoration. Do not imply automatic installation, binding or synchronization.
- AC6: Web and Desktop share implementation. Keep the workspace collection mounted while hidden; narrow layouts must not overflow or conceal the template action.
- AC7: Remove shelf-only collapse state and obsolete copy; preserve persisted tab/source/category and workspace preferences.
- AC8: Update regression/browser tests and the discovery code spec; verify scoped tests, lint, typecheck and screenshots.

## Out of scope
No new APIs, dependencies, database changes, recommendation feeds, notifications, organization permissions, or mobile redesign. Do not rewrite user template descriptions or change binding semantics. Preserve unrelated working-tree changes.
