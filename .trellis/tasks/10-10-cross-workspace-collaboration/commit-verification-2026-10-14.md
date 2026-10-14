# Completed collaboration slices — commit verification

The user requested local commits for completed work on
`codex/cross-workspace-collaboration`. This review reused the existing task;
it did not create or complete a Trellis task, open creation/execution gates,
push, merge or deploy.

## Implemented scope

- R42: issue-scoped field options and detail metadata; creator/handler editing
  of priority, labels and custom-property values with shared ordinary validation,
  original request identity, versioned history and atomic receipts.
- R29/R46: authorized exact recipient resolution and transaction-time target
  eligibility for creation, enrollment and responsibility changes. Resolution
  never substitutes for write authorization or changes notification recipients.
- R56: collaboration sources are rejected before body/thread/attachment
  projection in preview, both subtask modes and direct service writes. Ordinary
  children retain their own capture behavior.
- Core API/schema integration rejects malformed private responses, wrong
  resource/context identity and late account/server responses.

## Fresh verification

| Check | Result |
| --- | --- |
| Full TS lint/typecheck | 15 tasks passed; no cached results |
| Core tests | 254 files, 3406 tests passed |
| Guarded Go race service matrix | 428 cases passed, 0 skipped |
| Guarded Go race handler matrix | 152 cases passed, 0 skipped |
| Guarded Go race router matrix | 19 cases passed, 0 skipped |
| Guarded Go race util suite | 115 cases passed, 0 skipped |
| New backend regressions | All 44 new top-level tests executed and passed |
| Full guarded `go vet -p 2 ./...` | Passed |
| Go formatting and whitespace | Passed |
| Private sqlc v1.31.1 generation | All 93 generated Go files match |

The Go matrix used an independently created loopback PostgreSQL database,
migrated through 619 (643 migration-ledger entries). The reviewer confirmed
zero active connections, dropped only that database and verified it absent.
Existing databases and services were preserved; agent CLI execution was guarded.

Commands, with the isolated `DATABASE_URL` supplied privately:

```bash
pnpm exec turbo run lint typecheck --filter='!@multica/mobile' --concurrency=1 --force
pnpm exec turbo run test --filter='!@multica/mobile' --concurrency=1 --force -- --maxWorkers=2
bash scripts/go-test-with-agent-cli-guard.sh -- go -C server test -race -p 1 -parallel 2 ./internal/service ./internal/handler ./cmd/server -run 'Collaboration|SourceContext|BuildSource|CommentSubIssue|Property|Properties|ParseActorRef' -count=1 -json
bash scripts/go-test-with-agent-cli-guard.sh -- go -C server test -race ./internal/util -count=1 -json
bash scripts/go-test-with-agent-cli-guard.sh -- go -C server vet -p 2 ./...
```

The first router run correctly rejected the password-mode human-control target
because its fixture lacked a completed password credential. The reviewer added
credentials to the two test targets and reran the router matrix successfully;
production eligibility checks were preserved. Two unrelated committed views
test mocks/selectors also drifted from current contracts; their test-only
correction and subsequent frontend verification are recorded separately.

## Remaining acceptance

These checks cover completed implementation slices. Full P1–P8 acceptance,
Web/Desktop business flows, controlled-comment UI, native containment and real
AI execution remain outstanding. The task stays `in_progress`.
