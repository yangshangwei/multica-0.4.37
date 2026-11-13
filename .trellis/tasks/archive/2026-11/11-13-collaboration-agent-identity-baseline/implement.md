# S1 实施与验证计划

状态：completed。2026-11-13 用户批准执行后，B01–B10 验收和归档已完成。

## 1. 激活与归属

- [x] 用户确认首包范围；创建S1子任务，父任务保留全量24用例。
- [x] 核对DTO/可信身份源、共用投影、Core/schema、列表/回复提示、原canonical闭环。
- [x] 完成PRD、设计、基线矩阵与上下文清单；首包产品决定已收敛。
- [x] 用户批准最终方案，已启动本子任务。
- [x] 已读取规范、上下文和工作区差异；execution-baseline.json / environment-allocation.json 记录基准与隔离环境。

按server → core/views → 集成E2E的依赖顺序推进。文档/测试研究可并行；API合同变更顺序固定。采用trellis-implement实现、trellis-check独立检查，主会话负责协调、环境、规范、最终提交和证据。

| 执行范围 | 文件归属 |
| --- | --- |
| 服务端投影及回归 | `server/internal/service/collaboration_comments_view.go`；对应comment测试；`server/internal/handler/collaboration_comments_test.go`及recall预算测试 |
| Core与共享界面 | `packages/core/collaboration/content-schema.ts`及测试；`packages/views/collaboration/comment-list.tsx`、`comment-composer.tsx`、对应测试及一个必要的小展示helper；必要的en/zh-Hans词条 |
| E2E与证据 | `e2e/collaboration-multi-account-closure.spec.ts`；必要的同域fixture接线；本任务research中的报告与结果 |
| 主会话规范更新 | 协作closure QA、content/workbench相关规范中受本字段影响的句子；父任务结果映射 |

代理明确知悉共享工作区中还有其他改动，只修改分配文件，不重置或认领他人代码。测试数据库由主会话分配并传入精确包/命令，子代理不自行迁移、切库或清理其他环境。

## 2. 实现顺序

1. 按B01/B02先写失败服务端回归，证明当前响应缺具体身份；核对失权拒绝和预算计数。使用共享dbfx/testutil，不开真实Provider。
2. 增加 `author_agent` 和批量查名，三个公开读取入口共用；既有human权限、回执和runtime写入逻辑不变。
3. 按B03/B04先补解析及共享显示回归，再实现安全回退、具体名称+智能体标识和一致回复提示。
4. 运行窄层检查，完成API/schema及旧客户端兼容复核后，再扩展canonical E2E断言。保留旧author=null断言，额外验证author_agent而非替换原合同。
5. 完成独立数据库/API/production Web/renderer与三类测试二进制绑定。执行B05–B09；收集source、run、评论、批准、终结和清理证据。
6. trellis-check检查最终跨层差异、隐私投影、畸形字段、语言显示、测试归属和既有循环；必要修复后重跑对应受影响检查。
7. 更新规范和本包报告，父矩阵仅补S1子断言结果；总24用例状态不能自动全PASS。
8. 主会话提交本包文件，使用Lore格式说明兼容取舍/测试/未测；关闭并归档子任务须所有S1验收有实际证据。父任务S2–S5保持开放。

## 3. 验证入口

以下入口已按本轮记录执行；实际命令、命中数量与额外完整检查以 report.md / research 证据为准。

```bash
pnpm --filter @multica/core test collaboration/content-schema.test.ts collaboration/comment-actions.test.ts collaboration/comment.test.ts collaboration/command.test.ts collaboration/operation.test.ts api/ws-client-account.test.ts collaboration/run-handoff-queries.test.ts
pnpm --filter @multica/views test collaboration/comment-list.test.tsx collaboration/comment-composer.test.tsx collaboration/workflow.test.tsx collaboration/delivery-panel.test.tsx
pnpm typecheck
pnpm lint
```

Go：为service/handler分别准备自有数据库，执行 `TestCollaborationComment*`、`TestCollaborationRecall*` 和实际新增身份用例；使用 `go test -json -count=1` 保存命中/跳过清单。有共享查询/权限时复核相应race测试，执行 `go vet`；不得用 `-run` 代替DB隔离。最终全量/静态检查按实际改动与仓库要求运行，若有既存失败明确归因并保留证据。

有完整测试环境后，使用环境适配入口运行：

```bash
make env-exec ARGS="-- pnpm exec playwright test e2e/collaboration-multi-account-closure.spec.ts --workers=1 --retries=0"
```

离线/撤权/handoff专用E2E按B07–B09逐项切换其明确的Provider配置再运行，不能把closure Provider喂给不同协议的fixture。所有必需项skip视为未完成，不可只按进程退出0计通过。

## 4. 风险和停止条件

- 若实现需要修改人工权限、runtime身份、数据库结构或新增历史名称快照，停止该扩张并回到设计。
- 若新字段使分页超预算、旧响应整体失效或缺资源授权用户无法读已有评论，视为首包回归，修复后重新验证。
- 环境源不一致、测试误指旧数据库、相关源码运行中漂移均保留失败，不继续写未知环境；只处置本任务自有资源。
- 无真实支付、真实Provider、100人负载或生产发布动作；这些不是S1完成证明。

## 5. 最终输出

变更文件/简化说明、B01–B10实际结果、命令及命中数量、源码/环境身份、原始失败和修正结果、Web/renderer证据、兼容与剩余限制。评论作者可见不等于完整TC20审计链全量通过。

## 6. 执行结果

B01–B10 全部 PASS，独立最终审阅通过。完整证据、失败分类与清理见 report.md；父任务完整 24 用例仍开放。
