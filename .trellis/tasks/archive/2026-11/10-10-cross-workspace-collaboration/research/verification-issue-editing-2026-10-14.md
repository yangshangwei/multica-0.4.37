# 2026-10-14 协作任务字段编辑验证

## 范围

本切片实现 R41/R48 的服务端标题、正文、开始/截止日期编辑，并沿用统一工作流的当前账号/参与/角色检查、四个版本和原幂等键。`PATCH /api/collaboration/issues/{id}` 保留已有状态修改协议：一次请求只承载内容/日期或状态，两者混合（包括显式 null）拒绝。

本切片拥有 `service/collaboration_edit.go`、`handler/collaboration_edit.go`、对应测试、`queries/collaboration_edit.sql`，并仅扩展 `collaboration_workflow.go` 的编辑能力与回执恢复白名单；旧状态 handler 委托统一解析。router、sqlc、core 与详情日期投影由主会话整合。

## 固定契约

- title/description：省略或 null 保留；空字符串写入空值。复用 `util.SanitizeTextForPostgres`。
- start_date/due_date：省略保留；null/空字符串清空。复用 `util.ParseCalendarDate`，包括旧客户端 UTC 午夜 RFC3339 兼容；返回纯日期。
- 当前有效发起人或处理人可编辑，包括无来源成员身份的当前处理人，以及已退出来源空间但仍有效的发起人。来源成员/其他参与者/原处理人/机器/已失效账号不能借角色或旧请求写入。
- 真正变更写入 issue、协作 revision、必要 I1 facts、`collaboration_event` 历史与 operation receipt，同事务提交；纯 no-op 只保存回执，不推进版本或事件序号。
- 历史保存 before/after、真实 actor 和数据库锁后 `clock_timestamp()` 的 `edited_at`。无调用者时间或模拟 actor。
- 编辑不改变 handler、参与、authority/lifecycle epoch、补充请求、command 快照、grant、run 或队列控制；不产生普通 AI 调用及全员编辑通知。
- 响应为最小 `{code, collaboration_revision}`，版本保持十进制字符串；无法确认 commit 时使用原请求恢复。

## 环境与执行证据

专用数据库：`multica_collab_20261014_editing_1791943635066`，主会话创建并迁移到 616，最终回归前补到 617。通过私有 `/tmp/multica-collab-20261014-yOusnI/editing.env` 导出；测试前断言数据库名，无应用库回退。所有 Go 测试通过 `scripts/go-test-with-agent-cli-guard.sh`。

已执行红绿起点：

- 最小工作流分派回归：先失败（有效 Edit capability 没有路由到编辑 action），补两处工作流白名单后通过；最终由数据库角色矩阵覆盖，移除了只验证内部布尔分支的临时测试。
- `TestCollaborationEditRecordsCurrentActorAndImmutableExecution`：服务占位时，creator、external handler、departed source creator 三个用例均按预期失败。
- `TestCollaborationEditHTTPExternalHandlerAndExactVersions`：HTTP 占位时，明确得到预期 200、实际 503 的失败，随后填入统一 handler。其之前的并发 D04 编译中间态没有计作有效红灯。

最终已执行：

```bash
# cwd: server；先导出专用测试库并断言数据库名
../scripts/go-test-with-agent-cli-guard.sh go test -race -p 1 \
  ./internal/service ./internal/handler \
  -run '^(TestCollaborationEdit|TestCollaborationWorkflow|TestCollaborationLifecycle|TestCollaborationStatus)' \
  -count=1 -v
go vet -p 2 ./internal/service ./internal/handler
```

- `editing-final-race.log`：44 个顶层测试通过，0 失败、0 跳过、0 race 报告；其中编辑切片为 12 个 service + 5 个 handler 顶层测试。service 耗时 21.012s，handler 耗时 2.591s。
- `editing-vet.log`：退出码 0，无诊断。新增/修改 Go 文件 `gofmt -l` 无输出，已检查 diff 空白。
- `make sqlc` 由主会话统一执行成功，生成 `server/pkg/db/generated/collaboration_edit.sql.go`；本切片未手改生成文件、未添加迁移或依赖。
- 过程日志：`editing-red-workflow.log`、`editing-green-workflow.log`、`editing-red-service.log`、`editing-service-first.log`、`editing-red-handler.log`、`editing-handler-first.log`，均位于 `/tmp/multica-collab-20261014-yOusnI/`。

覆盖的关键结果：

- 无来源成员的当前处理人和离开来源的发起人可编辑；来源成员/owner、其他参与者、原处理人、被撤销者及 machine/restricted/unknown principal 均不能越权。
- 真实 `Transfer` 与 `EditIssue` 在提交屏障上以两种顺序竞争：转交先胜则旧处理人编辑 forbidden；编辑先胜则旧 revision 转交 stale_revision。账号禁用/epoch 撤销的锁等待之后重验拒绝。
- 同键并发仅一次修改；不同键同版本仅一方成功。失败回执不因后续版本变化而重新执行，未知 commit 不自动重试，可按原键恢复；失去本单访问后只能取自己的无内容回执。
- 缺省/null/空文本、日期清空、纯日期及 UTC 午夜旧格式遵循原规则。所有四个版本在 `9007199254740993` 以上无损；协作/issue/event sequence 达 int64 上限时完整回滚。
- 对 issue 写入、I1 event/scope revision、协作 revision、事件序号、history event、operation receipt 七个必需写入点注入故障，均不留下局部修改。标题沿用 I1 事实记录；正文/日期不制造统计事件。
- 活跃 run、grant、command 内容/作者/顺序、queue 控制、handler、来源成员与补充链保持不变，无普通 AI 或全员编辑通知。已结束的业务单可合法编辑内容，但不重开或恢复执行。
- root PATCH 的状态请求继续走原服务；显式混合请求、其他未实现字段、重复/Unicode 别名键、畸形值和溢出版本均拒绝。

## 明确剩余项

本单完整 timeline 读取与 R55 正文编辑新增提及通知属于 P3，尚未由本切片实现；因此不能仅凭本切片勾选整个 AC71/AC72/AC79 的产品验收。R42、R49/R50/R58 结构字段、双端界面、原生真实停止确认和正向共享执行继续按总计划推进。创建/执行门禁保持关闭，不提交、不部署。
