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

## Post-fix full Views and renderer acceptance

- `pnpm --filter @multica/views test`: 602 files and 7,332 tests passed after
  the focused fixes. The command completed in 208.94s with no failed tests.
  The existing jsdom canvas/navigation warnings did not fail the run. Full-run
  output is retained in `research/m1-views-full-suite-20261113.log`.
- The task-owned renderer fixture used source HEAD `463b201fa`, API commit
  `463b201fa` (health pid `10062`), production Web build
  `M-WGkBlRRSxxdDC6L3KVT`, database `multica_multica_0_4_37_493`, one new
  enforce project at policy revision 1, two new issues, and two actual
  local-synthetic artifact receipts. Provider calls were zero. Full
  machine-readable evidence and
  screenshots are in [m1-renderer-evidence-20261113](m1-renderer-evidence-20261113/).
- Web production renderer: candidate selection, preview `pass`, keyboard
  authorize, keyboard submit, receipt query, and separate current issue read
  passed. Authorization creation returned `201`; bound completion PUT returned
  `200`; receipt state was `consumed`; current issue was `done` at revision 2.
- Electron real renderer/preload: the same flow passed through the loopback
  proxy origin `http://127.0.0.1:5668`, with CORS preflight accepted. The
  window was reduced to 680x900, keyboard activation passed, and the fixture
  recorded zero daemon starts. The modal kept receipt and current status
  visibly separate after completion.
- Final verified combined Playwright command:
  `M1_FIXTURE_FILE=<task-owned fixture> NEXT_PUBLIC_API_URL=http://localhost:18573 FRONTEND_ORIGIN=http://localhost:13493 PLAYWRIGHT_BASE_URL=http://localhost:13493 pnpm exec playwright test e2e/issue-completion-authorization.spec.ts --workers=1 --retries=0`
  Result: 2 passed / 0 failed; combined marker is
  `research/m1-renderer-evidence-20261113/final-playwright-last-run.json`.
- POLA-1 was not used. The fixture project and issue identities are task-owned;
  no shared policy or project was modified.

## Remaining verification limits

- The historical pre-fix Views run still had 78 failures; the post-fix full run
  above is the authoritative result for this checkout. The historical run is
  retained only to explain the bounded repair and is not counted as current
  failure evidence.
- No real Provider, knowledge injection, publish, Hosted CI, deployment or
  release behavior was exercised. The accepted `skip_supported=false`
  restriction remains unchanged.

## Reviewed CLI File Identity

These hashes include the final-check fixes above:

```text
d65357fa081e47674f1227b4326bf5b4683c24d806f86572d17428841ddbccaa  server/cmd/multica/cmd_issue_completion.go
d733d0a59d6416893612ec27a2e5f1300a5a85dd94776cff096781ea3b29566e  server/cmd/multica/cmd_issue_completion_test.go
```
