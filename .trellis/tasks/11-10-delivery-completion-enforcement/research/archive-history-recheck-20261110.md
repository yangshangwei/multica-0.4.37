# Research: 续会归档证据与未决验收责任复核

- Query: 复核四个前序归档的原始证据与规划检查点，查明 D1/D2 是否已有批准，避免重复开发或将历史验收当成新门禁交付。
- Scope: internal；只读归档、规格、源码、原始日志；唯一新增文件为本研究。未执行 Git、产品测试、数据库、容器或供应商操作。
- Date: 2026-11-10
- Worktree: `/Volumes/artisan/code/2026/mc-delivery-completion-enforcement-20261110`。
- Head provenance: 主会话报告当前两分支均为 `1ce4e81223be3397fe15e2649debfac40720ddd9`；本研究角色未执行 Git，不独立声称验证提交祖先或 HEAD blob。

## Findings

### D1/D2 均无已批准合同

当前任务 `research/decision-register.md:18` 将 D1 明确记为“已异步提问，未收到回答”；`:19` 将 skip 标为未决定，旧建议不是合同。`:28` 的一次性受限代办仅为推荐，`:30` 的另外两种方案也均未实施。

前序提示包 `research/enforcement-decisions.md:12` 明确记录 skip 推荐未收到用户回答、没有隐式批准；`:15` 将其他人、VCS 和协作发起人消费私有评估列为强制门禁前必须解决的责任模型。`:4` 的 owner/admin 政策管理权以及 `:6` 的原人类私有评估是已定边界，不自动授予代办、skip 或机器验收权。

最新六业务记录 `docs/plans/2026-11-10-business-progress-and-execution.zh-CN.md:125` 与 `journal-3.md:985`（Session 152）继续保留 D1 未答、D2–D7 未定、子任务 planning 和无产品实现。Session 149–151 的 Completed 仅指相应增量，不能覆盖 Session 152 的待决状态或关闭 M1。

### 历史证据仍完整

以当前任务 `research/archive-evidence-audit.json` 的逐项清单为输入，重新读取四包的 delivery、design、verification、独审及相关文档，完整解析其 JSON，并重算归档文档、运行记录、原始日志及被引用源码的 SHA-256。

| 归档增量 | 文档散列 | 历史运行记录及日志 | 最终记录 | 保留非零记录 |
| --- | ---: | ---: | ---: | ---: |
| completion-hints | 15 | 29 | 9 | 4 |
| policy-evaluations | 14 | 34 | 10 | 6 |
| evidence-gate | 11 | 32 | 8 | 5 |
| durable-delegation-receipts | 12 | 74 | 13 | 15 |

52 份归档文档、169 份记录及对应日志均存在且散列匹配；每条 Go JSON 日志的具名 pass/fail/skip、启动与唯一终态、记录中的测试库存也匹配。进一步复核 verification/独审 JSON 中 42 个带散列的记录引用，无异常。以上是本次静态证据复核量，不是新增测试或独立覆盖数。

当前源码与原审计记录中的工作区散列一致，涉及提示 13、政策 26、观察 6、持久委派 36 个源码引用；引用彼此重叠，不相加成唯一文件数。`prior-head-verification.json` 的提示包 13 个路径在本 worktree 中也全部匹配冻结值。旧包源码发生的后续已验收变化仍按原审计的 later-manifest 关系解释，没有新发现的源码漂移。

提示包最终历史计数仍为 handler 普通/race 各 325（89 新提示 + 236 原合同）、真实路由各 103（23 新提示 + 80 原合同）、合同 50（21 技能 + 16 其他 service Resolve + 11 状态 + 1 迁移静态 + 1 runtime）。不累加普通/race或不同轮次；额外 27 项不是 27 个状态测试。四次提示失败原样存在，只有 NUL 的预期 400/实际 503 是产品行为 red。

### 规划审查和修正留证

`research/planning-review.json:3` 明确只是文档规划检查点，`:9` 起四个实现/完成标志均为 false，`:13` 起保持 planning、M1 in_progress、M2/M3 planned。

本次复算原审查的 18 个非 context-manifest 文件和 10 个规格/研究引用；PRD 与 intake-failures 两个差异恰好是补充独审已经绑定的格式修正和失败追加。补充独审的 7 个受审文件散列全部匹配。没有读取 implement/check JSONL（research 角色隔离）。

