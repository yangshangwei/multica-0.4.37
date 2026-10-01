# 认证与切换实施契约

> 实施更新（2026-10-01）：用户已批准并启动开发。下文保留评审时的设计说明；当前完成项与未测边界以 [verification.md](verification.md) 为准。

2026-09-30 方案评审补充。以下为拟实现契约，尚无产品实现。用户已确认本轮只评审方案、首版仅本地/自托管运行环境。

## 范围与复用

保留三字段注册、原 user UUID、欢迎引导、JWT/cookie、共享 core/views 和现有部署产物。不增加身份服务、Redis 强制依赖（单实例允许内存限流）、工作区管理后台或新的发布包类型。多实例沿用已有 Redis；不会用“内网可信”替代认证。

复用 `clearClientSessionData`、`tearDownOnLogout`、`AuthSessionCoordinator.reportMain(null)`、现有 runtime owner 查询及 token 删除 SQL。密码迁移不得直接调用 `revokeAndRemoveMember`：该函数还会删除成员及授权、归档智能体，违背保留业务数据的要求。

不在范围内：Cloud Fleet 云运行环境（用户确认首版不支持）、SSO/邮件找回/邀请码（不属于注册流程）、多服务器账号记忆（首版切换即退出）、自动账号合并（无法仅凭姓名证明归属）、移动端密码 UI（另立任务）。不新增泛化的 token 管理框架。

## 1. 配置与旧会话资格

- `MULTICA_AUTH_MODE=legacy|password`，缺省 legacy；未知值拒绝启动。
- ALLOW_SIGNUP 只控制新账号注册，不阻止已有账号登录、合法旧账号绑定或运维恢复；DISABLE_WORKSPACE_CREATION 与本PRD的新用户自建空间要求冲突，列为目标部署发布阻断项。
- password 与 device auth 或非空 `MULTICA_CLOUD_URL` 冲突时拒绝启动。password 模式的普通 HTTP、daemon HTTP/WS 均显式拒绝 `mcn_`；关闭创建/启动 Fleet 云节点能力。本地 daemon、Remote MCP 与普通插件仍在范围内。
- `MULTICA_PASSWORD_MIGRATION_CUTOFF` 和 `MULTICA_PASSWORD_MIGRATION_DEADLINE` 为拟新增 RFC3339 UTC 配置；两者同时为空表示关闭自助绑定，只填一个、非法或 deadline <= cutoff 均拒绝启动。不能在进程启动时动态生成窗口。
- 旧 JWT 只在 `iat < cutoff`、`now < exp`、`now < deadline` 时有迁移资格；iat/exp 必填且类型合法，未来 iat 拒绝。所有副本使用相同配置，重启不延长窗口。建议发布窗口 7 天，但由部署计划填入绝对时间，不默认自动开放。
- 未绑定且无迁移资格者返回明确认证错误，不允许业务访问；合法受限会话仅可 me/setup/logout。临时密码会话仅可 me/change/logout。凭据种类记录在服务端 context，客户端不能伪造。

## 2. 个人派生凭据与原子撤销

采用**版本绑定 + 数据库权威校验**。JWT 与每一种代表个人的派生凭据携带签发时的 `auth_version`。密码凭据 session_version 初始 1，绑定/更改/恢复事务递增。已有缺版本凭据视为 0，在 password 模式不获得普通业务权限。

| 凭据 | 拟新增存储/校验 | 绑定或改密后的处理 |
|---|---|---|
| 用户 JWT/cookie | claim auth_version | 每次认证查版本及 must_change_password；旧版本拒绝 |
| `mul_` PAT | PAT 表增加 auth_version，沿用 user_id | 撤销该用户的旧行；使用时查询有效行及用户版本 |
| `mat_` task token | task_token 增加 auth_version，沿用 user_id | user_id 是运行环境 owner；按该字段撤销，并停止关联执行 |
| `mdt_` daemon token | daemon_token 增加签发 user_id 与 auth_version | 签发时固定 owner，不能事后仅靠可变的 runtime owner 猜测；撤销旧行 |
| member actor 的 `mpc_` | CallbackGrant 保存 auth_version | 每次使用校验该用户当前版本和 membership；版本不符拒绝 |
| plugin actor 的 `mpi_` / `mpc_` | 保持 installation/scopes 校验 | 独立工作区主体不随 installed_by 改密撤销，不得进入人类认证/签发入口 |
| `mcn_` | password 模式不启用 | 直接拒绝，部署前排空旧 Fleet 云任务 |

