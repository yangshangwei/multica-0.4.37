# 实施与验收计划

状态：方案已批准、前置门槛已核验，进入实施。主会话负责范围、集成、规格和提交；Trellis implement/check 子代理负责有明确文件边界的实现与复核。

## 2026-11-01 集成验收更新

- [x] 固定快照、工作区归属及晚到差异；交付源码已提交 `0e1ee8e1e`。
- [x] 协议/兼容读者、checkpoint/seal/recovery、评论 recall/CLI、handoff/source/wakeup/实际启动、共享 Web/Desktop 与指导完成。
- [x] 全量 Go 协作、TS、lint/typecheck、vet/build/sqlc、独立审查和六项实际 Web/Electron 用例通过；一项已有 CLI skip 单列保留。
- [x] 原始失败和复验记录、源码/组件/数据环境证据已保存；详见 `verification-integration-2026-11-01.md`。
- [ ] 真实模型验收仍单列；原系列继续 E2a trial 2，剩余23次，新的 runtime 证据另记来源。
- [ ] 父任务整体验收与知识候选实现尚未完成，不据上述确定性通过归档整个任务。

下方为原分工清单，已实现项的当前验收以本节和验证报告为准；普通评论的50条是独立 recall 默认页大小，不是启动快照预载保证。

## 0. 实施入口

- [x] 恢复现有任务；记录基线 `4c833e737`；读取父任务、代码和 spec。
- [x] 完成三条研究，收敛 PRD/design/本计划和上下文 manifests。
- [x] 用户明确批准本方案并要求进入实施；批准记录见本会话和 entry-verification。
- [x] 前置迁移、Web/Desktop、正式 E0 均通过；79 份证据哈希复核。E1 上游 429/503 与剩余行为实验保持未通过。
- [x] 实施 HEAD `5accf9023`；产品树仅新增实验测试，工作副本仅本任务规划变更。其他任务证据保持原样。
- [x] `task.py validate` 通过；已 start，状态 in_progress。

## 1. 先落协议和兼容读者（AC2、AC8）

Owner：一个 implement 代理，独占 `server/pkg/protocol`、相关 records SQL/生成文件和对应 service records helper。

- [ ] 写失败测试：typed checkpoint/seal/snapshot/page 的 malformed、错 identity、unknown version、offset/byte/count；新增内部事件穿插后旧 records 可读。
- [ ] 定义新协议与版本协商，保持旧 identity/consent digest 和 v1 outcome。
- [ ] 先实现 records 的新旧投影、稳定 stream/ordinal 去重和 through_seq 分页；验证最终 outcome 晚到不重复。
- [ ] 需要 SQL 时由该 owner 单独执行 `make sqlc`；生成文件禁止多个代理同时写。

回退点：只有兼容读者，没有新生产者；原功能应保持不变。

## 2. 两条独立服务端链路（AC1、AC2、AC6、AC7）

Lane A：checkpoint，独占新的 checkpoint service/handler/client helper；协议/公共 router 修改交由主会话协调。

- [ ] DB 回归：Stop/撤权/作者退出前后两种锁序、同请求 ACK、改写冲突、错 attempt、recording 总预算、封口不得含正文。
- [ ] 实现 append-only chunk、原 key/hash ACK、单调缺失元数据；保留全部 proof/capacity 行为。
- [ ] daemon 有界 sender、timer/backpressure、ledger 原请求恢复；默认测试只用 fake executable。
- [ ] 验证停止时上传挂起不妨碍 lifecycle Stop；不可恢复丢块封住 stream，不跨缺口推进 retained_through；ACK 丢失/旧 seal 不降低服务端已提交水位；最终拼接和 restart 不重复、不伪装完整。

Lane B：comments，独占 `collaboration_comments*`、新的 runtime recall helper、`internal/cli`/`cmd_collaboration` 评论读取路径。

- [ ] 纯/DB 测试：稳定游标、200+ 条、同时间戳、中文/emoji/JSON escape、总数过滤、预算包括 metadata。
- [ ] 实现列表预算和超长占位、revision 绑定全文分块；测试占位后前进、全文重组、编辑 stale、删除/撤权即时生效。
- [ ] 作者当前能力与历史 admission 分开；普通评论永远 data_only。
- [ ] CLI legacy 数组、新分页 envelope、flag allowlist 和内容读取操作；错 issue/凭据覆盖先于 HTTP 拒绝。

并行规则：独立测试可并行；修改同一 service/router/protocol/SQL 生成输出需交接。service 与 handler 的共享 DB 全套测试串行，不能仅因使用不同 workspace 就并发。

## 3. 交接包、来源与实际注入（AC3、AC4、AC5、AC7）

依赖：1；接入 2 的 checkpoint/recall 契约。Owner：handoff implement 代理，独占 `collaboration_squad*` 的 handoff/wakeup 改动和新 DTO helper；host_runner 与 Lane A 顺序交接。

