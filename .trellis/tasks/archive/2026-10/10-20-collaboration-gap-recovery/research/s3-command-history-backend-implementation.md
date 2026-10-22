# S3b: 不可变命令历史与精确取消溯源后端

状态：后端实施与专项验证完成，交主会话独立检查及共享客户端集成。范围仅为 AC-I05 后端；没有创建/执行开关、真实 provider、发布或父任务完成声明。

## 已确定的接口与存储

- `GET /api/collaboration/issues/{id}/commands`，handler `GetCollaborationCommandHistory`，service `CommandHistory`。
- 协议 1：`issue_id`、`items`、`next_cursor`；limit 默认 50、1–100，游标不超过 1024 字符，绑定本单及实际存在的 `created_at/id`，按两字段降序分页。重复/遗漏/未知字段和跨单游标拒绝。
- 每项保留 command/session/target/input-event/channel/batch/current-group/seq、原作者、原文、附件 ID、state/reason/created_at。typed target 从保留的 target 身份读取，不读私人资源配置。原评论链接仅来自本单精确 input event。
- `cancellation` 在非取消状态为 null；取消但旧证据缺失时是字段全 null 的对象；已知字段为 event_id/event_seq/actor_user_id/actor/created_at/kind（manual 或 automatic）。author/actor 的 ID 独立保留，不依赖当前账号摘要仍存在。
- 每页借用 `readCollaborationContent` 的当前账号及独立来源成员/参与者读取基准；历史 DTO 不授予 cancel/control 权限。
- 迁移 735 在 command 上仅增加 nullable `cancellation_event_id`，不引入新的历史账本。新取消 state/reason、精确事件与指针关联共享调用者事务。原 M11/非启动证明/输入搬运 guard 保持原样，另加取消指针、终态 reason 及取消事件不可变 guard。
- 老手动事件仅在同单、精确 command ID 的候选唯一且 action/code/target/channel/batch/seq 完全相符时回填。旧批量事件及邻近时间不用于推断 actor/time。
- 迁移 736 的独立 concurrent index 为 `collaboration_command_history_idx (session_id, created_at DESC, id DESC)`；协调者已登记清理并生成 SQL、接路由、迁移专用库。

## 取消写入覆盖

`recordCommandCancellation` 和 `recordCommandCancellations` 只使用真实受信请求 principal 记录人类操作者，绝不使用可能代表输入作者的 `t.actor`。无实际人类请求的 daemon/reconciler 事件保留 null actor。

覆盖手动 waiting cancellation、remove/leave、lifecycle close、旧账号失效、promotion stale authority、M11 unread detach、sealed nonlaunch return 以及 terminal batch cancellation。批量 SQL 返回实际修改的 command 行，每条记录对应一个独立精确事件；重复调用/原键恢复不会重复事件。取消、Stop、suppressed、claimed 及两个 pause 层仍分别保留原语义。

## 验证记录（持续更新）

所有数据库测试由协调者分配的隔离 `service_s3` 运行。RED 使用同一 helper/guard，未显式固定 GOTOOLCHAIN；以下后续检查均固定 Go 1.26.6，入口：

```bash
GOTOOLCHAIN=go1.26.6 python3 /tmp/multica-gap-recovery-20261021-h6phxwo9/with-db.py service_s3 bash ../scripts/go-test-with-agent-cli-guard.sh go test ...
```

1. 实施前 RED：`TestCollaborationCommandCancellationExactProvenance` 的 manual/remove/leave/complete 四个数据库场景均以“cancelled command lost its exact cancellation event association”失败；无编译/fixture 错误。
2. 初次编译遇到本地 `TimestampToPtr` 返回 `*string` 与新 DTO `*time.Time` 不匹配，已使用真实事件时间修正；不是测试通过证据。
3. 首轮 GREEN：`-run '^TestCollaborationCommand(Cancellation|History)' -count=1 -v`，6 个顶层测试全部通过，无跳过。包括评论/附件显示编辑删除后的原文与引用、未消费取消、分页及当前 ACL、legacy unknown、6 个写入故障回滚、丢提交响应后原键恢复。
4. 扩展 race（日志 `/tmp/multica-gap-recovery-20261021-h6phxwo9/s3-focused-race.log`）：新增 9 个 S3 顶层测试全通过，包括 account invalidation、promotion、nonlaunch 系统身份，M11 cancel-first/claim-first 确定性竞争，以及公开 staged Switch 后取消。相邻 FIFO/取消与 promotion/非启动返回/lifecycle/账号与参与者移除测试通过。
5. 上述扩展命令整体 exit 1：旧 `TestCollaborationSquadHumanJoinClaimAndUnreadCancellation` 从一次 `ListRuntimeRuns` 中寻找 member 和 continuation；S1 现在每页只返回一个候选，未赋值的 continuation UUID 导致第 209 行 `unsupported_runtime_protocol`。协调者授权后只改该测试：member 收尾后重新 List，使用其真实 continuation ID，保留原全部语义断言。单测 race 重跑通过，最终组合命令也包含该测试。原失败日志继续保留。
6. 第一版 HTTP fixture 试图重复插入 RegisterTarget 已创建的 channel，正确收到 `23505`。fixture 改为复用现有 Ensure/Bind primitive，未修改产品实现；失败记录保存在 `s3-final-focused.jsonl`。
7. 最终组合检查实际 exit 0，15 个顶层测试、24 个子用例全部通过，零失败/跳过。JSON 输出逐条核对真实 test identity，确保 handler TestMain 没有以 DB 不可达提前退出。