新增列为 additive migration；版本默认 0，daemon 签发 user_id 对 legacy 旧行允许 NULL；password 模式拒绝无确定签发人/版本的个人凭据，重新登录后重新签发。不加外键。若查询确需新增索引，遵守每个 concurrent index 单独迁移，不预先添加无用途索引。

在 password 模式中，PAT 与 daemon token 缓存不能替代数据库有效性/版本校验。第一版绕过这两类授权缓存，Redis 删除失败不产生撤销空窗；legacy 缓存行为保持原有契约。DB 不可用返回 503，不能退回仅验签或缓存结果。

所有 mint/renew 路径（含 CLI、task claim、Remote MCP、member hook）在事务内先锁 user 行，再校验**授权来源的版本**与账号状态，最后签发同一版本的凭据。不能在晚到签发时直接读取最新版本，从而把旧请求升级成新权限。绑定/改密/运维恢复取同一行锁；同事务涉及多用户时按 UUID 排序获取。版本溢出或不合法必须报错，不能回绕为 0。

```text
旧请求授权 v1 ── 等待用户锁 ── 读取 v2 ── 拒绝，不签发
重置事务      ── 持有用户锁 ── v1→v2 + 撤销旧行 ── 提交
新登录        ── 校验新密码 ── 签发 v2 ── 正常访问
```

登录在 KDF 校验后、会话签发前复核凭据版本及哈希未被并发重置；不把已失效密码升级到最新版本。KDF 不持有长事务锁。并发更改密码时，事务内再次比对前序验证的版本，不允许旧密码验证结果覆盖后续修改。

迁移撤销该用户提供的运行环境中的旧执行，可能包括其他成员提交给该环境的任务；保留任务历史，不自动重放有外部副作用的任务。保留 membership、智能体、工作区插件安装和其他 owner 的运行环境。正常重新登录后重新签发凭据及连接，取消的执行由用户明确重试。运维恢复使用同一撤销逻辑，不能另写遗漏凭据类型的 SQL 脚本。

## 3. WebSocket 与在途操作

普通 realtime WS 和 daemon WS 的独立握手入口都要验证凭据类型、版本、受限状态。已有连接绑定签发身份/版本。第一版在发送受保护事件前进行数据库授权校验，同时在提交后尽力关闭旧连接；通知丢失不允许绕过发送前校验。静默客户端也受约束，不能只检查客户端下一条消息。

每条入站受保护消息也必须在调用 RPC、heartbeat、订阅处理器、消费待办队列或产生业务副作用前，校验连接绑定凭据的有效性、版本与受限状态；失败不执行。不能等到发送响应时才检查，因为此时业务写入可能已经发生。参考现有 `server/internal/daemonws/hub.go:800` 与 heartbeat 的待办消费路径。

需要测量每次发送带来的 DB 开销；不得用无跨实例失效的 TTL 缓存换取性能。DB 故障停止推送并关闭或进入可恢复错误状态，不继续消费敏感队列。

撤销以数据库提交为分界：提交后开始的认证和事件授权不得认可旧版本；此前已经授权并进入网络缓冲区的字节无法追回，已执行的外部命令也不能回滚。凭据签发使用上述锁保证不产生新权限；其他在途业务写入按现有事务语义完成，不承诺全站请求瞬间回滚。测试在重置提交后再创建事件，断言旧静默订阅无法获得该事件。

## 4. 限流与 KDF 容量

以下是待基准验证的首版默认值，并非实测容量承诺。采用固定时间窗，原子计数准入，包括成功、失败及未知用户名；成功不清零，避免并发失败统计和成功请求清零绕过。

| 操作 | 账号维度 | 来源 IP 维度 |
|---|---|---|
| 登录 | 规范化用户名 10 次/5 分钟 | 300 次/分钟 |
| 注册 | 不建立用户名长期锁 | 30 次/5 分钟 |
| 绑定、改密码 | 会话 user UUID，合计 10 次/5 分钟 | 合计 60 次/分钟 |

合法请求只有同时满足两个维度才能进入 KDF。固定窗口临界点可出现两倍短时突发，由 KDF 并发门禁限制资源消耗；不要称为严格滑动窗口。超限 429，Retry-After 为相关阻断窗口最长剩余秒数。输入格式错误和超大请求在 KDF 前拒绝，并受 IP 请求额度保护。

