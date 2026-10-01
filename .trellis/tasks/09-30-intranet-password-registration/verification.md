# 开发与验证记录

更新：2026-10-01。工作树：`/Volumes/artisan/code/2026/multica-password-registration`，分支 `feat/intranet-password-registration`，实现基线 `d0143c47c`。原工作区的其他任务改动未合并、覆盖或提交。

## 已实现

- PBKDF2-HMAC-SHA256 密码存储、输入边界、KDF 容量门禁；用户名/姓名/密码自助注册，不自动创建工作区。
- 密码登录、原 UUID 绑定、改密、运维临时密码恢复；受限账号不能预热业务数据或启动运行环境。
- JWT、PAT、task/daemon 凭据、代表成员的插件回调版本校验；签发与撤销共用用户锁；独立插件身份保留。
- realtime/daemon WS 握手、入站副作用和出站数据校验；撤销后本地执行通过状态轮询的 401 中断，依赖故障 503 不误杀。
- 单实例有界限流和 Redis 共享限流、可信代理解析、登录 CSRF 媒体类型门禁。
- Desktop/Web 共享认证表单、原欢迎引导、同名用户的用户名展示/搜索、重试等待提示。
- 主进程协调服务器切换、关闭副窗、清 cookie/存储/请求/daemon 凭据；浏览器回调 nonce 和 API 重定向拒绝。
- 迁移 462–464、nullable email 对外空字符串兼容；并发索引重试清理；有密码账号时拒绝破坏性 down migration；删除工作区保留全局凭据。
- `.env.example`、compose、SELF_HOSTING 中的配置、迁移和恢复说明。未新增依赖。

## 已取得的证据

| 检查 | 结果 |
|---|---|
| 既有 auth/middleware 基线 | 实现前通过 |
| 新库迁移 | 全部执行成功，含 462–464 |
| 密码 DB 集成与 race | 注册/同名并发/绑定/改密/临时密码、身份归属及凭据撤销通过 |
| 真实 DB + WS | 静默订阅、入站 RPC/订阅、PAT/mdt 撤销、连接 context 保留，通过普通及 race 测试 |
| 插件回调 | 旧版本拒绝、锁等待期间重置、成员与独立插件身份区分，通过 |
| Redis 实测 | 两个 limiter 实例共用实际 Redis，48 并发只放行10；故障不放行，race 通过 |
| 内存容量 | 50,000 活跃桶满时拒绝新桶，不驱逐旧限制，通过 |
| 本地执行撤销 | 新回归先复现 401 不终止，修复后 watcher 与判定测试通过；没有运行真实智能体 CLI |
| 真实 Electron 双服务器 | 同主机不同端口、HttpOnly/多Path cookie、bearer/空间头清理、副窗关闭、保留无关域cookie，通过；生产切换helper+真实窗口，daemon为测试替身 |
| 密码浏览器端到端 | 三字段注册 → 原问卷 → 自建空间 → 跳过运行环境 → 项目页；第二浏览器同 UUID/空间，刷新恢复，通过 |
| 回滚演练 | 有凭据时三个 down 均拒绝，哈希及索引保留；空库 down/up 通过 |
| 全量 TS | 一轮完整通过 core2233、docs62、views5761、desktop840、web263；重定向增量后 core2235 全量通过 |
| 全量迁移/handler | 修复接线遗漏后独立完整运行通过，14.6s / 54.2s |
| lint/typecheck | 全仓通过；既有 lint warnings 未扩大清理 |
| knip | 执行完成但非绿：报告9个未使用文件、1个dependency、1个devDependency，未涉及本任务新增文件 |

测试数据库均隔离；临时 Redis 容器已停止。浏览器测试只使用测试账号和本地服务器，没有发送公司邮件或使用真实 agent 配额。

## 全量检查中修正的问题

1. ApiClient 组合取消信号后，旧测试对 signal 对象同一性断言失效；改为验证实际取消及 reason 传播。
2. 基线三个 UI 测试滞后于现有组件：补 squad mock、按现有 SVG 图标断言、限定 popover 内选择器。相关产品文件和测试在修复前均与基线相同；仅修改测试。
3. 注册/登录拒绝 text/plain 等简单跨站表单；密码模式不签发无法按账号即时撤销的 CDN wildcard cookie。
4. 新并发索引需要登记迁移重试清理，新全局表需要登记工作区删除清单，均已补齐。

## 最终流水线

首轮 `/tmp/multica-password-check.log` 实际退出 1：静态检查、全量 TS、脚本、Go race/vet 和生产构建通过；Playwright 117 通过、25 跳过、9 失败。9 个失败已按当前导航、模板与欢迎页契约修正，未跳过用例，见 `research/e2e-regressions.md`。

