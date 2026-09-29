# Software development onboarding roles

## Requirements

Focus the About you role selector on software development, retaining Other and its free-text input. Keep developer and product roles, clarify designer as UI / UX, and replace broad founder, marketing, content, research, operations, and student options with architect, QA, DevOps, security, engineering lead, and technical project manager roles. Update English and Simplified Chinese together.

## Acceptance criteria

- Ten single-select options, including Other as the final option.
- Existing selection, Other input, skip, and continue behavior still works.
- Newly selected roles retain distinct persisted identifiers; existing stored role values remain readable.
- Shared web/desktop UI uses the existing chip styling and icons without new dependencies.
- Preserve pre-existing welcome illustration edits.

## Verification

Run existing onboarding tests, affected-package type checks, targeted ESLint and static checks, and inspect the role selector at desktop and narrow widths.
