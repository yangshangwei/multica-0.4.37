# 验证记录

## 通过的命令

```text
pnpm --filter @multica/core test                         # 316 files, 4224 tests
pnpm exec vitest run collaboration --reporter=dot        # views collaboration: 24 files, 120 tests
pnpm exec vitest run collaboration/account-picker.test.tsx collaboration/creation-form.test.tsx collaboration/workflow.test.tsx collaboration/field-editing.test.tsx --reporter=dot  # 16 tests
pnpm typecheck                                            # 9/9 packages
pnpm lint                                                 # 6 packages, 0 errors; existing warnings only
pnpm --filter @multica/core lint
pnpm --filter @multica/views lint
pnpm exec playwright test e2e/collaboration-recipient-picker-web.spec.ts --list

set -a; source .env.worktree; set +a
bash scripts/go-test-with-agent-cli-guard.sh -- go -C server test -race ./internal/handler -run '^TestSearchCollaborationRecipient' -count=1
bash scripts/go-test-with-agent-cli-guard.sh -- go -C server test -race ./internal/service -run '^TestCollaborationRecipientSearch' -count=1
go -C server vet ./internal/service ./internal/handler ./cmd/server
git diff --check
```

The Go recipient tests used the isolated database generated from `.env.worktree` after applying migrations through 744, including `586_issue_collaboration_issue`.

## Independent check

`trellis-check` reviewed the cross-layer implementation and fixed:

- stable `data-recipient-row` E2E hooks;
- duplicate exact-lookup IDs by switching to `useId()`;
- E2E trigger selection around Base UI focus guards;
- obsolete exact-input locale keys;
- missing focused account-picker tests.

## Known non-blocking failures

The full service package race command timed out after 10 minutes in the unrelated existing `TestCollaborationSharedSkillWritersInvalidateEnabledDependents` test. The recipient search service/handler race subsets pass on the migrated database. The global locale parity script still reports the pre-existing `runPreparation.attachments_one` key missing from zh-Hans; the new recipientPicker keys are parity-complete.

## Full workspace test note

`pnpm test` reached the full workspace suite but exited because two existing `packages/core/platform/core-provider.test.tsx` cases failed under the concurrent turbo run: one timed out while waiting for deferred device authentication, and the StrictMode case observed three identity requests instead of two. The recipient-specific core and views suites remained green; no failure referenced the changed picker/search code.

## Integration verification — 2026-10-30

Integrated into `codex/cross-workspace-collaboration` from base `85b98c3bb`,
using the source implementation `751567567` and archive `6cd166388`.
The integration also closes an already-open picker when disabled, rejects
disabled selections, and exposes cached-data refresh/pagination failures with
the appropriate retry. Three component regressions failed before these fixes
and passed afterward. The browser test now locates the portalled popup and
uses the actual accessible names.

Fresh verification in an isolated worktree:

- Core collaboration/API Vitest: 69 files, 1,126 tests passed.
- Shared collaboration and locale-parity Vitest: 26 files, 191 tests passed.
- Recipient service, handler and server-route Go race tests: 42 top-level
  tests and 171 subtests passed, with zero skips, against a newly created
  local PostgreSQL database migrated through `745_drop_user_cloud_waitlist`.
- `pnpm typecheck`: 9 package tasks passed.
- `pnpm lint`: 6 package tasks passed with existing warnings and zero errors.
- `go vet` for service, handler and server packages passed.
- `make sqlc` produced no additional generated-code changes.
- The production Web build passed; the Chromium recipient-picker E2E passed
  with self/recent/search, response privacy, Escape and a 375px viewport.
- `git diff --check` passed.

This integration did not repeat the complete workspace test suite or perform
native Electron acceptance. The original verification limitations above remain
documented separately from the fresh integration checks.
