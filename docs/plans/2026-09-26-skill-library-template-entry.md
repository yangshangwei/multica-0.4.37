# Skill Library Template Discovery Implementation Plan

> Execution guidance: use the reviewed task plan below; implementation has not started. This planning request does not authorize a transition into product implementation.

**Goal:** Make built-in and deployment-provided skill templates easy to discover without competing with the workspace skill collection, and make existing-workspace relationships explicit.

**Architecture:** Reuse the shared skills page and existing template preview/draft dialog. Keep template content and workspace skills as separate query-backed datasets; derive display relationships without introducing backend state or permissions.

**Tech Stack:** React, TypeScript, TanStack Query, existing Base UI components and NavigationAdapter, i18next, Vitest and Playwright.

**Branch:** plan/skill-library-template-entry, based on the current local main. Existing local commits are preserved.

**Status:** Reviewed planning proposal; Architect and Critic approved iteration 2. Product implementation and UI verification have not run.

## Canonical artifacts

| Document | Purpose |
|---|---|
| [Requirements](../../.trellis/tasks/09-26-skill-library-template-entry/prd.md) | User outcomes, scope and measurable acceptance criteria |
| [Design and decision record](../../.trellis/tasks/09-26-skill-library-template-entry/design.md) | Alternatives, wireframes, interaction contracts, boundaries and rollback |
| [Implementation sequence](../../.trellis/tasks/09-26-skill-library-template-entry/implement.md) | Small ordered changes, ownership, commands and review gates |
| [Test specification](../../.trellis/tasks/09-26-skill-library-template-entry/test-spec.md) | Acceptance-to-test mapping, edge cases and visual verification |
| [Current-state evidence](../../.trellis/tasks/09-26-skill-library-template-entry/research/current-state.md) | Source anchors, current behavior and evidence limits |
| [Verification environment](../../.trellis/tasks/09-26-skill-library-template-entry/research/verification-map.md) | Exact test commands and environment prerequisites |
| [Review record](../../.trellis/tasks/09-26-skill-library-template-entry/review.md) | Independent architecture/critic decisions and adopted revisions |

The task directory is the source of truth. This file is an entry point, not a second specification. Runtime .omx planning aliases point to the same documents.
