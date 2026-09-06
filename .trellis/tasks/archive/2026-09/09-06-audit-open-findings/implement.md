# Implementation plan

Branch `fix/audit-open-findings` in worktree `.claude/worktrees/fix-audit-open-findings`, based on `main` at `eb4c2f47b`. Tests precede behavior edits in every lane; each lane ends in one conventional commit.

## Environment

```bash
export DATABASE_URL='postgres://multica:audit-local-only@127.0.0.1:53656/fix_open_findings?sslmode=disable'
GOTEST="bash scripts/go-test-with-agent-cli-guard.sh go test"
```

## Lane A: credential minting (server)

1. [ ] Add `server/cmd/server/machine_credential_mint_test.go`: real-router fixture (workspace, member, runtime, observer agent, running task, `mat_` token row). Cases: task token → 403 on `POST /api/cli-token`, `POST /api/tokens`, `GET /api/tokens`, `POST /api/tokens/current/renew`, `DELETE /api/tokens/{id}`; PAT row count unchanged; agent level unchanged. Controls: human JWT → 200 `/api/cli-token`, 201 `POST /api/tokens` (cleanup row).
2. [ ] RED: `$GOTEST ./cmd/server -run TestMachineCredential -count=1 -v` fails on the 403 expectations.
3. [ ] Router: `r.With(handler.RequireHumanActor).Post("/api/cli-token", ...)`; `r.Use(handler.RequireHumanActor)` inside the `/api/tokens` route; comment explains the laundering chain.
4. [ ] Handlers: `IssueCliToken` and `CreatePersonalAccessToken` start with the `isMachineCredentialActor` backstop.
5. [ ] Docs: `multica-creating-agents/SKILL.md` limits paragraph names the boundary; `references/creating-agents-source-map.md` gains the row (code + test).
6. [ ] GREEN: same command passes. Run the neighbours: `$GOTEST ./cmd/server ./internal/handler -run 'Token|Cli|Approval|HumanActor' -count=1`.
7. [ ] Commit `fix(server): refuse to mint human credentials for machine actors`.

## Lane B: trigger principal (server)

1. [ ] Add `server/internal/handler/autopilot_trigger_principal_test.go` (fixture via `testutil`, router built like the audit's minimal chi router with real `Auth` + `RequireWorkspaceMember`): schedule and webhook triggers created by a task-token actor record `created_by_id` = originator; `DispatchAutopilotForPlan` / webhook worker enqueue nothing for a private target the originator cannot invoke, and enqueue one task when the originator is the target's owner; a task whose `originator_user_id` is NULL gets 403 and no trigger row; a member creator still records themselves.
2. [ ] RED: `$GOTEST ./internal/handler -run TestCreateAutopilotTrigger_ -count=1 -v` fails (created_by is the runtime owner; private target enqueued).
3. [ ] Add `requireAutomationPrincipal(w, r, workspaceID) (pgtype.UUID, bool)` in `autopilot.go`, built on `resolveActor` + `invokeOriginatorFromRequest`; use it for `created_by` on both create paths. Update the `created_by` comments.
4. [ ] GREEN: same command passes. Run `$GOTEST ./internal/handler -run 'Autopilot|Trigger' -count=1` and `$GOTEST ./internal/service -run 'Autopilot|Principal' -count=1`.
5. [ ] Commit `fix(autopilot): make agent-created triggers act as the authorizing human`.

## Lane C: stale 401 (packages/core)

1. [ ] Extend `packages/core/api/client.test.ts` "ApiClient session expiry": table over `listProjects`, `uploadFile`, `publishPluginPackage` × bearer / cookie; a deferred fetch mock holds the first response, the test logs in again (`setToken` or `verifyCode`), then releases the 401; assert user, status, stored token and `onSessionExpired` untouched. Keep a same-epoch 401 case.
2. [ ] RED: `pnpm --filter @multica/core exec vitest run api/client.test.ts` fails the six stale cases.
3. [ ] `client.ts`: `authEpoch`; capture in the three request paths; `handleUnauthorized(sentEpoch)`; bump in `setToken`, the three login methods and teardown.
4. [ ] GREEN: vitest passes; `pnpm --filter @multica/core typecheck` and `lint` pass.
5. [ ] Commit `fix(core): ignore a 401 that answers a request from a superseded session`.

## Final gate

- [ ] `gofmt -l` on changed Go files is empty; `cd server && go vet ./...`.
- [ ] `$GOTEST ./cmd/server ./internal/handler ./internal/service -count=1` with `DATABASE_URL` set, confirming real test names in `-v` output.
- [ ] `pnpm --filter @multica/core typecheck lint test`.
- [ ] Re-run the seven original audit files from the audit worktree against this branch (copy into a scratch checkout): the two inverted tests must now fail, the positive ones pass.
- [ ] Spec update, journal, task archive, then report. No merge or push.

## Rollback points

After each lane commit the branch is releasable on its own. `git revert <lane-commit>` undoes a lane without touching the others. Drop the scratch database with `DROP DATABASE fix_open_findings` when done.
