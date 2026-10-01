# S04 实施分解

状态：计划，下面步骤未执行。需求见 prd.md；依赖：S01、S02、S03 的相关契约及验证通过；具体依赖见父 implement.md。

## 负责文件与模块

- `server/internal/service/admin_operation.go（新增）`
- `server/internal/service/task.go`
- `server/internal/handler/admin_execution.go、admin_installation.go（扩展）`
- `server/internal/handler/daemon.go、daemon_rpc.go、daemon_ws.go`
- `server/pkg/db/queries/agent.sql、admin_operation.sql（新增/扩展）`
- `server/internal/daemon/client.go、daemon.go`
- `packages/core/admin/operation-queries.ts（新增）`
- `packages/views/admin/operations/（新增）`

括号中的新增路径为计划落点；实现前按真实导出和模块规则核对。不能编辑 generated 文件代替源SQL。

## 步骤

复用S01提供的actor/组织范围内按幂等键查询，实现两个管理员各自请求operation映射到一个取消根事实的协调；不得用单一未确认target唯一索引丢弃第二人的请求。两响应丢失和根确认后子请求同步中断属于本切片必测。

1. 读取父设计及本任务JSONL，确认上游验证记录、当前修改和拟触及文件所有权。
2. 将下面三类行为分别写成权威层回归用例，先运行确认是预期行为失败。
- [ ] 所有HTTP/WS/single/batch/fallback/内部claim与reclaim的准入一致。
- [ ] cancel/complete、停止/领取、换绑/ack并发，旧task取消不影响重试新task。
- [ ] 响应丢失按原key查询、commit后通知失败轮询恢复、ack超时仅未确认。
3. 实现最小后端/协议路径，参数/schema/权限/事务先完成，再接入Query与页面。
4. 对应回归通过后测试失败路径、竞态与旧版本兼容；不添加绕过授权的测试专用生产分支。
5. 按父 test-spec.md 运行所属包 test/typecheck/lint/static检查；DB/协议变更运行对应Go和race验证。
6. 对照父AC做纵向验收，写 verification.md；共同模块变更由集成人串行检查。
7. 更新已证实的spec知识并按Lore协议形成可审阅提交；保留未测项，不推送或部署未授权环境。

## 验证命令

使用[父测试规格](../10-01-platform-admin-console/test-spec.md)中适合变更层的命令。新测试选名以实际文件为准；运行前准备task-owned数据库和生产模式Web。S07负责全量集成与容量/原生证据，不能用子任务定向通过替代全部验收。
