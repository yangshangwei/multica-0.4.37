# 执行计划：未合并分支与 worktree 整合

## 0. 任务和现场确认

- [x] 记录当前任务、分支、HEAD、origin 同步状态和根 worktree 的完整 dirty manifest。
- [x] 保存审计报告到 `research/branch-worktree-audit.md`，记录 86 个 worktree、53 个 dirty worktree、1 个 prunable 注册和候选来源清单。
- [x] 不在当前根 worktree 写产品代码；所有实验先在独立集成 worktree 进行。

## 1. 保护来源

- [x] 创建临时归档引用，指向每个候选分支和 detached HEAD。
- [x] 对每个 dirty worktree 导出 tracked binary patch，并归档未跟踪文件；记录 SHA256、来源 HEAD 和路径清单。
- [x] 检查归档可以列出并恢复，确认后才允许后续应用 patch。
- [x] 暂不清理 `prunable` worktree，也不删除任何分支。

## 2. 创建集成 worktree

- [x] 从当前 HEAD 创建 `codex/merge-unmerged-worktrees-20261108` 和对应的干净 worktree。
- [x] 设置任务 branch/scope，确认集成 worktree 初始状态 clean。
- [x] 运行 `git log --cherry-mark --right-only`，生成 committed-source decision ledger。

## 3. 处理已提交来源

- [ ] 跳过已 patch-equivalent 的实现提交，不重复 cherry-pick。
- [ ] 逐项审查并选择性 cherry-pick context experiment、recipient E2E、CI/spec 和必要证据提交。
- [ ] 对每个选择记录来源 commit、理由、冲突解决和测试结果。
- [ ] 将空基线和仅有重复证据的来源标记为 dropped/archived。

## 4. 合并 Runtime handoff 批次

- [x] 先比较 handoff-launch、handoff-cursor、runtime-recording-recovery 的三方 changed-path 清单。
- [x] 依次整合协议/服务端、ledger/recording、client/API/UI 和测试；不要重复覆盖 23～60 个重叠路径。逐路径审查后确认这些快照均已存在于当前 HEAD，旧内容会回退当前更严格行为，因此全部 kept-current。
- [x] 每个逻辑组提交一次，提交前运行 `git diff --check` 和相关 Go/TypeScript 测试。该批次没有可安全提交的差异，集成 worktree 保持 clean。

## 5. 合并 Knowledge 批次

- [ ] 从 `mc-knowledge-ui-qa-20261101` 提取生产源文件，排除研究/证据/生成物。
- [ ] 应用 `mc-knowledge-codex-20261104` 中当前缺失的测试或控制改动，跳过已等价提交。
- [ ] 验证 Knowledge/Codex 服务端测试、客户端类型检查和相关 E2E。

## 6. 合并 Recipient/UI 和 E2E 批次

- [ ] 审查 recipient integration 的 39 个源文件差异，只应用当前缺失的功能测试和 fixture。
- [ ] 在产品代码稳定后应用 core E2E、acceptance repair、context audit 等测试改动。
- [ ] 保留原始失败证据，不能用修复后的运行结果覆盖旧证据。

## 7. 集成回当前分支

- [ ] 集成 worktree 全部检查通过后，重新导出并校验根 worktree dirty manifest。
- [ ] 在根 worktree 创建带名称的 `stash -u` 备份，记录 stash SHA；不删除该 stash。
- [ ] 把已验证的集成提交合并到当前分支，使用 `git stash apply` 恢复原有改动。
- [ ] 逐项解决恢复冲突；不把未识别的旧改动静默纳入产品提交。

## 8. 最终质量门禁

- [ ] 运行目标 Go 测试、`pnpm typecheck`、`pnpm test`、目标 E2E。
- [ ] 运行完整 `make check`，读取并记录所有输出。
- [ ] 再次运行 branch/worktree inventory，确认所有候选来源都有明确结论。
- [ ] 运行 `git diff --check`，确认没有意外未跟踪源文件或未解决冲突。

## 9. 收尾

- [ ] 更新必要的 `.trellis/spec/` 规则或明确记录“不需要更新”。
- [ ] 按 Lore Commit Protocol 创建工作提交和任务记录提交，不推送远端。
- [ ] 只有在归档和恢复证据确认后，才把 stale/prunable worktree 标记为可清理；清理动作另行执行。

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
