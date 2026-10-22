# AC33 current verification boundary — 2026-10-21

This is a continuation evidence note for AC33. It does not change the
acceptance ledger and does not claim the combined Web/Electron execution
journey passed.

## Current source checks

The current worktree contains the bounded command-history projection repair:
an absent terminal reason is serialized as `null` rather than an empty string,
while non-empty cancellation reasons remain unchanged. The focused regression
and its legacy-evidence companion pass against the current service source:

```sh
cd /Volumes/artisan/code/2026/multica-0.4.37/server
GOTOOLCHAIN=go1.26.6 go test ./internal/service \
  -run '^TestCollaborationCommandHistory(ProjectsEmptyTerminalReasonAsNull|LegacyManualAndBrokenEvidence)$' \
  -count=1
```

Result: `ok github.com/multica-ai/multica/server/internal/service 0.647s`.

The package vet check also passes:

```sh
GOTOOLCHAIN=go1.26.6 go vet ./internal/service
```

The AC33 client fixture remains clean and statically registered:

```sh
cd /Volumes/artisan/code/2026/multica-0.4.37
pnpm exec playwright test \
  e2e/collaboration-cross-client-execution.spec.ts \
  e2e/collaboration-control-journey-web.spec.ts \
  e2e/collaboration-control-journey-desktop.spec.ts \
  --list --reporter=list
pnpm exec tsc --project /tmp/multica-gap-recovery-20261021-h6phxwo9/cross-client-static/tsconfig.json
node packages/views/node_modules/eslint/bin/eslint.js \
  --config /tmp/multica-gap-recovery-20261021-h6phxwo9/cross-client-static/eslint.config.mjs \
  e2e/fixtures/collaboration-cross-client-execution.ts \
  e2e/collaboration-cross-client-execution.spec.ts
```

Results: Playwright discovered five tests (the one combined AC33 test plus the
four AC33/95 control tests), strict TypeScript exited 0, and scoped ESLint
exited 0 with no diagnostics. The only output was the repository's existing
pnpm package-field warning. `gofmt -d` on the explicit provider fixture and
the three untracked-file whitespace checks produced no output; their
`git diff --no-index --check` exit 1 is the expected status for comparing an
untracked file with `/dev/null`.

Current source hashes used for this check:

| File | SHA-256 |
| --- | --- |
| `e2e/collaboration-cross-client-execution.spec.ts` | `3485662826b2afc9ab82dadbbe3e3d71dfad6bcb9c3bee9c4419c63a6d3b5416` |
| `e2e/fixtures/collaboration-cross-client-execution.ts` | `eab460c2681c95696ea90034ed63b34fc523169bf2d10d151c079c98cdda6c40` |
| `e2e/fixtures/collaboration-cross-client-provider.go` | `356dfab491c55e0647e1523e30334c8043c597c2426e045ca22dd25545f17c88` |
| `server/internal/service/collaboration_command_history.go` | `a23ad56dae751bc9cf7f0ef226292f3bea7ff3661cc7ebc6310bde49ef79e526` |
| `server/internal/service/collaboration_command_history_test.go` | `1acc62dad720ef75b0bdaf38fd4c312f8c85d0b5299ac2119b8820cb467ec9b2` |

## Re-run blocker

The previous AC33 combined journey reached all runtime assertions and failed
only when the Web command-history projection returned `reason: ""`; that
failure is retained in `cross-client-execution.md` and the raw attempt-3
report. The current source repair is present and the focused service test
passes, but the task-owned password/managed API at `http://localhost:18996`
is still the old listener (`pid=61506`, health commit `8ecb98883`). It has not
been rebuilt with the repair, so a browser rerun against that listener would
reproduce the stale failure and is not current acceptance evidence.

The production Web (`http://localhost:13916`) and native renderer
(`http://localhost:6090`) answer HTTP 200. Re-running AC33 requires the
coordinator to rebuild/restart only the task-owned API from the current source,
record the new API PID/commit/configuration identity, and then execute the
existing command with the explicit Linux/arm64 daemon and provider binaries
from `linux-binaries.json`. No provider account, quota, rollout, or product
gate is required for this synthetic run.

## AC95 status

AC95 is already evidenced by `control-journeys.md`: two Web tests and two
actual Electron tests passed. Those journeys cover agent, squad, and
whole-issue Stop/Resume isolation, pending `stop_unconfirmed` proof explanation,
fresh Resume, untouched lanes/holds, and wide/narrow/native 200% geometry.
This note leaves that evidence unchanged.

## Scope and changed files

No product source, shared schema, acceptance ledger, environment, or gate was
changed by this continuation. The only new file is this evidence note.
