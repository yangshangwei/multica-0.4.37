# 知识沉淀与 runtime 集成验收

当前状态：完整确定性验收与独立终审已通过，G2 确定性门关闭；已合回 `codex/cross-workspace-collaboration`，合并提交 `8cb04e3295474073b4b991bc4883ef3901c14f01`。
最终产品提交：`96cedf4850319a06c98fdda3d67d82595f01b6b2`。
知识开关保持默认 `false`，没有推送、部署或发布。

## 交付范围

- 人工候选、精确版本批准、纠错/派生、过期与双侧撤回；来源权限不会因批准扩大。
- 独立目标修订、可恢复失效任务、仅携带目标身份/修订的通知；客户端最多 30 秒可见租期。
- 普通 issue/comment/Chat/quick-create/autopilot 和协作运行的协议、客户端、路由及启动接线。
- 独立的披露授权与每次物理启动授权、原请求恢复、原生 session 累计 exposure 及旧客户端拒绝保护。
- Shared Web/macOS Electron 页面、内置指导、server/Core/views 规范及运维说明。

此前独立提交：A1 `71379546c`、B1 `b6b761708`、C1 `bc9c3f10b`、数据库显式配置防护
`bbbfc731d`（主分支 `8ea4ef7b2`）、B2 `53a166258`。最终合入范围和每个文件的 SHA-256
见 [product-source-final.json](research/integration-verification-2026-11-01/product-source-final.json)。
主分支已有的五个 runtime 回归以只读源码副本参与验证，来源见
[main-regression-overlay.json](research/integration-verification-2026-11-01/go-final/main-regression-overlay.json)。

## 确定性证据

| 范围 | 当前结果与证据 |
| --- | --- |
| 迁移和 SQL | 747–760 的 fresh/upgrade/reapply/down 保留合同通过；13 个单语句 concurrent 索引，无新增 FK。最终 sqlc 1.31.1 生成 129 文件、零漂移，见 [sqlc-drift.json](research/integration-verification-2026-11-01/go-final/sqlc-drift.json) |
| 前端 | 5 个非 Mobile workspace：1,063 文件、13,060 测试；lint/typecheck/test 通过，零缓存命中；最终 3,408 源文件指纹未变化，见 [summary.json](research/integration-verification-2026-11-01/frontend-final/round2/summary.json) |
| 审批、恢复、删除投影 | 30 个叶子场景 normal/race 通过；实际角色/lead/账号/来源写者、纠错竞争、错误候选与跨 kind 回执均有执行证据，见 [handoff](research/approval-matrix-completion-2026-11-01/handoff.md) |
| 准备与普通写者 | 12 个叶子场景 normal/race 通过；真实建单/描述、审批与无关写入、披露前撤权、build→finalize、评论合并作者校验，见 [handoff](research/preparation-matrix-completion-2026-11-01/handoff.md) |
| 普通实际输入组合 | 最终 `-race`：14 顶层 / 27 pass 事件 / 23 叶子，零失败或跳过；真实 server/DB/HTTP/WS/daemon，Provider 为测试可执行文件，见 [command](research/integration-verification-2026-11-01/go-final/ordinary-composite-final-command.json) |
| Host 实际输入组合 | 最终 `-race`：8 顶层 / 16 事件 / 13 叶子，零失败或跳过；真实 server/DB/Runner/HostExecutor，见 [command](research/integration-verification-2026-11-01/go-final/host-composite-final-command.json) |
| 知识版 squad | root→member→continuation 全部协商 KnowledgeVersion 1，保留作者/读者和原请求身份，双评论合并，正常结算；进程终止为标明的合成观察，实际 argv/stdin 由其他组合层证明，见 [handoff](research/squad-knowledge-acceptance-2026-11-01/handoff.md) |
| Web/macOS Electron | 当前整合快照 7 Web + 1 原生 Electron 用例通过，零跳过/自动重试；原 30 图/18 组视觉审查通过。见 [combined verification](research/ui-platform-acceptance-2026-11-01/combined-source/verification.json) |
| Go build/vet | `build-combined` 及清理修正后的 `vet` 均退出 0，执行期间全部 server 源码指纹不变 |
| 全量 Go | 82 包：73 个含测试包通过、9 个无测试包；10,284 顶层 / 20,557 pass 事件 / 18,797 叶子，零失败；21 个既有跳过逐项保留。按 `rest`→`affected-rest`→`db-clean` 的最新完整包结果合并，见 [combined inventory](research/integration-verification-2026-11-01/go-final/combined-inventory.json) |
| 独立终审 | [final acceptance review](research/final-acceptance-review-2026-11-01.md) 对照实际源码、具名测试、失败修正及明确边界复核 |

