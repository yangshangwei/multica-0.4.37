# Research: 遗漏恢复规划独立复核

- Query: 复核当前 PRD/design/implement/acceptance-matrix 的资源权限、@all、取消 provenance、恢复游标安全和执行顺序。
- Scope: internal；只读当前任务规划、runtime-recovery 研究及已取得的父合同/代码证据；唯一更新为本文件。
- Date: 2026-10-20
- Reviewed: `prd.md`、`design.md`、`implement.md`、`acceptance-matrix.json`、`research/runtime-recovery.md`；父合同/源码锚点沿用 `research/product-gaps.md`，未重新穷举源码。

## Findings

结论：产品分工和范围已收敛，无新增用户产品选择。以下两处应在实施前作最小规划修正；它们是可由技术设计解决的问题，不是需要用户另授权的产品阻塞。

### F1 — Medium：全局“command sequence 不能改写”与已接受 Switch 行为冲突

- 当前 `design.md:76` 将 command sequence 与已冻结正文/proof 一并规定不能改写。
- 父 `acceptance.md:48`（AC40/R26）要求换队长时 waiting 输入进入新队长通道末尾。已核对的 `server/internal/service/collaboration_authority.go:635`、`:653`、`:657` 明确只为 waiting 输入分配新 lane sequence 并移动；取消/已消费输入保留原通道及历史。这是现有已接受行为，不是本轮新增重排权。
- **最小修正：** 把 `design.md:76` 的序号不变限定为已消费快照、已取消/已停止输入及历史 membership；明确只有经原 R26/M06 Switch primitive 的 waiting 输入可按合同迁入新通道末尾，且保留原历史关系。手动取消仍不得修改自己的或其他等待项 FIFO。
- **验证补句：** `implement.md:40`–`:41` 的历史/队列验证覆盖一次 Switch 后取消的准确当前身份与旧消费历史留存；不得因本轮一般序号禁令删除现有合法 Switch。

### F2 — Medium：游标回绕必须明确先完成全局 reservation 优先检查

- `design.md:22` 采用持久 keyset+回绕；`:24` 将 reservation/派生/创建顺序合入 candidate key；`:28` 要求 daemon 在候选优先级内轮转，但未定义游标位于高优先项之后或优先级发生变化时的查询/回绕约束。
- `research/runtime-recovery.md:17`–`:18` 和 `:86`、`:88` 明确当前 reservation 持有者优先是安全/进度前提，真正 pending 派生的判断不能依赖截断候选页。简单执行 `key > cursor` 会在回绕前遗漏位于游标前的 reservation 持有者，并先返回 lower-priority 项；daemon 的独立尝试位置也可能再次打乱这个优先关系。
- **最小修正：** 增加可执行不变量：优先级选择先于该等级内公平轮转；每次给较低等级机会前，须按当前授权/状态对整个作用域检查并完成 reservation 优先候选的有界回绕，而非只检查当前 page。轮转元数据不能授权越过 reservation 或释放真实 namespace；派生存在性查询使用与原准入合同一致的完整过滤。具体采用分等级 cursor 或等价检查由实现研究决定。
- **验证补句：** 在 `implement.md:29` 增加“cursor 已越过 reservation key、新出现/变化的 reservation、持有者在页外、多个 poller/重启”用例；断言优先者得到机会、无新根抢占，满足真实独立容量的同等级后继仍有限轮次被尝试。不要只验证首屏排序。

### 已确认一致的部分

- **S2 资源提供/采纳是技术机制，未改 M03/M05。** `design.md:36`–`:42` 保留原 inventory 保护；普通参与 owner 仅提供明确资源，不创建 target；creator/handler 采纳后各 actual owner 独立 consent，所有 owner 先参与，未齐不进入 @。采纳前提供撤回与采纳后 grant 撤回分开。需要实现时固定提供记录的 read/write ACL、完整 roster 可见性和过期拒绝 DTO，但无需用户选择更宽的目录或授权方案。
- **@all 范围正确。** `prd.md:28`、`:35`、`:53` 与 `design.md:46` 保持 all/all 特例只指当前有效人类，和显式 AI 独立；同事件去重、原草稿/权限、编辑仅新增人员均保留。
- **取消 provenance 正确。** `design.md:48`–`:52` 采用当前 ACL 的独立 immutable input history；manual/auto state+reason+精确事件同事务；历史缺 actor/time 则为空而非猜测。suppressed 没有 command、M11 unread 可 waiting cancel、claimed/in-flight 走停止均明确。来源 read basis 不授予取消权，原键恢复不会重做取消。
- **Proof 方案安全方向正确。** `design.md:30`–`:32` 的独立 sidecar、私有权限/跨进程锁/fsync、尝试前记录位置、每轮 8 项及共享 deadline 保留原 proof/nonce/receipt；错误/ErrBusy 推进尝试位置不 ACK、不清容量、不自动 Resume。实现仍需确定 sidecar 的 deployment/installation 作用域、崩溃/损坏处理和并发锁边界，属于实现研究；不得把游标当 ledger 权威。
- **顺序及账目正确。** S1 先恢复安全基础；S2/S3 独立、S4/S5 依稳定控制/授权/lifecycle，S6 承担集成；共享 SQL/router/API/locale/schema 单一协调者整合（`design.md:11`–`:18`、`implement.md:17`–`:24`）。矩阵 97 个 ID 唯一，全部 `not_verified_in_this_task`，没有用旧增量冒充当前完整验收；AC15/16/96=P4、AC19/20/57/76/95=P5、AC86/90/97=P3 与父主归属一致。

## Caveats / Not Found

- 未写产品代码/测试、未启动任务、未修改规范或其他任务，未运行产品验证。F2 是规划缺少决定性不变量，不是本轮已复现的运行时错误。
- 资源 offer 的 API/ACL、单对象错误分类、候选优先分页和 recovery cursor 锁实现仍需研究；规划已分配相应前置。没有把这些实现未知提升为用户产品决策。
- 真实 provider/OS、password/admin、打包 Main、S3/Redis 与父 97 项仍需各自真实证据；本审阅不宣称它们已完成。
- `acceptance-matrix.json` 为 80,047 bytes，超过单个自动注入文件 65,536 bytes。它可作为主会话台账；如需要纳入自动上下文，应按切片提供小摘要/分段，主会话检查 manifest。研究角色未读取 implement/check JSONL，故不确认 manifest 已通过。
- External references: none；本复核以仓库合同、任务研究和本轮规划为依据。
