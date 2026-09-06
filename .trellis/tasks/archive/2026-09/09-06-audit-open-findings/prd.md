# Close the three open audit findings

## Goal

Close the three findings from the 2026-09-06 main audit that the `fix/agent-autonomy-gates` line did not address. Each fix lands with a regression test that fails on `eb4c2f47b` and passes after the change. The audit reproductions live in the temporary worktree `multica-main-audit-y_ih5y70/checkout` (seven untracked test files) and its `evidence/` directory; they are inputs, not deliverables.

## Requirements

1. **Machine credentials cannot become human credentials.** A task token (`mat_`) or cloud PAT (`mcn_`) must not obtain a JWT from `POST /api/cli-token`, and must not create, list, renew or revoke personal access tokens under `/api/tokens`. Human JWT, cookie and `mul_` PAT callers keep today's behavior, including the daemon's PAT renew and the CLI login flow that exchanges a browser JWT for a PAT.
2. **Automation created from a task is accountable to the human who authorized the task.** A trigger created by an agent actor records the task's originator as its authorization principal, never the runtime owner whose user id the task token carries. Dispatch then admits or refuses the run as that human, exactly as it does for a human creator. A task with no human originator cannot create a trigger.
3. **A 401 only ends the session it belongs to.** When a request sent under an earlier credential is answered 401 after a new login, in bearer or cookie mode, through `fetchRaw`, `uploadFile` or `publishPluginPackage`, the new session, its stored token and the session-expired callback stay untouched. A 401 answering a request sent under the current credential still tears the session down exactly as today.
4. No migration, dependency, wire-shape or route-path change. Agents without a declared autonomy level keep their existing behavior. Mobile is not touched.

## Acceptance

- [ ] Real-router Go test: a task token gets 403 from `POST /api/cli-token`, `POST /api/tokens`, `GET /api/tokens`, `POST /api/tokens/current/renew` and `DELETE /api/tokens/{id}`; no PAT row is created or removed; the agent's autonomy level is unchanged. A human JWT still gets 200 from `/api/cli-token` and 201 from `POST /api/tokens`.
- [ ] Handler Go test: agent-created schedule and webhook triggers carry `created_by` = the task's originator, not the token's user; automatic dispatch enqueues nothing for a private target that originator cannot invoke and does enqueue when the originator may invoke it; a task without a human originator gets 403 and no trigger row; a member creator still records themselves.
- [ ] Vitest: a stale 401 after re-login leaves the new session intact for the three request paths in both auth modes; a 401 for the current credential still expires the session; the existing MUL-7028 test still passes.
- [ ] The builtin `multica-creating-agents` skill names the credential boundary and its source map points at the code and test.
- [ ] `gofmt`, `go vet`, guarded `go test` for `cmd/server`, `internal/handler`, `internal/service`; `pnpm --filter @multica/core typecheck`, `lint`, `test` all pass on the branch.
- [ ] Each regression test is shown failing before its fix.

## Source

`.omx/reviews/agent-autonomy-merge-readiness-2026-09-06.md` (what the previous line fixed); audit notes `credentials-review.md`, `automation-plan.md`, `sessions-reproduction.log` in the audit worktree's `evidence/` directory; this session's re-run of all seven audit tests against `eb4c2f47b`.
