# Registration validation

## Requirements

- Show registration failures in the selected interface language, including validation and unknown server errors.
- Allow usernames made entirely of digits, recommend using an employee ID, and remove the leading-letter requirement.
- Preserve 3–32 ASCII letters/digits/underscores, case normalization, whitespace trimming, uniqueness, and password requirements.
- Keep web and desktop behavior shared and preserve unrelated in-progress work.

## Acceptance criteria

- Numeric usernames, including leading zeros, register and log in successfully.
- Username guidance explicitly recommends an employee ID and explains numeric usernames.
- Chinese registration errors never render raw English server messages; invalid username, name, and password remain actionable.
- English localization, rate-limit handling, and uncertain-registration guidance continue working.
- Targeted frontend/backend regression tests and relevant lint/type/static checks pass.
