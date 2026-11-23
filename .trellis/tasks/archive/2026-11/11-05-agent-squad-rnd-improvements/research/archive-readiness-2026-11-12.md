# Research: 研发体系优化任务归档准备度

- Query: 核心功能是否已实现；能否按当前完整范围归档；若按阶段结项，哪些事项必须保留。
- Scope: internal；只读检查既有任务文档、归档子任务记录、相关源码和工作区状态；仅新建本研究报告。
- Date: 2026-11-12

## Findings

### 结论

**主要的第一阶段能力已经实现，但整个父任务的原路线没有完成，不能直接把它解释为“全部完成”归档。** 可选做法是把父任务明确收口为已交付的 M0/M1 增量，先保存和提交本轮 UI/CLI 改动，再把下面列出的剩余功能与验收责任迁移到有明确引用的后续任务；这是范围变更后的阶段结项，并非全路线通过。

PRD 明确要求父任务在后续里程碑交付前保持未完成（`prd.md:29`）；设计列出了独立的 M2/M3，而不只是测试清单（`design.md:18`、`design.md:19`）。`task.json` 的当前字段为 M0 integrated、M1 in_progress、M2/M3 planned；其 13 个 children 均在 `.trellis/tasks/archive/2026-11/` 中具有 `status=completed`，不等于尚未拆分的未来里程碑完成。

### 已实现的核心能力

| 范围 | 可确认的交付 | 证据与限制 |
|---|---|---|
| M0 | 结构化凭据脱敏、发布测试数据库前置条件与失败退出、小队交付/验收合同、队长六类检查说明 | `delivery.md:11`；实现提交记录 `1cd9c1e60`、`5975edae9`、`028e93bb0`。历史独立检查和本地集成记录，不是本次重新运行测试。 |
| M1 权限/委派/CI | Reporter/Reviewer 任务 API 最小权限、即时派发结果消费、关键路径 PR smoke、持久化委派回执 | `delivery.md:57`、`progress.md:69`；实时任务委派回执读取代码见 `server/internal/handler/comment_delegation.go:337`。宿主 shell 沙箱不在这些权限结论内。 |
| M1 受控制品与证据 | 受控 local-synthetic 制品、一次领取与 unknown 不重跑、项目政策、不可变持久评估、完成提示 API | `progress.md:5`、`:65`、`:73`、`:77`。local-synthetic 及数据库已观察 PR head 的范围不证明真实部署或远端 Git 当前事实。 |
| M1 完成事务门禁 v1 | 显式人类授权、原事务消费/撤销/恢复、普通/批量/创建/VCS/协作/R38 等实际 writer 接入 | `progress.md:86`；权威元数据记录 `18b743291` 已集成、独立验收并归档，`full_delivery_gate_complete=true`。源码存在 `PrepareDeliveryMutationInTx`、`FinishDeliveryMutationInTx` 与授权 API。 |
| 11 月 12 日提示消费者增量 | Core 严格 DTO/client、共享 Web/Desktop 单事项 status-only 提示协调、CLI status/status-only update 提示 | `progress.md:94`；源码定位见下文。已有定向验证记录，但相关改动仍在工作区，不能把该增量算作已提交归档交付。 |

### 剩余工作必须区分为功能与验收

1. **M1 用户流程与治理剩余范围需单独收口。** 最新包只实现提示消费者：拖拽 move、批量 status、创建即完成/评论转子任务、混合字段更新、独立协作/R38、完成授权 UI、私有 evaluation 的获得/共享都明确不在本包范围（`m1-ui-cli-completion-hints-prd.md:57`）。这些入口的后端 v1 强制约束已经有独立实现，不应把“提示 UI 尚未接入”误写成“后端没有门禁”。尤其当前 UI/CLI 提示不发送 authorization/operation binding（`:88`），提示确认不构成完成许可；若要完整面向用户的 enforce 闭环，需要另行规划验收相应授权流程。
2. **M2 尚未实现的产品能力：** 只读确定性报告快照（scope、时区、统计窗、水位与 coverage）、父任务/项目/小队共享预算预留与结算（含重试和 unknown usage）、能力与负载调度（`design.md:18`、`:45`、`:46`）。这些不能归为“只差真实环境测试”。
3. **M3 尚未实现的持续治理：** 固定负载 SLO 与故障/备份恢复演练、版本化模型行为评测、制品供应链证据、保留实例定制的模板升级及知识有效期/失效治理（`design.md:19`、`:47`、`:48`）。
4. **真实供应商与完整可靠性验收仍开放。** 历史 3 次 Codex 定向样本中 reporter/reviewer 通过，leader 保留了一次 180 秒超时、缺最终回答的失败结果（`progress.md:53`、`delivery.md:65`）。不能把两次成功或假 Provider 检查变成“真实小队完全可靠”的结论。Hosted CI、生产与部署验证也不能由本地证据代替；它们是否纳入后续任务应按阶段交付口径明确。
5. **skip 不是 v1 的漏实现。** 当前 v1 的检查都被明确划为不可豁免，`skip_supported=false`、`waivable_checks=[]`；这是已验收的限定，不应因旧设计提到 skip 就重新打开 v1（`.trellis/spec/server/delivery-completion-enforcement.md:156`）。未来若需要新的可豁免政策，应独立设计。

### 未提交变更对归档的影响

本次只读状态检查发现此增量的确切相关文件仍有修改或未跟踪文件，包括：

