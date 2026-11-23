# Verification ledger

## Original evidence

- Audit: `.impeccable/audit/2026-11-19-core/report.md`.
- Native Electron F02 reproduction: closed right task sidebar remained in AX and accepted Tab focus on invisible Properties.
- Source/token verification: F01/F03-F11 recorded in audit and manifest with source hashes.
- Pre-remediation related regression baseline: 7 files / 113 tests passed; gaps were outside current assertions.

## Required regression checks

- [x] Squad local edits survive changed server instructions after onUpdate/debounce.
- [x] Save completion/failed save does not discard newer local input.
- [x] Missing squad and cold member failures offer truthful recovery; cached transient failures retain drafts.
- [x] Read-only squad view never mounts an editable content surface.
- [x] Squad tabs preserve unsaved confirmation and expose manual-activation tab/panel relationships.
- [x] Picker selection and keyboard active-option semantics expose the same candidate being submitted, including IME/empty-value guards.
- [x] Assignee group failures expose Retry without replacing successful candidates or clearing search.
- [x] Real mounted editor carries localized description/comment/reply name.
- [x] Disclosure expanded state matches visible content.
- [x] Hidden sidebar is inert, absent from AX, preserves content, and does not reclaim focus from unrelated controls.
- [x] Reduced motion snaps content and panel geometry, including live preference change.
- [x] Brand text contrast >=4.5 in both themes and relevant button states.
- [x] Header action target >=44px in narrow/coarse conditions; desktop density retained.

## Final checks

- [x] Targeted behavior tests.
- [x] Views/UI/Web typecheck and lint, plus desktop typecheck for shared caller wiring.
- [x] i18n parity and existing contrast/brand regression suites.
- [x] Impeccable detector on changed targets, with false positives reviewed.
- [x] Native Electron AX/keyboard smoke for observed regressions.
- [x] Full-scope independent Trellis check review.

## Integrated evidence in progress

- Native Electron: task status exposes listbox/selected state and Down changes the announced candidate; closing sidebar with popup open removes both from AX, and12 Tabs reach visible chat rather than hidden properties. Description/comment names and squad tab/panel names confirmed; details in native-remediation.json.
- Compiled CSS: narrow and coarse collection header actions resolve to44px minima; wide fine-pointer retains32px. This is compiled CSS evidence, not physical-device hit testing.
- Brand contrast: minimum4.680:1 across light/dark default/hover/active/expanded pairs; dark default now6.123:1. See evidence/brand-contrast-final.json.
- Main scoped lint: no errors; two existing project-detail hook dependency warnings. Web and Desktop typecheck passed.

## Final result

- Final integrated Views:28 files,453 tests passed at maxWorkers2, no timeout increases.
- Web theme/type scale:3 files,56 tests passed. Core query tests:3 files,24 tests passed. Total distinct checked tests533.
- Core/UI/Views/Web/Desktop node+web typechecks passed. Modified TS/TSX lint had0 errors; two pre-existing project-detail hook warnings remain.
- Detector:31 modified UI/style targets,0 findings,exit0.
- Original table combined test timed out under host load; exact-step diagnosis found no stuck operation. Split independent priority and clear-assignee cases retains all revision assertions and adds exactly-once checks; default5s unchanged. Final batch green.
- Independent review resolved6 integration edges; record in review.md.
- Product changes are verified and committed as `b6b07c3067bccaf3a76de960872a41e0f687d276`. No unrelated work was reverted or included in the commit.
