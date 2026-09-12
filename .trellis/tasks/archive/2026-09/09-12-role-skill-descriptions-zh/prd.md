# Chinese role skill descriptions

## Request

The team works in Chinese and needs the seven built-in role skills' actual
discovery descriptions to be readable and maintainable in Simplified Chinese.
The current list has translated summaries but the editable descriptions still
come from English SKILL.md frontmatter.

## Acceptance criteria

- All seven `builtin_role_skills/*/SKILL.md` descriptions use concise Chinese
  that identifies the relevant situation and concrete result.
- Descriptions preserve the existing roles' delivery and authority boundaries,
  including proposed ADRs in task comments and approval before high-risk actions.
- Canonical names, other frontmatter, instruction bodies, supporting files and
  behavior versions remain unchanged.
- Chinese display descriptions agree with the canonical frontmatter. Default
  English descriptions already stored in workspaces retain localized display.
- Both languages remain searchable when only one locale is loaded. Customized
  descriptions and skills without verified built-in provenance remain intact.
- Template updates never silently overwrite an existing workspace copy. Any
  update to an existing copy must identify the workspace and preserve its body.
- Update the seven existing skills in the user's visible workspace, 毕宿五
  (`aldebaran-96xe`), including both stored metadata and SKILL.md frontmatter.
  Reopen each skill to verify the saved values and unchanged instruction body.
- Existing presentation, editor, picker and template checks pass; lint and
  typecheck cover the affected frontend package. No dependencies are added.

## Scope

The seven skills are release check, documentation change, security review,
architecture decision record, code review, requirement clarification and test
report. This is a language change, not a change to role permissions or execution
workflows. Relevant server and frontend spec notes must reflect the new canonical
description language.
