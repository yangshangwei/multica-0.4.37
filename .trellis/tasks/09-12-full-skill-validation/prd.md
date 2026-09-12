# Full repository and real skill validation

## Request

Run the checks omitted from the localization task: all repository unit tests,
database integration tests, the complete end-to-end suite, and real-agent
selection and invocation tests for the seven Chinese role skills.

## Acceptance

- Run the existing unit-test commands across all configured packages, including
  the independently configured mobile package; report skipped or failed cases.
- Run all Go test packages against an isolated migrated database so database
  integration tests actually execute rather than silently skipping.
- Run the full configured E2E suite against an isolated API/web environment.
- Exercise the seven localized skills through an installed authenticated agent,
  using realistic Chinese requests, positive and negative selection cases, and
  evidence of actual skill loading and outputs rather than self-reported success.
- User authorization explicitly includes real-agent account access and quota
  use. No external messages, production actions or permission changes are part
  of the test scope.
- Preserve the user's workspace and source changes. Record commands, outcomes,
  failures, skips, environmental limitations and evaluation limitations honestly.

## Scope

This is validation of commit fe38a164b and its skill-localization changes.
Unrelated product fixes require a separate scope decision; routine environment
repairs and narrow fixes necessary to test the requested behavior are allowed.
