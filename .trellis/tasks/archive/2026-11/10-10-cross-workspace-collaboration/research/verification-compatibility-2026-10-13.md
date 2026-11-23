# 客户端兼容边界验证（2026-10-13）

本报告验证当前已实现的 core 读取边界，不表示 R01–R58 或 AC01–AC89 全量验收。检查依据为当前 PRD、design D02/D09–D11、implement P1/P6/P8、完整验收附表、数据契约、执行契约及 `review-2026-10-13-clients.md`。已读取原生注入的完整上下文文件及 core 的 API 身份代次、类型、状态和质量规范。

## 范围和当前实现

本轮写入仅限 `packages/core/api/client.ts` 的协作读取/相关日志分支、`packages/core/api/collaboration.test.ts`、`packages/core/collaboration/schema.test.ts` 和本报告。未修改 legacy schema、其他人的代码、服务端 DTO、package 导出或依赖。

当前 core 只提供协作详情、创建结果查询、账号工作台三种 GET 封装及详情/列表 schema；尚无协作 Query、mutation、账号 WS、草稿、页面或原生接线。与 `server/internal/handler/collaboration.go`、`collaboration_workbench.go` 的当前 DTO 对照，字段名称、字符串版本、账号摘要和空列表形状一致。此项为静态对照，网络测试使用 Response fixtures，没有替代 Go handler 的真实 HTTP/数据库验证。

## 已修复的问题

| 问题 | 原路径证据 | 修正与回归 |
| --- | --- | --- |
| 协作 HTTP 错误和工作台游标进入日志 | schema 已使用 redact，但 `fetchRaw` 仍记录服务端原始 `error` 和完整查询字符串 | 协作路径仅记录固定失败说明及去掉查询参数的端点；三种读取的 HTTP 私有 sentinel 和工作台 cursor sentinel 均覆盖。保留 ApiError status/body 供调用方判断结果，不改变通用错误接口。 |
| 旧身份请求完成后仍能返回数据或权限拒绝 | 通用请求检查 endpoint 代次，但同一 ApiClient 的账号轮换不会改变 endpoint；成功正文解析和 403 传播没有协作级检查 | 复用现有 session scope，成功响应再检查 auth epoch；换账号/换 Server 后以 AbortError 丢弃旧结果。覆盖 bearer 正文解析期间轮换、cookie 登录替换、旧 401/403、当前 401 仍正常使会话失效。 |
| 非 JSON 成功响应暴露私有解析片段 | `Response.json()` 的 SyntaxError 包含正文片段，绕过 schema 的 redact | 协作读取将语法损坏交给原有 schema 返回 null/unavailable；取消信号优先于该降级。覆盖全部三种 GET，以及取消期间正文恰好解析失败的竞争。 |
| 合法形状的错单/错来源响应被接受 | schema 只验证 UUID 格式，不验证其与请求身份一致 | 详情匹配请求 issue，创建查询匹配明确来源空间；复用现有 `sameResourceIdentity`，不创建第二套 UUID 规则。 |

修正保持现有公开方法签名；`fetchCollaboration` 将三种读取的身份/解析保护放在同一处，普通 API 沿用原请求行为。

## 已验证的契约

- 旧 `IssueSchema.revision`、`CommentSchema.revision` 保持数字，添加协作字段不使 mixed fixtures 的整列解析失败；未把新协作 DTO 写入旧 Issue 缓存。
- `collaboration_revision`、`lifecycle_epoch`、`control_version` 必须是正 signed-64-bit 十进制字符串。覆盖超过 JS 安全整数、int64 上界、越界、0、前导零、小数、科学计数、符号、空白/换行、空字符串、数字和缺失值。原解析器通过，无需改变生产 schema。
- 关键对象、source/creator/session 身份、external 标记、权限对象/boolean 及 queue_state 损坏返回 unavailable；不回退普通任务或空成功列表。
- 可选名称/头像单独降级；外部账号额外 email/username 不保留在 parsed DTO。详情和工作台 schema 日志不带 received、Zod 收到值或私有 sentinel。
- 合法空工作台与损坏列表、负数计数、坏 UUID 区分；不支持接口的 404 保留 HTTP 拒绝，不伪装零条待办。
- 三种账号读取保留 Authorization，去除当前空间镜像带入的工作空间头，并传递 AbortSignal；工作台 view/limit 仍按显式参数发送。
- 请求账号/Server 变化、当前认证失效和调用者取消有明确边界；当前 401 不会被错误降级成 unavailable。

## 验证命令与证据

本轮先运行原有 `pnpm --filter @multica/core test collaboration`：2 文件、14 测试通过。新增 API 回归在修正前有 14 个失败，直接复现上述漏口；扩展 schema 的 23 测试独立通过。第一次修正后 2 文件、39 测试通过。补入取消与损坏 JSON 的竞争后复现 1 个失败，已修正取消优先级。

最终检查（修改完成后执行）：

| 命令 | 结果 |
| --- | --- |
| `pnpm --filter @multica/core test api collaboration --maxWorkers=2` | 通过；35 文件、855 测试，含协作 schema/API 的 44 测试 |
| `pnpm --filter @multica/core typecheck` | 通过 |
| `pnpm --filter @multica/core lint` | 通过 |
| `git diff --check -- packages/core/api/client.ts packages/core/api/collaboration.test.ts packages/core/collaboration .trellis/tasks/10-10-cross-workspace-collaboration/research/verification-compatibility-2026-10-13.md` | 通过 |

pnpm 输出已有根配置提示：package.json 的 `pnpm.onlyBuiltDependencies` / `pnpm.overrides` 不再读取；本轮未改相关配置。

## 未实现或不在本轮验证范围

1. D10 的 server/viewer/issue/session/view Query key、access epoch、`cancelQueries({revert:false})`、来源成员失效重算、账号 WS/快照、draft 清理尚未接线。当前不存在协作查询缓存，因此只能确认此读取模块没有向旧缓存写入；不能宣称暖缓存撤权或跨账号持久缓存验收通过。
2. 当前 session DTO 只有 id、queue_state、control_version，没有未结 run、proof、执行 gate 等控制快照。不能由 paused/closed 推断实际停止，也不能据此交付真实停止 UI。扩展 DTO/服务端实现由主会话按 P4–P6 统一推进。
3. 尚未验证真实旧 Web/Desktop/Mobile/CLI/daemon 二进制请求、旧 writer/调度入口、低版本协议领取、原生下载、私有附件/通知/操作回执的新 schema、执行端进程树或数据库竞争。旧 numeric schema fixtures 不是全量 legacy wire 验收。
4. `ApiError` 的结构化响应仍保留在调用方内存；本轮没有改其公共合同。后续协作消费者不能把整个 Error/body 送入日志或持久草稿，需只消费明确错误码和已验证的回执字段。当前内置请求/schema 日志的私有值已覆盖。
5. 不增加创建/执行入口，不更改启用门禁，不发布、不提交。本轮无需修改已经规定 redact/epoch/版本隔离的规范；建议主会话在 core 规范记录本次实证的 HTTP 错误与 JSON 语法错误补充点，避免后续新增协作端点只加 schema 而漏掉传输边界。

全功能的剩余阶段仍以主任务进度为准；本报告只收口现有客户端兼容读取基础。
