# 2026-10-17 新需求核对与实施验证

## 结论

新版需求确实要求调整既有单目标实现。本轮完成了多目标存储与输入身份、目标/整单控制、撤权和生命周期联动、旧执行入口隔离，以及服务端/core 的 v2 读取契约。代码经独立复核和最终集成验证，当前交付范围没有未关闭问题。

任务仍为 **in_progress**。这次验证覆盖服务端基础与 core 契约，不代表 97 项完整产品验收完成；实际 @提交、账号通知、真实调度/停止证明和 Web/Desktop 业务闭环仍需后续实现。创建和正向执行入口保持未开放。

## 需求与代码核对

| 范围 | 核对结论及本轮处理 |
| --- | --- |
| 人类交接、字段编辑、权限与原请求幂等 | 复用既有基础，回归验证转交、补充、验收、日期/属性编辑及来源成员离开后的权限 |
| 单 session/grant/run 假设 | 新增 typed target、逐目标序号、根执行组与成员关系；新指令固定目标，无法确定目标的旧记录保留身份并隔离 |
| 两级停止和撤权 | 统一 scoped stop；单目标不误停其他目标，整单覆盖所有目标，单个成员结束不结算整组，旧失效事件不误停同目标的新授权工作 |
| 旧执行旁路 | service/SQL/claim finalization 一致排除协作来源及其捕获内容；普通子任务和纯父引用保持原行为 |
| 客户端兼容 | v2 完整读取投影；未知/损坏协议不能被忽略为空闲，旧服务器仍可提供人类读取但无执行能力 |
| @提及与界面 | 现有普通 parser 会合并 agent/squad，候选还会混入空间目录；已记录后续专用入口要求，本轮没有冒充这部分已经交付 |

原始核对证据见[执行审计](research/revision-execution-audit-2026-10-17.md)、[提及与客户端审计](research/revision-mention-client-audit-2026-10-17.md)。

## 主要改动文件与简化

- `server/migrations/620–642`：目标/组/派生数据、旧记录隔离及独立 concurrent indexes；`650`：普通执行 SQL 边界。没有修改旧迁移或引入外键。
- `server/internal/service/collaboration_queue.go`、`collaboration_targets.go`、`collaboration_control.go`、`collaboration_groups.go`、`collaboration_invalidation.go`：固定输入目标，统一组收尾与控制。
- `collaboration_workflow.go`、`collaboration_lifecycle.go`、`collaboration_account_reconciliation.go`：删除重复的单 run 处理，复用同一组/目标停止原语。
- `issue_admission.go`、`issue_task_transaction.go`、`task.go`：复用现有 service/SQL 检查，补入队、复用、领取、启动与凭据收尾检查。
- `collaboration_execution_view.go`、`handler/collaboration.go`、`packages/core/collaboration/`：借用现有授权快照和私有解析通道，增加明确版本及一致性检查。
- 对应 Go/core/真实路由测试、sqlc 生成文件、内置任务 skill/source map 与[当前代码规范](../../spec/server/collaboration-targets.md)同步。

完整产品文件列表及 SHA-256 见[机器可读验证摘要](verification-summary-revision-2026-10-17.json)。没有新增项目依赖。

## 最终验证

| 检查 | 结果 |
| --- | --- |
| 仓库指定 Go 1.26.6，六个受影响包，`-race` | 323 个顶层测试、638 个子测试通过；0 失败 |
| 同一范围的本机 Go 1.27 验证 | 通过；三个旧路由 fixture 修正后重跑受影响包 |
| Go vet | 两个工具链的 `go vet ./...` 均通过 |
| core 全量 Vitest | 255 个文件、3452 项通过；0 失败/跳过 |
| 全仓 TypeScript | 9 个任务通过 |
| 全仓 lint | 6 个任务通过，0 错误；Desktop 既有 1 条 hook 警告保留 |
| 数据迁移 | 隔离库升级；旧数据/未知证明保留；23 个新增基础文件的保留数据与并发回退拒绝、空库回退通过；650 另有独立回归 |
| sqlc 再生成 | 97 个 Go 文件逐一 SHA-256 比较，0 变化 |
| 独立复核 | 两个停止边界问题已修复并回归，无剩余阻塞项 |

Go 广泛回归中的 `TestTriageQueueScaleBaseline`（需显式启用性能测试）和 `TestTriageWireFixtures`（仅供跨包 fixture 导出）按原规则跳过；它们不属于本任务验收。新协作与边界测试没有因缺少数据库而跳过。

复核修正的两个问题是：目标已缺失但其运行/组仍保留时，整单操作不能漏停后报告成功；旧 active-run 指针悬空时，不能报告 already_ended。新测试要求整笔事务回滚、版本/事件/屏障保留。详情见[独立复核报告](research/verification-revision-review-2026-10-17.md)。

第一次合并验证仅三处真实路由测试仍使用旧的无目标插入。改为显式 typed target 和 input_event_id 后，相关路由与整套指定工具链回归均通过，没有放宽 schema 或改变产品路由。

## 复现与证据

证据目录：`/var/folders/3n/gbt3p39s5pdc4l55js62gxnm0000gn/T/multica-collab-revision-20261017-hgtrswox`。其中 `integration.env` / `check.env` 为权限 0600 的任务独占数据库配置，只 source，勿输出或提交内容。原库基线为迁移 619，本次运行没有对应用库做测试回退。

```bash
# 先 source 上述目录的 integration.env
GOTOOLCHAIN=go1.26.6 bash scripts/go-test-with-agent-cli-guard.sh -- go -C server test -race -p 2 -parallel 2 ./internal/service ./internal/handler ./cmd/server ./internal/migrations ./internal/collaboration ./internal/middleware -run 'Test.*(Collaboration|LegacyExecution|Triage|FinalizeTaskClaim|ManagedReclaimRejectsEarlierDeliveryRollback|SourceContext)' -count=1
GOTOOLCHAIN=go1.26.6 go -C server vet ./...
pnpm --filter @multica/core exec vitest run --maxWorkers=2
pnpm typecheck
pnpm lint
make sqlc
```

日志：`go126-race.jsonl`、`go126-vet.log`、`integration-go-race*.jsonl`、`core-full.json`、`workspace-typecheck.log`、`workspace-lint.log`、`sqlc-final.log`，以及独立 reviewer 的 `check-*` 记录。测试未使用已安装 provider CLI 或真实付费 AI。

## 后续边界

下一步按 P1–P8 接续完整资源 consent、私有内容/@提交/通知和运行协议，再接通真实调度、正向恢复及双端工作台。增加独立 runtime writer 前还须统一当前 Stop 的 group/run/command 后置锁顺序并补跨 writer 竞争测试。原生 containment/proof、容量 1 小队接续、完整 Web/Desktop 端到端与生产发布均未在本轮验证。

## 提交与最后格式检查

- `e6bc3759e`：普通执行入口隔离。
- `f5259e245`：多目标基础、组/范围控制、v2 读取与规范。

暂存检查发现 45 个新迁移文件多出末尾空行，已仅去掉多余 EOF 空行；SQL 有效内容逐字节保持不变。随后重新通过 Go 1.26.6 race 的完整迁移升级/回退演练，并再次确认 97 个 sqlc 文件无变化。最终源码摘要已更新，格式修正没有引入行为变更。
