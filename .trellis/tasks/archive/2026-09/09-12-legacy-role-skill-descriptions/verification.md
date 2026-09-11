# Verification

## Root cause and reproduction

Both English descriptions in the user screenshot exactly match the role skill defaults originally shipped in `ca3a79d18`. The frontend only compared the latest defaults, so it preserved the older default text as though it had been customized.

Two pure regressions and one list regression failed before the fix. The isolated browser reproduced both English descriptions in list/picker and the missing Chinese purpose in each detail page, using the actual historical SKILL.md bodies.

## Fix

The existing shared presentation resolver recognizes the two exact historical English descriptions and selects explicit `description_v1` translations. The provenance/name gate, current defaults, custom edits, stored skill bodies, and mutation/invocation identifiers are unchanged. English and Chinese historical-purpose searches both work with only the active locale loaded.

## Results

- All views tests: 421 files, 5,086 tests passed after the final wording update.
- Targeted skills and locale parity suite: 14 files, 299 tests passed.
- Views typecheck: passed.
- Views lint: zero errors, the same 26 pre-existing warnings.
- Knip: the same five pre-existing unused files; no new findings.
- Diff checks: passed.
- Independent review: approved; clarified the ADR wording so consequences refer to the decision.
- Browser QA: 13 final legacy checks passed, plus 13 latest-mode regression checks. No console/page errors or external/API requests. Visual verdict: 96/100, pass.

## Scope and artifacts

Browser QA used real shared components with isolated fixture data; it did not modify user workspace records or verify a live backend/packaged Electron application. Artifacts are in `.omx/qa/skill-localization/legacy/`, including before/after screenshots, exact historical source provenance, browser results, and visual verdicts. Command logs and the final state are in `.omx/state/legacy-role-skill-descriptions/`.
