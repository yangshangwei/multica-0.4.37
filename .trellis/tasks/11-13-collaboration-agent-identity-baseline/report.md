# S1 验收报告

具体 Agent 评论作者展示与现有闭环回归已实现；B01–B10 的必需断言已通过，独立最终审阅通过，S1 达到关闭条件。父任务 PAY-241 的完整 24 用例及 S2–S5 仍开放。

## 交付

- 服务端在同一授权读取事务内批量投影 `author_agent: {agent_id, name}`，保留人员专用 `author`。列表、详情和字节预算补读共用投影；资源撤回不抹除历史归因，失去任务权限后仍拒绝读取。
- Core 对缺失、null、畸形摘要安全回退，原身份、版本和内容校验保持严格。
- Web/Desktop 共用作者选择函数，列表与回复提示显示具体名称及“智能体”标识；人员名称、头像、外部标识和编辑权保持原行为。没有新增详情请求、依赖、迁移或权限模型。

## 验证证据

| 范围 | 实际结果 |
| --- | --- |
| 服务端评论、补读、授权 | Service 53、Handler 9 个测试/子测试通过，无跳过 |
| Go 静态与竞争检查 | service/handler `go vet` 通过；新增用例 race 6+1 通过 |
| Core 协议及恢复 | 7 文件，146 项通过 |
| 共享视图 | 四文件串行 50 项通过；空名/空白名审阅补充后的列表文件 19 项通过 |
| 完整 TypeScript 检查 | `pnpm typecheck --concurrency=1`：9/9 包通过 |
| 完整 lint | `pnpm lint --concurrency=1`：6/6 包通过 |
| 构建 | production Web、Desktop renderer/preload、当前 CLI 和显式受控 Provider/daemon 构建通过 |
| B05/B06 | `b05-run3` 通过：三个独立账号、三 Agent、五次真实受控进程（1/2/2）、两轮交付、首轮退回、精确发起人验收；三会话作者/回复提示一致；同一任务 Electron 定向检查通过 |
| B08 | `supplemental-run2` 的 Web/Electron 离线结果通知、信息/验收读取与撤权清理均通过 |
| B09 | `supplemental-run1` 的人工转交链通过；`supplemental-run2` 的运行中撤权、终结证明及显式 Resume 通过 |
| 视觉 | 实测名称为 108 字符：390px Web 与 450 CSS px Electron（200%）中作者两行、回复三行，文字与容器均不横向溢出；截图人工检查 95/100 |

源码、命令、命中清单及原始日志索引见 `research/server-results.json`、`client-result.json`、`acceptance-result.json`、`visual-review.json`。运行截图和完整报告保留在 `/Users/artisan/.multica/acceptance/s1-agent-identity-20261113-01/`。

## 来源与并行工作边界

浏览器验收副本是 `c79dfe304` 加本任务 12 个产品/测试文件；使用独立数据库、API18970、production Web13890、renderer6064。12 个文件在根工作区与副本逐项哈希一致，构建和程序哈希未漂移。Next 自动生成的声明仅发生已有规范允许的 types 路径变更。

验收期间另一会话提交了 F2（`fafb27b1d`）。复核确认其不修改 S1 身份、权限、评论/补读和字节预算合同；最终根工作区补跑 service/handler 编译和新增定向回归 6+1，均通过。完整浏览器证据仍明确归属于前述隔离副本，不冒充 F2 的联合浏览器验收。

## 保留的失败与修正

1. 服务端与客户端新增回归先失败、后通过；一轮共享机器高负载导致 Views 原 5 秒用例超时，降低并发后原标准通过，未提高超时。
2. B05 第一次失败是新增 Agent 文本选择器同时匹配作者标签和原权限文本；第二次失败是 Playwright `:scope/:has` 组合返回空结果。最小 DOM 复现后改为直接作者栏 XPath，第三次完整闭环通过。
3. 补充场景第一次错误开启宿主执行门禁，旧命名空间测试按预期收到 409。改正本任务环境为 `enabled=true, creation_enabled=true, host_execution_enabled=false` 后原测试通过。未放宽产品门禁、修改旧 fixture 或覆盖旧失败。

## 清理与限制

受控运行均保留终结证明；任务执行开关恢复原值。API、Web、renderer 已停止，IPv4/IPv6 端口均关闭；Docker 仅剩原 PostgreSQL。原人用 API18572/Web13492 仍运行；新测试数据库、证据及隔离副本保留。

本包不证明真实模型质量、真实支付修复、生产发布、完整安装版 Electron Main 或 100 人性能；未执行全仓 Go 全量套件。父任务 TC20 只补齐具体作者展示子断言，不等于完整审计闭环通过。
