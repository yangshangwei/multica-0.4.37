# Onboarding runtime connection

## Requirements

- Remove the unavailable cloud computer choice from runtime onboarding on desktop and web.
- Identify why installed local agent CLIs do not appear on this machine, using current process and log evidence.
- Correct the confirmed connection failure with a focused change; preserve workspace ownership, authentication and existing runtime selection.
- Keep unrelated password-form work untouched and add no dependencies.

## Acceptance criteria

- Onboarding offers the supported local connection actions and Skip, without a cloud preview.
- The cause of the empty runtime list is documented with reproducible evidence.
- A behavioral fix has a failing regression check before implementation and passing checks afterward.
- Relevant tests, lint/type checks and a visual inspection of the modified onboarding are completed, with limitations recorded.

## Execution

1. Trace runtime list, desktop daemon startup and actual local environment.
2. Remove the two cloud previews and their unused presentation code/translations; adjust existing expectations.
3. Reproduce and fix the confirmed daemon startup failure.
4. Verify the affected packages and the local runtime path, then record evidence.
