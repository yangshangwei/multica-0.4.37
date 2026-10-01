# 测试规格与验收证据

状态：实施测试计划；本轮仅验证规划文档，不声称下面的测试已执行。

## 1. 分层原则

纯解析、枚举和分页参数矩阵放 packages/core 的 node Vitest；共享页面的代表性用户操作和可访问性放 packages/views；Web 登录回跳/路由放 apps/web；桌面安装身份和本地 daemon 交接放 apps/desktop；服务端状态与安全竞态在 Go handler/service/auth/daemon；跨层主流程放 e2e。

每个规则选一个权威测试层，组件不复制纯函数矩阵。Go DB 夹具使用 internal/testutil；默认测试不执行用户安装的 agent CLI，不消耗真实模型配额。时钟、随机数和故障依赖可注入，TTL 测试不用长时间 sleep。

## 2. 验收映射

| AC | 主责任任务 | 必测证据 |
| --- | --- | --- |
| AC01 | S01 | 管理 JWT/cookie 进入；零空间进入；普通用户与空间 admin 直接 API 均拒绝；同源 next 回跳 |
| AC02 | S01 | 完整 token 类型矩阵；PAT→JWT 被拒绝；历史转换 JWT 在角色提升后失效；撤权与写事务竞态 |
| AC03 | S05 | 禁用/恢复/恢复密码，旧 JWT/PAT/task/daemon/plugin 与双向 WS 失效；目标 B 撤销不登出 A |
| AC04 | S02 | 公钥/nonce/部署/用户版本验证；多 runtime 去重；重装、多用户、未关联历史和抢占绑定 |
| AC05 | S02 | 桌面关闭/daemon运行、桌面活跃/引擎坏、未知探测、缓存/DB异常不误报正常或全体离线 |
| AC06 | S03 | 业务任务/执行尝试分离；来源与执行目标；分页稳定、字段投影和组合筛选 |
| AC07 | S04 | 所有 claim/reclaim/HTTP/WS/batch/fallback 门禁；在途不误杀；取消 ack 与未确认状态 |
| AC08 | S04 | 原 key 幂等、响应丢失查询、A/B同时取消各自留痕和按key恢复、根确认与子请求同步中断、payload 冲突、迟到 ack、完成/取消竞态、重启后协调 |
| AC09 | S06 | 固定规则、去重/合并、检测器故障、确认/负责人冲突、恢复/关闭和新 episode |
| AC10 | S03/S06 | 总览与下钻同窗口/时区；缺用量不算零；无权限标题/正文/日志/产物不能从 API、错误或导出泄露 |
| AC11 | S01/S05 | 最后管理员互降互禁竞态、密码复核失败、审计失败回滚、秘密不在日志/操作/hash 中 |
| AC12 | S02/S07 | 旧客户端业务可用且受限管理动作关闭；新端能力协商；安全回退保留门禁、撤销和审计 |
| AC13 | S07 | 1000 安装、100万历史、10观察者、集中重连与冷/热分页的实际数据和环境记录 |

## 3. 拟新增回归文件

- `server/internal/handler/admin_auth_test.go`、`admin_user_test.go`：平台权限、无空间、目标撤销、复核与错误语义。
- `server/internal/service/platform_admin_test.go`：最后管理员锁、角色升级撤销、审计原子性与幂等。
- `server/internal/handler/admin_installation_test.go`、`server/internal/service/managed_installation_test.go`：接入与绑定证明、台账与状态。
- `server/internal/handler/admin_execution_test.go`、`server/internal/service/admin_operation_test.go`：元数据与取消、门禁、fence、操作确认。
- `server/internal/service/admin_alert_test.go`：告警 episode、状态机与补偿扫描。
- `server/internal/migrations/platform_admin_migration_test.go`：回填、索引完成条件、已有数据拒绝破坏性回滚。
- `packages/core/admin/schemas.test.ts`、`queries.test.ts`：缺字段、未知枚举、禁止授权默认放行、scope/key隔离与时间窗。
- `packages/views/admin/admin-shell.test.tsx`、`installation-detail.test.tsx`、`execution-detail.test.tsx`：权限/未知态/操作确认代表性流程。
- `apps/web/app/admin/layout.test.tsx`：Web路由门禁与零空间管理员；`apps/desktop/src/main/managed-installation.test.ts`：密钥保管与本地交接。
- `e2e/platform-admin.spec.ts`：共享登录、账号与执行完整主路径、403及退出隔离。

以上是拟定文件，不是本轮新增的测试。实现采用测试先行验证真实行为，不能用“mock helper 返回允许”代替鉴权。

## 4. 故障和安全矩阵

每个管理写在业务状态写入、operation 写入、audit 写入、commit 结果不确定、通知发送、ack 写入分别注入失败。确认：提交前全回滚；提交后原事实可查；重复请求不重复副作用；敏感数据未进入降级日志。

竞态覆盖：撤权/写入、改密/签发、双管理员最后角色保护、停止接单/领取、完成/取消、换绑/回执、扫描器双实例、告警同时认领、输入版本落后。断言实际状态与审计数量，不只看 HTTP 200。

安装凭据测试覆盖复制/抢占/过期challenge的边界，明确此方案不是硬件证明。管理密钥不得进入 renderer、process argv、安装统计或日志。

## 5. 实施期间验证命令

先按所属包与新增文件运行最小有效测试，然后按变更范围执行：

```bash
pnpm --filter @multica/core test
pnpm --filter @multica/views test
pnpm --filter @multica/web test
pnpm --filter @multica/desktop test
pnpm typecheck
pnpm lint
pnpm knip
make test
make check
```

Go 定向测试在 server 目录执行 `go test ./internal/handler ./internal/service ./internal/auth ./internal/daemon -run 'Admin|Installation|Password|Claim|Cancel' -count=1`，先确认 task-owned 测试 DB 和相关测试选名。变更影响并发时追加对应 race 测试，最终集成由 make check 覆盖。

`make check` 默认为 classic auth 的隔离测试环境，不可把它替代 password 模式验收。密码模式须另启隔离环境，按 `.trellis/spec/web/frontend/e2e-run-environment.md` 使用 production build/start；记录 password 模式、API/Web进程及 source fingerprint，再运行聚焦 platform-admin E2E。测试配置不得更改现有用户运行环境。

shell/node脚本与桌面原生验证按变更范围补充。原生安装测试至少覆盖 Windows/macOS：main与daemon交接、重启、多窗口、断网重连、保留数据升级、清数据重装；CI模拟不能替代这些证据。

## 6. 证据与退出标准

子任务 verification.md 记录：commit/source fingerprint、命令、模式、夹具、通过/失败/跳过数量、未测项。S07 汇总全部 AC，不把父子任务引用当作真实测试通过。

所有阻断安全/状态问题解决、必需测试通过、迁移和旧客户端兼容有证据、容量差异被处理后，才可报告功能可发布。性能目标或原生验收未完成时明确标记差异，不填虚构通过结果。
