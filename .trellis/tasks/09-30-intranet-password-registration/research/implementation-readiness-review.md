# 实施前复核：认证撤销与服务器切换

日期：2026-09-30。范围：对照规划与当前代码的只读复核；本记录不代表功能已经实现或测试通过。

## 结论

注册三字段、复用原 user UUID、复用欢迎引导的产品主流程可以沿用。开始实现前，应把以下安全边界写入实现契约；不能把“批量删除令牌”和“退出当前窗口”视为完整的账号迁移及服务器切换。

本轮未修改产品代码。用户随后确认继续方案评审、首版仅本地/自托管；发现的处理已收敛到 ../security-contract.md，并同步其余规划文档。任务仍为 planning。以下各节保留首次发现与建议，最终工程契约以 security-contract.md 为准。

## 1. 旧会话的迁移资格

依据：`server/internal/handler/auth.go:157` 签发 `iat/exp`；现有 JWT 验证尚无本任务的迁移时间门禁。

实施契约应要求切换时间和迁移截止时间为部署配置中的固定 UTC 时刻，跨实例一致，重启不延长。旧 JWT 必须具有合法、必填的 `iat` 与 `exp`，满足 `iat < cutoff`、仍未过期且当前时间未越过迁移截止时间。缺失、非法、未来签发时间均拒绝。没有配置迁移窗口时默认关闭自助绑定。

凭据种类来自服务端认证结果，不信任客户端声明。只有通过验证的用户 JWT/cookie 能进入绑定；PAT、task token、cloud token 等不得因最终解析出 human user_id 而获得同等资格。

补入 T07：缺失时间字段、切换时刻边界、未来时间、重启、跨实例配置不一致与机器凭据伪装。

## 2. 撤销不能被缓存或在途签发绕过

依据：`server/internal/auth/pat_cache.go:20` 的 PAT 缓存期限为 10 分钟且失效操作吞错；`server/internal/middleware/auth.go:185` 使用缓存命中结果；`server/internal/handler/personal_access_token.go:104` 在前序认证后直接插入令牌。

存在两种必须覆盖的交错：

- 重置已经提交，但缓存仍返回已撤销 PAT。
- 旧请求先通过认证，重置随后撤销并提交，旧请求最后才创建一个新的 PAT。

建议第一版采用数据库权威校验：password 模式下缓存不得绕过撤销检查；用户派生凭据的 mint/renew 与绑定、密码修改、运维恢复共享用户行锁，并在事务内重新验证会话版本和受限状态。所有签发入口必须纳入，不能仅覆盖个人令牌页面。若选择让派生凭据保存签发版本，则需覆盖每种凭据的数据库结构和所有消费入口，不能只给 JWT 加 claim。

补入 T04：预热缓存后撤销、Redis 删缓存失败、旧签发请求暂停在认证与写入之间，以及多个实例并发重置。验收以撤销事务提交后的认证结果为准。

## 3. 长连接要阻断继续接收数据

依据：`server/internal/realtime/hub.go:679` 独立认证；`server/internal/middleware/daemon_auth.go:238` 也独立解析 JWT。修改普通 HTTP 中间件不会自动覆盖这些入口。

新建连接必须验证版本及账号受限状态。已有连接必须跨实例关闭，或在发送受保护事件前重新进行可靠授权检查；数据库/撤销状态不可用时不能继续推送。仅在下一条客户端消息上验证不充分，因为客户端可能长期静默接收。

补入 T04/T07：建立静默订阅，在另一实例绑定/重置账号后触发工作区事件，旧连接收不到新事件；同时验证 reconnect 和 daemon 控制流不能绕过。

## 4. 机器凭据的决策清单

| 凭据 | 现有依据 | 实施前必须固定的处理 |
|---|---|---|
| `mul_` PAT | `personal_access_token.go`、`pat_cache.go` | 事务撤销、缓存检查、在途签发锁与版本重验 |
| `mat_` task token | `server/pkg/db/queries/task_token.sql:11` | 按账号定位关联任务、停止或排空、撤销，并阻断旧执行继续签发 |
| `mdt_` daemon token | `server/pkg/db/queries/daemon_token.sql:10` | 按 workspace/daemon 明确归属与撤销范围，不能误停其他成员运行环境 |
| `mcn_` cloud PAT | `server/internal/auth/cloud_pat.go:263` | 外部 Fleet 验证及缓存不能靠本地 SQL 撤销；选定可靠本地版本门禁或在 password 模式禁用，并明确重新启用流程 |
| plugin 凭据 | `server/pkg/db/queries/plugin.sql:93` | 区分工作区安装权限与 installed_by；不能直接把安装者重置解释成删除整个工作区插件 |

以上是必须完成的设计决策，尚非已选定的撤销实现。默认不能为了迁移某个成员而撤销整个工作区的其他成员权限。未解决的凭据类别应阻断发布，或先明确不支持并由服务端禁用，不能静默漏过。

## 5. 服务器切换由主进程协调

