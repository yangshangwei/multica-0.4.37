# Make expanded description useful and Close reliable

## Goal

The user explicitly clarifies that expanding collaboration creation should enlarge its description editor, not leave unused space after the form. The user also reports that the top-right Close icon does nothing and approved a new task with direct repair. These are corrections to the previously delivered composer, with the current screenshot as visual evidence.

## Requirements

- In expanded mode, allocate available extra body height to the description textarea. Retain compact default layout and natural content growth; keep AI assistance, recipient/policy, files, advanced fields and errors reachable.
- Keep the existing single mounted form, textarea and assistant through expand/collapse, preserving draft values, files, selections, focus and active streams.
- Header/source and footer remain in normal flow outside the body scroll. Other content consumes spare editor height first; only genuine content overflow scrolls. Do not invent a fixed editor viewport-height or second textarea scrolling area.
- Close must respond in default/expanded modes via pointer, touch and keyboard. Closing preserves draft/files, aborts owned AI streams and restores opener focus. Correct desktop drag-region hit testing at the narrowest appropriate surface, without global unrelated redesign.
- Existing source locks, API/current-owner guards, full identity, native Enter/IME and upload recovery behavior remain intact.

## Acceptance criteria

- [x] At901×1042 and1280×900, expanded empty/short-description textarea consumes most newly available height; no large unused gap remains below the final field.
- [x] Expanding advanced fields, attachments or AI clarification reduces spare editor height; long content scrolls body with footer/header reachable. Default appearance remains compact.
- [x] Default/expanded Close works and retains drafts/returnsfocus; native Electron drag-area behavior is verified with the strongest available evidence and honest limits.
- [x]390×740/320×568 both themes retain44px coarse targets, viewport gutters, full identities, usable body and fixedactions.
- [x] Existing relevant creation/assist/focus tests, typecheck/lint/detector and real-browser layout assertions pass; no mirrored-style-only tests.

## Out of scope

No API/data/storage changes, dependencies, AIprovider execution, unrelated dialog redesign or app-wide drag-region rewrite. Realbackend/physicalaccessibility acceptance remains separate unless needed to reproduce Close.
