# Skill library separation implementation plan

**Goal:** Separate workspace management from template discovery using approved option 1.

**Architecture:** Keep the existing shared catalog, query and copy editor. Replace the workspace deployment shelf with a direct catalog action, preserving filters, persistence, virtualizer lifetime and creation focus.

**Tech stack:** React, TypeScript, TanStack Query, Zustand, Base UI, Vitest and Playwright.

Requirements, design, cleanup sequence, ownership and checks are in the [completed Trellis task](../../.trellis/tasks/archive/2026-09/09-29-skill-library-separation/implement.md). See its [verification record](../../.trellis/tasks/archive/2026-09/09-29-skill-library-separation/verification.md) for test and browser evidence.