拟新增 `MULTICA_PASSWORD_LIMITER_MODE=single|shared`，password 模式要求显式配置。single 仅支持一个 API 实例：进程内原子计数，最多 50,000 个活跃桶，每分钟清理到期桶，满容量不驱逐活跃限制，拒绝新桶请求并返回 503。进程重启会清空计数，这是单实例方案的公开限制。shared 必须配置 Redis，使用原子脚本计数及到期；故障返回 503，不降级到各实例内存。应用不能自行发现隐藏副本，部署检查必须校验副本数与该声明相符。

IP 默认取网络对端；只有来自显式可信代理网段的请求才读取转发链，按可信链逆向解析。共享 NAT 按上表及实际员工峰值验证，不凭客户端随意提供的 X-Forwarded-For 分桶。

所有 KDF 入口共用进程级上限 `max(1, min(4, GOMAXPROCS))`，容量满立即 503 + Retry-After: 1，不建立无界队列。正在执行的标准库 KDF 不可假装已取消；客户端断开后，计算完成才释放槽位并丢弃结果。记录目标 CPU 下单次与并发 p50/p95、内存、业务 API 延迟和过载恢复，不为追求测试通过随意降低 600,000 次工作因子。

## 5. 服务器切换状态机

主进程是切换协调者，只接受主窗口、串行切换；切换期间禁止新增 issue 窗口、daemon token sync 和旧授权回调。renderer 复用现有清理函数，不分发 token 给其他窗口。

```text
A 正常 → 验证 B → 冻结旧会话/关闭副窗 → 清理 A → 原子保存 B → 重载 → 无凭据探测 B
           ↓失败                 ↓失败          ↓失败          ↓失败
        保留 A 会话           不激活 B       留 A 但已退出   B 保持阻断，重试重载
```

清理顺序：地址仍为 A 时结束旧 WS、清 daemon 凭据并停止 daemon，取消旧请求，推进会话/服务器代际，清 auth、Query cache、workspace mirror、草稿和 tabs，随后由主进程清除旧配置对应 Electron session 中的认证/CSRF cookie（含 HttpOnly、Domain、Path）。同主机不同端口 cookie 不隔离，也必须覆盖；不要求用户清整个浏览器数据。

所有窗口清理完成或已关闭后才持久化 B。A 不可达不阻止本地清理；远程 logout 失败不恢复旧 token。任何本地清理失败禁止激活 B。重启时无旧凭据，不依赖保存成功后的单次 reload 保证隔离。

健康/能力探测复用主进程无凭据 fetch，拒绝重定向。普通 Desktop bearer 请求采用 omit cookie，并保留 Web cookie 模式；认证请求禁止跨源重定向。无凭据请求不能带 Authorization、Cookie、X-Workspace-ID、X-Workspace-Slug 或 X-CSRF-Token。旧成功响应与旧失败响应均受代际检查；仅 api authEpoch 的旧 401 防护不足以保护登录成功回调。

新客户端 + 旧后端仍保留明确 legacy 支持，不能把未知/矛盾的能力配置解释为可尝试 device fallback；password 声明优先禁止设备登录。旧客户端能显示何种错误需测试真实已发布版本，新服务器不能凭空为旧版本添加升级 UI。

## 6. 实现分工和分发

先串行固定数据/版本/模式/错误契约。随后 A 负责 server 的凭据、HTTP/WS、恢复命令及 DB 测试；B 负责 core 协议/状态和客户端测试。A/B 不同时改共享契约。core 稳定后，views 表单与 desktop 主进程隔离可并行；Web 接线随后集成验证。主分支仅规划，无须为文档新建实现工作树。

恢复命令复用 server 二进制的受控运维子命令，必须在连接监听前分流；随现有 selfhost 镜像/离线包交付，不新增公开 HTTP 重置端点或单独发布体系。具体子命令接线在核对 main 初始化依赖后实施。临时密码无回显输入，不放参数/环境变量/日志；允许无 credential 的历史 UUID 首次建立 username，保留原 user。

## 7. 验证与评审结论

每条契约对应 test-spec.md 中增补的竞态与故障用例。仅靠文档完整不能宣称安全上线。真实已发布客户端、数据库迁移/恢复、目标 CPU、多副本与反向代理配置均是实施后的发布门禁。

本轮工程评审完成；允许据此继续细化或另行启动实现，但**未启动实现、未通过功能验收**。没有新增需本轮另建 TODO 的范围，延期事项已列于范围段。