- 已修改：`packages/core/api/client.ts`、`schemas.ts`、`types/api.ts`、`issues/index.ts`、`modals/store.ts`；`packages/views/issues/actions/use-issue-actions.ts`、`components/issue-detail.tsx`、`surface/use-issue-surface-actions.ts`、`surface/actions-context.tsx`、双语 issues 文案及 modal registry；`server/cmd/multica/cmd_issue.go` 和测试。
- 新增未跟踪：`packages/core/issues/completion-hint.ts` 及测试、`packages/core/api/delivery-completion-hint.test.ts`、`packages/views/issues/actions/completion-hint-gate.ts` 及测试、`packages/views/modals/issue-completion-hint.tsx`，以及四份 `m1-ui-cli-completion-hints-*` 规划文档。

该列表只用于识别本任务重叠，不判定其他修改所有权，也没有暂存或提交。阶段归档前应将这些改动的范围、当前证据与提交固定下来；任务记录自动归档提交不能代替产品代码提交。

### 文档状态漂移

- `progress.md:3` 的 11 月 11 日摘要仍说 UI/CLI 自动提示开放；`:94` 的 11 月 12 日记录已给出实现与定向检查。以最新具体现实为准，不能继续宣称完全未写。
- `progress.md:98` 将“v1 门禁”混入“未完成”列表，与 `task.json` 中 `full_delivery_gate_complete=true`、11 月 11 日验收及最新提示 PRD `:95` 不一致。应理解为提示包没有重新验收/关闭完整路线，不能推翻已归档的 v1 事实。
- 部分旧 `delivery.md`、`implement.md` 和全局路线文档保留历史 M1 未交付项；它们不能覆盖最新已接受的子任务结果。

### 建议的阶段归档分流

若用户决定不再让这个大父任务长期占用进行中列表，可先形成以下明确继任范围，再对已交付阶段结项：

1. M1 剩余交付 UX/授权流程与范围收口（含当前 UI/CLI 提示增量提交/验收交接）。
2. 真实小队供应商行为与可靠性验收（保留原超时失败，不改判）。
3. M2 确定性报告快照 → 父级预算 → 能力/负载调度。
4. M3 持续质量、评测、供应链与模板/知识治理。

本报告不创建这些任务、不缩减原需求、不更改状态，也没有执行归档。是否保留这些事项或明确放弃某一范围，是主会话与用户的范围决定。

## Files found

- `prd.md`：首批需求和父任务保持开放的约束。
- `design.md`：M0–M3 目标、能力边界、依赖与回退。
- `implement.md`：首批实施和后续执行包。
- `task.json`：13 个归档子任务与 M0/M1/M2/M3 当前里程碑状态。
- `progress.md`：11 月 11 日 v1 验收和 11 月 12 日提示消费者增量。
- `delivery.md`：M0/M1 历史交付、提交与真实样本限制。
- `m1-ui-cli-completion-hints-prd.md`、`design.md`、`implement.md`、`acceptance.md`（均带相同前缀）：最新提示增量的范围和验收矩阵。

## Code patterns

- `server/internal/service/delivery_completion.go:178`：`PrepareDeliveryMutationInTx` 复用真实业务事务；`:306` 完成后再次确认并生成回执。
- `server/internal/service/delivery_completion_authorization.go:40`：显式授权创建；`:241` 授权撤销。
- `server/internal/handler/delivery_completion_authorization.go:89`：HTTP 人类完成授权入口。
- `server/internal/handler/comment_delegation.go:337`：持久化委派回执恢复读取。
- `packages/core/api/client.ts:3097`：提示客户端；`:3107` 固定发送 `evaluation_key:null`，`:3118` schema 解析。
- `packages/core/issues/completion-hint.ts:28`：纯提示意图分类。
- `packages/views/issues/actions/completion-hint-gate.ts:46`：共享提示 gate；`:104` 发提示请求后决定是否继续原 mutation。
- `packages/views/issues/surface/use-issue-surface-actions.ts:108`：列表/看板单事项状态更新接入提示。
- `server/cmd/multica/cmd_issue.go:1451`、`:1567`：CLI status-only 与 status 入口；`:1641` 提示执行器。

## External references

无。本次不需要外部资料，未访问真实 Provider 或账户。

## Related specs

- `.trellis/spec/server/delivery-completion-enforcement.md`：v1 唯一事务授权核心和不可豁免边界。
- `.trellis/spec/server/delivery-completion-hints.md`：提示非授权语义。
- `.trellis/spec/core/frontend/index.md`、`.trellis/spec/views/frontend/index.md`：共享客户端和业务 UI 边界。
- `.trellis/workflow.md`：archive 会将任务标记为 completed，故不能仅当作隐藏未完成工作的目录搬移。

## Caveats / Not Found

- 本次未执行任何产品、单元、集成或真实模型测试；所有通过数字均来自历史记录。本次只核对已有文档、源码存在性与状态，不等于当前工作树全量质量门通过。
- 没有证明当前全部修改都属于本任务，也没有处理其他并行工作的内容。
- M1 “其余治理”的最终验收范围没有一份最新、完整、已收口的父任务清单；阶段归档应先补清范围，不把广泛设计方向直接变成已交付承诺。
- 用户尚未决定缩减原范围或把剩余工作迁出；本报告只提供可审阅的归档建议。
