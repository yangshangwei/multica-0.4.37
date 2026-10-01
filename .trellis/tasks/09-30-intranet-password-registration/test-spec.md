# 测试与验收计划

> 实施更新（2026-10-01）：用户已批准并启动开发。下文保留评审时的设计说明；当前完成项与未测边界以 [verification.md](verification.md) 为准。

状态：实现与本地验收已执行，实际覆盖及部署前未测边界见 verification.md。每个产品行为选择一个规范测试层，UI 不重复穷举纯函数矩阵。

| 测试组 | 必须覆盖 | 层 / AC |
|---|---|---|
| T01 注册 | 三字段成功、姓名重复、用户名规范化冲突、并发同名只有一条 user/credential、DB 失败回滚、ALLOW_SIGNUP=false | Go handler+真实测试 DB；AC01/04/07 |
| T02 密码 | 正确/错误、随机盐不同哈希、损坏编码、超长输入、工作因子上界、Unicode 不被裁剪 | auth 单测；AC08 |
| T03 登录 | 未知用户名与错误密码一致；正确账号 UUID 不变；注册响应丢失后可登录 | handler；AC03/07 |
| T04 会话 | bearer 与 cookie；新版本 claim；绑定/重置后旧 JWT 拒绝；DB 异常不放行；旧 WS/PAT 不能持续使用/续签 | middleware/router 集成；AC05/06/08 |
| T05 模式 | password 禁 device/send-code/verify-code/Google；配置冲突拒启动；legacy 不变；能力字段缺失不假装支持 | handler/config/core；AC05/07 |
| T06 绑定 | 合法旧 JWT 绑定原 UUID；原 membership/任务/comment/onboarded_at 不变；并发与重放不覆盖；目标 user_id 注入无效 | handler DB；AC06 |
| T07 绑定拒绝 | 无 session、过期、仅 device ID、截止后、PAT/task/cloud/plugin token 不得绑定；受限会话不得执行普通 API | router/auth；AC06/08 |
| T08 可靠限流 | 账号和 IP、共享 NAT、多副本、无 Redis 有界内存、Redis 异常503、代理头伪造、KDF 并发上限 | limiter 单测/集成；AC08 |
| T09 客户端协议 | malformed response/空 token/user.id；成功 epoch 更新；旧请求401/晚到响应不破坏新登录；断网进入 recovering | core；AC07/09 |
| T10 UI | 登录注册切换、label/键盘/autocomplete、show password、重复提交、错误反馈、密码不持久化；无需邀请码/邮箱 | views；AC01/04/07 |
| T11 欢迎衔接 | 注册进入 StepWelcome；职业问卷保留；空间步骤和 runtime 复用；新注册未设置 onboarded_at；恢复不重复创建空间 | 现有 onboarding suite+E2E；AC02 |
| T12 服务器切换 | A→B 无 Authorization/cookie/workspace头泄漏、旧 query/WS/多窗口清理；保存失败；在途A响应不能写入B | desktop/platform+集成；AC09 |
| T13 运维恢复 | 无回显输入、临时密码只能进入修改流程、修改后旧会话失效、旧 JWT 过期且无 credential 时按原 UUID 创建凭据、用户名冲突回滚、密码不进入命令历史/日志、无公开任意重置接口 | Go/运维演练；AC08 |
| T14 无邮箱 | 用户/成员显示正常、邀请不把空邮箱当账号、JWT兼容、analytics 不发伪邮箱、旧客户端 parsing 不崩溃 | handler+core/views；AC07/08 |

## 端到端验收脚本

1. 使用隔离数据库与测试服务器，不调用公司真实邮箱、真实 agent CLI 或生产账号。
2. 全新桌面 profile → 填服务器 → 注册 → 欢迎页 → 填工作空间 → 运行环境现有跳过路径 → 进入应用。
3. 记录 user/workspace ID；重启及第二个 profile 登录，确认相同 user 与空间。
4. 用同名姓名、不同用户名注册第二人，确认账号隔离、不能查看第一人的空间。
5. 旧设备 fixture 有任务和评论 → 模式切换 → 绑定 → 比对原 UUID 和归属；重放旧设备 ID/JWT/PAT 全部拒绝。
6. 启动服务器 B；切换时捕获 B 请求，断言没有 A token、cookie、workspace header；失败重试仍无泄漏。
7. Web cookie 登录与现有 CLI 授权衔接冒烟；不消费真实 agent 配额。

