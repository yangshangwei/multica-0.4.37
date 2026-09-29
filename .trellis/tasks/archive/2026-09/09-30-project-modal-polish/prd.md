# Create project modal polish

User approved the 2026-09-29 critique with “请按照建议修改”. Preserve project creation, resource attachment, ordered squad selection, runtime deferral, drafts, and platform parity. Scope is the shared web/desktop modal and narrowly scoped child presentation.

## Accepted design and implementation plan
1. Use one subtle AI configuration surface; visually group selected squads and runtime. Keep existing semantic typography/colors.
2. Reduce redundant optional/help copy. Display the default badge beside a single selection as well as multiple selections. Retain accurate prepare-on-create/run-on-task semantics and all warning/error states.
3. Keep repository/local-directory action close to the resource label with concise supporting text.
4. Runtime field should emphasize provider and a bounded real machine display name, retain full identity in tooltip, and show textual online/offline status. Never invent device names or change bindings; reuse canonical runtime name helpers. Make deferral secondary.
5. Content-driven modal height with viewport maximum; preserve expanded mode, body scroll, accessible controls and fixed actions.

## Constraints
No dependencies, no global theme changes, no backend changes. Locale files contain unrelated work: edit only task-specific keys. Avoid changing every runtime-picker consumer: use existing field variant or narrowly scoped opt-in only if needed. Existing tests protect submission/local-directory/runtime behavior; run before and after. Add behavioral tests only for any meaningful changed presentation contract.

## Acceptance and verification
- Existing create-project, local-mode, project squad and runtime-picker tests pass. Locale parity, views typecheck and lint pass (report unrelated baseline problems honestly).
- Desktop and narrow rendered checks for empty/selected/multi/long runtime states. Visual verdict persisted under .omx/state/project-modal-polish/ralph-progress.json.
- Mechanical detector on edited UI and git diff whitespace checks.
- User-owned changes preserved; task-only review at end.
