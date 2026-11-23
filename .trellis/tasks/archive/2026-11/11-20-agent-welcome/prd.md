# Agent welcome empty state

## Goal

Help a workspace with no agents understand their value and create its first AI teammate. Match the supplied screenshot: a spacious introduction and three-step checklist on the left, illustrated feature cards and manual carousel controls on the right.

## Authorization and background

- The user requested this empty-state page and explicitly said existing agents should open the list directly.
- After the proposed layout, successful-empty gating, responsive layout, themes and locales were summarized, the user replied: “请建一个任务，规划并推进”. This authorizes creation, planning and implementation of that summarized scope.
- `packages/views/agents/components/agents-page.tsx` owns the shared Web/Desktop directory and currently renders a small `CollectionPageState` for an empty raw agent array. Filters, archived scope and auxiliary-query readiness are separate.
- Creation, documentation and runtime routes already exist. The page already queries runtimes.

## Requirements

- R1: Render the welcome only after the agent query succeeds with zero records. Preserve loading and retryable error states. Search, ownership, category, archived and other filters must never trigger the welcome when the underlying directory contains agents, including archived records.
- R2: Use the reference's Chinese headline “招第一位 AI 队友”, agent badge, descriptive copy, primary creation link, secondary documentation link, and three steps: connect a runtime, create an agent, assign an issue. Step one links to runtime management; runtime readiness comes from successfully fetched online runtimes. Never infer successful setup from unknown data.
- R3: Provide three illustrative cards: describe responsibilities with runtime/provider/model chips; assign a task to an agent; review its delivered comment. Include previous/next and position controls, a faded next-card preview where space permits, and clear illustrative labeling. No real sample records are created.
- R4: Ordinary query updates after creating or receiving the first agent remove the welcome and reveal the existing list. Removing all records reveals it again. No persisted onboarding flag or separate route.
- R5: Preserve the approval queue entry and ordinary directory actions. Reuse shared routing, primitives, semantic tokens, icon system and bilingual locale files.
- R6: Wide screens use two columns. Narrow screens stack the intro and example without document overflow. All controls support keyboard access, visible focus and practical touch targets. Respect reduced motion; no automatic cycling.

## Acceptance criteria

- AC1/R1: Successful empty data renders the heading; pending, failed and filtered-empty data do not. Auxiliary activity/presence/template queries cannot delay a successfully empty page.
- AC2/R2: Creation, docs and runtime links resolve under the current workspace. Runtime completion only reflects a successful online-runtime observation.
- AC3/R3,R6: All three slides are selectable by keyboard/click; endpoints disable appropriately, hidden slides are inert, and a live status announces the selected slide.
- AC4/R4: Changing query data from empty to one agent replaces the welcome with the list without reloading. An archived-only directory remains a list.
- AC5/R5,R6: English/Chinese and light/dark render at desktop, tablet and narrow widths with no horizontal page overflow; existing list tests and affected type/lint checks pass.
- AC6/R3,R6: Capture and inspect actual rendered screenshots against the reference and retain the visual verdict with evidence.

## Out of scope

Backend changes, endpoints/dependencies, native mobile UI, creation/approval/runtime behavior changes, template catalogs inside the directory, automatic daemon launches, and unrelated squad/automation work.

## Planning convergence

The approved summary and reference resolve product decisions. Runtime progress uses existing server observations; no blocking questions remain. Design and execution artifacts preserve that scope.
