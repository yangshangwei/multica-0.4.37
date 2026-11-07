# Option C implementation and verification

Status: verified on 2026-11-07. User approved option C and implementation in the existing task.

## Delivered

- Compact source/context header and prominent borderless title/plain-description editors match the ordinary composer language.
- Recipient and AI controls live after the description in the scrolling body. Full account IDs, permitted external status, selected-name text and contextual explanations remain visible/reachable.
- The fixed footer contains attachment entry, draft notice and cancel/create. File lists, sharing explanation, progress/errors/recovery remain in the body, including 32 long filenames.
- One existing attachment owner renders separate trigger/details slots. No duplicate mutations, stores, API changes or new dependencies were introduced.
- Advanced fields expose a retained-selection count and still expand for invalid values.
- Composing Enter cannot submit; ordinary title Enter still submits. Missing-title guidance is adjacent to the title. Initial focus accommodates asynchronous source loading without taking focus from a live control.
- Account popup height follows available anchor space. Compact upload cancellation hands its focus to the persistent attachment entry before removing Cancel, avoiding Base UI's later modal-root restoration.

## Changed product files

| Files | Purpose |
| --- | --- |
| `packages/views/collaboration/creation-form.tsx` | Composer hierarchy, source/footer layout, configured summary, IME and initial focus |
| `packages/views/collaboration/account-picker.tsx` | Optional compact trigger and available-height popup |
| `packages/views/collaboration/attachment-picker.tsx` | Single-owner trigger/details composition and deterministic cancellation focus |
| `packages/views/collaboration/focus.ts` | Owning-modal fallback recovery without cross-dialog/live-control focus theft |
| The four matching test files | Behavioral regressions and default-caller compatibility |
| `packages/views/locales/{en,zh-Hans}/collaboration.json` | Placeholders, explanations and configured-count copy |

The creation spec records the executable layout, identity, keyboard and focus contracts. Task plans reflect approved option C rather than the earlier bottom-recipient-toolbar variant.

## Final checks

| Check | Result |
| --- | --- |
| `pnpm --filter @multica/views exec vitest run collaboration locales/parity.test.ts --maxWorkers=2` | 33 files / **349 tests passed** on final product bytes |
| `pnpm --filter @multica/views typecheck` | Passed |
| `pnpm --filter @multica/views lint` | Passed; 0 errors, 27 pre-existing warnings |
| Scoped `impeccable detect --json` | `[]`, exit 0; see `detector-final.json` |
| Scoped `git diff --check` | Passed |
| Production component browser verification | 26 full-confirmation scenarios plus the final 2 targeted focus cases passed, covering 28 contracts |
| Visual verdict | **94/pass**; `research/visual-verdict-final.json` |

Logs retained under `verification-logs/`. The initial unrestricted parallel test run timed out under resource contention; the bounded full suite passed without changing timeouts or weakening assertions. Final IME/focus regressions failed before their fixes and passed afterward.

## Browser evidence

See [the detailed browser report](research/browser-verification-final.md) for provenance, scenario results and limits.

- [Desktop light](research/browser-round-2/desktop-light.png)
- [Desktop dark](research/browser-round-2/desktop-dark.png)
- [English 320px layout](research/browser-round-2/english-320-empty-light.png)
- [Complete recipient IDs at 320px](research/browser-round-2/coarse-320-light-picker.png)
- [32 attachments at 320px](research/browser-round-2/attachments-32-320-light-bottom.png)
- [Final cancellation focus](research/browser-final-focus/targeted/attachment-cancel-light.png)

The full confirmation passed all cases except cancellation focus in two themes. A frozen-build event trace showed our deferred recovery briefly focusing the attachment entry before Base UI restored the modal root again. The final minimal patch hands focus over before cancellation removes the focused button. Only the two affected browser cases were rerun; their final source comparison shows the only product delta is `attachment-picker.tsx`. Historical failures remain in the raw evidence.

## Limits and remaining risks

The browser harness builds real shared components/CSS/locales with mock API responses, memory draft storage and blocked external transport. It verifies desktop plus 390×740 and 320×568, both themes, native form events, CDP composition, synthetic keyboard/touch, text ranges, scroll geometry and focus indicators. It does not establish real backend acceptance, Electron Main/native packaging, physical touch/OS IME, spoken screen readers or dedicated 200% zoom acceptance. Ordinary creation was referenced by source and a supplementary earlier screenshot, not mounted in this fixture.

No known in-scope functional failure remains. All verification-owned browsers and loopback servers were closed. A final archive-portability change makes the fixture builder resolve its own paths dynamically; `node --check` passed, and product/CSS/locale bytes remained unchanged.