依据：`apps/desktop/src/renderer/src/pages/endpoint-setup.tsx:94` 的保存路径只在 embedded 时停止 daemon；`apps/desktop/src/main/index.ts:832` 保存后仅重载发起窗口。`auth-session-coordinator.ts` 只比较 user UUID，没有服务器身份。`packages/core/api/client.ts:900` 使用 `credentials: "include"`，仅删除 bearer token 不等于清除 cookie。

建议复用现有退出清理，但将服务器切换协调放在主进程：

1. 验证、规范化目标配置；只允许主窗口发起，并串行处理切换，禁止切换期间新建 issue 窗口或启动 daemon 同步。
2. 在地址仍指向 A 时广播退出、关闭 issue 窗口，停止 WS、清 daemon 凭据并停止旧 daemon；清空旧请求、认证存储、workspace mirror、Query cache 和相关视图状态。
3. 按旧配置对应的 Electron session 清理认证 cookie；覆盖 HttpOnly、Domain、Path，以及同主机不同端口（cookie 不按端口隔离）。不依赖 renderer 的 document.cookie 清理 HttpOnly。
4. 收到清理完成后再原子保存 B；保存失败保留 A 配置但保持退出，清理未完成则禁止激活 B。重载失败不能让旧 renderer 配合新 daemon 配置继续工作。
5. 新窗口无凭据探测 B 的健康与认证能力，然后才允许登录。旧请求的成功响应和失败响应均需代际保护，不能只防旧 401；重建 ApiClient 后也不能接纳旧实例回调。

`runtime-config:test` 已使用主进程 fetch 和 `redirect: "manual"`，可沿用此无凭据探测路径。健康 HTTP 200 只证明可达，不能证明支持密码协议。现有 `AuthSessionCoordinator.reportMain(null)` 可以关闭 issue 窗口；不需要另建跨窗口 token 分发机制。

补入 T12：相同 UUID 的两个服务器、同主机不同端口、HttpOnly cookie、issue 窗口发起保存、重复保存、daemon sync 与保存交错、清理失败、保存成功但重载失败、旧登录响应晚到，以及旧请求携带的 `X-Workspace-Slug` 和 `X-CSRF-Token`。不只检查 `X-Workspace-ID`。

## 6. 限流仍需固定可测参数

实现契约需要给出账号/IP 默认限额、内存桶容量及溢出行为、成功登录是否重置失败计数、实例数声明与启动校验、密码修改限流规则。KDF 并发门禁需覆盖注册、登录、绑定、修改密码，不能只有登录受保护。限额耗尽返回 429 与 Retry-After，基础设施不可用返回 503；二者不要混为认证失败。

参数需在本地和目标 CPU 的基准后记录；本轮没有性能测量，不能声称建议参数已满足公司 NAT 或多副本部署。

## 下一步与验证边界

若本轮继续方案评审：把上述决策固定到 design/architecture/migration-rollout，并同步 T04/T07/T08/T12 和 implement.md 后复核一次。

若转入代码实现：先固定共享认证契约，再按 implement.md 执行数据层、服务端撤销、共享状态、表单和平台接线。后端与前端只能在契约稳定、文件所有权不重叠后并行。

本轮仅核查代码与文档；未运行 Go、Vitest、E2E 或迁移演练。上述内容属于发现及测试要求，不是漏洞复现报告或验收通过记录。

## 收敛结果

| 发现 | 置信度 | 方案处理 |
|---|---|---|
| 迁移时间资格未固定 | 9/10 | 固定UTC配置、必填iat/exp、严格边界，未配置关闭绑定 |
| 缓存与在途签发可破坏撤销 | 9/10 | 所有个人派生凭据绑定版本，DB权威检查、用户锁、继承来源版本 |
| 静默WS仍可能接收推送 | 9/10 | 两类WS握手及发送前授权，明确提交前已授权数据不能追回 |
| 机器凭据归属不完整 | 9/10 | 固定签发owner，区分member/plugin回调；用户确认排除Fleet |
| 服务器切换缺多窗口和cookie清理 | 9/10 | 主进程串行协调，清理后保存，失败不激活B，Desktop omit cookie |
| 限流参数和资源容量不明确 | 8/10 | 固定窗准入、single/shared显式配置、有界桶和KDF并发 |

复用/代码质量：不直接调用成员移除流程；重用局部查询与既有会话清理。性能：新增DB认证/WS发送校验需要基准，首版不通过不可靠缓存掩盖开销。测试：新增矩阵已写入 test-spec.md，并含分支示意图；既有功能测试未运行。

评审结论：本轮架构、复用边界、测试和性能方案复核完成；未解决的产品范围选择为0。迁移演练、目标CPU容量和真实旧客户端兼容性仍是发布前验证要求，不能据此标记功能完成。后续不支持的范围见 security-contract.md，无新增全局TODO或新发布产物。

最终独立复核补齐第7项（置信度9/10）：WS不仅发送前校验，入站RPC/heartbeat也必须在副作用和队列消费前检查版本。已同步security-contract.md第3节及T04，其余指定边界未发现新的必须修复矛盾。
