# Core menu audit fixes

Preserve the existing neutral sidebar and route behavior.

1. Harden: distinguish exact-page aria-current from ancestor location; make labels truncate independently of counters; protect trailing content. Add regression coverage for navigation state.
2. Adapt: retain desktop density; use 44px minimum rows for compact screens and coarse pointers, including slotted search.
3. Clarify: separate AI members from resources/connections, and label the workspace issue entry explicitly in both supported locales. Preserve glossary terms and destination order.
4. Polish: inspect desktop/mobile output in a bounded pass, run targeted tests, typecheck, lint and static detector; report any runtime verification limits.

No dependencies, route changes, or unrelated cleanup. Existing dirty files belong to other work.

## Verification

- Navigation regression was red for missing aria-current, then passed after implementation.
- Sidebar suites: 42 tests passed across 4 files.
- Views typecheck and scoped ESLint passed; static Impeccable detector returned no findings.
- Independent diff review found no blocking defects.
- Real Next UI with browser-local API mocks: desktop rows 32px, touch rows 44px, current link semantics, long-label/counter separation and scroll reachability verified. No browser page errors or horizontal overflow.
- Visual verdict: 94/pass; screenshots and geometry in .omx/state/core-menu/.
- Limits: no physical-device, real-account or screen-reader validation. No product routes, data or dependencies changed.
