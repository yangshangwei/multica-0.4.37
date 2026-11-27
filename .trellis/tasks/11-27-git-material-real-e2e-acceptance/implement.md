# 实施与验收计划

状态：in_progress；用户已确认需求、具体设计与跨层合同。当前先交付运行时执行代码改造，后续真实平台/模型验收独立保留。

## 变更计划与行为保护

改动前先锁住现有版本审核、原意图恢复、手动等待和普通任务准入回归。以失败测试证明无connection配置仍失败、物料写入在服务器发生和无运行端回执可误判的现状，再逐层替换。删除物料专用App/VCS凭据路径和死代码；复用现有队列、载体、UI与只读解析，不加依赖、不做无关清理。

## 顺序与职责

- [x] 用户确认无App、GitHub/GitLab统一所选运行时执行，明确要求修改需求/设计/代码；父任务记录D11。
- [x] 独立研究队列/daemon、物料状态/核验、共享UI，形成design.md与contracts.md。
- [x] 完成本轮方案审阅后task.py start，记录当前分支/源快照与既有文档改动归属。
- [x] 后端owner先定义公共settings、operation descriptor/receipt、SQL/migration和唯一identity；生成sqlc，冻结跨层接口（当前续作继续核对生成一致性）。
- [x] 后端lane：去connection/plan_agent硬依赖，system carrier、操作/任务原子派发、当前权限/批准/版本、回执消费、无server provider、计划手动闸门和runtime删除历史保留。
- [x] 运行时lane：共用claim/start/backend.Execute接入物料任务、能力声明、descriptor注入、CLI本地桥、只读观察/daemon回调和结果不确定恢复。所有远端写入仍由所选运行时工具完成。
- [x] 前端lane：core schema/response兼容/旧pending拒绝，单一显式RuntimePicker，运行事实/故障文案与中英文，删除连接/方案Agent前置查询；定向测试记录见research/frontend-implementation.md，当前续作重跑并补生产构建验证。
- [x] 更新内置物料技能、source-map和规范，说明runtime身份/审批/观察的真实边界。
- [x] 集成确定性runtime fixture：真实任务派发→测试runtime工具产生Git效果→daemon读核验→服务端消费，不手工伪造最终状态。
- [x] 独立trellis-check覆盖全改动与普通任务回归，修复后重新运行相关检查；记录证据和未覆盖项，本地提交当前代码交付。

## 写入边界

| Owner | 范围 |
| --- | --- |
| Backend | internal/material、service/material*、material handlers、task/capability准入、migrations/queries/generated和runtime teardown |
| Runtime | pkg/protocol/material.go（先冻结）、daemon运行/观察/本地API、CLI命令及测试 |
| Frontend | packages/core/materials、共享projects material组件、locales及测试 |
| Main | 任务/需求/设计/规范、合同协调、内置技能、集成环境和最终验证 |

共享task.go、task_start.go、daemon claim handlers和protocol只由指定owner写，另一lane提出变更请求，不并发覆盖。新迁移序号先查当前末尾，不硬编码占用编号。

## 验证门槛

1. Go domain/协议/CLI/只读观察：无App/VCS、异常payload、完整路径/字节、假造receipt拒绝、无隐藏mutation或server network路径。
2. DB service/handler/router：新建并核对每包独占数据库再迁移；先定向，再含普通claim/reclaim/start的扩展-race回归；锁冲突不误取消、被拒material不堵普通队列。需要Redis时独立实例。
3. Runtime：fake可执行程序只访问任务fixture，覆盖原生执行调用、精确runtime/profile、旧daemon拒绝、断线/丢回复/重试、token/host challenge隔离；不执行用户安装的真实Agent CLI。
4. TS：settings/schema/draft、实际picker wiring、无连接查询、权限读失效、离线/版本冲突/原请求、auto=false不丢；运行受影响包Vitest、pnpm lint/typecheck和适用build。
5. 生产构建Web/Electron两provider旅程：配置runtime→候选→真实fixture发布→人类审核→runtime合并→基线→手动/自动方案，截图并保留visual verdict。fixture通过不描述为真实模型验收。
6. make sqlc、gofmt、go vet、git diff --check，完整用例结果与源/构建绑定。全仓Go是后续完整验收项；若当前改动影响范围要求扩大，在专用隔离环境执行并记录。

## 后续真实验收矩阵（不因代码通过而勾选）

先GitHub yangshangwei/test--multica，后绑定GitLab；对象、runtime/CLI/模型、调用次数、故障停止和远端操作清单在运行前明确。只用合成资料，保留原真实验收目标：

| Case | 关键证据 |
| --- | --- |
| LIVE-00 | 无App/VCS，产品调度所选真实runtime完成Git操作与受信任观察 |
| LIVE-01 | PRD→真实PR/MR→Multica人类审核→原生门禁→获批字节基线 |
| LIVE-02 | 完整输入→真实Agent生成设计/拆解→自行提交→方案审核合并 |
| LIVE-03 | 自动关闭零模型启动，重开不释放旧待办，人工启动一次 |
| LIVE-04 | 终态人工重试同issue、保留旧run、无并发重复 |
| LIVE-05 | 需求升级使旧方案/批准失效，新计划固定新基线 |
| LIVE-06 | 原生门禁/当前权限拒绝与恢复，完整审计 |
| LIVE-07 | 不确定回执先查事实再恢复，零重复副作用 |

完成后汇总独立平台结论、全仓Go检查、Web/Desktop证据与限制，再决定归档本任务。当前代码交付和真实验收状态分别记录，不能提前宣称真实闭环完成。
