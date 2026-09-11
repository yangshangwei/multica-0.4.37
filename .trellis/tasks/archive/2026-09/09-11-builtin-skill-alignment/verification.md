# Verification — 2026-09-11

## Behavior and scope

- Writer is Contributor with documentation-only instructions; Architect remains Observer and hands off proposed ADR drafts in comments.
- Only Feature Delivery and Discovery leads default to requirement clarification. Nine changed role templates and the ADR/release role skills have incremented versions.
- Actual releases use the release gate; independent high-risk actions use proportionate checks with explained N/A items. Operator and per-action human approval requirements remain explicit.
- Autopilot credential disclosure instructions require explicit user intent, Operator, and approved secret_access. No new API permission enforcement or filesystem sandbox is claimed.
- General platform skills exclude onboarding. Trusted Mika identity and persisted kickoff provenance in the task's own chat control both claim formats and bundle resolution. Continuations and retries retain availability.
- Existing workspace agent rows and materialized same-name skill copies are not overwritten or migrated.

## Evidence

- Before the fixes, updated role-default tests failed on Writer autonomy and all six lead bindings; `TestBuiltinSkillsExcludeOnboarding` failed because all nine platform skills were still generally distributed.
- `make sqlc`: passed; only the intended chat query generated code changed.
- `go test -json ./internal/service ./internal/handler -count=1` under `scripts/go-test-with-agent-cli-guard.sh`: both packages passed against a newly created, fully migrated local PostgreSQL database. Runner events: 4,050 test/subtest passes, 66 existing conditional skips, zero failures. Service: 11.791s; handler: 52.951s.
- The new database regressions passed without skips: inline and slim claims each cover seven identity/session/continuation/retry cases, plus a cross-agent session resolution rejection. General builtins remain resolvable.
- Service scope tests verify scope-read errors propagate and ordinary builtin downloads do not query onboarding scope.
- Final focused service/template/skill tests after the last prose edit: passed.
- `go vet ./...`: passed.
- `pnpm lint`: passed, six cached tasks.
- `pnpm typecheck`: passed, nine cached tasks.
- `git diff --check`: passed.

No real-agent CLI, production account, or browser E2E was run. Go subprocess tests used the repository's agent CLI guard. The test database was separate from the checkout's development database; its credentials were stored only in a temporary file outside the repository.

Cleanup completed: the isolated test database and its temporary credentials/logs
were removed after verification. No shared development database was modified.
