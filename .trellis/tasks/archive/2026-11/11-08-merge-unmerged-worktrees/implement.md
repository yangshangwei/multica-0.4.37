# 执行计划：未合并分支与 worktree 整合

## 0. 任务和现场确认

- [x] 记录当前任务、分支、HEAD、origin 同步状态和根 worktree 的完整 dirty manifest。
- [x] 保存审计报告到 `research/branch-worktree-audit.md`，记录 86 个 worktree、53 个 dirty worktree、1 个 prunable 注册和候选来源清单。
- [x] 不在当前根 worktree 写产品代码；所有实验先在独立集成 worktree 进行。

## 1. 保护来源

- [x] 创建临时归档引用，指向每个候选分支和 detached HEAD。
- [x] 对每个 dirty worktree 导出 tracked binary patch，并归档未跟踪文件；记录 SHA256、来源 HEAD 和路径清单。
- [x] 检查归档可以列出并恢复（已验证逐文件内容、补丁可应用性；未执行完整目录恢复演练），确认后才允许后续应用 patch。
- [x] 暂不清理 `prunable` worktree，也不删除任何分支。

## 2. 创建集成 worktree

- [x] 从当前 HEAD 创建 `codex/merge-unmerged-worktrees-20261108` 和对应的干净 worktree。
- [x] 设置任务 branch/scope，确认集成 worktree 初始状态 clean。
- [x] 运行 `git log --cherry-mark --right-only`，生成 committed-source decision ledger。

## 3. 处理已提交来源

- [x] 跳过已 patch-equivalent 的实现提交，不重复 cherry-pick。
- [x] 逐项审查并选择性 cherry-pick context experiment、recipient E2E、CI/spec 和必要证据提交。
- [x] 对每个选择记录来源 commit、理由、冲突解决和测试结果。
- [x] 将空基线和仅有重复证据的来源标记为 dropped/archived。

## 4. 合并 Runtime handoff 批次

- [x] 对每个来源以来源 HEAD → 工作树的增量为起点审查；路径存在、HEAD 更老不构成已覆盖的证据。
- [x] 优先检查快照 blob 是否已在当前 HEAD 历史中出现，记录对应路径/提交；其余差异逐行为映射到当前实现和测试。
- [x] 为每个候选记录 merged、equivalent、superseded（具体证据）或 deferred（具体原因），未解决的差异保持开放。
- [x] 仅应用证明确实缺失的行为，保留当前更严格校验；有变更的逻辑组运行针对性验证并独立提交。

## 5. 合并 Knowledge 批次

- [x] 审查 Knowledge UI58条差异；当前实现已覆盖，生产源文件无需重复提取，证据单独保留。
- [x] 应用 `mc-knowledge-codex-20261104` 中当前缺失的测试或控制改动，跳过已等价提交。
- [x] 验证 Knowledge/Codex 服务端测试、客户端类型检查和相关 E2E。

## 6. 合并 Recipient/UI 和 E2E 批次

- [x] 审查 recipient integration 的 39 个源文件差异，只应用当前缺失的功能测试和 fixture。
- [x] 在产品代码稳定后应用 core E2E、acceptance repair、context audit 等测试改动。
- [x] 保留原始失败证据，不能用修复后的运行结果覆盖旧证据。

## 7. 集成提交的落地前质量门禁

- [x] 在干净集成 worktree 的固定 SHA 上运行目标 Go/TS/E2E 检查以及完整 `make check`。
- [x] 保存命令、环境身份、退出码和日志；确认全部候选有证据支持的结论。
- [x] 由独立 check 代理审查最终增量和来源决策，修复发现后重跑受影响检查。

## 8. 集成回当前分支

- [x] 集成 worktree 全部检查通过后，重新导出并校验根 worktree dirty manifest。
- [x] 落地前核对 root dirty/untracked 路径与集成差异无重叠，保存刷新后的内容哈希和索引；无重叠时优先直接 `merge --ff-only --no-autostash`，避免不必要移动原始工作。有重叠才使用已归档的 `stash -u`，记录 SHA 并保留。
- [x] 把已验证的集成提交快进合并到当前分支；若确实创建了 stash，使用 `git stash apply` 恢复并保留 stash。
- [x] 对照落地前清单逐文件核验原始改动和暂存状态未丢失；若有恢复冲突，逐项处理，不把旧改动静默纳入产品提交。

## 9. 合回后的根工作区验证（按已批准接续更新）

- [x] 核验根 HEAD 等于完整门禁通过的固定候选，所有集成产品路径字节和 mode 与候选完全一致。
- [x] 对照落地前清单逐文件核验原始 dirty/untracked 内容、权限和暂存差异保持不变；对照09:47来源归档确认无关原始内容保全。
- [x] 保存完整门禁退出码、整份日志失败扫描、生产 Web/API 的候选/构建/进程身份和清理结果。完整门禁在干净集成目录执行，不把无关 dirty 根目录测试作为候选验证。
- [x] 核对原登记来源 worktree/分支仍保留，并记录全部候选的已有决策证据，不重新审计已完成来源。
- [x] 检查集成增量 `git diff --check` 及未解决冲突；根目录原有未跟踪来源保持原样。

## 10. 收尾

- [x] 更新必要的 `.trellis/spec/` 规则或明确记录“不需要更新”。
- [x] 按 Lore Commit Protocol 创建工作提交和任务记录提交，不推送远端。
- [x] 只有在归档和恢复证据确认后，才把 stale/prunable worktree 标记为可清理；清理动作另行执行。

## 验证命令

```bash
git status --short --branch
git log --cherry-mark --right-only HEAD...<candidate>
git diff --check
pnpm typecheck
pnpm test
make test
make check
```

## 主要回滚点

- 来源归档完成后、patch 应用前；
- 每个 Runtime/Knowledge/Recipient/E2E 批次提交后；
- 根 worktree stash 创建后、stash apply 完成前；
- 最终质量门禁失败时保留集成分支，不覆盖任何来源 worktree。

## 接管纠正（2026-11-08）

- 用户明确授权接管已批准的现有任务，不创建新任务。
- 旧审核记录已保存于 `research/takeover-review-20261108/`，不覆盖历史证据。
- 旧“全部备份完成”和 Runtime 全量 kept-current 的完成标记撤销，重新基于实物证据验收。
- 生产改动仅限现有候选中经证明缺失的增量；来源目录只读，不扩大为功能重构。
- 根工作区现有产品/文档/研究改动属于其他未完成任务，单独保护，不纳入本任务产品提交。

## Final execution evidence (2026-11-08)

All acceptance steps completed against product candidate52162f19a. Complete gate06 passed and root fast-forward preserved all original work. The full candidate was tested in the clean integration checkout; the unrelated dirty root overlay was deliberately preserved, not certified by this gate. No stash was needed. Source cleanup candidates remain retained; no deletion is part of this completion.

Spec review: existing integration commits updated input/collaboration/context/E2E contracts. The final watchdog correction changes no public contract; its lifecycle boundary is executable in the regression and explained beside the implementation and in research/watchdog-fix.md. No additional spec edit is needed, especially not to the root's unrelated dirty server specs.

Task archival uses --no-commit followed by an explicit record allowlist and Lore commit. A new independent journal records this session so the original dirty journal/index stay byte-identical. Raw logs/patches/tools remain preserved with external snapshot hashes, not folded into product commits.
