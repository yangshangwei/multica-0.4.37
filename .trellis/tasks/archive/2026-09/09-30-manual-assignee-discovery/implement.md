# Implementation and verification

1. Protect existing behavior with the current actor-picker and manual-create
   suites; add focused regressions for direct assignment and recent-only writes.
2. Extend the existing actor picker with optional direct-assignment controls,
   custom/controlled triggers and disabled actor metadata. Reuse its layout,
   search, pagination, focus handling and category logic rather than copying it.
3. Add a manual-create adapter to supply members, agents, squads, permission
   decisions and scope-guarded preferences. Replace only the manual create call.
4. Record successful manual agent/squad assignment through a recent-only store
   action, preserving quick-create defaults and fallback identity.
5. Update the picker contract and both supported locales. Run focused tests,
   package typecheck/lint, diff checks and the design detector. Inspect the
   actual UI in an isolated local runtime and record a visual verdict.

## Ownership

- Parent: shared picker, manual adapter/wiring, locales, UI tests, docs and final
  verification.
- Bounded executor: recent-only action and its tests in the existing core
  quick-create store; no other files.

## Simplification

Reuse the existing discovery UI and preference scope guards. Keep ordinary
issue assignee pickers untouched; do not introduce another picker framework.