```bash
# 工作目录 server/；输出保存在仓库外证据目录
GOTOOLCHAIN=go1.26.6 python3 /tmp/multica-gap-recovery-20261021-h6phxwo9/with-db.py service_s3 \
  bash ../scripts/go-test-with-agent-cli-guard.sh go test -race -p=1 \
  ./internal/service ./internal/migrations ./internal/handler \
  -run '^(TestCollaborationCommand(Cancellation|History)|TestCollaborationCancellationMigration|TestCollaborationSquadHumanJoinClaimAndUnreadCancellation)' \
  -count=1 -json
```

完整输出：`/tmp/multica-gap-recovery-20261021-h6phxwo9/s3-final-verified.jsonl`。

- service：12 顶层、24 子用例，包括 11 个新增 history/cancellation 测试和修正后的旧 M11 测试。覆盖五类人类触发来源、后台 account/promotion/nonlaunch 的无代理人归因、M11 cancel↔claim 确定性竞争、remove/complete 自动 detach、公开 staged Switch 后精确当前身份、原文/附件引用、close/reopen、source/participant/machine/disabled ACL、恶意与跨单游标、旧手动记录和损坏关联、写入故障、原键 unknown commit 恢复。
- internal/migrations：1 顶层，实际执行 735/736 SQL；10 类 legacy 证据，singleton backfill、重跑、actor/time/reason/pointer 不可变、允许 outbox ACK、独立 concurrent index，以及 retained/busy/empty 三种回退结果。
- handler：2 顶层，实际 DB/handler 调用；严格查询字段/数量/编码/limit、分页与 exact decimal、no-store/no-referrer、剥离 ambient workspace、外部参与者、来源成员、机器及撤权后旧游标。

8. 真实中断后的 736 恢复及迁移注册表检查：4 顶层全部通过，零失败/跳过。

```bash
GOTOOLCHAIN=go1.26.6 python3 /tmp/multica-gap-recovery-20261021-h6phxwo9/with-db.py service_s3 \
  bash ../scripts/go-test-with-agent-cli-guard.sh go test -race ./cmd/migrate \
  -run '^(TestCollaborationCommandHistoryMigrationRepairsInterruptedIndex|TestConcurrentIndexCleanupsMatchTheirMigrations|TestEveryConcurrentUpBuildHasCleanup|TestEveryConcurrentDownBuildHasCleanup)$' \
  -count=1 -json

GOTOOLCHAIN=go1.26.6 go vet ./internal/service ./internal/handler ./internal/migrations ./cmd/migrate
git diff --check
```

输出：`/tmp/multica-gap-recovery-20261021-h6phxwo9/s3-index-recovery-verified.jsonl`。该测试在私有 schema 中持有真实 command writer，观察 index 已存在且 invalid 的并发构建等待后取消，再由实际迁移 runner 的注册清理恢复有效 index；命令行及 migration ledger 均保留。vet 和 diff 检查 exit 0。另已执行三个 migration 文件编号/上下行静态检查，全部通过。

## 文件与简化

- 新服务：`server/internal/service/collaboration_command_history.go`、`collaboration_command_cancellation.go`。
- 新 handler：`server/internal/handler/collaboration_command_history.go`。
- 最小事务接线：service 的 `collaboration_queue.go`（仅手动取消分支）、`collaboration_batches.go`、`collaboration_workflow.go`、`collaboration_lifecycle.go`、`collaboration_account_reconciliation.go`、`collaboration_groups.go`、`collaboration_squad_join.go`。S2 在 queue 中的 receipt/action 变更保留且不归为本切片成果。
- SQL：新 `server/pkg/db/queries/collaboration_command_history.sql`；workflow/lifecycle/reconciliation/batches 的四个实际取消/结束查询改为返回精确变动 command。复用原 command/event，未增加第二份历史账本或复制 actor/time 列。
- 迁移：735/736 的 up/down 四文件。生成 SQL、routes 与迁移恢复注册表由协调者完成。
- 测试：service `collaboration_command_history_test.go`、`collaboration_command_history_execution_test.go`；handler 同名 history 测试；`internal/migrations/collaboration_command_cancellation_test.go`；`cmd/migrate/migrate_collaboration_command_history_test.go`。现有 `collaborationFailureTx` 增加 Query 故障注入，确保 RETURNING 批量写入不会绕过测试；旧 squad 测试修正单页假设。

没有添加依赖。原取消事件由一个 helper 负责关联，原批次/证明/暂停和 Switch primitive 继续负责各自状态；没有并行的新控制或读取权限模型。

## 交接与限制

后端本切片无已知未解决错误。共享客户端/视图及 Web/Electron 接线、独立 Trellis 检查和规格整合由主会话负责；本实现没有本轮原生 provider、OS containment、安装包或完整 97 项验收证据，也未运行完整仓库 make check。旧缺证据的记录继续显示缺失；未来取消写入必须调用同一事务内的精确记录 helper，不得用后台 t.actor 或时间邻近补 actor/time。

主会话首次生产构建时冻结的产品代码与本次最终产品代码相同，之后只有测试/研究记录变化。主会话按其生产环境身份规则另行捕获最终验收源码，不以本报告替代浏览器/原生验收。