Web 人工流程与 fake-runtime 输入组合采用分层证明。没有将其描述成每个平台上的一条
UI→真实 Provider 旅程；原生“仅目标管理员”未单独重跑，Web UI6 与共享组件拥有该行为。
UI 快照之后只新增已独立审阅的查询计数接入和内置指导压缩；最终输入组合已包含这些字节。

最终数据库为新建并迁移的 `multica_knowledge_final_clean_20261101_80a134db`，handler→service 串行，Redis 使用任务专属实例。全套期间产品源码未变化；只有两个测试的 teardown 补齐，后续 11 个夹具场景→4 个既有实时投递测试在 normal/race 中均通过，关闭该测试增量。详见 [cleanup evidence](research/fixture-cleanup-2026-11-01/handoff.md)。21 个跳过包括平台/root、可选依赖/外部集成、性能/导出/空目录场景及两项既有测试缺口；Redis 测试全部实际运行。环境为 macOS arm64、Go 1.27.0、Node 22.23.2、pnpm 10.28.2；这是本机验证，不是远端 CI 或跨平台证明。

## 全套发现与修正

| 发现 | 修正与验证 |
| --- | --- |
| 普通项目描述编辑阻塞知识准备 | 仅 UpdateProject 改用 `NO KEY UPDATE`，保留 revision、lead/association 和删除保护；原实写者 red→green，131 顶层/238 叶子 race 回归及独立审查通过 |
| 六处查询遗漏 runtime 计数 | 复用现有服务 RuntimeLookup，保持相同事务、ID、返回值和错误；静态防回退用例、13 个反向依赖包及独立审查覆盖 |
| 失效测试污染共享目标历史 | 复用既有独立 workspace fixture；原顺序 red→3/3 green，整包 handler 和知识 race 通过；没有削弱缺失修订计数的拒绝规则 |
| workspace 删除清单缺少新表 | 分类为保留 candidate/invalidation、删除 target counter；实际删除回归证明先写最终 hint、保留历史与 source 删除防护；未修改产品清理逻辑 |
| 复用验收库残留扫描与通知 | 53 份全来源扫描至少需要 12,720 步，超过测试固定 10,000 上限；另有两个测试遗漏第二来源清理，旧事件指向已删 issue。保留旧库，只读诊断后新建验收库，并补齐两个测试的准确清理；受保护历史未删除，顺序回归 normal/race 通过 |
| 内置指导超过上下文预算 | 描述压到 219 字符、正文 496 行；完整协作规则仍由必读的既有参考文档承载；模板套件和独立语义审查通过 |

复用库诊断见 [retained database diagnosis](research/go-final-retained-db-diagnosis-2026-11-01.md)。所有旧全来源任务后来完成，没有发现扫描游标停滞；测试造成的孤立事件与实际受保护的来源删除契约分别处理。整包复跑使用全新专用库的规则已写入测试规范。

各初始失败保留在原始 JSONL 中。最终通过记录替代当前判定，不改写历史失败。

## 测试隔离事件

2026-10-31 两次旧 C2 “compile-only”命令使用 `go test -run '^$'`，未显式指定
DATABASE_URL，但 handler TestMain 仍可能执行。那两次命令未选择 cmd/server。
历史目标推定为 localhost/multica，缺少调用前连接快照；退出 0 也可能来自连接失败后跳过，
因此无法确定当时是否发生 fixture 清理。事后只读检查发现保留名称对应行均为零，不能证明
原先匹配的数据未受影响。没有进行推测性恢复或默认数据库写入。

现在两个 TestMain 都在连接前要求显式数据库，缺失时退出 2；子进程回归还使用不存在的
socket 与强制只读配置。原始调查见
[testing-database-note.md](research/server-execution-2026-11-01/testing-database-note.md)，
主分支只读调查随最终合入保留。

## 仍开放的独立门

- 原系列下一入口 E2a trial 3、剩余 22 次；最近 trial 2 为 upstream 502 无效样本。
  等有恢复依据后安排独立窗口，保留原场景、模型、编号与停止规则。
- 新知识 runtime 的真实验收按 [独立登记计划](real-model-acceptance-plan.md) 绑定自己的
  最终版本、框架、CLI/模型和运行身份；没有借用旧 E0/E1 结论，也没有新增调用预算。
- Windows 原生 lifecycle/ACL/Job、各安装包组合、实际升级与多账号/额度溯源仍属于父任务
  和跨平台发布矩阵。技能层保持本轮范围之外，父任务不标整体验收完成。

测试服务已关闭的 UI 环境、任务数据库、隔离 checkout 和本地原始 trace 保留用于复核；
反复截图与大体积 trace 不进入提交，路径/哈希见
[local-artifact-retention.json](research/ui-platform-acceptance-2026-11-01/local-artifact-retention.json)。

最终合并后的 4,437 个 server 文件、3,408 个 frontend 文件及 156 个审阅路径全部匹配验收指纹，见 [main integration](research/integration-verification-2026-11-01/main-integration.json)。
