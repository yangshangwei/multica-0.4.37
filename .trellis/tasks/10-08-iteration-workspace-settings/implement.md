<!-- Approved by user on 2026-10-08: 确认无误，请继续推进. Implementation is authorized for business pages and the independent settings page shown in the final preview. -->
# 实施与验证计划

状态：实施收尾中；用户已于 2026-10-09 授权按父任务 [closeout-plan.md](../10-08-iteration-workspace-settings/closeout-plan.md) 完成修复、验证、提交和归档。此前的规划状态由该授权取代。

## 实施前提

- 用户已批准本方案与页面预览，并明确授权完成 closeout-plan.md 的收尾工作。
- 重新检查工作树及邻近未提交文件；保护当前 .agents/.codex、next-env.d.ts、其他任务证据及 docs/qa/core-e2e-test-plan-2026-10-08.md。
- 使用 Trellis 原生子代理：后端合同与共享 UI 可按明确文件所有权分工，主会话负责整合与独立 check。所有子代理从 `Active task: .trellis/tasks/10-08-iteration-workspace-settings` 开始，读取已整理上下文。

## 1. 固定行为与基线

- [ ] 对照 research/contract.md 重新确认 gate 引用和原有测试；运行相关现有用例。
- [ ] 增加有意义的失败回归：无部署标志也可由管理员启用，但默认禁用及权限不变；设置入口可发现；关闭恢复不受 enabled 状态影响。
- [ ] 用共享组件测试覆盖交互状态，用后端集成测试覆盖事务/权限，不跨层重复完整矩阵。

## 2. 后端与部署合同

责任范围：server/internal/featureflags、相关 iteration handler/service、server/cmd/server/main.go 及对应 tests；部署配置和现行交付文档由主会话整合。

- [ ] 删除 iterations_i1 helper、Available 字段和各运行入口 gate；保留所有 workspace enabled、身份与权限判断。
- [ ] 保持既有能力接口形状，不添加迁移；验证 env 缺失/false 与 DB enabled false/true 的组合。
- [ ] 启用发布提交后的 iteration:updated，复用现有广播和幂等语义；验证分拣能力与跨客户端更新。
- [ ] 更新对应 gate 测试 fixture，保留 stale revision/timezone、越权、未知回执、整集关闭和通知去重测试。

## 3. 共享设置页面与数据刷新

责任范围：packages/views/iterations、packages/views/settings、相关 locales、packages/core/iterations/projects/triage/realtime，以及相关 tests。

- [ ] 添加 IterationSettingsTab 并注册 ?tab=iterations，在分拣台之后加入导航。
- [ ] 使用既有 Settings*、Switch、TimezoneSelect 和语义样式，按 design.md 完成所有状态及中英文文案。
- [ ] 启停独立提交；时区保存/取消独立；脏草稿和未知结果不允许发起相冲突的操作。
- [ ] 复用 IterationOperation preview/apply，只做必要的触发器适配；保留完整明细、2,000 条限制、原因和恢复。
- [ ] 设置页挂载独立恢复组件；本地/WS refresh 涵盖 iteration settings、capabilities、分拣迭代能力与共享项目时区。
- [ ] 旧列表页移除重复配置控件，改为设置链接；启用条件控制工作侧边栏，关闭后历史仍可从设置进入。
- [ ] 复用已有 SettingsPage 平台路由，无需新增桌面业务路由/全局窗口 overlay。

## 4. 集成与验证

- [ ] 组件：设置目录/直达、启停、时区草稿、只读、能力缺失、权限丢失、并发刷新、关闭取消与确认、恢复流程。
- [ ] 后端：human/admin/member 边界、状态保留、disabled assignment、全量预览/超限、原子回滚、历史和执行不变、同请求回执恢复与通知去重。
- [ ] API：原响应 schema 继续通过 malformed-response 和旧服务回退测试；不把撤权 404 当作版本差异。
- [ ] Web/Desktop：运行生产构建的真实路径，启用、创建一期、安排任务、关闭、读历史、重启客户端恢复、两个客户端状态同步。
- [ ] 覆盖英文/中文、深色/浅色、窄窗口、键盘焦点与开关状态；按 visual-verdict 留存截图结果。
- [ ] 对受影响包运行 pnpm lint、pnpm typecheck、pnpm test；Go 运行相关 go test 和 go vet；静态包边界分析按 package.json/Makefile 实际脚本执行。风险需要时扩大到 make check。
- [ ] 不重复跑与本改动无关的全量矩阵；只对新变化、失败或未覆盖的风险追加检查。

## 5. 文档与收尾

- [ ] 清理 .env.example、docker-compose.selfhost.yml 的旧 gate，更新离线脚本 fixture 和当前升级指引。
- [ ] 更新 .trellis/spec/core/frontend/iteration-operations.md、server/iterations.md、server/offline-delivery.md；旧 task/release 证据保持历史原貌。
- [ ] 明确告知旧 enabled=true 空间重新可见和已有通知可能继续处理，禁止自动批量开关工作空间。
- [ ] 用 trellis-check 独立复核后再进入提交/收尾阶段；提交遵守 Lore trailers 与本仓库 Conventional 前缀。

## 回退边界

实现仅改控制路径和 UI，无数据迁移。回退代码不得撤销已经提交的用户启停、时区或历史记录。关闭行为不可用简单的开关回滚恢复，应通过既有原始操作回执判定成功，再由用户明确创建新的周期安排。

## 本轮已完成的规划验证

- [x] 核对分拣台布局、设置导航、迭代启停/时区/历史及权限合同。
- [x] 完成主方案、替代方案取舍、升级影响与测试计划。
- [x] 页面交互原型的启停、原因校验、时区保存和视口检查通过；证据见 preview/verification.json。
- [x] 用户已批准本版方案并授权实施及收尾。
