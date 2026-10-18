# 终端、领取与取消：现状依据

日期：2026-10-01。以下是本次源码检查得到的事实；新实体和接口见 `../terminal-execution-design.md`，不得把建议描述为已经实现。

## 三种现有身份并不等价

| 标识 | 证据 | 当前语义及局限 |
| --- | --- | --- |
| `install_id` | `packages/core/client-usage/install-id.ts:3`；`apps/desktop/src/renderer/src/platform/client-usage-reporter.tsx:39` | StorageAdapter 中随机 UUID，用于客户端使用统计；客户端提供，不是设备访问凭据。 |
| `device_id` | `apps/desktop/src/main/device-identity.ts:66`；`apps/desktop/src/shared/device-identity.ts:1` | Electron userData 的 `device-identity.json`，32–64 位小写十六进制；旧内网设备认证将其用于用户身份，不能代替密码模式用户或管理安装实例。 |
| `daemon_id` | `server/internal/daemon/identity.go:19` | 稳定 UUID，当前写入 `~/.multica/daemon.id`；同一个 OS 用户 HOME 下共享 CLI profile，但不同 OS 用户、不同 HOME 或数据丢失仍会不同。源码称 machine-scoped，不构成物理硬件认证。 |
| runtime ID | `server/internal/handler/daemon.go:187`、`:396` | 运行时注册按工作空间、daemon 和 provider/profile 建立记录；一个 daemon 可有多个 runtime，不能把 runtime 行数当桌面数量。 |

`DaemonRegisterRequest` 当前没有受管理安装实例 ID 或持有私钥证明；`legacy_daemon_ids` 用于运行时旧身份合并。旧合并不能被提升为后台资产归属证明。

## 当前上报与状态

- `server/internal/handler/client_usage.go:47`：按用户、平台、安装 ID、UTC 日期 upsert；可附探测计数，不是连续心跳。
- `apps/desktop/src/renderer/src/platform/client-usage-reporter.tsx:43`：同日同探测签名去重，登录、状态变化和窗口 focus 触发；因此当天有数据不代表此刻在线。
- `server/internal/daemon/config.go:24`：daemon 默认心跳为 15 秒。
- `server/internal/handler/daemon.go:1035`、`:1204`：runtime 心跳可写短期 liveness store；DB 最长 60 秒刷新，缓存故障回退 DB。
- `server/internal/handler/heartbeat_scheduler.go:96`：批量落库 tick 30 秒；明确约束 `60 + 15 + 30 < 150`。
- `server/internal/service/task.go:189`：claim 可接受的 DB 心跳新鲜度为 150 秒；`server/cmd/server/runtime_sweeper.go:31` 复用同一阈值。
- `server/internal/handler/runtime_liveness_store.go:14`：已有可选 Redis/no-op 接口；没有 Redis 时使用 DB，不能为后台新增强制 Redis 依赖。

管理界面需要新增独立桌面实时上报和数据新鲜度。CPU/内存、真实账单和物理硬件资产不存在于以上上报，不能用零值填充。

## 当前身份认证和领取路径

- `server/internal/middleware/daemon_auth.go:62`：daemon 接口支持 daemon token、PAT/JWT 等；密码模式经过原始会话版本校验，不得把用户 ID 当平台管理授权。
- `server/internal/handler/daemon.go:1595`：HTTP 批量领取校验 daemon/workspace/runtime 范围，再调用 `ClaimTasksForRuntimes`。
- `server/internal/handler/daemon_rpc.go:35`：`tasks.claim` WS RPC 用认证连接上下文复用批量 HTTP handler，不能只在 HTTP 路由加停止接单判断。
- `server/internal/handler/daemon.go:3278`：单 runtime 领取仍存在；`server/internal/daemon/client.go:289`：批量端点不支持时有单 runtime 回退。
- `server/internal/service/task.go:3422`：`claimTask` 在事务里锁 agent，重新确认绑定 runtime、并发能力，再领取。
- `server/internal/service/task.go:3559`、`:3804`：单个与批量 service 路径还包含 deferred promotion、丢失领取响应的 dispatched reclaim 和空队列缓存。
- `server/pkg/db/queries/agent.sql:794`：`ClaimAgentTask` 通过 SQL 校验 runtime 所属、在线、新鲜度、私有 runtime 归属与串行条件。
- `server/pkg/db/queries/agent.sql:907`、`:951`：dispatched reclaim 有独立写路径，后台接单门禁必须覆盖。
- `server/internal/service/task.go:3690`：`FinalizeTaskClaim` 原子写 task token 和投递回执；失败可按 `dispatched_at` CAS 归还原领取，不等于整个领取都在这个事务中。

## 当前取消并不等于已收到停止确认

- `server/internal/service/task.go:2833`：取消事务先修改任务状态并更新聊天恢复指针；之后做聊天收尾、agent 状态协调、广播和容量唤醒。
- `server/internal/service/task.go:2802`：显式用户取消还确认 delegated-failure recovery 输入；后台取消必须保留，避免后续自动恢复又生成任务。
- `server/pkg/db/queries/agent.sql:1586`、`:1594`：只把 queued/dispatched/running/waiting_local_directory/deferred 转为 cancelled，已有终态不被反向覆盖。
- `server/internal/daemon/daemon.go:5021`：daemon 通过状态轮询及重连协调判断取消；网络/503 与凭据失效语义不同。
- `server/internal/daemon/client.go:446`：cancel ack 在 runner 返回、日志刷出后发送；瞬时错误重试。
- `server/internal/handler/daemon.go:4696`：ack 接收 branch/durable-work-dir/error；空 body 兼容旧 daemon。写入采用 cancelled CAS 且不覆盖已有结果，之后可重新广播并完成 deferred chat 收尾。
- 以上 ack 未关联平台操作 ID、安装证明和绑定代次；要新增管理结果记录，不能根据 HTTP 200 或广播成功推断本地进程已停止。
- `server/internal/handler/issue_cancel_status_no_cancel_test.go`：业务 issue 的取消状态与执行取消是独立行为。

## 对规划的直接约束

1. 新建资产身份关联与操作账本；沿用 `agent_task_queue`、task token、现有取消和回执，避免再建执行队列。
2. 管理权限读取运行元数据；私有任务正文、原始错误、路径、日志、产物继续走业务可见性判断。
3. 新管理事务需要明确的 `InTx` 写入接口及提交后副作用；不能在新事务里调用会自行提交的旧取消包装器。
4. 使用仓库原有 Go/PostgreSQL/Node 能力；新索引必须独立单语句 `CREATE [UNIQUE] INDEX CONCURRENTLY`，禁止新增外键与级联。

检查方式：静态阅读源码与既有测试；本文件不声称跑过业务测试、压测或真实终端验证。
