# Revision verification

The three skill revisions and authorized workspace synchronization are complete.
The user deferred real regression with "先完成修订，稍后切到codex复测". This
record verifies implementation and local behavior, not improved model output.

## Delivered changes

- ADR requires source-backed facts and explicit unknowns, including claims in
  alternatives. Observer permissions, proposed status and human acceptance remain.
- Requirement clarification separates requirements from unverified advice. Its
  linked CSV reference distinguishes quoting from formula evaluation and requires
  target-software/open-import-save-reopen validation before safety claims.
- Documentation reporting requires final-file searches, accurate paths/lines,
  coverage limits and classification of legitimate retained references.
- Skill versions are ADR 3, requirement clarification 2 and documentation 2.
  All seven frontmatter blocks are unchanged; the four other bodies are unchanged.
- The reusable six-file evaluation harness preserves complete skill bundles,
  literal search evidence, account opt-in and failed execution artifacts.

## Verification results

| Check | Result | Evidence |
| --- | --- | --- |
| Go role-template/service checks and CSV reference delivery | Passed | Guarded `go test ./internal/service -run 'TestAgentRoleTemplates_\|TestRoleSkillTemplates_\|TestRequirementClarificationSkillIncludesCSVReference' -count=1`; this task's command output |
| Go formatting and static analysis | Passed | `gofmt` on changed Go files; `go vet ./internal/service`; this task's command output |
| Views type check | Passed | `corepack pnpm --filter @multica/views typecheck`; this task's command output |
| Skill presentation and template-flow tests | 61 passed in 2 files | `skills/lib/skill-presentation.test.ts` and `skills/components/create-skill-template-flow.test.tsx`; this task's Vitest output |
| Template handler catalog contract | Passed; test executed | `.omx/reports/skill-output-grounding-20260912/template-api-tests.log` |
| Harness on macOS | 21 passed, 1 filesystem-specific skip | `eval/harness-tests-macos-final.log` under the report directory |
| Harness in isolated Linux Python container | 22 passed, no skips | `eval/harness-tests-linux-container.log` and `eval/python-container-result.json` |
| Skill metadata and snapshot integrity | Passed | All seven frontmatters parse and match baseline; all eight skill files match the verified snapshot |
| Independent review | Approved | Skill/CSV/version/delivery review and harness review after both filesystem fixes |

The template handler check verifies the catalog response; it is not a new full
database integration run. The Linux container had no network or account mounts,
used read-only source mounts, and was removed after completion. Offline tests
never invoked a real agent CLI. Go reference delivery and harness regressions
were observed failing before their fixes and passing afterwards.

## Existing workspace synchronization

The authenticated desktop UI updated workspace 毕宿五 (`aldebaran-96xe`). All
three skills were reopened and their complete stored text matched the intended
patches: ADR 1,685 characters, documentation 1,343, requirements 1,278. The CSV
reference was added and reopened; its complete 1,076-character text matched the
source. Existing workspace-specific content and metadata were preserved. The
desktop was returned to the Skills list with no unsaved edits.

## Remaining validation

- The three attempted baseline invocations returned HTTP 402 before model tool
  use. They provide no completed baseline or semantic result. No real model was
  called after the deferral instruction.
- Revised original cases, new domain cases and negative controls are preserved
  for later Codex evaluation. The harness currently supports Claude only; Codex
  needs its own native adapter and independent semantic review.
- No claim is made that the three output-quality problems are empirically
  eliminated. No spreadsheet application was tested by the CSV reference work.
- The parent full-validation task stays in review: its previous 10 E2E failures
  and 2 uncollectable cases remain outside this revision.

Detailed local evidence: `.omx/reports/skill-output-grounding-20260912/SUMMARY.md`
and `summary.json`. Raw local reports are intentionally not committed.
