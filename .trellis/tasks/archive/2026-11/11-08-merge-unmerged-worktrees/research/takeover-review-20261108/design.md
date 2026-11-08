# 技术设计：未合并分支与 worktree 整合

## 集成边界

本任务把 Git 对象、未提交文件和产品提交分成三个边界：

1. **来源边界**：现有根 worktree、已命名分支、detached worktree。来源目录在提取阶段只读，不在原地重写历史。
2. **集成边界**：从 `e541cfb43` 创建的干净临时分支和 worktree。所有 patch 应用、冲突解决、测试和工作提交都在这里完成。
3. **落地边界**：集成分支验证通过后，暂存当前根 worktree 的现有改动，再把已验证提交合并回 `codex/cross-workspace-collaboration`，随后用 `git stash apply` 恢复原有工作。

## 来源分类

### 已等价进入当前分支

`codex/knowledge-codex-20261104`、`codex/collaboration-final-owned-20261103`、
`codex/collaboration-fixture-repair-20261103` 以及 recipient picker/运行时验收中的大部分提交，
用 `git log --cherry-mark` 标记为等价。它们不做整支 merge；只检查其 worktree 是否还有未提交文件。

### 需要选择性审查的提交

- recipient picker 的新增 E2E/证据提交；
- context experiment 测试；
- CI/spec 验证提交；
- runtime/knowledge 证据和研究提交。

空基线提交直接归档。`upstream/main` 作为长期分叉保留，不纳入此次整合。

### 未提交代码批次

| 批次 | 来源 | 处理策略 |
| --- | --- | --- |
| Runtime handoff | `mc-handoff-launch-20261031`、`mc-handoff-cursor-20261101`、`mc-runtime-recording-recovery-20261101` | 以当前 HEAD 为基准，按协议/服务端、ledger/recording、client/UI、测试分层；对 23～60 个重叠路径做三方审查 |
| Knowledge | `mc-knowledge-ui-qa-20261101`、`mc-knowledge-codex-20261104` | 先生产代码，再测试；研究和证据单独归档 |
| Recipient/UI | `mc-recipient-integration-20261030-072838-a3fe` | 核心实现已在当前分支，优先提取缺失测试和 fixture，生产改动逐文件确认 |
| E2E/诊断 | `mc-core-e2e-1a32ca36c-20261104`、`mc-acceptance-repair-20261107`、`mc-context-audit` 等 | 在产品批次稳定后合入，保持原始失败证据 |

## 数据流

```text
现有 worktree（只读）
  ├─ tracked diff ───────────────┐
  ├─ untracked archive ──────────┤
  └─ branch/HEAD inventory ──────┤
                                  v
                     干净集成 worktree
                         ├─ dedupe
                         ├─ batch apply / conflict review
                         ├─ targeted tests
                         └─ logical commits
                                  v
                    当前根 worktree（stash → merge → apply）
```

## 冲突与回滚

- 不使用整支 `git merge` 解决历史分叉；已提交内容用选择性 cherry-pick，未提交内容用二进制 patch/未跟踪归档。
- 每个批次在应用前记录来源 HEAD、来源 patch SHA256 和 changed-path 清单。
- patch 应用优先使用三方策略；若同一文件同时来自 handoff、recording、Knowledge，则暂停该文件的自动应用，按当前 HEAD、来源 HEAD、来源工作树三方比较。
- 每个批次单独提交，出现失败时回退到上一个批次提交，不重置原始来源 worktree。
- 根 worktree 的 stash 只在集成分支验证通过后创建；恢复使用 `git stash apply`，保留 stash 直到冲突和测试全部解决。

## 兼容性与验证

- 保留已有 collaboration protocol/version 分支和当前默认关闭的 host admission 语义。
- 不因合并方便而放宽权限、替换 Provider 身份、改变数据库迁移规则或降低 E2E 证据要求。
- E2E 运行必须使用任务拥有的 API/Web/Provider 环境，记录真实命令、端口和提交身份。

## 完成形态

最终交付是一组可审查的工作提交，以及一份来源决策清单：每个分支/worktree 标为 `merged`、`patch-equivalent`、`archived` 或 `dropped`。本任务不推送远端、不部署。
