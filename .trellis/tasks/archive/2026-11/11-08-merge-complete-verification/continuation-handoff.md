# 新会话接续引导：合并后完整测试与近期业务影响验收

你正在接续 Trellis 任务：
`.trellis/tasks/11-08-merge-complete-verification`

先运行：

```sh
python3 ./.trellis/scripts/get_context.py
python3 ./.trellis/scripts/get_context.py --mode phase --step 2.2 --platform codex
```

阅读任务目录中的 `prd.md`、`design.md`、`implement.md`、`implement.jsonl` 和
`check.jsonl`。任务已是 `in_progress`，不要创建新任务、不要重置根 worktree、不要
清理其他分支/worktree，也不要修改产品代码。

当前身份和边界：

- 根分支：`codex/cross-workspace-collaboration`
- 根 HEAD：`ee037d4ee3c4e67598e0767c056a8244a7a1ccbf`
- 干净候选 worktree：`/Volumes/artisan/code/2026/mc-merge-unmerged-worktrees-20261108`
- 产品候选 SHA：`52162f19af7e4a1e2459ce3f08e4627ca14c3dfc`
- 当前 HEAD 与候选在 `apps/`、`packages/`、`server/`、`e2e/`、`scripts/` 和构建配置上
  无差异；根 worktree 的大量 dirty/untracked 内容属于其他任务，必须保持原样。

已经通过的检查：

- lint/typecheck：15/15；UI wildcard exports：通过。
- TypeScript targeted：Web 14/14、Views 37/37、renderer/check 脚本回归通过。
- TypeScript 全量（非 mobile）：Core 331 文件/4464 测试，Docs 10/64，Views 599/7287，
  Web 34/290，Desktop 102/1272，全部通过。
- targeted Go race：auth capability、handler runtime lookup、service capability、daemon
  watchdog、Context experiment（`-tags=agentintegration`）和 RK1 continuation 全部通过。
- Go 全量 race：regular packages 全部通过；`internal/service` 1290.621s；`pkg/agent`
  327.264s。`go vet -p 2 ./...` 通过。第一次总脚本的非零码是 zsh 外层变量名
  `status` 冲突，agent 包已用正确 bash wrapper 重跑并通过。

完整门禁结果：

- `make check` 在独占环境 `check-20261108112551-74475` 执行；静态、TS、Go、API、生产
  Web、Desktop renderer 均通过。
- Playwright：441 个测试，327 passed、112 skipped、2 failed，退出码 1。
- 原始日志：
  `.trellis/tasks/11-08-merge-complete-verification/make-check.log`
  SHA256 `10221e3f66435befa5c15ff43813576e3733e20cd8e04774cebabe86155c1d71`。
- 两个原始失败：
  1. `e2e/iterations-i1-desktop.spec.ts:33`：`e2e/fixtures/iterations-i1.ts:82`
     等待 assignment dialog 隐藏超时；Web 同类链路通过。
  2. `e2e/collaboration-workbench-web.spec.ts:183`：stale lifecycle conflict 场景等待
     lifecycle POST response 超时，页面显示 Waiting for the server。
- 两个失败都在同一候选构建、同一独占数据库/端口下单独复跑并通过：
  `targeted-rerun.log`，2 passed，19.2s，SHA256
  `b50c28a814a8a91bd91221c697a88ffa026e48880b9ca03cfc70c8e3d20cee37`。
  结论应写成“原始完整门禁保留 2 个时序敏感失败；单独复跑未复现稳定业务回归”，不要
  把完整门禁写成全绿。

下一步只做收尾：

1. 将原始失败上下文（候选 worktree 的 `test-results/.../error-context.md`）复制或摘要到
   当前任务 `research/`，保留原始失败证据和 targeted rerun 证据。
2. 创建/更新 `progress.md` 和 `final-report.md`，按 Collaboration、权限/Task capability、
   Daemon/Context/Knowledge、Projects/导航、UI/构建五个业务域总结 PASS、SKIP、FAIL/未复现。
3. 记录当前候选环境、端口/数据库、退出码、日志 SHA256，以及 112 个条件跳过的边界；
   明确 mobile、真实 provider、密码/平台专属和部署场景未覆盖。
4. 检查候选 worktree clean、根 worktree 原有 dirty 内容未改变；不要把构建生成的
   `apps/web/next-env.d.ts` 变更带入结果。
5. 按 Trellis Phase 3 做最后质量检查、必要的 spec 更新（通常只需记录无需改 spec），再
   依照 Lore Commit Protocol 只提交本任务证据文件；不要提交产品代码或根目录其他任务。
