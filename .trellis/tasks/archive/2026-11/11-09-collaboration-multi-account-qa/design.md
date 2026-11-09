# 跨空间协作闭环测试设计

## 验证拓扑

| 角色 | 工作空间 | 资源 | 责任 |
|---|---|---|---|
| 产品账号 P | 产品空间 P | 需求 Agent P、独立运行环境 P | 创建、补充信息、退回、最终验收 |
| 研发账号 D | 研发空间 D | 修复 Agent D、独立运行环境 D | 实现模拟结果、修改、提交 |
| 测试账号 Q | 测试空间 Q | 验证 Agent Q、独立运行环境 Q | 检查材料、记录问题、复验 |
| 无关账号 X | 无需空间成员身份 | 无 | 访问拒绝检查 |

P/D/Q 各有独立浏览器 context 和认证；任何身份都不通过加入对方空间取得访问。共享的是来源 P 的同一 issue，资源始终留在各自空间。

## 主线：订单去重规则复核

1. 公共 API 创建合成账号、空间，登记三个测试运行环境及 Agent；只在任务拥有的数据库开启必要测试门。
2. P 通过 Web 创建 AI 辅助协作任务并交给 D，附合成说明；邀请 Q。验证三人可访问同一 issue，其他空间和 X 不获得访问。
3. P/D/Q 各自提供资源；P 或 D 采纳。拥有者本人分别授权，明确恢复。保存 offer/target/owner/runtime 关联。
4. P 的需求 Agent 接收预览确认后的指令，通过实际 CLI 留下规则说明和共享上下文版本。
5. D 请求 P 补充重复回调的规则，P 返回信息，处理责任回到 D；验证信息请求的关联与状态。
6. D 触发修复 Agent 第一次执行。模拟 Provider 读取前序上下文并写回结果，保留输入版本、输出及实际启动/退出证据。
7. D 转交 Q，Q 触发验证 Agent，记录预设的缺项。Q 提交第一版结果，P 退回并指定 D 修改。
8. D 再次触发修复 Agent，随后触发 Q 已授权的验证 Agent 复验，确认最新上下文与修改依据被接续；至少观察五次真实模拟进程执行。
9. D 提交第二版结果，P 验收完成。三个会话看到相同终态，第一版结果、退回原因、第二版结果、各运行与上下文历史仍可追溯。
10. 任务完成后不能继续提交 AI 指令；独立对照任务不应被连带改变。

关键业务动作优先通过 UI；账号/运行环境创建可使用现有公共 API fixture。直接 SQL 仅用于任务环境测试门初始化和只读事实校验，不伪造主线业务状态或执行结果。

## 补充边界矩阵

| 编号 | 检查 | 断言 |
|---|---|---|
| B01 | X 读取共享任务/文件 | 拒绝且正文不泄露 |
| B02 | D/Q 读取 P 的其他任务 | 无额外工作空间权限 |
| B03 | 登记/采纳但未授权 | 不可启动 |
| B04 | 他人代替拥有者授权 | 拒绝且资源授权不变 |
| B05 | 授权但尚未恢复 | 不自动执行 |
| B06 | 普通评论 | 不生成 AI 运行 |
| B07 | 原请求键重复或响应丢失后查询 | 不重复创建/派发/交付 |
| B08 | 停止、等待退出、显式恢复 | 停止意图和退出事实区分，旧输入不被错误重跑 |
| B09 | 撤权及重新授权 | 阻止新执行，重新授权不等于恢复 |
| B10 | 上下文并发修改 | 冲突显式返回，不覆盖他人版本 |
| B11 | AI 运行成功 | 业务任务仍等待人工交付/验收 |
| B12 | AI 小队 | Leader 委派、成员执行、Leader 继续，资源归属据实记录 |

危险或会打断主线的场景使用额外测试任务；报告明确标为补充测试，不能冒充主线步骤。

## 设施复用与缺口

- 复用 `e2e/fixtures/collaboration-host-runtime.ts` 的账号、托管运行环境、实际 poller、Provider callback、启动/退出记录和 Web 提及操作。
- 复用 `e2e/fixtures/collaboration-memory.ts` 与 `e2e/fixtures/collaboration-memory-provider.go` 的上下文读写能力。
- 普通 host-provider 支持 scripted comment/squad_activity/write_file；memory-provider 自动读写上下文但回调只接受 release，不能向它发送 `daemon.act`。主线使用任务专用模拟 Provider 组合必要动作，或分别指定 Provider；不得假设二者协议可互换。
- 新增 `e2e/collaboration-multi-account-closure.spec.ts` 和必要的同名 fixture，以最小补充组合三方主线。必要时只扩展本次测试使用的模拟 Provider，不修改产品代码。
- `adoptHostTarget` 假设拥有者也是管理方且所有资源属于该拥有者，不能直接套用于外部参与者；新 fixture 必须拆成 owner offer、P/D adopt、owner consent。
- 人工 detail journey 使用 legacy 认证，不能原样混进 password host 环境；复用 UI 动作模式和断言，保留新场景的 password 登录事实。
- 现有 squad 测试的成员同属一个 owner，报告不得称其为多 owner 小队证明；本次多 owner 证据来自主线三个直接目标。

## 隔离与环境

- 建立新的任务私有配置、注册表、数据库和证据目录，记录分配清单。不要只改环境名：`dev-env.sh up` 可能按 cwd 复用环境，同名数据库也必须先检查。
- 配置文件按数据解析，不 source 不可信 dotenv；不继承默认 `.env.worktree` 或默认 `DATABASE_URL`。
- `MULTICA_AUTH_MODE=password`、`MULTICA_PASSWORD_LIMITER_MODE=single`，启用托管安装所需配置并关闭 device/cloud 冲突路径。真实配置以当前验证器为准。
- Web 必须 production build/start；核对浏览器实际 API/WS 地址、`/health`、`/api/config`、数据库身份及 listener PID。
- 复用 host fixture 需要真实 Desktop renderer URL，即使主线只用 Web，也提供本次构建的 renderer；不把无关 URL 填入满足检查。
- 编译 `./internal/daemon` 测试二进制只用 `go test -c`；运行其明确的 poller 入口。Provider 和 CLI 都是本次编译的绝对路径，空白账号目录，不使用环境中安装的真实模型 CLI。
- `make check` 强制 legacy auth，不作为 password host 主线入口。所有共享数据库门用例串行；独立审阅可并行。
- 测试结束只关闭本次拥有的进程，保留数据库和脱敏日志。失败环境不重置成通过证据；重跑分配新 run ID。

## 报告与判定

输出 `artifacts/qa/collaboration-multi-account-20261109/report.md` 及同目录脱敏 JSON、截图、日志索引。含环境与源码身份、拓扑、逐步矩阵、明确数量、问题严重性与复现命令、未覆盖项。

成功需要主线所有 AC 实际通过且每项证据一致；部分通过、被门禁跳过、环境阻塞或仅 fixture 编译成功不能标记完整闭环通过。总体结论区分产品缺陷、测试设施问题和环境问题。既有失败和修正后的重跑分别保留。
