# Embedded documentation drift

The fresh candidate `cb192014b` fails the CI-equivalent documentation consistency check:

```sh
pnpm generate:docs-bundle
git diff --exit-code -- server/internal/docs/content
```

The generator succeeds and produces four changed `pages/*.json` bodies: `collaboration`, `desktop-app`, `feature-overview` and `tutorial`. Their source MDX already contains newer sidebar/navigation wording; the embedded server bundle was not regenerated after those source changes. The reserved-slug generator and its output are unchanged.

The smallest repair is to retain the canonical generator's four outputs in the isolated worktree. No source documentation, generator behavior, API or UI logic is changed. This is within the task's CI contract-repair scope. Regenerating a second time must leave the output byte-identical, and the final diff must contain only those canonical outputs.

Command logs: `.omx/private/qa-work/main-readiness-20261123/auxiliary/docs-bundle.log`; result summary: `auxiliary/results.json`. Raw output stays in the task-private runtime. Backend verification and later API builds must bind the generated output hashes.
