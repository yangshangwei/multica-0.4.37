# Workspace name series

## Problem and outcome

Workspace naming currently samples only celestial names. Users want names suited
to software development while retaining playful alternatives and choosing their
preferred series beside the existing Random action.

## Approved requirements

- Six curated bilingual series, 20–30 names each: workshop (default), computing,
  AI exploration, space, nature, and voyage. Space favors recognizable names.
- A split Random button opens a series menu with examples, selected indication,
  and an All series option. The current series remains visible when closed.
- Choosing a series changes only the preference, never existing form values.
- Random applies a localized name and an English URL base plus four random
  alphanumeric characters. The server remains the uniqueness authority.
- Avoid repeats within a creation session until the applicable pool is exhausted;
  All series samples series uniformly among pools with unseen names.
- Persist the selected series per account on this device. Anonymous selections
  must not overwrite any account preference. No cross-device synchronization.
- Automatically generated URL and task prefix follow subsequent random names.
  Manually edited URL or prefix remain intact. Typing into a generated name
  retains its stable English URL until the URL is explicitly edited.
- Names remain editable. Interface language determines the next generated name;
  changing language does not rewrite a name already entered.
- Preserve creation-disabled, pending, reserved URL, conflict and resume flows.
- Shared Web/Desktop UI, keyboard-accessible menu, narrow-screen layout.

## Acceptance criteria

- [x] All six bilingual series contain 20–30 unique, usable entries.
- [x] Fresh account defaults to workshop; selection restores for the same account
  and stays separate for another account.
- [x] Menu selection never changes name, URL, or prefix; keyboard selection and
  Escape/focus return work with the real UI primitives.
- [x] Random respects selected series, avoids duplicates, and terminates after
  pool exhaustion; All is not biased toward the largest catalog.
- [x] Untouched generated URL/prefix follow randomization; manually edited values
  survive randomization, and submission matches the displayed fields.
- [x] English and Chinese strings have parity and locale switching preserves
  existing form values.
- [x] Focused tests, affected-package typecheck/lint and visual checks pass.

## Scope

No backend/schema changes, dependencies, AI name generation, naming gallery,
multi-select series, or mobile-native changes. Existing unrelated work is outside
this task. The user approved this design and requested task creation/execution
on 2026-09-30.