2026-10-01 续作增加回归时，两次并行流水线捕获了刻意保留的 red 测试，均不能作为最终结果。修复完成后重新执行 `PATH=/opt/homebrew/opt/libpq/bin:$PATH make check`，日志 `/tmp/multica-password-check-complete.log`：前五阶段全部通过，最终 Playwright 125 通过、27 跳过、1 失败，make 退出 2。剩余失败是编队卡片图标说明参与可访问标题，旧 exact-heading 定位未匹配。只将该断言改为精确匹配模板 listitem，未修改产品代码；生产环境重跑 `workspace-defaults.spec.ts` 3/3 通过，随后全站 E2E 重跑 126 通过、27 按配置跳过，6.7 分钟，退出 0；日志 `/tmp/multica-password-e2e-recheck.log`。该命令使用另外分配的隔离数据库/端口，执行静态检查、全量 TS、脚本回归、Go race/vet、生产 Web 构建及全量 Playwright。

## 发布前仍需验证

- 实际公司 CPU/NAT/代理/副本部署的峰值容量；本机 M4 的 PBKDF2 样本约57.45ms/op，仅为开发测量。
- 已安装旧桌面版本的升级路径。新的浏览器登录回调要求 Web 页传递 desktop_state，旧 Web 页须同步升级；不接受无 nonce 回调。
- 曾使用 CloudFront wildcard cookie 的部署须先使旧签名授权失效；账号版本不能追回已签发的 CDN cookie/资源 URL。
- 已授权的网络数据和外部执行副作用无法回滚；取消的任务不会自动重放。
- 不包含移动端密码 UI、Cloud Fleet 云节点、自动账号合并、生产发布或部署。

完整流水线与分阶段复验的结果分别记录，不能把首次 make check 的非零退出改写为通过。最终提交信息在收尾时补充。

## 2026-10-01 续作修复与复核

- 上传隔离：附件和插件包的 multipart 路径加入服务器切换冻结、取消及响应代际校验，覆盖成功/失败 body 晚到；8 项回归均先失败，修复后连同既有 client/redirect 用例共 121 项通过。调用者取消原因仍传递。
- 受限账号：ClientUsageReporter 仅在正常 authenticated 状态上报；setup/change 受限状态及同 UUID 转为正常状态的测试先红后绿。
- 改密后导航：真实浏览器复现运维临时密码改密返回 200 后仍回到登录页。请求序列确认没有新 401；根因为 AccountGate 的旧表单卸载后继续跳转 /login，覆盖已恢复路由树的跳转。移除多余导航，最终 E2E 已通过。
- 独立服务端安全复核及新增真实 DB 测试：迁移 JWT 边界、并发绑定/重放、实际锁等待中的登录/PAT 签发/PAT 续签/改密/task 与 daemon 签发遇到重置、恢复撤销与用户名冲突回滚、撤销/DB故障下 heartbeat 与 realtime 门禁。最终串行 race 通过；测试注入方式和剩余边界见 `research/security-review.md`。
- `pnpm knip` 复跑仍退出 1：9 个既有未使用文件、1 个依赖、1 个开发依赖，与前轮清单相同；不扩大到无关清理。

密码专属最终组合验收：`password-registration.spec.ts`、`password-migration.spec.ts`、`password-server-switch.spec.ts` 共 4 项通过，0 跳过、0 失败、0 flaky，32.3s。使用隔离密码 API 和独立复制的 Web 开发服务，不干扰全仓生产构建。开发服务冷编译曾触发超时，预热后保持同一断言的组合运行通过；详细命令和证据见 `research/password-acceptance.md`。

本轮静态检查、全量 TypeScript（core2245、docs62、views5761、web263、desktop840，共9171）、脚本回归、隔离 DB 全量 Go race/vet、生产 Web 构建均通过。Go handler112.211s、service25.855s、agent248.371s。

## 本地验收结论

开发和本地可执行验收完成，进入待合并与部署验收阶段。全量静态/单测/Go race/vet/生产构建通过；全站 E2E 最终 126 通过、27 条条件跳过，密码专项独立 4/4 通过。跳过项没有改成通过项，首次完整 make check 的退出 2 保持记录，后续仅测试定位发生变化。生产 API/Web 复验进程均已停止，独立 Go 检查数据库已清理。

剩余发布门禁是实际部署 CPU/NAT/代理/多副本容量、已发布旧桌面版本升级兼容、迁移备份与发布窗口，以及实际无回显终端操作验收；本轮不执行生产发布。knip 的既有未使用项保留。分支 `feat/intranet-password-registration` 保留，原工作区其他任务不变。

## 提交

- `45552ee43`：既有 UI/E2E 测试契约修正（9 个测试文件）。
- `8d01dd606`：密码注册、原账号迁移、凭据撤销、客户端门禁与平台隔离（128 个实现、测试和部署规范文件）。
- 任务状态保持 `in_progress`，阶段为 `ready_for_integration`：开发与本地验收已完成，尚待合并及目标部署验收。
