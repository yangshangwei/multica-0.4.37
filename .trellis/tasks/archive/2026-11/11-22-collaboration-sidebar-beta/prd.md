# Collaboration sidebar: default collapse and quiet Beta notice

## Goal

Reduce the unfinished collaboration feature's visual weight in the shared Web/Desktop sidebar while keeping it discoverable and usable.

## Background and approval

The current collaboration directory precedes Workspace and permanently shows Current handling, Participating and Collaboration notifications (`packages/views/layout/app-sidebar.tsx:618`). Its heading shows Cross-workspace but does not communicate Beta maturity.

The preceding user-facing proposal specified default collapse, a neutral Beta marker, automatic reveal on collaboration destinations, separate resume/disclosure actions, contextual explanation, retained unread feedback and a Beta marker on creation. The user approved that concrete proposal on 2026-11-22: “直接按照 ‘默认折叠 + 轻量 Beta’方 修改”. This approves task registration and implementation of the presented scope; there are no unresolved product decisions.

## Requirements

- R1: Keep the collaboration entry above Workspace. In a workspace, fresh preferences show a compact collapsed heading instead of three permanently expanded entries.
- R2: The heading label resumes the previous collaboration destination. A separate, accessibly named arrow expands/collapses its directory without navigation. Preserve explicit Current handling / Participating links and selected-list highlighting on detail destinations.
- R3: Reveal the directory automatically on a collaboration destination. Allow manual collapse there, and retain manual disclosure choices through the existing core sidebar preference mechanism. Automatic route reveal must not become a saved preference.
- R4: Show a small neutral Beta marker next to Collaboration space and next to New collaboration task. Use existing semantic tokens and typography; keep ordinary hover, selected and focus affordances.
- R5: Replace the permanent Cross-workspace heading suffix with an explanation reachable through hover and keyboard focus: “用于跨工作空间协作，目前为 Beta，部分功能仍在完善。” Include the same quiet explanation in the collaboration page header so touch users can discover it.
- R6: Keep authoritative collaboration unread feedback available when collapsed; preserve notification access, read/archive actions and exact links. Unknown/unavailable counts must not be represented as zero.
- R7: Keep English and Simplified Chinese copy aligned. The shared implementation must work on Web/Desktop and compact navigation, including keyboard and coarse-pointer controls.

## Acceptance criteria

- [x] AC1 (R1, R4): A fresh workspace sidebar shows a single compact collaboration heading with a neutral Beta marker and no visible child links.
- [x] AC2 (R2, R3): Disclosure works by pointer and keyboard without resuming/navigation; clicking the title still invokes the existing exact destination resume behavior.
- [x] AC3 (R3): Entering a collaboration list/detail reveals the relevant navigation; manual collapse and later route changes follow existing group behavior, and persisted legacy preferences acquire the collapsed default safely.
- [x] AC4 (R4, R5, R7): Both locales show Beta on the directory and creation entry, a focus/hover explanation, and a readable quiet page-header explanation without overflow.
- [x] AC5 (R6): Collapsed unread feedback follows the protected notification query; notifications remain usable after disclosure changes.
- [x] AC6 (R2, R7): Account navigation remains available with zero/pending/unavailable workspaces, selected states remain legible on hover, and compact query navigation dismisses the sheet as before.
- [x] AC7: Focused behavior tests, affected-package type/lint checks, locale parity and a bounded visual inspection pass; record exact evidence and verification limits.

## Out of scope

Backend collaboration functionality, access rules, routes, account/workspace ownership, notification semantics, broad sidebar restyling, new dependencies, mobile app UI and changes to existing user drafts.

## Risks and constraints

Keep the title and disclosure as sibling controls to avoid nested buttons. Collapsing UI must not stop unread observations or retain a hidden open portal. Preserve the existing account/workspace resume adapters and old stored group preferences. This is a small presentation/interaction task; PRD-only planning is sufficient.
