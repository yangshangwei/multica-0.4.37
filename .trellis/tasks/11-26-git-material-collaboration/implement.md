# 实施与验证计划

状态：实现与独立检查通过，整理本地提交。用户已确认需求到方案审核闭环并要求完成开发；父任务跨模块编码与 QA 愿景不在本期。

## Ordered delivery

- [x] 核对 main、拉取 origin/main、建立 `codex/git-engineering-material-workflow`。
- [x] 读取父任务 D1–D10、PAC1–PAC15 与稳定目录设计；创建开发子任务。
- [x] 收到本轮首期范围答复，更新 PRD/设计的范围与政策细节；同步父任务进展。
- [x] 完成当前后端和前端复用研究，将确切文件、权限、SQL 与 UI 模式加入上下文清单。
- [x] 冻结领域/HTTP/provider 合同及状态转换矩阵并激活任务，见 `contracts.md`。最终合并前的目标与路径复查同时受供应商 API 的原子性限制。
- [x] 服务端主线：领域类型、独立迁移/查询、当前权限检查、不可变修订/决定、持久化操作和恢复；先写能证明拒绝路径与幂等性的测试。
- [x] Git 适配线：GitLab/GitHub 发布、审核同步、门禁/完整变更观察、按 HEAD 合并与内容核对。两平台复用同一行为矩阵，各自 HTTP 合同测试。
- [x] 客户端线：schema/API/query，项目配置和物料列表/详情，候选正文/差异/评论/决定/恢复；共用 Web/Desktop 页面及双语文案。
- [x] 方案主线：固定基线输入、唯一方案任务、开关默认/关闭持久化、统一人工等待闸门、幂等派发及当前方案审批；检查所有创建/指派/恢复/领取入口。
- [x] 插件/自动产出接入：宿主窄 API、bridge/SDK capability、候选输出及相关内置技能/source-map；不得扩大只读研究角色权限。
- [x] 集成：默认开启、关闭后人工启动、版本更新、失败恢复、原生门禁阻塞以及未经方案批准不编码。
- [x] 独立 `trellis-check` 审查并修复；主会话核验未修复发现、更新 spec、按 Lore/Conventional 规则提交本任务改动。

## Ownership and sequencing

先冻结合同，然后可由三个独立实现 lane 分别承担后端持久化/权限、provider 适配、共享客户端；跨层集成和任务状态由主会话负责。后端 migrations 与 sqlc 生成只由一个 owner 修改。每个实现/检查 agent 使用明确 task path 和对应 JSONL；禁止修改其他任务/已有无关产物。

## Verification commands and evidence

以下是交付门槛，不等同于已执行记录。前端初次检查见 `research/frontend-implementation.md`；续作的实际执行结果及未覆盖范围见 `research/resume-verification.md`。

- 领域/provider：`go -C server test <owning-package> -count=1`，必要恢复/并发用例加 `-race`。
- DB/handler/router：先创建并记录任务独占数据库、核对实际库名、迁移，再按包串行执行；不得使用继承环境中的未知 DATABASE_URL。
- Go 静态检查：`go -C server vet <affected-packages>` 与相应构建；SQL 变更用根 `make sqlc` 的固定生成版本。
- 客户端：定向 Vitest 覆盖异常响应、权限/版本变化、保存失败与待办/运行真实显示，再执行 `pnpm lint`、`pnpm typecheck`、`pnpm test`。
- 集成：生产 Web 与 Desktop 共享链路，记录构建/源版本和测试环境；UI 变动按仓库 visual-verdict 要求保留结果。
- 整体后端：依据实际改动与验证环境运行 `make test`；如果全仓存在无关失败，保留原始失败并说明作用域，不通过删除测试获得通过。

## Regression matrix

| Boundary | Required evidence |
| --- | --- |
| Submission | Ordinary dialogue/incomplete draft produces no intent; reviewable revision produces one PR/MR; unknown result recovers |
| Authority | Workspace/project isolation; machine approval refused; removed reviewer/originator blocks subsequent actions |
| Revision | Old approval cannot merge changed HEAD; target changes and extra files block; historical decisions retained |
| Provider | Full changed-file set; symlink/rule-file denial; native checks/approvals honored; conditional merge |
| Baseline | Actual merge and approved bytes verified; queued/timeout/external unapproved merge not effective |
| Handoff | Full immutable PRD; digest/read failure blocks; same baseline gives one task/run |
| Manual hold | Disabled config survives reload/assignment/recovery/claim; manual start reuses task; re-enable does not drain old todos |
| Plan review | Version-bound design/task plan approval; source change expires approval; coding refused before approval |
| Product | Web/Desktop parity, long content, errors, stale private cache, server enum/response drift |

## Rollback points

Schema/additive reads → controlled publication → automatic merge → stage dispatch 分别独立启用和核验。停止新副作用后继续只读对账，保留真实远端变更和历史证据；不删除已审核材料或用 Git 回滚伪装操作从未发生。

## Final gate

2026-11-27 完成独立 provider、backend、frontend 复查与跨层复查，回归/构建/本地 Web 和 Electron 旅程通过。准确命令、失败保留、源摘要及验收限制见 `research/final-verification.md`。未运行全仓 Go 套件；本次采用受影响包的完整或扩展定向竞态测试。
