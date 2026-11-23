# Skills Welcome Commit Plan

Status: approved when the user resumed the task on 2026-11-21. The feature
commit is `86b910dfa0fd7e5593739fc5defee2bf99831c35`; its 12 scoped paths match
the reviewed contents. Archive and journal records use separate commits.

## 1. Feature Work

Intent line:

```text
feat(skills): explain reusable team methods before first creation
```

Stage only these task-owned code and specification paths:

- `packages/views/skills/components/skill-welcome.tsx`
- `packages/views/skills/components/skill-welcome.module.css`
- `packages/views/skills/components/skill-welcome.module.css.d.ts`
- `packages/views/skills/components/skill-welcome.test.tsx`
- `packages/views/skills/components/skills-page.tsx`
- `packages/views/skills/components/skills-page.test.tsx`
- `packages/views/locales/en/skills.json`
- `packages/views/locales/zh-Hans/skills.json`
- `e2e/skill-welcome.spec.ts`
- `e2e/skill-welcome-desktop.spec.ts`
- `.trellis/spec/views/frontend/skill-welcome.md`
- `.trellis/spec/views/frontend/index.md` — only the new Skills Welcome row.
  The unrelated Triage Welcome row was already in HEAD when this task resumed.

Proposed Lore body and trailers:

```text
Introduce reusable workspace skills where a member first encounters an empty
directory, and connect that explanation to the existing creation and docs flows.
Successful defined raw query data controls visibility; populated and filtered
directories retain their existing behavior. Static manual examples demonstrate
the five actual creation methods, explicit assignment and subsequent-run reuse.

Constraint: Preserve shared Web/Desktop routes, imports, recovery and API compatibility
Rejected: Separate onboarding route or persisted dismissal | Saved collection data already determines the appropriate entry state
Confidence: high
Scope-risk: narrow
Reversibility: clean
Directive: Keep templates separate from saved instances and require successful defined raw data before showing the welcome
Tested: 474 Skills and locale tests; scoped ESLint; views, Web and Desktop typechecks; isolated production Web and Desktop builds; complete Web welcome, existing library and native renderer journeys; reference-based visual review
Not-tested: Packaged Electron Main, installers and native mobile UI
```

## 2. Task Bookkeeping

After the feature commit, archive only `skills-welcome` through the Trellis
workflow, retaining its requirements, approved design, implementation plan,
reference and acceptance/review reports. This produces a separate task archive
commit. Do not archive any other active task.

## 3. Session Record

Record this task's accepted outcome and the feature commit in the developer
journal, as a separate bookkeeping commit. Include only this session's journal
and index changes; do not absorb pre-existing journal edits from other tasks.

## Excluded State

The checkout contains extensive unrelated work. The exact dirty snapshot and
pre-existing staged-path observation are retained at
`artifacts/qa/skills-welcome-20261121/commit-scope.json`. None of those unrelated
paths belongs in these commits. The shared spec-index row needs scoped staging.

Generated QA candidates, logs, private environment configuration, screenshots
under `.impeccable/review/skills-welcome`, and visual runtime state under
`.omx/state/skills-welcome` remain local evidence rather than feature code.
The task's supplied reference and small Markdown reports remain with the task.
Do not push, amend or merge as part of this plan.
