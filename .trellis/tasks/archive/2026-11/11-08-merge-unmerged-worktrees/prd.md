# 整理并合并未进入当前分支的 worktree 代码

## Goal

在不丢失任何现有 worktree 改动的前提下，把真正未进入
`codex/cross-workspace-collaboration` 的代码和必要测试整理成可审查、可回滚的提交。

## Confirmed background

- 当前分支 HEAD 为 `e541cfb43`，与 `origin/codex/cross-workspace-collaboration` 同步。
- 当前根 worktree 有 315 个未提交项（53 个已跟踪、262 个未跟踪），不能直接作为合并现场。
- 仓库登记了 86 个 worktree，其中 53 个有未提交改动，另有 1 个 `prunable` 注册。
- 12 个本地/`origin` 分支的 tip 不在当前分支祖先链上；其中大量提交已经通过等价补丁进入当前分支。
- 未提交的主要代码集中在 runtime handoff、recording recovery、Knowledge UI/服务端和 E2E worktree；runtime handoff 与 recording worktree 存在大量重叠路径。

## Requirements

- R1. 在任何合并或清理前，为当前根 worktree 和所有候选 worktree 保存可恢复的提交、patch 和未跟踪文件归档。
- R2. 在独立、干净的集成 worktree 中工作；不直接在当前有脏改动的根 worktree 上做批量 merge。
- R3. 以提交可达性和 patch 等价性双重去重，跳过已在当前分支生效的实现，不把 `upstream/main` 的长期分叉历史纳入本次合并。
- R4. 对未提交 worktree 按功能边界提取并拆分生产代码、测试、文档和证据；重叠 worktree 先做三方审查再应用。
- R5. 按 runtime handoff → Knowledge → recipient/UI → E2E/诊断 → 文档/证据的依赖顺序整合，每个批次形成独立、可回滚的提交。
- R6. 每个批次通过针对性验证，最终通过仓库完整质量门禁；任何失败证据必须保留并标明未完成状态。
- R7. 在所有候选改动已合入或明确归档前，不删除分支、不清理 worktree、不推送远端。

## Acceptance Criteria

- [x] 当前根 worktree 的 315 个未提交项可以完整恢复，且没有文件因合并而丢失。
- [x] 所有候选分支/worktree 都有明确结论：已等价合入、选择性合入、归档待审或明确放弃。
- [x] runtime handoff、recording recovery、Knowledge 和 recipient/UI 改动没有因重叠路径发生静默覆盖。
- [x] 集成结果在独立 worktree 中形成逻辑清晰的提交，并可在当前分支上复现。
- [x] 运行针对性 Go/TypeScript/E2E 检查以及最终 `make check`；验证输出被记录到任务目录。
- [x] stale/prunable worktree 仅在归档和恢复证据确认后才进入清理候选，且本任务不执行远端推送或部署。

## Out of scope

- 合并 `upstream/main` 或解决 upstream 的整体历史分叉。
- 自动把所有研究、日志、截图和 `.omx` 产物当成产品代码提交。
- 在没有单独审查和测试的情况下整支合并任何候选分支。
- 推送远端、创建 PR、合并到远端主分支或部署。

## Risks and deferred decisions

- 当前根 worktree 的未提交改动与后续集成可能在少数源文件上冲突；恢复 stash 时必须保留备份并人工审查。
- `mc-knowledge-ui-qa-20261101` 规模最大，是否全部保留取决于源文件级审查和测试结果。
- 证据/文档提交是否需要进入产品分支，按验收需要逐项决定，不作为默认合并项。
