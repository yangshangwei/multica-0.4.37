# M1-S1 Final Quality Check

Date: 2026-11-13. Scope: approved ordinary single-issue completion flow across
preview/read APIs, Core, shared Web/Desktop views and CLI. No Provider, release,
Hosted CI or deployment action was performed.

## Findings fixed

- `server/cmd/multica/cmd_issue_completion.go` previously accepted any non-empty
  authorization applicability and treated any non-nil consumed receipt as proof.
  It now enforces the state/applicability pairs, validates consumed receipt
  authorization/operation/issue/target identity and advancing revision, and
  matches the returned expiry to the original authorization request.
- The CLI previously sent a completion PUT when its retained grant was already
  known expired or had an inapplicable state. It now refuses before writing the
  dispatch marker or sending PUT unless the grant is issued, `not_checked`, and
  still before expiry. The server writer remains authoritative for races after
  this local check.

## Regression evidence

Before the fixes, this command failed on both new assertions:

```text
go -C server test ./cmd/multica -run '^TestIssueCompletion(AuthorizationValidationBindsStateAndReceipt|ExpiredAuthorizationDoesNotSubmit)$' -count=1
```

Failures observed: a consumed authorization with another operation's receipt
was accepted; an expired authorization unexpectedly submitted. After the fixes,
the focused regression plus the existing CLI completion cases passed. The fake
HTTP service was corrected to echo the exact requested expiry; its earlier
`max_expires_at` reply did not match the server authorization contract.

## Verification

- `go -C server test ./cmd/multica -count=1`: passed.
- `go -C server vet ./cmd/multica`: passed.
- `go -C server build ./cmd/multica`: passed.
- Built-in skill contract tests (`TestBuiltinSkillsConformToTemplate`,
  `TestWorkingOnIssuesSkillCoversIssueLoopContracts`,
  `TestCreatingAgentsSkillCoversAgentCreationContracts`): passed.
- `git diff --check` and `gofmt -d` for the reviewed CLI files: passed.
- Backend evidence in [m1-backend-resume.md](m1-backend-resume.md): fresh
  normal/race handler runs 76/76 each and router runs 4/4 each, zero skips.
- Client evidence in [m1-clients-resume.md](m1-clients-resume.md): Core full
  suite 4,546 passed; M1 and affected surface tests 84/84; Views typecheck and
  lint passed. Eight unrelated timed-out suites passed 319/319 when rerun alone.

## Remaining verification limits

- The full Views suite was not rerun after the focused fixes. Its earlier run
  had 78 failures: surface failures from the subsequently fixed auth-store
  access/mock issues and timeouts in eight unrelated suites, which passed in
  isolation. This is recorded in the client lane report.
- The running Desktop was connected to the local `Test` project whose policy
  did not enforce completion. No authorization modal could be reached there, so
  visual/modal end-to-end validation remains unproven. During this inspection,
  `POLA-1` was moved from `todo` to `done` by its ordinary allowed flow, then
  restored to `todo`; its status history retains both changes. No additional UI
  mutation was made.
- No real Provider, knowledge injection, publish, Hosted CI, deployment or
  release behavior was exercised. The accepted `skip_supported=false`
  restriction remains unchanged.

## Reviewed CLI File Identity

These hashes include the final-check fixes above:

```text
d65357fa081e47674f1227b4326bf5b4683c24d806f86572d17428841ddbccaa  server/cmd/multica/cmd_issue_completion.go
d733d0a59d6416893612ec27a2e5f1300a5a85dd94776cff096781ea3b29566e  server/cmd/multica/cmd_issue_completion_test.go
```
