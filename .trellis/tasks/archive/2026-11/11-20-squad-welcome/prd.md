# AI小队空状态首页

## Goal
When a workspace has no saved AI squads, explain how squads coordinate work and make creating the first squad obvious through a welcome page inspired by the user's supplied screenshot.

## Background and approved direction
The user supplied a light, spacious two-column screenshot, explicitly requested impeccable, and approved the proposed design before asking on 2026-11-20: “创建 Trellis 任务并进入规划，然后推进执行”. This approval covers planning and execution of the previously presented scope. No further product decision is open.
The existing SquadsPage renders a generic CollectionPageState for a successfully empty list. Existing workspace/template tabs and creation chooser are functional. The welcome page is a scoped addition to this established product, not a global design-system replacement.

## Requirements
- R1: Default workspace view shows the welcome page only after a successful squad list response containing zero squads. Loading, failed/unknown reads, filtered-out squads, and a non-empty list never become this welcome page. Explicit template URLs remain usable.
- R2: Use the supplied composition: generous neutral canvas, left introduction, black/semantic-primary creation button, secondary documentation link, and right light-blue collaboration demonstration with subtle borders/shadow. Keep the app shell and existing template navigation.
- R3: Heading: “让智能体组队，像团队一样干活”. Concise copy explains leader-led delegation by member roles. “新建AI小队” opens the existing creation chooser; “阅读文档” opens the application’s squad documentation, including offline/desktop routing.
- R4: Three manually selectable demonstrations explain delegation, roles, and result handoff. The initial slide shows task → leader → selected working member, with another member idle. Show a partial following slide on wide containers, dots and previous/next controls. Do not auto-advance. Clearly mark illustrative content as an example and do not run or create agents.
- R5: Localize all user-visible and accessible copy in English and Simplified Chinese; preserve product glossary and truthful collaboration semantics. Use existing icons, type/color tokens and components. Support light/dark themes, keyboard access, reduced motion, and narrow web/desktop windows.

## Acceptance criteria
- AC1 (R1): Tests distinguish successful-empty, pending, failed, populated, filtered-empty and template views.
- AC2 (R2/R5): Desktop capture resembles the screenshot’s composition; narrow 390px capture has no horizontal page overflow or hidden primary CTA; dark and English captures remain readable.
- AC3 (R3): Creation CTA navigates once through AppLink to newSquad; docs navigates through the shared adapter to docsPage(DOCS_SLUGS.squads). Template catalog and both creation methods remain reachable.
- AC4 (R4/R5): Keyboard/pointer controls switch all three slides, mark the active dot, respect ends, and keep inactive demonstration content out of assistive reading/focus. No automatic transitions.
- AC5 (R5): Scoped lint/typecheck/tests, locale parity and docs slug parity pass; complete one batched visual inspection and a bounded fix/confirmation pass if necessary.

## Out of scope
No backend/API/schema changes, new dependencies, native mobile work, agent execution, global token changes, or redesign of populated squad management, templates or creation forms.