保留了两项 intake 失败：旧会话状态路径读取错误，以及 staged diff 的 EOF 空行失败。后者的失败退出码、诊断和旧提交记录仍在 `research/staged-diff-failure.json`，并由 `planning-review-supplement.md:12` 解释；没有将其改写成产品行为 red 或重新运行该历史 Git 命令。

政策包独审中的历史规格散列与当前 `delivery-policy-evaluations.md` 不同，但该差异可精确解释：当前文件在 `## Later explicit hint consumer` 之前的 7,764 字节，SHA-256 仍为历史独审的 `628588484993129086082322da2d5ee4d42968e08fea70acb2ec8158e4f68dd4`；完整当前文件匹配后续提示包的 `025872a5e5e802cf72bfff0cb20b6f23c05ba002d80cd04471650fc793f6d18e`。这是后续提示合同追加，不是未解释漂移。

`research/local-integration.json` 仅证明上次文档检查点的集成：集成头 `723ef1b8f`、1,049 个非重叠旧路径和三个重叠文档保留。它没有声称本轮实时路径保护；本轮主目录检查与后续保护仍由主会话拥有。

## Files Found

- 当前 `research/archive-evidence-audit.md/json`：四包逐文件、运行记录、原始日志与后续源码绑定清单。
- 当前 `research/prior-head-verification.json`：提示包 13 个历史 HEAD 散列，本次仅复核当前工作区字节。
- 当前 `research/planning-review.md/json`、`planning-review-supplement.md/json`：规划状态与格式修正独审，均不授予实施或产品完成状态。
- 当前 `research/intake-failures.md`、`staged-diff-failure.json`、`local-integration.json`：原失败及有限集成范围。
- 四归档目录的 `delivery.md`、`design.md`、`verification-2026-11-10.json`、`research/independent-review.md/json`：各增量已交付行为与历史验收限度。
- 提示归档 `research/completion-writers.md`、`enforcement-decisions.md`：全部完成入口、私有评估消费与 skip 等待决定。
- `docs/plans/2026-11-10-business-progress-and-execution.zh-CN.md:113`、`:123`：最新提示增量和规划检查点，优先于早期“未实施”记载。
- `.trellis/workspace/artisan/journal-3.md:902`、`:924`、`:950`、`:976`：Session 149–152。

## Code Patterns

- `server/internal/handler/delivery_evaluation.go:78`：API 仅接受当前 human principal。
- 同文件 `:151`：lookup 绑定 workspace、经认证的当前人类 UserID 与 request key；不是只持 key 即可读取。
- 同文件 `:201`：恢复原评估走当前授权重验；原观察保持不可变，不因此成为完成许可。
- `server/pkg/db/queries/delivery_policy.sql:42`：数据库查询同样要求 `workspace_id + evaluator_user_id + request_key`。

这些代码事实支持保留评估隐私和当前权限边界；它们无法回答未来应由谁承担最终验收责任，不能据此替用户选择 A/B/C。

## Related Specs

- `.trellis/spec/server/delivery-policy-evaluations.md:25`：owner/admin 政策管理；`:36` 至 `:48`：原人类 namespace、禁止他人/机器读取、completion permission 永远 false。
- `.trellis/spec/server/delivery-completion-hints.md:27`：成功提示两个许可字段均 false；`:62`：skip、跨主体消费需独立合同。
- `.trellis/spec/server/delivery-evidence-observation.md`：数据库 PR 观察、当前权限与实际制品绑定，不是远程实时 Git HEAD 或完成许可。
- `.trellis/spec/server/durable-delegation-receipts.md`：unknown 不重派、原身份恢复、queued/recorded 不是完成证明。

## External References

无外部网络引用。仅使用本地仓库和被归档引用的本机原始日志。历史 PostgreSQL 17 amd64/arm64-host 记录仅支持既有功能证据，不推导性能或本轮环境结论。

## Caveats / Not Found

- 未发现批准 D1/D2 的材料；用户当前消息亦明确未回答，两者仍需业务答复。
- 未新增或重跑 Go、前端、DB、浏览器、性能、部署或真实模型测试；本研究新增产品验收数为 0。
- 未连接或查询历史数据库，未检查或更改容器运行状态；历史清理记录不是本轮实时数据库清点。
- 原始日志仍只是本机可读取证据，存在和散列匹配不代表远端已归档。
- 不改变真实供应商停止规则或预算；不关闭子任务、M1、M2/M3 或六项业务。
