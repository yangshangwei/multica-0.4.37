# 测试数据交付与验收

## 已交付数据

工作区：**桌面手测 10-08（保留数据）**，slug `desktop-qa-1008-1743`。当前 PostgreSQL：`multica_multica_0_4_37_492`，API `localhost:18572`，Desktop renderer `localhost:5666`。

新增 **44 条任务 DQA-16 至 DQA-59**，工作区共 **59 条任务**。所有新样例以 `桌面场景 ·` 开头；普通任务 42 条，待分拣和暂缓样例各 1 条。

| 场景 | 新增数量 / 入口 |
| --- | --- |
| 任务状态与优先级 | 全部 7 种状态、5 种优先级；我的任务/任务列表/看板 |
| 项目生命周期 | 5 个项目：客户交付、平台体验、数据迁移、身份认证、旧方案归档 |
| 计划迭代 | 下一轮回归；18 条参与记录和 18 条范围事件 |
| 层级与依赖 | DQA-18 父任务，3 个直接子任务，其中一个再含子任务；3 条依赖 |
| 长文本与日期 | DQA-26 长标题；无日期、逾期、临近截止日组合 |
| 标签/订阅 | 4 个标签、52 条关联、53 条订阅 |
| 评论协作 | 8 条评论，含回复、智能体评论和已解决线程 |
| 收件箱 | 6 条通知，覆盖提及、评论、分配、状态变化、失败，以及已读/未读/归档 |
| 聊天 | 2 个会话、6 条消息、Markdown 和快捷建议 |
| 模拟智能体 | 回归助手 + 离线演示设备 + 1 个快捷操作 |
| 执行历史 | 5 次：完成 2、失败 1、取消 1、排队 1；16 条过程消息 |
| 项目进展 | 1 条进展、1 条风险记录 |

已有 15 条任务、2 个项目、6 个迭代、4 条评论及分拣/迭代设置按原始 ID 与完整字段逐条比对保持一致。原有活动迭代不写入；三份已有冻结历史通过真实 API 复查。新增数据最终保留，不执行 live reset。

## 验证证据

- `make seed ARGS='--dry-run'`：真正只读；目标 DB/workspace 正确。
- `make seed`：写入成功；再执行 `make seed` 的 `inserted` 为 `{}`。
- 完整当前 schema 隔离测试库：seed → reset → reseed → repeat；非 fixture 哨兵保留。
- `SEED_TEST_DATABASE_URL=<scratch> node --test scripts/seed-rich-test-data.test.mjs`：**13/13 通过，0 跳过**，包含中途唯一冲突后编号与父子记录一起回滚。
- Node `--check`（两个脚本）及 diff whitespace 检查通过。
- 全仓 `pnpm lint`：6/6；最终 `pnpm typecheck`：9/9。首轮 typecheck 期间另一项 MCP 功能正在编辑，产生临时错误；其完成后复验通过。
- 31 个真实 API GET 检查通过：42 条正式任务、7/5 状态/优先级、5 个项目统计、2 条进展、18 条计划迭代范围、评论线程/子任务/标签/订阅、两个分拣视图、收件箱/归档、两个会话/消息、5 次执行/过程消息、既有冻结历史。
- 当前 renderer + 当前 preload 的独立 Electron profile：**9 个页面通过**（任务、评论线程、项目、项目风险、迭代、分拣、收件箱、聊天、智能体）；0 页面错误、0 API 请求失败、0 非 2xx、daemonStarts=0。

证据目录：`.artifacts/rich-test-data-20261012/`，含 baseline、写入/重跑回执、保留性比对、API 检查、native screenshots、测试和静态检查日志。

## 边界与复用

执行记录为合成数据；未启动模型或真实 agent CLI。离线排队样例可在普通重连宽限期（默认 3 小时）后被服务端 sweeper 更新，重复 seed 保留此真实产品变化。

保留编辑：`make seed`。预览：`make seed ARGS='--dry-run'`。显式清理本数据包：`make seed ARGS='--reset'`；若用户新增数据引用样例，清理会拒绝，避免孤儿记录。查看具体参数与保护条件见 `scripts/seed-rich-test-data.mjs` 和 `.trellis/spec/server/development-fixtures.md`。
