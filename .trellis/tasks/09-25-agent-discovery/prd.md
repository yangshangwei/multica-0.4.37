# 让用户按职责和小队找到智能体，并明确角色模板与成员关系

## Goal

Help workspace members choose an appropriate agent among many similar roles, distinguish role templates from saved agents, and retain the existing management and execution workflows.

## Background and authorization

The user supplied a screenshot with 22 agents, reviewed the business UX recommendations, confirmed the proposed treatment of built-in templates and existing instances, then explicitly requested a new Trellis task and implementation on 2026-09-25. This task executes that approved first iteration. The prior review is recorded in `.impeccable/critique/2026-09-25T12-07-43Z__packages-views-agents-components-agents-page-tsx.md`.

## Requirements

- R1: Keep saved agents as the default directory. Move the role-template catalog into the existing New agent / Use template flow, with clear template language.
- R2: In the role picker, distinguish templates with no active instances, one active instance, and multiple active instances. Prefer opening an existing instance while retaining an explicit way to create another. Match by template identity, not editable name.
- R3: Make existing members easier to identify by real template role and squad relationships, and provide task-oriented narrowing. Unknown/custom agents must remain findable and must not receive invented capabilities or permissions.
- R4: Use a concise default list and an explicit Display control while retaining customizable management columns and existing user preferences. Clarify the 30-day activity window.
- R5: Preserve selection, search, access checks, archive behavior, virtual scrolling, platform navigation, and existing task/chat entry points. Filtering within My agents must retain that scope.
- R6: Preserve saved agent names, descriptions, instructions, runtime settings, history, access, and many-to-many squad relationships. Browsing must not create, update or archive business entities.
- R7: Support Web and Desktop through shared packages and translate new UI in all four shipped locales.

## Acceptance Criteria

- [x] AC1: The default directory shows saved agents without a template inventory; the existing creation flow exposes clearly named role templates.
- [x] AC2: Template cards correctly handle zero, one, multiple, renamed, and archived instances without creating duplicates implicitly.
- [x] AC3: Users can distinguish coordinating roles from specialists and narrow members by business role/use and squad, including shared specialists and custom agents.
- [x] AC4: New preferences start with a concise list; existing custom columns persist. Display settings are discoverable and activity labels state their period.
- [x] AC5: Search and filters compose within the selected scope; permission-sensitive actions and multi-select still behave correctly.
- [x] AC6: Existing identities/configurations/history are unchanged; key flows have regression tests and browser evidence at ordinary and narrow desktop widths.
- [x] AC7: Scoped tests, typecheck, lint, static analysis and an independent review are recorded before completion.

## Out of Scope

Personal favorites, personal recent-use tracking, intelligent recommendations, a new preview workspace, new permission models, agent generation, role-template instruction rewrites, backend migrations, and automatic cleanup of inactive agents. These were later-stage ideas in the review, not part of this first identification/template iteration.
