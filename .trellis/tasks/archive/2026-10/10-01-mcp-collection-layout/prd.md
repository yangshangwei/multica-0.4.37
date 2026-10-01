# MCP collection layout

## Approval
The user approved the proposed design and implementation on 2026-10-01, including creation of a Trellis task if useful.

## Goal
Make MCP a full-width resource collection consistent with Agents and Skills, with cohesive Lucide icon tiles.

## Acceptance criteria
- The standalone MCP page uses the shared CollectionPageHeader, Server route icon, existing supporting description, and a right-aligned custom-add action. One page heading; no centered 768px settings wrapper.
- Shared configurations and MCP market have separately accurate counts. Unknown/loading data must not claim zero. Accessible tab names remain stable.
- Market content has 16px/24px gutters, a moderate-width search, existing category filters, and a visible result count.
- Template grid uses container width: one column narrow, two medium, three wide. Three existing templates fit one row on a wide page. Embedded settings/discovery remain usable.
- Cards match SkillTemplateCard: rounded-lg surface border, 16px title, 14px two-line description, secondary category, aligned visible configuration action. Full description remains accessible.
- Template icon tiles use a 40px rounded square and 20px Lucide icon from the shared registry: Bug and Workflow in existing blue palette; Brain in existing purple palette; meaningful fallback.
- Shared configuration management uses the available width; settings retain their appropriate embedded shell. Preserve creation, explicit assignment, rename, replacement and permission behavior.
- Verify wide/narrow layouts, light/dark, English/Chinese, and long text. Maintain keyboard focus contrast and 44px coarse-pointer targets.
- No new dependencies, backend/API changes, global stores or unrelated edits.

## Validation
Existing MCP unit suites before and after; focused regression for standalone header/action wiring and counts if needed. Views lint and typecheck; available static checks. Real browser verification using existing MCP E2E flows plus responsive/light/dark screenshots.