## 验证命令

参考 implement.md 执行分阶段命令。全量完成后执行 pnpm lint、pnpm typecheck、pnpm test、make test、pnpm knip 和仓库要求的 make check。记录每个命令结果、测试 DB 环境及未覆盖项，不把文档检查当作功能测试。

## 发布阻断条件

任何免密码旁路、凭证跨服务器泄漏、无效限流、旧凭据未撤销、旧用户归属丢失、nullable email 未兼容、绑定 gate 仅在 UI、未通过迁移恢复演练，都阻断发布。

## 静态安全审查

检查 KDF 使用受支持标准库、随机盐使用 crypto/rand、派生值比较使用 crypto/subtle。单元测试验证输入输出与拒绝行为，不使用计时断言声称证明常量时间。

## 工程评审新增的必须用例

| 组 | 新增故障/竞态 | 预期和规范层 |
|---|---|---|
| T03 | 密码校验完成后并发重置；旧登录回包晚到 | 不给旧密码签发新版本；core 不接纳旧服务器成功回包 |
| T04 | 已预热 PAT/daemon 缓存，Redis 删失败；旧请求晚到 mint/renew | DB 版本/有效行检查拒绝旧权限；签发事务不能升级来源版本；Go DB 并发测试 |
| T04 | 静默 realtime/daemon WS，另一副本重置后产生新事件 | 旧连接无权接收新事件；DB 故障停止推送；跨实例集成 |
| T04 | 另一副本重置提交后，旧WS发送RPC/heartbeat/订阅指令 | 在处理器与队列消费前拒绝，无DB写入或待办消费；不能只阻断响应 |
| T06 | 改密撤销个人 runtime，但同空间有其他 owner 与 plugin | membership/agent/plugin 安装保留，其他 owner 不受影响，旧执行停止且不自动重放 |
| T07 | iat/exp 缺失/非法/未来、cutoff/deadline 等号、进程重启 | 严格拒绝非法或越界旧会话；重启不延期；fake clock + router |
| T07 | member actor mpc_ 与 plugin actor mpi_/mpc_ | 前者旧版本拒绝，后者权限独立；所有机器凭据不能绑定或签人类权限 |
| T05 | password + CLOUD_URL、未知模式、矛盾 capability | 冲突拒启动，mcn_ 各入口拒绝，客户端不降级 device |
| T08 | 并发抢最后额度、成功不清零、固定窗口临界突发、满50k桶 | 原子准入，满容量不驱逐活跃桶；超限429/基础设施503，Retry-After准确 |
| T08 | shared无Redis/故障、single重启、代理伪造、KDF客户端取消 | 不隐式多实例降级；明确重启丢计数；计算完成才释放槽位；无无界等待 |
| T12 | 同UUID不同服务器、同主机不同端口、HttpOnly cookie | 不按UUID误复用旧会话；B无旧cookie或Bearer；Electron集成 |
| T12 | 副窗请求保存、重复保存、daemon sync交错、本地清理失败 | 仅主窗可串行切换；失败不激活B；主进程测试 |
| T12 | 保存后重载失败、重启、旧成功/失败请求晚到 | 不恢复旧凭据/缓存；显示恢复入口；平台+core测试 |
| T13 | 无credential旧UUID恢复、版本竞争、恢复命令分发 | 创建凭据保留UUID；无监听的运维子命令可从交付包运行；DB+运维演练 |

```text
连接服务器[T12] → 能力/模式[T05] → 注册[T01,T02,T08] → 正常会话[T03,T04,T09] → 引导[T11]
                               → 旧会话[T07] → 绑定[T06] ────────────────────┘
                               → 临时密码[T13] → 修改[T04,T08] ─────────────┘
所有正常会话 → 跨设备[T03] / WS与派生凭据[T04,T07] / 服务器切换[T12]
资料与无邮箱映射[T14]；交互错误和无障碍[T10]
```

各分支的失败必须有可见反馈：校验400、用户名占用409、认证401、模式/受限权限403、额度429、依赖或容量503；注册提交后回包失败须提示改用登录。确切错误码在实现阶段集中到现有 envelope，不用中文消息分支。上述用例为验收要求，实际结果及未覆盖边界分别记录在 verification.md 和 research/security-review.md。