- [ ] DB 测试：不可变 delegation 正文与 hash、完整 readers、leader_turn 顺序、多个 proof/comment/join 合并及 claim 后原因归下一轮。
- [ ] 实现有界 handoff、runtime results/evidence/cause 召回、typed prior result 和缺失语义。
- [ ] 实现逻辑 prompt snapshot 与 live refresh；测试重取稳定、memory purge 不经快照泄露、普通评论 revision 变化且旧版不可还原时拒绝 launch，新增 256 KiB 预算不拒绝接近旧 2 MiB/1024 条上限的合法输入。
- [ ] Execute 前冻结/构造实际输入；BeforeLaunch authorize 后绑定 snapshot/hash 与实际 attempt，复查来源后才允许进程启动。测试 Git preparation、provider 内部 retry、外层 fresh fallback 和绑定失败，不能提前假定 launch nonce。
- [ ] fake-provider fixture 断言完整用户材料/派活/结果及 trust boundary；拒绝或错身份在启动前失败。

回退点：停止新能力启动，保留 reader；不得删除审计事件或改旧 consent。

## 4. Shared Web/Desktop 与内置指导（AC8、AC9）

Owner：frontend implement 代理；范围 `packages/core/collaboration`、必要 API client 方法、`packages/views/collaboration`、en/zh-Hans 文案。不复制共享逻辑到 apps。

- [ ] 核对人类授权投影与 runtime 投影，UI 不调用 business-token 接口。
- [ ] schema + malformed-response tests；预算进入 query identity；旧字段缺失显示 unknown。
- [ ] 列表加载范围、作者来源、占位恢复/编辑保护、run 缺失范围和交接引用；用现有布局/组件。
- [ ] 共享组件仅测展示/交互和关键回归，不重复纯 helper/服务权限矩阵。
- [ ] 同步 `multica-working-on-issues/SKILL.md`、`references/collaboration-workflow.md`、`references/working-on-issues-source-map.md`。
- [ ] 视觉迭代按仓库 visual-verdict 要求保存结果；生产 Web 与原生 Desktop 分别验证入口，不互相代替。

## 5. 集成质量门与证据（AC1–AC10）

先运行新增套件，再扩大。下面是待执行命令，不是已通过记录：

```bash
go -C server test ./pkg/protocol ./internal/cli ./cmd/multica ./internal/daemon/collaborationrun -count=1
go -C server test ./internal/service -run 'Collaboration' -count=1 -timeout 40m -v
go -C server test ./internal/handler -run 'Collaboration' -count=1 -timeout 40m -v
go -C server vet ./...
go -C server build ./...
pnpm lint
pnpm typecheck
pnpm test
```

- [ ] SQL 变更执行 `make sqlc` 并核对生成差异；若有 migration，验证唯一前缀和 fresh/upgrade/reapply，保留原历史。
- [ ] 相关 e2e 使用 production Web（build/start），按 `.trellis/spec/web/frontend/e2e-run-environment.md`；测试用例文件在实施时从现有协作 e2e 中确定并写入证据清单。
- [ ] 每条证据记录 source HEAD/dirty diff、准确命令、DB/平台、测试名、退出码、通过/失败/跳过，日志落本任务 `research/verification/`。确认 DB tests 实际运行，无静默 skip。
- [ ] 独立 trellis-check 审阅完整 diff、权限/去重/读者/分页/兼容；问题修复后仅重跑受影响检查。
- [ ] 真实 E0/E5/E6 沿用独立实验框架和正式 gate，串行运行；环境失败保存原结果，不扩大重试或替换正式 smoke。
- [ ] 主会话更新受影响 spec、验收映射与任务状态。提交遵守 Conventional intent line + Lore trailers；只提交本任务文件，不携带外部工作。

## 6. 完成与回退条件

AC1–AC10 都有实际证据才可标完成；缺少真实平台/模型结果时精确记录未验证范围，不用组件或 fake-provider 的通过代替。生产部署/发布不属于此次默认交付。

回退顺序：关闭新 writer/新启动能力 → 保留兼容 reader 和旧模式 → 检查历史仍可读。不要回退到会把新内部事件当未知错误的 server；不要清除事件、重放用户指令、恢复被撤销的 token 或自动 Resume。

## 2026-11-01 本轮恢复与验收进度

以下只标记已验证的本轮增量，不表示原 1–6 节或 AC1–AC10 全部完成。详情与准确命令见 `research/verification/2026-11-01-recording-recovery/summary.md`。

- [x] 上传持续失败/挂起仍完成原 Stop 与终止证明；失败记录保持可恢复。
- [x] 新 Runner 从磁盘重放原 checkpoint/seal；原 proof 优先且 ACK 后不重发；不重新 Claim/Permit/启动模型。
- [x] 总重放预算、旧 seal ACK 丢失、后续单调水位更新和畸形/缺字段 ACK 回归。
- [x] 修复执行面板测试的独立查询响应串用；本轮 10 文件增量按哈希合回工作区。
- [x] 全量 TS、Go 相关四包（1 个既有明确跳过）、38 个 race 通过事件、专用 DB 8+11 个通过事件，以及最终 lint/typecheck/vet/build。
- [ ] 独立终审：代理限流及模型路由不可用；保留未通过，不以主会话手工检查替代。
- [ ] 完整任务锁序/权限矩阵、启动注入整合、生产 Web/原生 Desktop 与真实模型最终验收。
