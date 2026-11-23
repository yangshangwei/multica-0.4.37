# Research: F1/F2 修订后最终规划复核

- Query: F1 的 Switch 例外、F2 的全局优先级与有界回绕是否符合当前合同和源码；是否仍有阻止 S1 开工的规划缺陷。
- Scope: internal；使用主会话提供的 Active task 路径，仅复核当前规划、原审阅及相关源码/规范；S2–S6 只检查已有结论的一致性。
- Date: 2026-10-21

## Findings

**结论：F1、F2 均关闭；本次范围内没有待修正的实施阻塞项。规划已可提交用户实施审阅。** 这是规划结论，不是实现完成或运行验证结论。

### F1 — Closed：Switch 只移动尚未入组的 waiting 输入

- 修订：`design.md:78` 明确保留已消费快照、已取消/已停止输入及历史 membership，仅原 R26/M06 Switch 能给尚未入组的 waiting 输入分配新通道序号；`implement.md:41` 覆盖 Switch 后取消的当前身份、历史留存和取消不升级为 Stop。
- 原合同：父 `prd.md:48`（R26）、`:99`（M06）与 `acceptance.md:48`（AC40）允许换队长后的等待输入移到新通道末尾，同时保持原 typed target、历史、暂停与明确恢复。
- 源码符合这一边界：`server/internal/service/collaboration_authority.go:635` 读取 waiting；`:644` 拒绝已有 group 的输入；`:653`、`:657` 在目的通道分配序号并移动。`server/pkg/db/queries/collaboration_batches.sql:57` 按原 `seq,id` 读取，`:69`–`:70` 只更新 `state='waiting' AND group_id IS NULL`；`:73`–`:75` 将旧消费关联作为独立历史保留。
- 规范也明确 waiting 保序移到尾部，旧取消输入与 group/run/proof 历史不移动（`.trellis/spec/server/collaboration-targets.md:210`–`:217`；`collaboration-switch-and-skills.md:43`–`:50`）。修订没有赋予公平扫描或手动取消新的重排权，也没有禁止合法返回 waiting 的当前输入迁移而保留其旧消费快照。

### F2 — Closed：先确认全局优先等级，再在等级内轮转

- 修订：`design.md:24`–`:26` 明确 reservation 优先、不受候选页截断的派生存在性查询、低等级放行前完成全作用域优先检查与有界回绕；扫描未完成不得用低等级补名额，新出现/变化的 reservation 必须重新纳入。daemon 只能在服务端确认的同等级内轮转（`:26`、`:30`）。
- 基线：`server/pkg/db/queries/collaboration_runtime.sql:32`–`:47` 将未释放 reservation 排在前面，并过滤撤权、失效参与代次、held dependency、forced/settled 等记录；`:60`–`:68` 保留实际 namespace 占用。`server/internal/service/collaboration_runtime.go:610`–`:629` 先验占用，再以派生候选延后新根。新分页不能把当前页充当完整派生集合，也不能让游标绕过真实占用。
- 回归计划已覆盖原审阅要求：`implement.md:29` 包含游标越过 reservation、优先者在页外、新出现/变化的优先者、多 poller/重启、同等级合法后继有限进度及撤权派生不永久阻塞。它与父执行契约 `research/execution-contract.md:15`、`:24`–`:32` 的完整准入、实际容量和公平性一致。
- 落实时仍须遵守已写出的约束：可选 wire 元数据必须与 `run_ids` 精确对应，无法确认等级时不能假定整个旧列表同级后任意轮转；完整存在性检查使用当前有效谓词。这里是 `design.md:26`、`:30` 的实现义务，不是新增产品选择或未关闭规划问题。

### S1 安全边界与其余切片

- `design.md:28`、`:30`–`:34` 及 `implement.md:30`–`:33` 保留局部错误与全局错误的区分、unknown commit 原身份恢复、损坏/unknown 容量屏障、原 proof/nonce/receipt 和显式 Resume。与 `.trellis/spec/server/collaboration-targets.md:133`–`:159`、`:178`–`:186` 及 `collaboration-runtime-cli.md:47`–`:61` 一致。持久 cursor 的具体表/sidecar、锁边界和错误分类须在对应切片落实，没有需要先改变的安全合同。
- 对照 `research/planning-review.md:26`–`:32`，S2 的提供/采纳/逐 owner consent（`design.md:38`–`:44`）、S3 的人类 `@all` 与准确取消历史（`:48`–`:54`）、S4 的独立 provider/OS 证据（`:58`–`:64`）、S5 的有限代管（`:68`–`:74`）未引入相反约束。S6 保留全部父验收与发布 gate（`implement.md:52`–`:53`），没有以 unsupported 或历史增量宣称完整交付。

## Files Found / Related Specs

下列路径以仓库根目录为基准；上文裸 `design.md` / `implement.md` 指本子任务，父文件指 `.trellis/tasks/10-10-cross-workspace-collaboration/`。

| 文件 | 用途 |
| --- | --- |
| `.trellis/tasks/10-20-collaboration-gap-recovery/research/planning-review.md` | 原 F1/F2 与既有 S2–S6 结论 |
| `.trellis/tasks/10-20-collaboration-gap-recovery/design.md`、`implement.md` | 当前修订及回归义务 |
| `.trellis/tasks/10-10-cross-workspace-collaboration/prd.md`、`acceptance.md`、`research/execution-contract.md` | R26/M06/AC40、准入与停止证明原合同 |
| `server/internal/service/collaboration_authority.go`、`server/pkg/db/queries/collaboration_batches.sql` | Switch 输入迁移与消费历史边界 |
| `server/internal/service/collaboration_runtime.go`、`server/pkg/db/queries/collaboration_runtime.sql` | 当前 reservation、派生准入、截断与错误边界 |
| `server/internal/daemon/collaboration.go` | 当前固定 proof 预算、首候选选择和未知容量保留 |
| `.trellis/spec/server/collaboration-targets.md`、`collaboration-switch-and-skills.md`、`collaboration-runtime-cli.md` | 队列、历史、Switch、实际容量及 proof 规范 |
| `CLAUDE.md`、`.trellis/workflow.md` | 仓库规则及规划审阅/实施边界 |

## Caveats / Not Found

- **尚未实现的行为：** SQL 仍有固定 `LIMIT 16`（`collaboration_runtime.sql:47`、`:56`）；daemon 仍固定选择首个未 blocked 候选（`collaboration.go:207`–`:211`），proof 仍从库存前缀最多尝试 8 项（`:131`–`:150`）。这些是 S1 的实施目标，没有因本审阅而修复。
- 未运行产品测试、构建、数据库或真实 provider/OS 验证；未重新验证 97 行矩阵或历史测试结果。`prd.md:24` 已明确旧测试属于 10-20 记录。本次不把它们计作新证据。
- 未读取 `implement.jsonl` / `check.jsonl`，不独立宣称 manifest 或 native injection 验证通过；该检查由协调者负责。
- 无新增可操作的规划阻塞。实际启动仍由主会话完成最新摘要审阅、实施确认与 `task.py start`（`implement.md:8`、`:92`；`.trellis/workflow.md:455`、`:466`），本审阅没有启动、归档、提交或更改任何产品文件。
- External references: none；仅依据当前仓库合同与源码，不引入外部版本/API 假设。
