# Journal - artisan (Part 1)

> AI development session journal
> Started: 2026-09-01

---



## Session 1: Prepare agent autonomy changes for main

**Date**: 2026-09-06
**Task**: Prepare agent autonomy changes for main
**Branch**: `fix/agent-autonomy-gates`

### Summary

Repaired the seven merge-review findings with regression-first changes, preserved ungraded-agent compatibility, and obtained independent approval after final local verification.

### Main Changes

- Reuse transaction connections for squad ACL checks and merge batch update field presence deterministically.
- Validate the final staged snapshot, preserve existing marker examples, and retain work on conflicts or inspection failure.
- Use complete API teardown, real Coordinator ceiling coverage, and accurate autonomy guidance.

### Git Commits

| Hash | Message |
|------|---------|
| `665a2c05f` | (see git log) |
| `2560b3b1d` | (see git log) |
| `6e4256ae3` | (see git log) |
| `066665951` | (see git log) |
| `4859dadf5` | (see git log) |

### Testing

- [OK] 64 Go race packages passed; four affected packages passed again after the final EOF correction.
- [OK] 25 API cases passed on the rebuilt isolated backend with zero orphan rows; TypeScript, lint, Go build/vet and independent reviews passed.

### Status

[OK] **Completed**

### Next Steps

- The branch is ready to merge into local main; no merge or remote push has been performed.

---

## 2026-09-06: 关闭审计遗留的三项边界问题（09-06-audit-open-findings）

### What was done

三条独立修复线，各自 RED→GREEN 后单独提交（分支 `fix/audit-open-findings`）：

- `a61c55d73 fix(server): refuse to mint human credentials for machine actors`
  — `/api/cli-token` 与 `/api/tokens` 全组加 `RequireHumanActor` 路由门禁，
  `IssueCliToken`/`CreatePersonalAccessToken` 加 403 backstop；SKILL.md 与
  source-map 同步。
- `dfd2cb06b fix(autopilot): make agent-created triggers act as the authorizing human`
  — `requireAutomationPrincipal` 沿委托链解析任务 originator 作为触发器
  `created_by`；无人类发起人 403。
- `e6f34a1a7 fix(core): ignore a 401 that answers a request from a superseded session`
  — `ApiClient.authEpoch`：三条请求路径捕获 epoch，过期 401 无副作用。

### Verification

- 全量 Go（cmd/server + internal/handler + internal/service，fix_open_findings
  DB 克隆）：4528 个测试 0 失败，-v 确认新测试名在运行。
- core：typecheck / lint（仅 2 个既有 warning）/ vitest 1748 全过。
- 审计 7 探针复验：正向探针（automation principal、stale session、execenv
  continuity）全部通过；反转探针（credentials laundering、autonomy routes、
  template access reuse）全部按预期失败（漏洞已关闭）。
- gofmt / go vet 干净；临时数据库 `fix_open_findings` 已 DROP，探针副本已清除。

### Learnings

- 沉淀到 `.trellis/spec/server/security-boundaries.md`（两条 server 契约）与
  `.trellis/spec/core/frontend/api-client-auth-epoch.md`（authEpoch 模式）。
- `pnpm --filter @multica/core exec ...` 在嵌套于主检出的 worktree 里会解析到
  主检出的包——要用 worktree 本地 `./node_modules/.bin/` 直接调用。
